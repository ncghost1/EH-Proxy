package proxy

import (
	"EH-Proxy/config"
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/slb"
	"EH-Proxy/pkg/system/sysPrint"
	"sync"
)

type proxy struct {
	serverGroup *ServerGroup
	config      *config.ProxyConfig
	stop        chan struct{}
}

var once sync.Once
var proxyInstance *proxy

// GetProxyInstance 获取 proxy 实例，懒汉式单例模式
func GetProxyInstance() *proxy {
	once.Do(func() {
		c, err := config.NewProxyConfig()
		if err != nil {
			sysPrint.PrintlnAndLogWriteFatalMsg(err.Error())
		}
		proxyInstance = &proxy{
			config: c,
			stop:   make(chan struct{}, 1),
		}
		sg := NewServerGroup(c.LoadBalancerType)
		for _, s := range c.InitServerList {
			err = sg.AddServer(proxyInstance, s.Addr, s.Weight, s.Probe)
			if err != nil {
				sysPrint.PrintlnAndLogWriteFatalMsg(err.Error())
			}
		}
		proxyInstance.serverGroup = sg
	})
	return proxyInstance
}

func (p *proxy) HealthCheckOption() bool {
	return p.config.HealthCheckOption
}

type ServerGroup struct {
	serverMap    map[string]*server.Server // 服务器哈希表，key: address
	mapRWLock    sync.RWMutex              // 哈希表读写锁
	loadBalancer slb.LoadBalancer          // 负载均衡器
	pfailCount   int32                     // 主观下线的服务器数目
}
