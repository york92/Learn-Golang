// Package client 是 Go 版 IM 客户端 SDK（CLI 与测试使用），演示客户端该做的事：
// 认证、心跳、假死检测、指数退避+抖动重连、发送重试（幂等）、
// 收推送去重+ACK、按 user_seq 游标增量同步并检测空洞。
package client

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	mrand "math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"example.com/im/internal/protocol"
)

var ErrNotConnected = errors.New("client: not connected")

type Options struct {
	Addr     string
	Token    string
	DeviceID string

	// Cursor 是已处理到的 user_seq（真实客户端持久化到本地库）。
	Cursor int64

	OnPush       func(*protocol.Push)       // 已去重、可展示的消息
	OnReadNotify func(*protocol.ReadNotify) // 已读回执
	OnState      func(connected bool)
	Logf         func(format string, args ...any)

	BackoffBase time.Duration // 默认 500ms
	BackoffMax  time.Duration // 默认 30s
}

type Client struct {
	opt Options
	seq atomic.Uint32

	mu      sync.Mutex
	conn    net.Conn
	pending map[uint32]chan *protocol.Frame
	wmu     sync.Mutex

	// 收件箱游标状态
	cmu     sync.Mutex
	cursor  int64
	ahead   map[int64]struct{} // 已收到但领先于游标的 user_seq（出现空洞时暂存）
	seen    map[int64]struct{} // 已交付的 msg_id（去重）
	syncing atomic.Bool

	attempt   int
	kickDelay time.Duration
}

func New(opt Options) *Client {
	if opt.BackoffBase <= 0 {
		opt.BackoffBase = 500 * time.Millisecond
	}
	if opt.BackoffMax <= 0 {
		opt.BackoffMax = 30 * time.Second
	}
	if opt.Logf == nil {
		opt.Logf = func(string, ...any) {}
	}
	return &Client{opt: opt, cursor: opt.Cursor, pending: map[uint32]chan *protocol.Frame{},
		ahead: map[int64]struct{}{}, seen: map[int64]struct{}{}}
}

