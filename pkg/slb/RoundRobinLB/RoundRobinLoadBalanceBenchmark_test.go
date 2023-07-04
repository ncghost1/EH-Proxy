package RoundRobinLB

import (
	"EH-Proxy/pkg/server"
	"log"
	"math/rand"
	"strconv"
	"testing"
)

const (
	serverNum = 1000
	testHost  = "127.0.0.1"
	maxWeight = 400
)

var (
	testServerPort = 10001
)

func BenchmarkSelectNode(b *testing.B) {
	loadBalancer = CreateRRLB()
	for i := 0; i < serverNum; i++ {
		addr := testHost + ":" + strconv.Itoa(testServerPort)
		testServerPort++
		s, err := server.NewServer(addr, int32(rand.Intn(maxWeight)+1), "")
		if err != nil {
			log.Fatal(err)
		}
		err = loadBalancer.AddServerNode(s)
		if err != nil {
			log.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := loadBalancer.SelectNode()
		if err != nil {
			b.Error(err)
		}
	}

}
