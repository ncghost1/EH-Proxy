package proxy

import (
	"EH-Proxy/config"
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/slb"
	"EH-Proxy/pkg/system/sysPrint"
	"log"
	"sync"
	"sync/atomic"
)

func NewServerGroup(balancerType slb.LoadBalancerType) *ServerGroup {
	lb, err := slb.LoadBalancerFactory(balancerType)
	if err != nil {
		log.Fatalln(err)
	}
	return &ServerGroup{
		serverMap:    make(map[string]*server.Server),
		mapRWLock:    sync.RWMutex{},
		loadBalancer: lb,
		pfailCount:   0,
	}
}

func (s *ServerGroup) ServerMap() map[string]*server.Server {
	s.mapRWLock.RLock()
	defer s.mapRWLock.RUnlock()
	return s.serverMap
}

func (s *ServerGroup) SetServerMap(serverMap map[string]*server.Server) {
	s.mapRWLock.Lock()
	defer s.mapRWLock.Unlock()
	s.serverMap = serverMap
}

func (s *ServerGroup) AddServer(p *proxy, addr string, weight int32, probe string) error {
	s.mapRWLock.Lock()
	defer s.mapRWLock.Unlock()
	if _, ok := s.serverMap[addr]; ok {
		return sysPrint.ErrServerExists
	}
	newServer, err := server.NewServer(addr, weight, probe)
	if err != nil {
		return err
	}
	s.serverMap[addr] = newServer
	err = s.loadBalancer.AddServerNode(newServer)
	if err != nil {
		return err
	}
	go p.healthCheck(newServer)
	return nil
}

func (s *ServerGroup) DeleteServer(addr string) error {
	s.mapRWLock.Lock()
	defer s.mapRWLock.Unlock()
	sv, ok := s.serverMap[addr]
	if !ok {
		return sysPrint.ErrServerNotExists
	}
	err := s.loadBalancer.DeleteServerNode(sv)
	if err != nil {
		return err
	}
	sv.CloseStopHealthCheck()
	delete(s.serverMap, addr)
	return nil
}

func (s *ServerGroup) SetWeight(addr string, weight int32) error {
	s.mapRWLock.Lock()
	defer s.mapRWLock.Unlock()
	if sv, ok := s.serverMap[addr]; !ok {
		return sysPrint.ErrServerNotExists
	} else {
		err := sv.SetWeight(weight)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ServerGroup) IsServerExists(addr string) bool {
	s.mapRWLock.Lock()
	defer s.mapRWLock.Unlock()
	if _, ok := s.serverMap[addr]; !ok {
		return false
	} else {
		return true
	}
}

func (s *ServerGroup) GetServer(addr string) (*server.Server, error) {
	s.mapRWLock.Lock()
	defer s.mapRWLock.Unlock()
	if sv, ok := s.serverMap[addr]; !ok {
		return nil, sysPrint.ErrServerNotExists
	} else {
		return sv, nil
	}
}

func (s *ServerGroup) LoadBalancer() slb.LoadBalancer {
	return s.loadBalancer
}

func (s *ServerGroup) SetLoadBalancer(loadBalancer slb.LoadBalancer) {
	s.loadBalancer = loadBalancer
}

func (s *ServerGroup) PfailCount() int32 {
	return atomic.LoadInt32(&s.pfailCount)
}

func (s *ServerGroup) setPfailCount(pfailCount int32) {
	atomic.StoreInt32(&s.pfailCount, pfailCount)
}

func (s *ServerGroup) addPfailCount(delta int32) {
	atomic.AddInt32(&s.pfailCount, delta)
}

func (s *ServerGroup) saveServerListToDisk() error {
	s.mapRWLock.Lock()
	s.mapRWLock.Unlock()
	p := GetProxyInstance()
	newServerList := make([]config.ServerConfig, 0)
	for _, s := range p.serverGroup.ServerMap() {
		srv := config.ServerConfig{
			Addr:   s.Addr(),
			Weight: s.Weight(),
			Probe:  s.Probe(),
		}
		newServerList = append(newServerList, srv)
	}
	p.config.InitServerList = newServerList
	err := config.WriteConfig(p.config)
	if err != nil {
		return err
	}
	sysPrint.PrintlnSystemMsg("Save current config to disk success.")
	return nil
}
