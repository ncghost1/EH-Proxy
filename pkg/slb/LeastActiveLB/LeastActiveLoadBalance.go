// Power of 2 choices + Least Active Load Balance

package LeastActiveLB

import (
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/system/sysPrint"
	"sync"
)

const (
	CHOICES = 2
	INT_MAX = int32(1<<31 - 1)
)

type LALB struct {
	serverMap     map[*server.Server]struct{}
	serverMapLock sync.Mutex
}

// CreateLALB 创建一个 Least Active Load Balancer
func CreateLALB() *LALB {
	return &LALB{
		serverMap:     make(map[*server.Server]struct{}, 0),
		serverMapLock: sync.Mutex{},
	}
}

// Reset 重置负载均衡器
func (lb *LALB) Reset() {
	lb.serverMapLock.Lock()
	defer lb.serverMapLock.Unlock()
	lb.serverMap = make(map[*server.Server]struct{}, 0)
}

// AddServerNode 向负载均衡器添加一个服务器节点
func (lb *LALB) AddServerNode(serverNode *server.Server) error {
	lb.serverMapLock.Lock()
	defer lb.serverMapLock.Unlock()
	if _, ok := lb.serverMap[serverNode]; ok {
		return sysPrint.ErrServerExists
	}
	lb.serverMap[serverNode] = struct{}{}
	return nil
}

// DeleteServerNode 从负载均衡器中删除一个服务器节点
func (lb *LALB) DeleteServerNode(serverNode *server.Server) error {
	lb.serverMapLock.Lock()
	defer lb.serverMapLock.Unlock()

	if _, ok := lb.serverMap[serverNode]; !ok {
		return sysPrint.ErrServerNotExists
	}
	delete(lb.serverMap, serverNode)
	return nil
}

// InitServerNode 重置负载均衡器，并使用 serverNodeList 中的服务器节点进行初始化
func (lb *LALB) InitServerNode(serverNodeList []*server.Server) error {
	lb.Reset()
	for _, s := range serverNodeList {
		err := lb.AddServerNode(s)
		if err != nil {
			return err
		}
	}
	return nil
}

// SelectNode 选取一个服务器节点
// Least Active Load Balancer 每次随机选取两个节点，最终选择连接数更少或权重更高（连接数相同）的一方
func (lb *LALB) SelectNode() (*server.Server, error) {
	if len(lb.serverMap) == 0 {
		return nil, sysPrint.ErrNoServer
	}

	PendingServers := make([]*server.Server, 0)
	pending := 0

	for s := range lb.serverMap {
		if s.Pfail() == server.IS_PFAIL {
			continue
		}
		PendingServers = append(PendingServers, s)
		pending++
		if pending == CHOICES {
			break
		}
	}
	minActive := INT_MAX
	var choiceServer *server.Server
	for _, s := range PendingServers {
		if minActive >= s.ActiveReq() {
			if minActive == s.ActiveReq() {
				if choiceServer.Weight() < s.Weight() {
					choiceServer = s
				}
			} else {
				choiceServer = s
			}
		}
	}
	return choiceServer, nil
}
