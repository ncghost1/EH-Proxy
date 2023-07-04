package server

import (
	"EH-Proxy/pkg/system/sysPrint"
	"context"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

const (
	NoHealthCheck = ""
	NoAck         = time.Duration(-1)
	IS_PFAIL      = int32(1)
	NOT_PFAIL     = int32(0)
	DefaultWeight = 100
	MaxWeight     = 1000000
)

// Server EasyProxy 所代理的服务器
// 目前仅支持代理 HTTP
type Server struct {
	addr            string        // 连接地址
	weight          int32         // 权重
	probe           string        // 健康检测接口地址，若 probe 为空则不进行健康检测
	stopHealthCheck chan struct{} // 健康检测关闭信号通道
	lastAck         time.Duration // 上次回复时间
	activeReq       int32         // 活跃请求数
	pfail           int32         // 主观下线状态
}

func (s *Server) StopHealthCheck() chan struct{} {
	return s.stopHealthCheck
}

func (s *Server) CloseStopHealthCheck() {
	if s.stopHealthCheck != nil {
		close(s.stopHealthCheck)
	}
}

func weightCheck(weight int32) error {
	if weight < 0 {
		return sysPrint.ErrServerWeightNegative
	}
	if weight == 0 {
		weight = DefaultWeight
	}
	if weight > MaxWeight {
		return sysPrint.ErrServerWeightGreaterThanMax
	}
	return nil
}

// NewServer 创建一个 Server
// addr: 连接地址 IP:PORT / domain name
// weight: 权重
// probe: 健康检测接口地址，该接口应返回 HTTP 200 OK，留空则不对该服务器进行健康检测
func NewServer(addr string, weight int32, probe string) (*Server, error) {
	err := weightCheck(weight)
	if err != nil {
		return nil, err
	}
	_, _, err = net.SplitHostPort(addr)
	if err != nil {
		return nil, sysPrint.ErrServerAddrInvalid
	}
	if probe != NoHealthCheck {
		// 解析探测地址
		_, err := url.Parse(probe)
		if err != nil {
			return nil, sysPrint.ErrServerProbeInvalid
		}

		return &Server{addr: addr, weight: weight, probe: probe, stopHealthCheck: make(chan struct{}, 1)}, nil
	}
	return &Server{addr: addr, weight: weight, probe: probe}, nil
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) SetWeight(weight int32) error {
	err := weightCheck(weight)
	if err != nil {
		return err
	}
	atomic.StoreInt32(&s.weight, weight)
	return nil
}

func (s *Server) Weight() int32 {
	return atomic.LoadInt32(&s.weight)
}

func (s *Server) LastAck() time.Duration {
	return time.Duration(atomic.LoadInt64((*int64)(&s.lastAck)))
}

func (s *Server) SetLastAck(lastAck time.Duration) {
	atomic.StoreInt64((*int64)(&s.lastAck), int64(lastAck))
}

func (s *Server) Probe() string {
	return s.probe
}

func (s *Server) ActiveReq() int32 {
	return atomic.LoadInt32(&s.activeReq)
}

func (s *Server) IncrActiveReq() {
	atomic.AddInt32(&s.activeReq, 1)
}

func (s *Server) DecrActiveReq() {
	atomic.AddInt32(&s.activeReq, -1)
}

func (s *Server) Pfail() int32 {
	return atomic.LoadInt32(&s.pfail)
}

func (s *Server) SetPfail(pfail int32) {
	atomic.StoreInt32(&s.pfail, pfail)
}

// HeartBeat 对 probe 发送 GET 请求以检测服务器健康状况
func (s *Server) HeartBeat(ctx context.Context) (ackTime time.Duration) {
	resp, err := http.Get(s.probe)
	if err != nil {
		return NoAck
	}
	if resp.StatusCode != http.StatusOK {
		return NoAck
	}
	return time.Duration(time.Now().UnixMilli())
}