// Connected 报告当前是否已连上并完成认证。
func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// WaitConnected 阻塞直到连上或超时。
func (c *Client) WaitConnected(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.Connected() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// Cursor 返回当前已连续处理到的 user_seq。
func (c *Client) Cursor() int64 {
	c.cmu.Lock()
	defer c.cmu.Unlock()
	return c.cursor
}

// Run 维持连接直到 ctx 取消：断了就重连。
func (c *Client) Run(ctx context.Context) {
	for ctx.Err() == nil {
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		delay := c.nextDelay()
		c.opt.Logf("disconnected: %v, reconnect in %v", err, delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// nextDelay：指数退避 + 抖动，d = min(max, base*2^n)，实际等待 d/2 + rand(d/2)。
// 抖动避免网关恢复时大量客户端同时重连（惊群）；服务端下发 retry_after 时优先遵循。
func (c *Client) nextDelay() time.Duration {
	if c.kickDelay > 0 {
		d := c.kickDelay
		c.kickDelay = 0
		return d
	}
	d := c.opt.BackoffBase << min(c.attempt, 10)
	if d > c.opt.BackoffMax || d <= 0 {
		d = c.opt.BackoffMax
	}
	c.attempt++
	return d/2 + time.Duration(mrand.Int63n(int64(d/2)+1))
}

func (c *Client) session(ctx context.Context) error {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	nc, err := dialer.DialContext(ctx, "tcp", c.opt.Addr)
	if err != nil {
		return err
	}
	defer nc.Close()
	br := bufio.NewReader(nc)

	// 1. 认证
	if err := protocol.WriteFrame(nc, &protocol.Frame{Cmd: protocol.CmdAuthReq, Seq: c.seq.Add(1),
		Body: protocol.Marshal(&protocol.AuthReq{Token: c.opt.Token, DeviceID: c.opt.DeviceID, Platform: "go"})}); err != nil {
		return err
	}
	_ = nc.SetReadDeadline(time.Now().Add(5 * time.Second))
	f, err := protocol.ReadFrame(br, protocol.DefaultMaxBody)
	if err != nil {
		return err
	}
	var resp protocol.AuthResp
	if f.Cmd != protocol.CmdAuthResp || protocol.Unmarshal(f.Body, &resp) != nil {
		return errors.New("bad auth response")
	}
	if resp.Code != protocol.CodeOK {
		return fmt.Errorf("auth failed: %d %s", resp.Code, resp.Msg)
	}
	c.attempt = 0
	hb := time.Duration(resp.HeartbeatSec) * time.Second
	if hb <= 0 {
		hb = 30 * time.Second
	}

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-sctx.Done(); _ = nc.Close() }() // 让阻塞的 Read 立刻返回

	c.mu.Lock()
	c.conn = nc
	c.mu.Unlock()
	if c.opt.OnState != nil {
		c.opt.OnState(true)
	}
	defer func() {
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
		if c.opt.OnState != nil {
			c.opt.OnState(false)
		}
	}()

	// 2. 心跳
	go func() {
		t := time.NewTicker(hb)
		defer t.Stop()
		for {
			select {
			case <-sctx.Done():
				return
			case <-t.C:
				if c.write(&protocol.Frame{Cmd: protocol.CmdHeartbeat, Seq: c.seq.Add(1)}) != nil {
					return
				}
			}
		}
	}()

	// 3. 连上后立刻从游标处补消息（离线期间错过的）。
	//    必须放协程里：请求要等读循环收到响应，而读循环就在下面。
	go c.syncUntilDone(sctx)

	// 4. 读循环：读超时 = 2.5 个心跳周期，超时说明连接假死，主动重连。
	for {
		_ = nc.SetReadDeadline(time.Now().Add(hb * 5 / 2))
		f, err := protocol.ReadFrame(br, protocol.DefaultMaxBody)
		if err != nil {
			return err
		}
		switch f.Cmd {
		case protocol.CmdHeartbeatAck:
		case protocol.CmdSendAck, protocol.CmdSyncResp, protocol.CmdReadAck, protocol.CmdErr:
			c.mu.Lock()
			ch := c.pending[f.Seq]
			c.mu.Unlock()
			if ch != nil {
				select {
				case ch <- f:
				default:
				}
			}
		case protocol.CmdPush:
			c.onPush(f)
		case protocol.CmdReadNotify:
			var n protocol.ReadNotify
			if protocol.Unmarshal(f.Body, &n) == nil && c.opt.OnReadNotify != nil {
				c.opt.OnReadNotify(&n)
			}
		case protocol.CmdKick:
			var k protocol.Kick
			_ = protocol.Unmarshal(f.Body, &k)
			c.kickDelay = time.Duration(k.RetryAfterMs) * time.Millisecond
			return fmt.Errorf("kicked: %s", k.Reason)
		}
	}
}

// ---------------------------------------------------------------- 收件箱游标逻辑

func (c *Client) onPush(f *protocol.Frame) {
	var p protocol.Push
	if protocol.Unmarshal(f.Body, &p) != nil {
		return
	}
	// 无论是否重复都要 ACK，否则服务端会一直重传。
	_ = c.write(&protocol.Frame{Cmd: protocol.CmdPushAck, Seq: f.Seq, Body: protocol.Marshal(&protocol.PushAck{MsgID: p.MsgID})})

	needSync := c.deliver(&p)
	if needSync && c.syncing.CompareAndSwap(false, true) {
		go func() {
			defer c.syncing.Store(false)
			c.syncUntilDone(context.Background())
		}()
	}
}

// deliver 处理一条消息：去重 → 交付 → 推进游标。返回 true 表示出现空洞，需要 sync。
func (c *Client) deliver(p *protocol.Push) (needSync bool) {
	c.cmu.Lock()
	if p.UserSeq > 0 && p.UserSeq <= c.cursor { // 游标之前的：一定是重复
		c.cmu.Unlock()
		return false
	}
	_, dup := c.seen[p.MsgID]
	c.seen[p.MsgID] = struct{}{}
	if len(c.seen) > 20000 { // 简单限界
		c.seen = map[int64]struct{}{p.MsgID: {}}
	}
	if p.UserSeq > 0 {
		if p.UserSeq == c.cursor+1 {
			c.cursor++
			for { // 吸收之前暂存的、现在连上了的
				if _, ok := c.ahead[c.cursor+1]; !ok {
					break
				}
				delete(c.ahead, c.cursor+1)
				c.cursor++
			}
		} else {
			c.ahead[p.UserSeq] = struct{}{}
			needSync = true // 中间缺了一段：去服务端补
		}
	}
	c.cmu.Unlock()
	if !dup && c.opt.OnPush != nil {
		c.opt.OnPush(p)
	}
	return needSync
}

// syncUntilDone 循环拉取直到追平服务端。每批按 user_seq 顺序交付。
func (c *Client) syncUntilDone(ctx context.Context) {
	for ctx.Err() == nil {
		body := protocol.Marshal(&protocol.SyncReq{Since: c.Cursor(), Limit: 100})
		f, err := c.request(ctx, protocol.CmdSyncReq, body, 5*time.Second)
		if err != nil || f.Cmd != protocol.CmdSyncResp {
			return
		}
		var r protocol.SyncResp
		if protocol.Unmarshal(f.Body, &r) != nil {
			return
		}
		for i := range r.Msgs {
			c.deliver(&r.Msgs[i])
		}
		if !r.HasMore || len(r.Msgs) == 0 {
			return
		}
	}
}

// ---------------------------------------------------------------- 发送

func (c *Client) write(f *protocol.Frame) error {
	c.mu.Lock()
	nc := c.conn
	c.mu.Unlock()
	if nc == nil {
		return ErrNotConnected
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = nc.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return protocol.WriteFrame(nc, f)
}

// request 发送一帧并等待同 seq 的响应帧。
func (c *Client) request(ctx context.Context, cmd uint16, body []byte, timeout time.Duration) (*protocol.Frame, error) {
	seq := c.seq.Add(1)
	ch := make(chan *protocol.Frame, 1)
	c.mu.Lock()
	c.pending[seq] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, seq)
		c.mu.Unlock()
	}()
	if err := c.write(&protocol.Frame{Cmd: cmd, Seq: seq, Body: body}); err != nil {
		return nil, err
	}
	select {
	case f := <-ch:
		return f, nil
	case <-time.After(timeout):
		return nil, errors.New("timeout waiting response")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Send 发送消息：超时未收到 ServerAck 就带着【同一个 client_msg_id】重发。
// 服务端以 (uid, client_msg_id) 去重，因此重发不会产生重复消息。
func (c *Client) Send(ctx context.Context, req protocol.SendReq) (*protocol.SendAck, error) {
	if req.ClientMsgID == "" {
		req.ClientMsgID = newID()
	}
	if req.MsgType == 0 {
		req.MsgType = 1
	}
	body := protocol.Marshal(&req)
	var lastErr error
	for i := 0; i < 3; i++ {
		f, err := c.request(ctx, protocol.CmdSendReq, body, 3*time.Second)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if f.Cmd == protocol.CmdErr {
			lastErr = errors.New("server error")
			time.Sleep(300 * time.Millisecond)
			continue
		}
		var ack protocol.SendAck
		if err := protocol.Unmarshal(f.Body, &ack); err != nil {
			return nil, err
		}
		switch ack.Code {
		case protocol.CodeOK:
			return &ack, nil
		case protocol.CodeRateLimited, protocol.CodeInternal: // 可重试
			lastErr = fmt.Errorf("server code %d", ack.Code)
			time.Sleep(300 * time.Millisecond)
		default: // 业务错误（不是好友、不在群里……）：重试没有意义
			return &ack, fmt.Errorf("send rejected: code %d: %s", ack.Code, ack.Msg)
		}
	}
	return nil, lastErr
}

func (c *Client) SendText(ctx context.Context, toUID int64, text string) (*protocol.SendAck, error) {
	return c.Send(ctx, protocol.SendReq{ToUID: toUID, Content: text})
}

func (c *Client) SendGroupText(ctx context.Context, groupID int64, text string) (*protocol.SendAck, error) {
	return c.Send(ctx, protocol.SendReq{GroupID: groupID, Content: text})
}

// MarkRead 上报会话已读位置。
func (c *Client) MarkRead(ctx context.Context, conv string, seq int64) error {
	f, err := c.request(ctx, protocol.CmdReadReq, protocol.Marshal(&protocol.ReadReq{ConvID: conv, Seq: seq}), 3*time.Second)
	if err != nil {
		return err
	}
	var ack protocol.ReadAck
	if f.Cmd != protocol.CmdReadAck || protocol.Unmarshal(f.Body, &ack) != nil || ack.Code != 0 {
		return errors.New("mark read failed")
	}
	return nil
}

func newID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
