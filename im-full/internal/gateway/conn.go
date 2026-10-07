package gateway

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"example.com/im/internal/protocol"
)

const backendTimeout = 3 * time.Second

// Conn 代表一条客户端长连接。每条连接固定 2 个协程：
//
//	readLoop  —— 读帧、分发（主协程，即 run）
//	writeLoop —— 从 sendQ 取数据批量写出
//
// 读写分离的好处：慢写不会阻塞心跳读取；所有写操作串行化，无需对 net.Conn 加锁。
type Conn struct {
	id  uint64
	srv *Server
	nc  net.Conn
	br  *bufio.Reader

	sendQ     chan []byte
	done      chan struct{}
	closeOnce sync.Once

	// uid/deviceID 在认证成功、Manager.Add 之前写入；之后只读。
	uid      int64
	deviceID string
	authed   atomic.Bool

	limiter   *limiter
	lastRoute time.Time // 仅 readLoop 访问

	// 可靠推送：未收到 ACK 的推送在此排队重传。
	pushSeq atomic.Uint32
	pmu     sync.Mutex
	pending map[uint32]*pendingPush
}

type pendingPush struct {
	frame *protocol.Frame
	tries int
	timer *time.Timer
}

func newConn(s *Server, id uint64, nc net.Conn) *Conn {
	return &Conn{
		id:  id,
		srv: s,
		nc:  nc,
		// 百万连接下每连接缓冲区都是真金白银：1KB * 100万 = 1GB。
		br:      bufio.NewReaderSize(nc, 1024),
		sendQ:   make(chan []byte, s.cfg.SendQueueSize),
		done:    make(chan struct{}),
		limiter: newLimiter(s.cfg.RatePerSec, s.cfg.Burst),
	}
}

func (c *Conn) session() Session { return Session{UID: c.uid, DeviceID: c.deviceID} }

// ---------------------------------------------------------------- 生命周期

func (c *Conn) run() {
	defer c.cleanup()
	go c.writeLoop()

	if !c.handshake() {
		return
	}
	c.readLoop()
}

func (c *Conn) cleanup() {
	c.Close()
	if c.authed.Load() {
		c.srv.mgr.Remove(c)
		ctx, cancel := context.WithTimeout(context.Background(), backendTimeout)
		defer cancel()
		// Unbind 带 connID，store 只会删除"属于这条连接"的路由，不会误删新连接。
		_ = c.srv.routes.Unbind(ctx, c.session(), c.srv.cfg.GatewayID, c.id)
	}
	c.srv.all.Delete(c.id)
	c.srv.open.Add(-1)
}

// Close 立即关闭连接，幂等。
func (c *Conn) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.nc.Close() // 会让阻塞中的 Read/Write 立刻返回错误
		c.pmu.Lock()
		for seq, p := range c.pending {
			p.timer.Stop()
			delete(c.pending, seq)
		}
		c.pmu.Unlock()
	})
}

// closeAfterFlush 等队列里已有的数据写完再关（用 nil 作为哨兵）。
func (c *Conn) closeAfterFlush() {
	select {
	case c.sendQ <- nil:
	default:
		c.Close()
	}
}

func (c *Conn) kick(reason string, retryAfterMs int) {
	c.Send(&protocol.Frame{Cmd: protocol.CmdKick, Body: protocol.Marshal(&protocol.Kick{
		Reason: reason, RetryAfterMs: retryAfterMs,
	})})
	c.closeAfterFlush()
}

// ---------------------------------------------------------------- 握手

