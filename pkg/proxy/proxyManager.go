package proxy

import (
	"EH-Proxy/pkg/system/sysPrint"
	"bytes"
	"errors"
	"flag"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	defaultQueryBufferSize = 1024
)

var (
	ErrUnknownCommand = []byte(sysPrint.ERROR + "Unknown command error.")
	ErrFailToRead     = "proxy manager failed to read client message:"
	ErrReplyClient    = "reply to client failed, client addr:"
	ReplyOK           = []byte("OK")
)

type proxyManager struct {
	p          *proxy
	listener   net.Listener
	clientList []*client
	commandMap map[string]func(c *client, args [][]byte) error
	stop       chan struct{}
}

var managerOnce sync.Once
var proxyManagerInstance *proxyManager

// GetProxyManagerInstance 获取 proxyManager 实例，懒汉式单例模式
func GetProxyManagerInstance() *proxyManager {
	p := GetProxyInstance()
	managerOnce.Do(func() {
		proxyManagerInstance = &proxyManager{
			p:          p,
			listener:   nil,
			clientList: make([]*client, 0),
			commandMap: make(map[string]func(c *client, args [][]byte) error, 0),
			stop:       make(chan struct{}, 1),
		}
	})
	return proxyManagerInstance
}

type client struct {
	conn    net.Conn
	args    [][]byte
	sendBuf []byte
}

var (
	queryBufSize = flag.Int("queryBufferSize", defaultQueryBufferSize, "the proxy manager query buffer size")
)

var bufferPool = NewBufferPool()

// BufferPool byte slice 对象池
type BufferPool struct {
	pool *sync.Pool
}

func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, *queryBufSize)
			},
		},
	}
}

func (p *BufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

func (p *BufferPool) Put(b []byte) {
	p.pool.Put(b)
}

func (pm *proxyManager) AddClient(conn net.Conn) *client {
	cli := &client{
		conn: conn,
	}
	pm.clientList = append(pm.clientList, cli)
	sysPrint.LogWriteSystemMsg("client: " + conn.RemoteAddr().String() + " connected.")
	return cli
}

func (pm *proxyManager) Serve() {
	sysPrint.PrintlnSystemMsg("EH-Proxy-Manager start listening at:" + pm.p.config.ManagerAddr + ", ready to accept connections.")
	listener, err := net.Listen("tcp", pm.p.config.ManagerAddr)
	if err != nil {
		sysPrint.FatalMsg(err.Error())
	}
	pm.listener = listener
	signalQuit := make(chan os.Signal, 1)
	signal.Notify(signalQuit, syscall.SIGINT, syscall.SIGTERM)

	// 监听关闭信号
	go func() {
		select {
		case <-signalQuit:
			sysPrint.PrintlnAndLogWriteSystemMsg("EH-Proxy-Manager receive shutdown signal...")
		case <-pm.stop:
			sysPrint.PrintlnAndLogWriteSystemMsg("EH-Proxy-Manager receive shutdown command...")
		}
		close(signalQuit)
		close(pm.stop)
		pm.beforeExit()
	}()

	// 接受连接并处理
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-signalQuit:
				return
			case <-pm.stop:
				return
			default:
				sysPrint.PrintlnErrorMsg(err.Error())
				continue
			}
		}
		cli := pm.AddClient(conn)
		// 在新的 goroutine 中处理连接
		go pm.handleConnection(cli)
	}
}

func (pm *proxyManager) handleConnection(c *client) {
	defer c.conn.Close()
	for {
		// 读取客户端发送的数据
		buffer := bufferPool.Get()
		n, err := c.conn.Read(buffer)
		if err != nil {
			sysPrint.PrintlnErrorMsg(ErrFailToRead + err.Error())
			return
		}
		buffer = bytes.ToLower(buffer)
		args := bytes.Fields(buffer[:n])
		commandName := string(args[0])
		commandFunc, ok := pm.commandMap[commandName]
		if !ok {
			err = c.Reply(ErrUnknownCommand)
			if err != nil {
				sysPrint.PrintlnErrorMsg(ErrReplyClient + c.conn.RemoteAddr().String())
				return
			}
			continue
		}
		err = commandFunc(c, args)
		if err != nil {
			if err.Error() == ErrReplyClient {
				sysPrint.PrintlnErrorMsg(ErrReplyClient + c.conn.RemoteAddr().String())
				return
			}
			err = c.Reply([]byte(err.Error()))
			if err != nil {
				sysPrint.PrintlnErrorMsg(ErrReplyClient + c.conn.RemoteAddr().String())
				return
			}
			continue
		}
	}
}

func (c *client) Reply(buf []byte) error {
	err := c.sendToClient(buf)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) sendToClient(msg []byte) error {
	_, err := c.conn.Write(msg)
	if err != nil {
		return errors.New(ErrReplyClient)
	}
	return nil
}

func (pm *proxyManager) Shutdown() {
	pm.stop <- struct{}{}
}

func (pm *proxyManager) beforeExit() {
	pm.listener.Close()
	for i := range pm.clientList {
		pm.clientList[i].conn.Close()
	}
}
