package proxy

import (
	"EH-Proxy/pkg/server"
	"log"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	serverNum    = 10
	serverWeight = 100
	requestNum   = 200
)

var (
	testProxy           = GetProxyInstance()
	testProxyServeMu    = sync.Mutex{}
	testProxyIsServing  = false
	testServerPort      = 14514
	testServerProbePort = 34514
)

func init() {
	var wg sync.WaitGroup
	for i := 0; i < serverNum; i++ {
		addr := "127.0.0.1:" + strconv.Itoa(testServerPort)
		probeIpPort := "127.0.0.1:" + strconv.Itoa(testServerProbePort)
		probe := HttpScheme + probeIpPort
		testServerPort++
		testServerProbePort++
		err := testProxy.serverGroup.AddServer(testProxy, addr, serverWeight, probe)
		if err != nil {
			return
		}
		if err != nil {
			log.Fatal(err)
		}
		wg.Add(2)

		// Start Server
		go func() {
			wg.Done()
			err = http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err = w.Write([]byte("this is server:" + addr))
				if err != nil {
					log.Fatal(err)
				}
			}))
			if err != nil {
				log.Fatal(err)
			}
		}()

		// Start probe
		go func() {
			wg.Done()
			err = http.ListenAndServe(probeIpPort, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if err != nil {
					log.Fatal(err)
				}
			}))
			if err != nil {
				log.Fatal(err)
			}
		}()
		wg.Wait()
		time.Sleep(10 * time.Millisecond)
	}
}

func TestProxy_Serve(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		testProxyServeMu.Lock()
		wg.Done()
		if !testProxyIsServing {
			testProxyIsServing = true
			testProxyServeMu.Unlock()
			testProxy.Serve()
		}
	}()
	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	wg.Add(requestNum)

	// requestNum should not exceed the maximum number of open files limit in the process
	for i := 0; i < requestNum; i++ {
		go func() {
			_, err := http.Get(HttpScheme + testProxy.config.Addr)
			if err != nil {
				t.Errorf("request error:%v", err)
			}
			if err != nil {
				log.Fatal(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestProxyHealthCheck(t *testing.T) {
	var wg sync.WaitGroup
	addr := "127.0.0.1:" + strconv.Itoa(testServerPort)
	probeIpPort := "127.0.0.1:" + strconv.Itoa(testServerProbePort)
	probe := HttpScheme + probeIpPort
	testServerPort++
	testServerProbePort++
	err := testProxy.serverGroup.AddServer(testProxy, addr, serverWeight, probe)
	if err != nil {
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	wg.Add(2)

	// Start Server
	go func() {
		wg.Done()
		err = http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("this is server:" + addr))
			if err != nil {
				log.Fatal(err)
			}
		}))
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Start probe
	go func() {
		wg.Done()
		err = http.ListenAndServe(probeIpPort, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(testProxy.config.PfailTime + 1)
			w.WriteHeader(http.StatusOK)
			if err != nil {
				log.Fatal(err)
			}
		}))
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
	time.Sleep(testProxy.config.PfailTime + 2*time.Second)

	cnt := testProxy.serverGroup.PfailCount()
	if cnt != 1 {
		t.Errorf("pfail count error, expect:%d, actual:%d", 1, cnt)
	}

	pfail := testProxy.serverGroup.serverMap[addr].Pfail()
	if testProxy.serverGroup.serverMap[addr].Pfail() != server.IS_PFAIL {
		t.Errorf("server %s pfail status error, expect:%d, actual:%d", addr, server.IS_PFAIL, pfail)
	}

}
