package gateway

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"example.com/im/internal/protocol"
)

// Metrics 是最基础的运行指标；生产中接 Prometheus。
type Metrics struct {
	Accepted    atomic.Int64
	AuthFail    atomic.Int64
	SlowKick    atomic.Int64
	RateLimited atomic.Int64
	PushRetry   atomic.Int64
	PushGiveUp  atomic.Int64
	FramesIn    atomic.Int64
	FramesOut   atomic.Int64
}

type Server struct {
	cfg     Config
	backend Backend
	routes  RouteStore
	mgr     *Manager
	log     *slog.Logger

	lnMu sync.Mutex
	ln   net.Listener

	connID       atomic.Uint64
	open         atomic.Int64 // 所有连接数（含未认证）
	all          sync.Map     // connID -> *Conn
	wg           sync.WaitGroup
	shuttingDown atomic.Bool

	metrics Metrics
}

func NewServer(cfg Config, b Backend, r RouteStore) *Server {
	cfg.setDefaults()
	return &Server{cfg: cfg, backend: b, routes: r, mgr: NewManager(), log: slog.Default()}
}

func (s *Server) SetLogger(l *slog.Logger) { s.log = l }

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve 是 accept 循环。
func (s *Server) Serve(ln net.Listener) error {
	s.lnMu.Lock()
	s.ln = ln
	s.lnMu.Unlock()
	s.log.Info("gateway listening", "addr", ln.Addr().String())

	var backoff time.Duration
	for {
		nc, err := ln.Accept()
		if err != nil {
			if s.shuttingDown.Load() || errors.Is(err, net.ErrClosed) {
				return nil
			}
			// 例如 EMFILE（fd 耗尽）：不能 busy-loop，指数退避后重试（同 net/http）。
			if backoff == 0 {
				backoff = 5 * time.Millisecond
			} else if backoff *= 2; backoff > time.Second {
				backoff = time.Second
			}
			s.log.Warn("accept error", "err", err, "retry_in", backoff)
			time.Sleep(backoff)
			continue
		}
		backoff = 0

		if tc, ok := nc.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true) // 小包低延迟（Go 默认已开，显式写出便于理解）
			// TCP keepalive 只是兜底；NAT 老化和应用层假死仍要靠应用层心跳。
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(30 * time.Second)
		}
		go s.ServeConn(nc)
	}
}

// ServeConn 接管一条已建立的连接并阻塞直到它结束。
// TCP 接入（Serve）和 WebSocket 接入（HandleWS）都走这里，复用同一套 Conn 逻辑。
func (s *Server) ServeConn(nc net.Conn) {
	if s.shuttingDown.Load() || s.open.Add(1) > s.cfg.MaxConns {
		if !s.shuttingDown.Load() {
			s.open.Add(-1)
		}
		_ = nc.Close() // 过载保护：宁可拒绝新连接，也别拖垮已有用户
		return
	}
	s.metrics.Accepted.Add(1)
	c := newConn(s, s.connID.Add(1), nc)
	s.all.Store(c.id, c)
	s.wg.Add(1)
	defer s.wg.Done()
	c.run()
}

// PushToUser 向某用户所有在线设备推送（可排除某设备，比如"发送者的其他设备"）。
// 返回命中的本机连接数；0 表示该用户不在本网关（或不在线）。
// 生产中逻辑层先查路由表找到网关，再通过 gRPC 调到这里。
func (s *Server) PushToUser(uid int64, exceptDevice string, body []byte) int {
	n := 0
	for _, c := range s.mgr.Conns(uid) {
		if c.deviceID == exceptDevice {
			continue
		}
		c.PushReliable(body)
		n++
	}
	return n
}

// NotifyUser 向用户的在线设备发一次性通知（不重传、不要求 ACK），如"已读回执"。
// 丢了也没关系：这类状态客户端下次拉会话列表时会重新得到。
func (s *Server) NotifyUser(uid int64, exceptDevice string, cmd uint16, body []byte) int {
	n := 0
	for _, c := range s.mgr.Conns(uid) {
		if c.deviceID == exceptDevice {
			continue
		}
		if c.Send(&protocol.Frame{Cmd: cmd, Body: body}) {
			n++
		}
	}
	return n
}

func (s *Server) OnlineCount() int64 { return s.mgr.Count() }

func (s *Server) Stats() map[string]int64 {
	return map[string]int64{
		"online":       s.mgr.Count(),
		"open_conns":   s.open.Load(),
		"accepted":     s.metrics.Accepted.Load(),
		"auth_fail":    s.metrics.AuthFail.Load(),
		"slow_kick":    s.metrics.SlowKick.Load(),
		"rate_limited": s.metrics.RateLimited.Load(),
		"push_retry":   s.metrics.PushRetry.Load(),
		"push_giveup":  s.metrics.PushGiveUp.Load(),
		"frames_in":    s.metrics.FramesIn.Load(),
		"frames_out":   s.metrics.FramesOut.Load(),
	}
}

// Shutdown 平滑摘流：
//  1. 停止接收新连接（上游 LB 此时应已把本节点摘除）
//  2. 给每个连接下发 Kick，带随机 retry_after，让客户端错峰重连，避免"惊群"
//  3. 等连接自然退出，超时则强制关闭
func (s *Server) Shutdown(ctx context.Context) error {
	s.shuttingDown.Store(true)
	s.lnMu.Lock()
	if s.ln != nil {
		_ = s.ln.Close()
	}
	s.lnMu.Unlock()

	for _, c := range s.mgr.Snapshot() {
		c.kick("server_restart", 500+rand.Intn(5000))
	}
	s.all.Range(func(_, v any) bool {
		if c := v.(*Conn); !c.authed.Load() {
			c.Close()
		}
		return true
	})

	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.all.Range(func(_, v any) bool { v.(*Conn).Close(); return true })
		return ctx.Err()
	}
}