func (c *Conn) handshake() bool {
	cfg := &c.srv.cfg
	// 认证超时：防止恶意连接只建连不说话，白占文件描述符。
	_ = c.nc.SetReadDeadline(time.Now().Add(cfg.AuthTimeout))
	f, err := protocol.ReadFrame(c.br, cfg.MaxBody)
	if err != nil {
		return false
	}
	if f.Cmd != protocol.CmdAuthReq {
		return c.authFail(f.Seq, protocol.CodeBadRequest, "auth required")
	}
	var req protocol.AuthReq
	if err := protocol.Unmarshal(f.Body, &req); err != nil || req.DeviceID == "" {
		return c.authFail(f.Seq, protocol.CodeBadRequest, "bad auth request")
	}

	ctx, cancel := context.WithTimeout(context.Background(), backendTimeout)
	defer cancel()
	uid, err := c.srv.backend.Authenticate(ctx, req.Token)
	if err != nil {
		c.srv.metrics.AuthFail.Add(1)
		return c.authFail(f.Seq, protocol.CodeAuthFailed, "invalid token")
	}

	c.uid, c.deviceID = uid, req.DeviceID
	if old := c.srv.mgr.Add(c); old != nil {
		old.kick("login_elsewhere", 0) // 同账号同设备重复登录：踢旧留新
	}
	if err := c.srv.routes.Bind(ctx, c.session(), cfg.GatewayID, c.id, cfg.RouteTTL); err != nil {
		c.srv.mgr.Remove(c)
		return c.authFail(f.Seq, protocol.CodeInternal, "route bind failed")
	}
	c.authed.Store(true)
	c.lastRoute = time.Now()

	c.Send(&protocol.Frame{Cmd: protocol.CmdAuthResp, Seq: f.Seq, Body: protocol.Marshal(&protocol.AuthResp{
		Code:         protocol.CodeOK,
		HeartbeatSec: int(cfg.HeartbeatInterval / time.Second),
		ServerTime:   time.Now().UnixMilli(),
	})})
	return true
}

// authFail 回包后等待数据 flush 再断开，让客户端能看到失败原因。
func (c *Conn) authFail(seq uint32, code int, msg string) bool {
	c.Send(&protocol.Frame{Cmd: protocol.CmdAuthResp, Seq: seq, Body: protocol.Marshal(&protocol.AuthResp{Code: code, Msg: msg})})
	c.closeAfterFlush()
	select {
	case <-c.done:
	case <-time.After(c.srv.cfg.WriteTimeout):
	}
	return false
}

// ---------------------------------------------------------------- 读

func (c *Conn) readLoop() {
	cfg := &c.srv.cfg
	for {
		// 每读一帧就续一次读超时：超过 IdleTimeout 没收到任何数据（包括心跳）
		// 就判定为死连接。这是检测"假死连接"（NAT 超时、对端断电）的唯一可靠手段。
		_ = c.nc.SetReadDeadline(time.Now().Add(cfg.IdleTimeout))
		f, err := protocol.ReadFrame(c.br, cfg.MaxBody)
		if err != nil {
			c.srv.logReadErr(c, err)
			return
		}
		c.srv.metrics.FramesIn.Add(1)
		c.dispatch(f)
	}
}

func (c *Conn) dispatch(f *protocol.Frame) {
	switch f.Cmd {
	case protocol.CmdHeartbeat:
		c.Send(&protocol.Frame{Cmd: protocol.CmdHeartbeatAck, Seq: f.Seq})
		c.maybeRefreshRoute()
	case protocol.CmdSendReq:
		c.handleSend(f)
	case protocol.CmdSyncReq:
		c.handleSync(f)
	case protocol.CmdReadReq:
		c.handleRead(f)
	case protocol.CmdPushAck:
		c.handlePushAck(f)
	default:
		c.sendErr(f.Seq, protocol.CodeBadRequest, "unknown cmd")
	}
}

