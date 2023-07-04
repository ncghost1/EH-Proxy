package proxy

import (
	"EH-Proxy/config"
	"EH-Proxy/pkg/system/sysPrint"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	clientNum   = 10
	ReadBufSize = 4096
)

var (
	testProxyManager   = GetProxyManagerInstance()
	testClientConnList = make([]net.Conn, clientNum)
)

func init() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		wg.Done()
		testProxyServeMu.Lock()
		wg.Done()
		if !testProxyIsServing {
			testProxyIsServing = true
			testProxyServeMu.Unlock()
			testProxy.Serve()
		}
	}()
	go func() {
		testProxyManager.Serve()
	}()
	time.Sleep(100 * time.Millisecond)
}

func TestProxyManager_AddClient(t *testing.T) {
	for i := 0; i < clientNum; i++ {
		conn, err := net.Dial("tcp", testProxyManager.p.config.ManagerAddr)
		if err != nil {
			log.Fatal(err)
		}
		testClientConnList[i] = conn
	}
	time.Sleep(100 * time.Millisecond) // Waiting for proxy manager to add client
	if len(testProxyManager.clientList) != clientNum {
		t.Errorf("The added client is not equal to %d, clientList's length is %d", clientNum, len(testProxyManager.clientList))
	}
}

func TestProxyManagerCmdInfo(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(clientNum)
	for i := 0; i < clientNum; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := testClientConnList[i].Write([]byte("INFO"))
			if err != nil {
				t.Error(err)
			}
			buf := make([]byte, ReadBufSize)
			_, err = testClientConnList[i].Read(buf)
			if err != nil {
				t.Error(err)
			}
			if string(buf[:6]) != "[INFO]" {
				t.Errorf("'INFO' command response is not correct,the response is:\n%s", string(buf))
			}
		}(i)
	}
	wg.Wait()
}

func TestProxyManagerCommand(t *testing.T) {
	svport := 19999
	svProbePort := 39999
	newServerAddr := "127.0.0.1:" + strconv.Itoa(svport)
	newServerProbe := HttpScheme + "127.0.0.1:" + strconv.Itoa(svProbePort)

	// test AddServer
	_, err := testClientConnList[0].Write([]byte("ADDSERVER " + newServerAddr + " 150 " + newServerProbe))
	if err != nil {
		t.Error(err)
	}
	buf := make([]byte, ReadBufSize)
	n, err := testClientConnList[0].Read(buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf[:n]) != string(ReplyOK) {
		t.Errorf("'ADDSERVER' command response is not correct, expect:%s, actual:%s", string(ReplyOK), string(buf[:n]))
	}

	// test Exists
	_, err = testClientConnList[0].Write([]byte("EXISTS " + newServerAddr))
	if err != nil {
		t.Error(err)
	}
	n, err = testClientConnList[0].Read(buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf[:n]) != string(ServerExistsReply) {
		t.Errorf("'EXISTS' command response is not correct, expect:%s, actual:%s", string(ServerExistsReply), string(buf[:n]))
	}

	// test GetServer
	_, err = testClientConnList[0].Write([]byte("GETSERVER " + newServerAddr))
	if err != nil {
		t.Error(err)
	}
	_, err = testClientConnList[0].Read(buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf[:8]) != string("address:") {
		t.Errorf("'GETSERVER' command response is not correct.")
	}

	// test AddServer when the server already exists
	_, err = testClientConnList[0].Write([]byte("ADDSERVER " + newServerAddr + " 150 " + newServerProbe))
	if err != nil {
		t.Error(err)
	}
	n, err = testClientConnList[0].Read(buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf[:n]) != sysPrint.ErrServerExists.Error() {
		t.Errorf("'ADDSERVER' command response is not correct, expect:%s, actual:%s", sysPrint.ErrServerExists.Error(), string(buf[:n]))
	}

	// test DeleteServer
	_, err = testClientConnList[0].Write([]byte("DELETESERVER " + newServerAddr))
	if err != nil {
		t.Error(err)
	}
	n, err = testClientConnList[0].Read(buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf[:n]) != string(ReplyOK) {
		t.Errorf("'DELETESERVER' command response is not correct, expect:%s, actual:%s", string(ReplyOK), string(buf[:n]))
	}

	// test DeleteServer when the server does not exist
	_, err = testClientConnList[0].Write([]byte("DELETESERVER " + newServerAddr))
	if err != nil {
		t.Error(err)
	}
	n, err = testClientConnList[0].Read(buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf[:n]) != sysPrint.ErrServerNotExists.Error() {
		t.Errorf("'DELETESERVER' command response is not correct, expect:%s, actual:%s", sysPrint.ErrServerNotExists.Error(), string(buf[:n]))
	}

	// test Exists when the server does not exist
	_, err = testClientConnList[0].Write([]byte("EXISTS " + newServerAddr))
	if err != nil {
		t.Error(err)
	}
	n, err = testClientConnList[0].Read(buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf[:n]) != string(ServerNotExistsReply) {
		t.Errorf("'EXISTS' command response is not correct, expect:%s, actual:%s", string(ServerNotExistsReply), string(buf[:n]))
	}

	// test GetServer when the server does not exist
	_, err = testClientConnList[0].Write([]byte("GETSERVER " + newServerAddr))
	if err != nil {
		t.Error(err)
	}
	n, err = testClientConnList[0].Read(buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf[:n]) != sysPrint.ErrServerNotExists.Error() {
		t.Errorf("'GETSERVER' command response is not correct, expect:%s, actual:%s", string(ServerNotExistsReply), string(buf[:n]))
	}

	// test Save
	config.ConfigFilePath = "testSaveConfig.yaml"
	_, err = testClientConnList[0].Write([]byte("SAVE"))
	if err != nil {
		t.Error(err)
	}
	n, err = testClientConnList[0].Read(buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf[:n]) != string(ReplyOK) {
		t.Errorf("'SAVE' command response is not correct, expect:%s, actual:%s", string(ReplyOK), string(buf[:n]))
	}
	file, err := os.Open(config.ConfigFilePath)
	defer file.Close()
	if err != nil {
		t.Error(err)
	}
	data, err := ioutil.ReadAll(file)
	if err != nil {
		t.Error(err)
	}
	if strings.Index(string(data), "server-list") == -1 {
		t.Error("save to config yaml failed")
	}
}
