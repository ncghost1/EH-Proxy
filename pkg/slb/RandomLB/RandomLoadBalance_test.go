package RandomLB

import (
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/system/sysPrint"
	"log"
	"math/rand"
	"testing"
)

const (
	AllowableErrorRange = 0.005
	SelectNodeTestCount = 1000000
	DeleteNodeTestCount = 100000
)

var (
	testServerMap  = map[string]int32{"127.0.0.1:10001": 1, "127.0.0.1:10002": 4, "127.0.0.1:10003": 2, "127.0.0.1:10004": 3, "127.0.0.1:10005": 5}
	testServerList = make([]*server.Server, 0)
	serverCount    = len(testServerMap)
	loadBalancer   *RDLB
)

func init() {
	loadBalancer = CreateRDLB()
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

	expected := make([]int64, serverCount)
	for i := 0; i < serverCount; i++ {
		if i == 0 {
			expected[i] = int64(testWeights[i])
		} else {
			expected[i] = expected[i-1] + int64(testWeights[i])
		}
		if expected[i] != loadBalancer.weightSum[i] {
			t.Errorf("unequal testWeights. index %d, expected %d, actual %d", i, expected[i], loadBalancer.weightSum[i])
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

	for i := 0; i < serverCount; i++ {
		if i == 0 {
			expected[i] = int64(testWeights[i])
		} else {
			expected[i] = expected[i-1] + int64(testWeights[i])
		}
		if expected[i] != loadBalancer.weightSum[i] {
			t.Errorf("unequal testWeights. index %d, expected %d, actual %d", i, expected[i], loadBalancer.weightSum[i])
		}
	}
}

func TestSelectNode(t *testing.T) {
	sum := float64(loadBalancer.weightSum[serverCount-1])
	m := make(map[string]int, 0)

	for i := 0; i < SelectNodeTestCount; i++ {
		s, err := loadBalancer.SelectNode()
		if err != nil {
			t.Error(err)
		}
		m[s.Addr()]++
	}

	// IsValid 检查 checkNum 是否在 expectNum 的允许误差范围内，是则返回 true
	var IsValid func(float64, float64) bool
	IsValid = func(checkNum, expectNum float64) bool {
		if checkNum > expectNum+AllowableErrorRange || checkNum < expectNum-AllowableErrorRange {
			return false
		} else {
			return true
		}
	}

	for addr, cnt := range m {
		w := testServerMap[addr]
		if w < 1 {
			w = 1
		}
		expect := float64(w) / sum
		actual := float64(cnt) / SelectNodeTestCount
		if !IsValid(actual, expect) {
			t.Errorf("The difference between actual probability and expected probability exceeds %.3f, server address: %s ,weight:%d, expect:%.3f, actual:%.3f",
				AllowableErrorRange, addr, w, expect, actual)
		}
	}

	failIdx := rand.Intn(serverCount)
	loadBalancer.serverList[failIdx].SetPfail(server.IS_PFAIL)
	for i := 0; i < SelectNodeTestCount; i++ {
		s, err := loadBalancer.SelectNode()
		if err != nil {
			t.Error(err)
		}
		if s.Addr() == loadBalancer.serverList[failIdx].Addr() {
			t.Errorf("Selected servers that are considered subjective fail. addr:%s", s.Addr())
		}
	}
	loadBalancer.serverList[failIdx].SetPfail(server.NOT_PFAIL)
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
	}
}