func (c *Conn) handleSend(f *protocol.Frame) {
	var req protocol.SendReq
	if err := protocol.Unmarshal(f.Body, &req); err != nil || req.ClientMsgID == "" || (req.ToUID == 0 && req.GroupID == 0) {
		c.sendAck(f.Seq, &protocol.SendAck{ClientMsgID: req.ClientMsgID, Code: protocol.CodeBadRequest})
		return
	}
	if !c.limiter.Allow() {
		c.srv.metrics.RateLimited.Add(1)
		c.sendAck(f.Seq, &protocol.SendAck{ClientMsgID: req.ClientMsgID, Code: protocol.CodeRateLimited})
		return
	}
	// 同步调用后端：同一连接上的消息严格按到达顺序处理，天然保序。
	// 代价是后端慢会阻塞该连接的读取 —— 所以必须带超时。
	// 更高吞吐的做法：每连接一个有界队列 + 独立 worker，或按 uid 哈希到 worker 池。
	ctx, cancel := context.WithTimeout(context.Background(), backendTimeout)
	defer cancel()
	ack, err := c.srv.backend.HandleSend(ctx, c.session(), &req)
	if err != nil || ack == nil {
		// 返回错误码而不是静默丢弃；客户端会带同一个 client_msg_id 重试，后端幂等。
		ack = &protocol.SendAck{ClientMsgID: req.ClientMsgID, Code: protocol.CodeInternal}
	}
	c.sendAck(f.Seq, ack)
}

func (c *Conn) handleSync(f *protocol.Frame) {
	var req protocol.SyncReq
	if protocol.Unmarshal(f.Body, &req) != nil {
		c.sendErr(f.Seq, protocol.CodeBadRequest, "bad sync request")
		return
	}
	if !c.limiter.Allow() {
		c.srv.metrics.RateLimited.Add(1)
		c.sendErr(f.Seq, protocol.CodeRateLimited, "rate limited")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), backendTimeout)
	defer cancel()
	resp, err := c.srv.backend.HandleSync(ctx, c.session(), &req)
	if err != nil || resp == nil {
		c.sendErr(f.Seq, protocol.CodeInternal, "sync failed")
		return
	}
	c.Send(&protocol.Frame{Cmd: protocol.CmdSyncResp, Seq: f.Seq, Body: protocol.Marshal(resp)})
}

func (c *Conn) handleRead(f *protocol.Frame) {
	var req protocol.ReadReq
	if protocol.Unmarshal(f.Body, &req) != nil || req.ConvID == "" {
		c.sendErr(f.Seq, protocol.CodeBadRequest, "bad read request")
		return
	}
	if !c.limiter.Allow() {
		c.srv.metrics.RateLimited.Add(1)
		c.sendErr(f.Seq, protocol.CodeRateLimited, "rate limited")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), backendTimeout)
	defer cancel()
	ack, err := c.srv.backend.HandleRead(ctx, c.session(), &req)
	if err != nil || ack == nil {
		c.sendErr(f.Seq, protocol.CodeInternal, "read failed")
		return
	}
	c.Send(&protocol.Frame{Cmd: protocol.CmdReadAck, Seq: f.Seq, Body: protocol.Marshal(ack)})
}

func (c *Conn) sendAck(seq uint32, ack *protocol.SendAck) {
	c.Send(&protocol.Frame{Cmd: protocol.CmdSendAck, Seq: seq, Body: protocol.Marshal(ack)})
}

func (c *Conn) sendErr(seq uint32, code int, msg string) {
	c.Send(&protocol.Frame{Cmd: protocol.CmdErr, Seq: seq, Body: protocol.Marshal(&protocol.ErrBody{Code: code, Msg: msg})})
}

func (c *Conn) maybeRefreshRoute() {
	cfg := &c.srv.cfg
	if time.Since(c.lastRoute) < cfg.RouteRefresh {
		return
	}
	c.lastRoute = time.Now()
	go func() { // 不要在读协程里做网络 IO
		ctx, cancel := context.WithTimeout(context.Background(), backendTimeout)
		defer cancel()
		_ = c.srv.routes.Bind(ctx, c.session(), cfg.GatewayID, c.id, cfg.RouteTTL)
	}()
}

// ---------------------------------------------------------------- 写

// Send 把帧放进发送队列，永不阻塞调用方。
// 队列满 = 对端消费太慢。此时宁可断开该连接，也不能让内存无限增长、
// 更不能让一个慢客户端拖慢给其他人投递的协程。客户端重连后靠 sync 补消息。
func (c *Conn) Send(f *protocol.Frame) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.sendQ <- f.Marshal():
		c.srv.metrics.FramesOut.Add(1)
		return true
	default:
		c.srv.metrics.SlowKick.Add(1)
		c.srv.log.Warn("send queue full, closing slow consumer", "conn", c.id, "uid", c.uid)
		c.Close()
		return false
	}
}

