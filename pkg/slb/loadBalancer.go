package slb

import (
	"EH-Proxy/pkg/server"
	"EH-Proxy/pkg/slb/LeastActiveLB"
	"EH-Proxy/pkg/slb/RandomLB"
	"EH-Proxy/pkg/slb/RoundRobinLB"
	"EH-Proxy/pkg/system/sysPrint"
)

type LoadBalancerType string

const (
	RoundRobin  LoadBalancerType = "round-robin"
	Random      LoadBalancerType = "random"
	LeastActive LoadBalancerType = "least-active"
)

type LoadBalancer interface {
	AddServerNode(*server.Server) error
	DeleteServerNode(*server.Server) error
	SelectNode() (*server.Server, error)
	Reset()
	InitServerNode([]*server.Server) error
}

// LoadBalancerFactory 创建一个 balancerType 指定类型的负载均衡器
func LoadBalancerFactory(balancerType LoadBalancerType) (LoadBalancer, error) {
	switch balancerType {
	case RoundRobin:
		return RoundRobinLB.CreateRRLB(), nil
	case Random:
		return RandomLB.CreateRDLB(), nil
	case LeastActive:
		return LeastActiveLB.CreateLALB(), nil
	default:
		return nil, sysPrint.ErrUnknownLoadBalancer
	}
}
