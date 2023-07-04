package proxy

import (
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/system/sysPrint"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	HttpScheme        = "http://"
	RequestTimeoutMsg = "Sorry,please retry later..."
)

var (
	invalidURLPath = []byte("Invalid URL path.")
)

// reverseProxyPool reverseProxy 对象池
type reverseProxyPool struct {
	pool *sync.Pool
}

func NewReverseProxyPool() *reverseProxyPool {
	return &reverseProxyPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return &httputil.ReverseProxy{Transport: &http.Transport{
					DisableKeepAlives: !GetProxyInstance().config.KeepAliveOption,
					Proxy:             http.ProxyFromEnvironment,
				}}
			},
		},
	}
}

func (p *reverseProxyPool) Get() *httputil.ReverseProxy {
	return p.pool.Get().(*httputil.ReverseProxy)
}

func (p *reverseProxyPool) Put(c *httputil.ReverseProxy) {
	p.pool.Put(c)
}

var (
	revProxyPool = NewReverseProxyPool()
)

func HttpHandleRequest(w http.ResponseWriter, r *http.Request) {
	p := GetProxyInstance()

	// Url 路径检测（如果启用了 Url 路径检测功能）
	if p.config.UrlPathCheckOption {
		path := r.URL.Path
		if _, ok := p.config.UrlPathMap[path]; !ok {
			if !p.config.UrlPathTrie.PrefixSearch(path) {
				w.WriteHeader(http.StatusBadRequest)
				_, err := w.Write(invalidURLPath)
				if err != nil {
					return
				}
			}
		}
	}

	// 使用负载均衡器选择一个节点进行转发
	s, err := p.serverGroup.loadBalancer.SelectNode()
	if err != nil {
		if err == sysPrint.ErrNoServer {
			fmt.Println(err)
		}
		log.Fatalln(err)
	}

	targetURL, err := url.Parse(HttpScheme + s.Addr())
	if err != nil {
		log.Fatal(err)
	}

	// 创建 ReverseProxy 对象
	reverseProxy := revProxyPool.Get()
	reverseProxy.Director = func(req *http.Request) {
		// 修改请求的目标地址为目标URL
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
	}

	var ctx context.Context
	var cancel context.CancelFunc

	// 是否启用断路器
	if p.config.CircuitBreakerOption {
		ctx, cancel = context.WithTimeout(context.Background(), p.config.RequestTimeout)
		defer cancel()
		r = r.WithContext(ctx)
	}

	sysPrint.LogWriteSystemMsg(string(p.config.LoadBalancerType) + " load balance:" + r.RemoteAddr + " -> " + s.Addr())
	s.IncrActiveReq() // 增加服务器活跃请求数
	reverseProxy.ServeHTTP(w, r)
	s.DecrActiveReq() // 减少服务器活跃请求数

	// 如果启用了断路器，检查请求是否超时
	if p.config.CircuitBreakerOption {
		select {
		case <-ctx.Done():
			if p.config.HealthCheckOption && s.Probe() != server.NoHealthCheck {
				s.SetPfail(server.IS_PFAIL) // 将超时服务器设为下线状态
				sysPrint.PrintlnAndLogWriteSystemMsg("Request for server:" + s.Addr() + " timeout,now considering it failure.")
				w.WriteHeader(http.StatusRequestTimeout)
				_, err = w.Write([]byte(RequestTimeoutMsg))
				if err != nil {
					sysPrint.PrintlnErrorMsg(err.Error())
					return
				}
			}
		default:
		}
	}

}

func (p *proxy) Serve() {
	if p.config.HealthCheckOption {
		for _, s := range p.serverGroup.ServerMap() {
			go p.healthCheck(s)
		}
	}
	httpServer := &http.Server{
		Addr: p.config.Addr,
	}
	http.HandleFunc("/", HttpHandleRequest)
	sysPrint.PrintlnSystemMsg("EH-Proxy start listening at:" + p.config.Addr + ", ready to accept connections.")

	go func() {
		err := httpServer.ListenAndServe()
		if err != nil || err != http.ErrServerClosed {
			sysPrint.FatalMsg(err.Error())
		}
	}()

	// 等待中断信号或关闭命令来停止 proxy
	signalQuit := make(chan os.Signal, 1)
	signal.Notify(signalQuit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-signalQuit:
		sysPrint.PrintlnAndLogWriteSystemMsg("EH-Proxy receive shutdown signal...")
	case <-p.stop:
		sysPrint.PrintlnAndLogWriteSystemMsg("EH-Proxy receive shutdown command...")
	}

	err := httpServer.Shutdown(context.Background())
	if err != nil {
		sysPrint.FatalMsg(err.Error())
	}
	p.beforeExit()
}

// healthCheck 对服务器进行健康检测
func (p *proxy) healthCheck(s *server.Server) {
	if s.Probe() == server.NoHealthCheck {
		return
	}

	setPfailFunc := func() {
		if s.Pfail() == server.IS_PFAIL {
			return
		}
		s.SetPfail(server.IS_PFAIL)
		p.serverGroup.addPfailCount(1)
		sysPrint.PrintlnAndLogWriteSystemMsg(s.Addr() + " is considered failure.")
	}

	for {
		select {
		case <-time.After(p.config.HeahthCheckInterval):
			ctx, cancel := context.WithTimeout(context.Background(), p.config.PfailTime)
			curtime := s.HeartBeat(ctx)
			select {
			case <-ctx.Done():
				cancel()
				setPfailFunc()
			default:
				cancel()
				if curtime == server.NoAck {
					setPfailFunc()
					continue
				}
				s.SetLastAck(curtime)
				if s.Pfail() == server.IS_PFAIL {
					s.SetPfail(server.NOT_PFAIL)
					p.serverGroup.addPfailCount(-1)
					sysPrint.PrintlnAndLogWriteSystemMsg(s.Addr() + " is back online.")
				}
			}
		case <-s.StopHealthCheck():
			sysPrint.PrintlnAndLogWriteSystemMsg("server:" + s.Addr() + " health check stopped.")
			return
		}
	}
}

func (p *proxy) Shutdown() {
	p.stop <- struct{}{}
}

// beforeExit 退出前执行逻辑
func (p *proxy) beforeExit() {
	err := p.serverGroup.saveServerListToDisk() // 将当前服务器列表保存到本地配置文件
	if err != nil {
		sysPrint.PrintlnErrorMsg(err.Error())
	}
}