func (c *Conn) writeLoop() {
	batch := make([][]byte, 0, 64)
	for {
		clear(batch)
		batch = batch[:0]
		select {
		case <-c.done:
			return
		case b := <-c.sendQ:
			if b == nil {
				c.Close()
				return
			}
			batch = append(batch, b)
		}
		// 攒批：把队列里现有的帧一次取完，用 writev 一次系统调用写出，
		// 比"一帧一次 write"省大量 syscall，也不需要每连接一个 bufio.Writer。
		closing := false
	drain:
		for len(batch) < 64 {
			select {
			case b := <-c.sendQ:
				if b == nil {
					closing = true
					break drain
				}
				batch = append(batch, b)
			default:
				break drain
			}
		}
		_ = c.nc.SetWriteDeadline(time.Now().Add(c.srv.cfg.WriteTimeout))
		nb := net.Buffers(batch) // WriteTo 会消费 nb，但不影响 batch 本身
		if _, err := nb.WriteTo(c.nc); err != nil {
			c.Close()
			return
		}
		if closing {
			c.Close()
			return
		}
	}
}

// ---------------------------------------------------------------- 可靠推送

// PushReliable 推送一条消息，并在收不到 PushAck 时按间隔重传。
// 注意：重传次数用完后只是放弃，消息并没有丢——它早已在存储层的收件箱里，
// 客户端下次上线/重连时通过 sync(last_seq) 拉取。推送只是"加速通知"。
func (c *Conn) PushReliable(body []byte) {
	seq := c.pushSeq.Add(1)
	p := &pendingPush{frame: &protocol.Frame{Cmd: protocol.CmdPush, Seq: seq, Body: body}}

	c.pmu.Lock()
	select {
	case <-c.done: // Close 已执行清理，别再登记定时器
		c.pmu.Unlock()
		return
	default:
	}
	if c.pending == nil { // 惰性创建：绝大多数连接大部分时间没有待确认推送
		c.pending = make(map[uint32]*pendingPush)
	}
	c.pending[seq] = p
	p.timer = time.AfterFunc(c.srv.cfg.PushRetryInterval, func() { c.retryPush(seq) })
	c.pmu.Unlock()

	c.Send(p.frame)
}

func (c *Conn) retryPush(seq uint32) {
	c.pmu.Lock()
	p, ok := c.pending[seq]
	if !ok {
		c.pmu.Unlock()
		return
	}
	p.tries++
	if p.tries > c.srv.cfg.PushMaxRetry {
		delete(c.pending, seq)
		c.pmu.Unlock()
		c.srv.metrics.PushGiveUp.Add(1)
		return
	}
	p.timer.Reset(c.srv.cfg.PushRetryInterval)
	c.pmu.Unlock()

	c.srv.metrics.PushRetry.Add(1)
	c.Send(p.frame)
}

func (c *Conn) handlePushAck(f *protocol.Frame) {
	c.pmu.Lock()
	if p, ok := c.pending[f.Seq]; ok {
		p.timer.Stop()
		delete(c.pending, f.Seq)
	}
	c.pmu.Unlock()

	var ack protocol.PushAck
	if err := protocol.Unmarshal(f.Body, &ack); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), backendTimeout)
		defer cancel()
		c.srv.backend.OnPushAck(ctx, c.session(), ack.MsgID)
	}
}

// ---------------------------------------------------------------- 日志

func (s *Server) logReadErr(c *Conn, err error) {
	var ne net.Error
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, net.ErrClosed), errors.Is(err, os.ErrDeadlineExceeded):
		s.log.Debug("conn closed", "conn", c.id, "uid", c.uid, "reason", err)
	case errors.As(err, &ne):
		s.log.Debug("conn net error", "conn", c.id, "uid", c.uid, "err", err)
	default:
		s.log.Warn("protocol error", "conn", c.id, "uid", c.uid, "err", err)
	}
}
