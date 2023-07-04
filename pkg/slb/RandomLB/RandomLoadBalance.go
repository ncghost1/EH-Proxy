package RandomLB

import (
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/system/sysPrint"
	"math/rand"
	"sync"
)

const (
	LAZYDEL_THRESHOLD = 0.2 // 懒惰删除阈值（删除节点权重和/总权重）
	DEFAULT_MAXRETRY  = 3   // 默认最大重试次数
)

// RDLB Random Load Balancer
type RDLB struct {
	weightSum     []int64                // 权重前缀和
	serverList    []*server.Server       // 服务器列表
	delWeightSum  int64                  // 已删除节点权重和（用于懒惰删除）
	rwLock        sync.RWMutex           // 读写锁
	maxRetry      int                    // 最大重试次数
	serverMap     map[*server.Server]int // Key-value: server-serverList索引 哈希表
	serverMapLock sync.Mutex             // serverMap 互斥锁
}

// CreateRDLB 创建一个 Random Load Balancer
func CreateRDLB() *RDLB {
	return &RDLB{
		weightSum:     make([]int64, 0),
		serverList:    make([]*server.Server, 0),
		delWeightSum:  0,
		rwLock:        sync.RWMutex{},
		maxRetry:      DEFAULT_MAXRETRY,
		serverMap:     make(map[*server.Server]int, 0),
		serverMapLock: sync.Mutex{},
	}
}

// Reset 重置负载均衡器
func (lb *RDLB) Reset() {
	lb.rwLock.Lock()
	lb.serverMapLock.Lock()
	defer lb.rwLock.Unlock()
	defer lb.serverMapLock.Unlock()
	lb.weightSum = make([]int64, 0)
	lb.serverList = make([]*server.Server, 0)
	lb.serverMap = make(map[*server.Server]int, 0)
	lb.delWeightSum = 0
}

// AddServerNode 向负载均衡器添加一个服务器节点
func (lb *RDLB) AddServerNode(serverNode *server.Server) error {
	lb.serverMapLock.Lock()
	defer lb.serverMapLock.Unlock()
	if _, ok := lb.serverMap[serverNode]; ok {
		return sysPrint.ErrServerExists
	}

	lb.rwLock.Lock()
	defer lb.rwLock.Unlock()
	if serverNode.Weight() < 1 {
		serverNode.SetWeight(1)
	}
	lb.serverList = append(lb.serverList, serverNode)
	lb.serverMap[serverNode] = len(lb.serverList) - 1

	if len(lb.weightSum) == 0 {
		lb.weightSum = append(lb.weightSum, int64(serverNode.Weight()))
	} else {
		lb.weightSum = append(lb.weightSum, int64(serverNode.Weight())+lb.weightSum[len(lb.weightSum)-1])
	}
	return nil
}

// DeleteServerNode 从负载均衡器中删除一个服务器节点
func (lb *RDLB) DeleteServerNode(serverNode *server.Server) error {
	lb.serverMapLock.Lock()
	defer lb.serverMapLock.Unlock()
	idx, ok := lb.serverMap[serverNode]
	if !ok {
		return sysPrint.ErrServerNotExists
	}
	delete(lb.serverMap, serverNode)

	lb.rwLock.Lock()
	defer lb.rwLock.Unlock()
	lb.serverList[idx] = nil                      // 将服务器节点设为 nil
	lb.delWeightSum += int64(serverNode.Weight()) // 累加已删除节点权值

	// 懒惰删除：比较删除节点权值是否大于阈值，若超过阈值则重构负载均衡器
	if float64(lb.delWeightSum)/float64(lb.weightSum[len(lb.weightSum)-1]) > LAZYDEL_THRESHOLD {
		// 重构前缀和与 serverList
		tempServerList := make([]*server.Server, 0)
		tempWeightSum := make([]int64, 0)
		for key, _ := range lb.serverMap {
			delete(lb.serverMap, key)
		}
		for i := 0; i < len(lb.serverList); i++ {
			if lb.serverList[i] != nil {
				tempServerList = append(tempServerList, lb.serverList[i])
				lb.serverMap[lb.serverList[i]] = len(tempServerList) - 1
				if len(tempWeightSum) == 0 {
					tempWeightSum = append(tempWeightSum, int64(lb.serverList[i].Weight()))
				} else {
					tempWeightSum = append(tempWeightSum, int64(lb.serverList[i].Weight())+tempWeightSum[len(tempWeightSum)-1])
				}
			}
		}
		lb.serverList = tempServerList
		lb.weightSum = tempWeightSum
	}
	return nil
}

// InitServerNode 重置负载均衡器，并使用 serverNodeList 中的服务器节点进行初始化
func (lb *RDLB) InitServerNode(serverNodeList []*server.Server) error {
	lb.Reset()
	for _, s := range serverNodeList {
		err := lb.AddServerNode(s)
		if err != nil {
			return err
		}
	}
	return nil
}

// SelectNode 加权随机选取一个服务器节点
// CDF 累计分布函数算法
func (lb *RDLB) SelectNode() (*server.Server, error) {
	lb.rwLock.RLock()
	defer lb.rwLock.RUnlock()
	if len(lb.serverList) == 0 {
		return nil, sysPrint.ErrNoServer
	}

	for i := 0; i <= lb.maxRetry; i++ {
		n := lb.weightSum[len(lb.weightSum)-1]
		target := rand.Int63n(n) + 1 // 在 1 ~ 权重总和 范围内随机取值

		// 使用二分查找在权重前缀和切片中查找节点对应 index
		l, r := 0, len(lb.weightSum)-1
		for l < r {
			mid := (l + r) >> 1
			if lb.weightSum[mid] >= target {
				r = mid
			} else {
				l = mid + 1
			}
		}
		serverNode := lb.serverList[r]
		if serverNode == nil {
			continue
		}
		if serverNode.Pfail() == server.NOT_PFAIL {
			return serverNode, nil
		}
	}

	// 达到最大重试次数未能成功获取可用节点，降级成随机起点轮询
	idx := rand.Intn(len(lb.serverList))
	if lb.serverList[idx] != nil && lb.serverList[idx].Pfail() == server.NOT_PFAIL {
		return lb.serverList[idx], nil
	}
	for i := (idx + 1) % len(lb.serverList); i != idx; i = (i + 1) % len(lb.serverList) {
		if lb.serverList[i] != nil && lb.serverList[i].Pfail() == server.NOT_PFAIL {
			return lb.serverList[i], nil
		}
	}
	return nil, sysPrint.ErrNoServer
}
