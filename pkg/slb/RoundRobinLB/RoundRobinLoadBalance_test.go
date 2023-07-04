package RoundRobinLB

import (
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/system/sysPrint"
	"log"
	"math/rand"
	"strconv"
	"testing"
)

const (
	SelectNodeTestCount = 1000000
	DeleteNodeTestCount = 1000000
)

var (
	testAddr       = []string{"127.0.0.1:10001", "127.0.0.1:10002", "127.0.0.1:10003", "127.0.0.1:10004", "127.0.0.1:10005"}
	testWeights    = []int32{1, 4, 2, 3, 5}
	serverCount    = len(testWeights)
	testServerList = make([]*server.Server, 0)
	loadBalancer   *RRLB
)

func init() {
	loadBalancer = CreateRRLB()
}

func TestAddServerNode(t *testing.T) {
	if len(testWeights) != serverCount {
		panic("testWeights' length must equal to " + strconv.Itoa(serverCount))
	}
	if len(testWeights) != len(testAddr) {
		panic("testWeights' length must equal to testAddr's length")
	}
	for i := 0; i < serverCount; i++ {
		s, err := server.NewServer(testAddr[i], testWeights[i], "")
		if err != nil {
			log.Fatal(err)
		}
		err = loadBalancer.AddServerNode(s)
		if err != nil {
			log.Fatal(err)
		}
		testServerList = append(testServerList, s)
	}

	// 将 testWeights 权重 < 1 的值设置为 1（最小值限制），loadBalancer.AddServerNode() 会做隐式转换
	for i := 0; i < len(testWeights); i++ {
		if testWeights[i] < 1 {
			testWeights[i] = 1
		}
	}

	// 反转 testWeights，令顺序与头插进入链表的节点一致
	for i := 0; i < len(testWeights)-1-i; i++ {
		t := testWeights[i]
		ta := testAddr[i]
		testWeights[i] = testWeights[len(testWeights)-1-i]
		testWeights[len(testWeights)-1-i] = t
		testAddr[i] = testAddr[len(testAddr)-1-i]
		testAddr[len(testAddr)-1-i] = ta
	}

	// 检查 loadBalancer.currentList 每个服务器节点权重是否符合预期
	i := 0
	p := loadBalancer.currentList.head
	for p.next != nil {
		if p.next.weight != testWeights[i] {
			t.Errorf("unequal testWeights. index %d, expect %d, actual %d", i, testWeights[i], p.next.weight)
		}
		p = p.next
		i++
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

	i = 0
	p = loadBalancer.currentList.head
	for p.next != nil {
		if p.next.weight != testWeights[i] {
			t.Errorf("unequal testWeights. index %d, expect %d, actual %d", i, testWeights[i], p.next.weight)
		}
		p = p.next
		i++
	}
}

func TestSelectNode(t *testing.T) {
	expectedAddr := make([]string, 0)
	weights := make([]int32, serverCount)
	copy(weights, testWeights)

	for {
		flag := false
		for i := 0; i < serverCount; i++ {
			if weights[i] > 0 {
				weights[i]--
				expectedAddr = append(expectedAddr, testAddr[i])
				flag = true
			}
		}
		if !flag {
			break
		}
	}

	for i := 0; i < len(expectedAddr); i++ {
		s, err := loadBalancer.SelectNode()
		if err != nil {
			t.Error(err)
		}
		if s.Addr() != expectedAddr[i] {
			t.Errorf("unequal testAddr. expect %s, actual %s", expectedAddr[i], s.Addr())
		}
	}

	// 每轮随机一个节点为主观下线状态
	for k := 0; k < SelectNodeTestCount; k++ {
		failIdx := rand.Intn(serverCount)
		testServerList[failIdx].SetPfail(server.IS_PFAIL)
		for i := 0; i < serverCount; i++ {
			s, err := loadBalancer.SelectNode()
			if err != nil {
				t.Error(err)
			}
			if s.Addr() == testServerList[failIdx].Addr() {
				t.Errorf("Selected server that is considered subjective fail. addr:%s", s.Addr())
			}
		}
		testServerList[failIdx].SetPfail(server.NOT_PFAIL)
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
