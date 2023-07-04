package RoundRobinLB

import (
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/system/sysPrint"
	"strconv"
	"sync"
)

type node struct {
	server *server.Server
	weight int32
	next   *node
}

type list struct {
	head *node
}

type RRLB struct {
	currentList     list
	currentListLock sync.Mutex
	prevNode        *node
	backupList      list
	backupListLock  sync.Mutex
	serverSet       map[*server.Server]struct{}
	serverSetLock   sync.Mutex
}

// CreateRRLB 创建一个 Round Robin Load Balancer
func CreateRRLB() *RRLB {
	rrlb := &RRLB{
		currentList: list{
			head: &node{
				server: nil,
				weight: 0,
				next:   nil,
			},
		},
		currentListLock: sync.Mutex{},
		prevNode:        nil,
		backupList: list{
			head: &node{
				server: nil,
				weight: 0,
				next:   nil,
			},
		},
		backupListLock: sync.Mutex{},
		serverSet:      make(map[*server.Server]struct{}),
		serverSetLock:  sync.Mutex{},
	}
	rrlb.prevNode = rrlb.currentList.head
	return rrlb
}

func (ls *list) HeadInsert(insertNode *node) {
	insertNode.next = ls.head.next
	ls.head.next = insertNode
	insertNode.weight = insertNode.server.Weight()
}

func (lb *RRLB) backupHeadInsert(insertNode *node) {
	lb.backupListLock.Lock()
	defer lb.backupListLock.Unlock()
	lb.backupList.HeadInsert(insertNode)
}

func (ls *list) searchAndDelete(serverNode *server.Server) bool {
	p := ls.head
	for p.next != nil {
		cur := p.next
		if cur.server == serverNode {
			p.next = cur.next
			cur.next = nil
			return true
		}
		p = p.next
	}
	return false
}

// Reset 清空所有服务器节点
func (lb *RRLB) Reset() {
	lb.currentListLock.Lock()
	lb.backupListLock.Lock()
	lb.serverSetLock.Lock()
	defer lb.currentListLock.Unlock()
	defer lb.backupListLock.Unlock()
	defer lb.serverSetLock.Unlock()
	lb.currentList = list{
		head: &node{
			server: nil,
			weight: 0,
			next:   nil,
		},
	}

	lb.backupList = list{
		head: &node{
			server: nil,
			weight: 0,
			next:   nil,
		},
	}
	lb.prevNode = lb.currentList.head
	lb.serverSet = make(map[*server.Server]struct{}, 0)
}

// AddServerNode 添加服务器节点
func (lb *RRLB) AddServerNode(serverNode *server.Server) error {
	lb.serverSetLock.Lock()
	defer lb.serverSetLock.Unlock()
	if _, ok := lb.serverSet[serverNode]; !ok {
		lb.serverSet[serverNode] = struct{}{}
	} else {
		return sysPrint.ErrServerExists
	}

	lb.currentListLock.Lock()
	defer lb.currentListLock.Unlock()

	// 权重最低设置为 1
	if serverNode.Weight() < 1 {
		serverNode.SetWeight(1)
	}
	n := &node{
		server: serverNode,
		weight: serverNode.Weight(),
		next:   lb.currentList.head.next,
	}
	lb.currentList.head.next = n
	return nil
}

// DeleteServerNode 同步删除服务器节点
func (lb *RRLB) DeleteServerNode(serverNode *server.Server) error {
	lb.serverSetLock.Lock()
	lb.currentListLock.Lock()
	lb.backupListLock.Lock()
	defer lb.serverSetLock.Unlock()
	defer lb.currentListLock.Unlock()
	defer lb.backupListLock.Unlock()
	if _, ok := lb.serverSet[serverNode]; !ok {
		return sysPrint.ErrServerNotExists
	} else {
		delete(lb.serverSet, serverNode)
	}

	if lb.currentList.searchAndDelete(serverNode) {
		return nil
	}

	if lb.backupList.searchAndDelete(serverNode) {
		return nil
	}

	// 集合中存在，链表却不存在节点，实际上应该不会发生这种情况，这里是做个兜底
	delete(lb.serverSet, serverNode)
	return nil
}

// InitServerNode 通过 serverNodeList 切片初始化节点
func (lb *RRLB) InitServerNode(serverNodeList []*server.Server) error {
	lb.Reset()
	for _, s := range serverNodeList {
		err := lb.AddServerNode(s)
		if err != nil {
			return err
		}
	}
	return nil
}

// SelectNode 通过 RoundRobin 算法选择一个服务器节点
// 使用双链表进行优化，时间复杂度 O(1)
func (lb *RRLB) SelectNode() (*server.Server, error) {

	// currentList 加锁
	lb.currentListLock.Lock()
	defer lb.currentListLock.Unlock()

	// 选择的节点为 prevNode 下一个节点
	cur := lb.prevNode.next

	checkNil := func() error {
		// 如果 cur 为空，则令 prevNode 回到 currentList 头部
		if cur == nil {
			lb.prevNode = lb.currentList.head
			cur = lb.prevNode.next

			// 如果 cur 此时依然为空值，说明 currentList 已经轮询一遍，交换两个链表的头结点
			if cur == nil {
				lb.backupListLock.Lock()
				defer lb.backupListLock.Unlock()
				if lb.backupList.head.next == nil {
					return sysPrint.ErrNoServer
				}
				lb.currentList.head.next = lb.backupList.head.next
				lb.backupList.head.next = nil
				lb.prevNode = lb.currentList.head
				cur = lb.prevNode.next
			}
		}
		return nil
	}

	// 检查 cur 是否为 nil，并做相应处理
	err := checkNil()
	if err != nil {
		return nil, err
	}

	// 当前节点被主观认为下线，直接加入 backup 链表，跳过当前节点
	for cur.server.Pfail() == server.IS_PFAIL {
		lb.prevNode.next = cur.next
		lb.backupHeadInsert(cur)
		cur = lb.prevNode.next
		err = checkNil()
		if err != nil {
			return nil, err
		}
	}

	if cur.weight <= 0 {
		sysPrint.PrintlnAndLogWriteErrorMsg("cur weight is " + strconv.Itoa(int(cur.weight)) +
			"This is an error that should not have occurred. Please provide feedback to the author.")
	}

	cur.weight-- // 头结点权重 - 1

	// 当前节点权重为 0，插入 backupList，并从 currentList 中删除
	if cur.weight == 0 {
		lb.prevNode.next = lb.prevNode.next.next
		lb.backupHeadInsert(cur)
	} else {
		lb.prevNode = cur
	}
	return cur.server, nil
}
