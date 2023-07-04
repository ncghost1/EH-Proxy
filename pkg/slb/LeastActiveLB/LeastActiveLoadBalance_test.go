package LeastActiveLB

import (
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/system/sysPrint"
	"log"
	"math/rand"
	"testing"
)

const (
	SelectNodeTestCount = 1000000
	DeleteNodeTestCount = 100000
)

var (
	testServerMap  = map[string]int32{"127.0.0.1:10001": 1, "127.0.0.1:10002": 4, "127.0.0.1:10003": 2, "127.0.0.1:10004": 3, "127.0.0.1:10005": 5}
	testServerList = make([]*server.Server, 0)
	serverCount    = len(testServerMap)
	loadBalancer   *LALB
)

func init() {
	loadBalancer = CreateLALB()
}

func TestAddServerNode(t *testing.T) {
	testWeights := make([]int32, 0)
	for addr, weight := range testServerMap {
		s, err := server.NewServer(addr, weight, "")
		if err != nil {
			log.Fatal(err)
		}
		err = loadBalancer.AddServerNode(s)
		if err != nil {
			log.Fatal(err)
		}
		testWeights = append(testWeights, weight)
		testServerList = append(testServerList, s)
	}

	// 将 testWeights 权重 < 1 的值设置为 1（最小值限制），AddServerNode() 会做隐式转换
	for i := 0; i < len(testWeights); i++ {
		if testWeights[i] < 1 {
			testWeights[i] = 1
		}
	}

	tempServerSet := make(map[*server.Server]bool, 0)
	for _, s := range testServerList {
		tempServerSet[s] = false
	}
	for s := range loadBalancer.serverMap {
		b, ok := tempServerSet[s]
		if !ok {
			t.Errorf("unexpect server node, server address:%s", s.Addr())
		}
		if b == true {
			t.Errorf("unexpected duplicate server node, server address:%s", s.Addr())
		}
	}

	err := loadBalancer.AddServerNode(testServerList[0])
	if err == nil {
		t.Error("Adding the same node repeatedly should fail.")
	}

	// test InitServerNode
	err = loadBalancer.InitServerNode(testServerList)
	if err != nil {
		t.Error(err)
	}

	for _, s := range testServerList {
		tempServerSet[s] = false
	}
	for s := range loadBalancer.serverMap {
		b, ok := tempServerSet[s]
		if !ok {
			t.Errorf("unexpect server node, server address:%s", s.Addr())
		}
		if b == true {
			t.Errorf("unexpected duplicate server node, server address:%s", s.Addr())
		}
	}
}

func TestSelectNode(t *testing.T) {
	for _, failServer := range testServerList {
		failServer.SetPfail(server.IS_PFAIL)
		m := make(map[*server.Server]struct{}, 0)
		for i := 0; i < SelectNodeTestCount; i++ {
			s, err := loadBalancer.SelectNode()
			if err != nil {
				t.Error(err)
			}
			if s.Addr() == failServer.Addr() {
				t.Errorf("Selected server that is considered subjective fail. addr:%s", s.Addr())
			}
			m[s] = struct{}{}
		}
		for _, s := range testServerList {
			if failServer == s {
				continue
			}
			if _, ok := m[s]; !ok {
				t.Errorf("node %s not selected", s.Addr())
			}
		}
		failServer.SetPfail(server.NOT_PFAIL)
	}
}

func TestDeleteServerNode(t *testing.T) {
	for k := 0; k < DeleteNodeTestCount; k++ {
		DelIdx := rand.Intn(serverCount)
		err := loadBalancer.DeleteServerNode(testServerList[DelIdx])
		if err != nil {
			t.Error(err)
		}
		for i := 0; i < serverCount; i++ {
			s, err := loadBalancer.SelectNode()
			if err != nil {
				t.Error(err)
			}
			if s.Addr() == testServerList[DelIdx].Addr() {
				t.Errorf("Selected server that is deleted. addr:%s", s.Addr())
			}
		}
		err = loadBalancer.AddServerNode(testServerList[DelIdx])
		if err != nil {
			t.Error(err)
		}
	}

	// 删除所有节点
	for i := 0; i < serverCount; i++ {
		err := loadBalancer.DeleteServerNode(testServerList[i])
		if err != nil {
			t.Error(err)
		}
	}

	_, err := loadBalancer.SelectNode()
	if err != sysPrint.ErrNoServer {
		t.Error(err)
	} else if err == nil {
		t.Errorf("expect no server error,but error is nil")
	}
}
