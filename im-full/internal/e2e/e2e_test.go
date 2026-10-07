// 全栈集成测试：真实的 TCP 网关 + HTTP API + WebSocket + 持久化存储。
package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/im/internal/api"
	"example.com/im/internal/client"
	"example.com/im/internal/gateway"
	"example.com/im/internal/protocol"
	"example.com/im/internal/service"
	"example.com/im/internal/store"
)

// ---------------------------------------------------------------- 测试环境

type env struct {
	t    *testing.T
	st   *store.Store
	svc  *service.Service
	gw   *gateway.Server
	hs   *httptest.Server
	tcp  string
	ctx  context.Context
	stop context.CancelFunc
}

func newEnv(t *testing.T, dir string, requireFriend bool) *env {
	t.Helper()
	st, err := store.Open(dir, store.Options{FsyncInterval: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(st, service.Config{AllowRegister: true, RequireFriend: requireFriend})
	gw := gateway.NewServer(gateway.Config{GatewayID: "t", PushRetryInterval: 300 * time.Millisecond}, svc, gateway.NewMemRouteStore())
	gw.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.SetPusher(gw)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go gw.Serve(ln)
	hs := httptest.NewServer(api.New(svc, gw, false, 0, slog.New(slog.NewTextHandler(io.Discard, nil))))
	ctx, stop := context.WithCancel(context.Background())
	e := &env{t: t, st: st, svc: svc, gw: gw, hs: hs, tcp: ln.Addr().String(), ctx: ctx, stop: stop}
	t.Cleanup(e.close)
	return e
}

func (e *env) close() {
	e.stop()
	e.hs.Close()
	sctx, c := context.WithTimeout(context.Background(), 2*time.Second)
	defer c()
	_ = e.gw.Shutdown(sctx)
	_ = e.st.Close()
}

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// call 发起 HTTP 请求，返回状态码与解析后的 data。
func (e *env) call(method, path, token string, in any, out any) (int, envelope) {
	e.t.Helper()
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.hs.URL+path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var env envelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if out != nil && env.Code == 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			e.t.Fatalf("decode %s: %v", path, err)
		}
	}
	return resp.StatusCode, env
}

type user struct {
	Name  string
	UID   int64
	Token string
}

// newUser 通过 HTTP 注册 + 登录。
func (e *env) newUser(name string) user {
	e.t.Helper()
	if code, env := e.call("POST", "/api/register", "", map[string]any{"username": name, "password": "secret1"}, nil); code != 200 {
		e.t.Fatalf("register %s: %d %+v", name, code, env)
	}
	var lr struct {
		User struct {
			UID int64 `json:"uid"`
		} `json:"user"`
		Token string `json:"token"`
	}
	if code, env := e.call("POST", "/api/login", "", map[string]any{"username": name, "password": "secret1"}, &lr); code != 200 {
		e.t.Fatalf("login %s: %d %+v", name, code, env)
	}
	return user{Name: name, UID: lr.User.UID, Token: lr.Token}
}

type peer struct {
	*client.Client
	pushes chan *protocol.Push
	reads  chan *protocol.ReadNotify
}

// connect 用 TCP 长连接登录。cursor 是客户端本地已处理到的 user_seq。
func (e *env) connect(u user, device string, cursor int64) *peer {
	e.t.Helper()
	p := &peer{pushes: make(chan *protocol.Push, 100), reads: make(chan *protocol.ReadNotify, 100)}
	p.Client = client.New(client.Options{
		Addr: e.tcp, Token: u.Token, DeviceID: device, Cursor: cursor,
		OnPush:       func(m *protocol.Push) { p.pushes <- m },
		OnReadNotify: func(n *protocol.ReadNotify) { p.reads <- n },
	})
	go p.Run(e.ctx)
	if !p.WaitConnected(3 * time.Second) {
		e.t.Fatalf("%s failed to connect", u.Name)
	}
	return p
}

func (p *peer) next(t *testing.T) *protocol.Push {
	t.Helper()
	select {
	case m := <-p.pushes:
		return m
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for push")
		return nil
	}
}

func (p *peer) expectNone(t *testing.T) {
	t.Helper()
	select {
	case m := <-p.pushes:
		t.Fatalf("unexpected push: %+v", m)
	case <-time.After(400 * time.Millisecond):
	}
}

func send(t *testing.T, c *client.Client, to int64, text string) *protocol.SendAck {
	t.Helper()
	ack, err := c.SendText(context.Background(), to, text)
	if err != nil {
		t.Fatalf("send %q: %v", text, err)
	}
	return ack
}

// ---------------------------------------------------------------- 测试

// 一个环境里串行跑完整业务流程（注册 → 好友 → 单聊 → 离线同步 → 已读 → 群聊）。
func TestFullFlow(t *testing.T) {
	e := newEnv(t, t.TempDir(), false)
	alice, bob, carol := e.newUser("alice"), e.newUser("bob"), e.newUser("carol")

	t.Run("auth", func(t *testing.T) {
		if code, _ := e.call("GET", "/api/me", "", nil, nil); code != 401 {
			t.Fatalf("无 token 应 401, got %d", code)
		}
		if code, _ := e.call("POST", "/api/login", "", map[string]any{"username": "alice", "password": "bad"}, nil); code != 401 {
			t.Fatalf("错误密码应 401, got %d", code)
		}
		if code, _ := e.call("POST", "/api/register", "", map[string]any{"username": "alice", "password": "secret1"}, nil); code != 409 {
			t.Fatalf("重复注册应 409, got %d", code)
		}
		// TCP 用错误 token 认证应失败
		nc, _ := net.Dial("tcp", e.tcp)
		defer nc.Close()
		protocol.WriteFrame(nc, &protocol.Frame{Cmd: protocol.CmdAuthReq, Seq: 1,
			Body: protocol.Marshal(&protocol.AuthReq{Token: "nope", DeviceID: "d"})})
		f, err := protocol.ReadFrame(bufio.NewReader(nc), 1<<16)
		var r protocol.AuthResp
		if err != nil || protocol.Unmarshal(f.Body, &r) != nil || r.Code != protocol.CodeAuthFailed {
			t.Fatalf("bad token should fail auth: %v %+v", err, r)
		}
	})

	t.Run("friends", func(t *testing.T) {
		if code, _ := e.call("POST", "/api/friends", alice.Token, map[string]any{"username": "bob"}, nil); code != 200 {
			t.Fatal("add friend failed")
		}
		var fr []store.User
		e.call("GET", "/api/friends", bob.Token, nil, &fr) // 互为好友
		if len(fr) != 1 || fr[0].Username != "alice" {
			t.Fatalf("friendship should be mutual: %+v", fr)
		}
	})

	ca := e.connect(alice, "alice-phone", 0)
	cb := e.connect(bob, "bob-phone", 0)
	var convAB string

	t.Run("direct message", func(t *testing.T) {
		ack := send(t, ca.Client, bob.UID, "hello bob")
		if ack.Seq != 1 || ack.MsgID == 0 {
			t.Fatalf("ack: %+v", ack)
		}
		convAB = ack.ConvID
		m := cb.next(t)
		if m.Content != "hello bob" || m.FromUID != alice.UID || m.FromName != "alice" || m.UserSeq != 1 || m.Seq != 1 {
			t.Fatalf("bob push: %+v", m)
		}
		echo := ca.next(t) // 发送者自己也会收到回显（多端同步己发消息）
		if echo.ClientMsgID == "" || echo.MsgID != ack.MsgID {
			t.Fatalf("sender echo: %+v", echo)
		}
		if cb.Cursor() != 1 {
			t.Fatalf("cursor should advance to 1, got %d", cb.Cursor())
		}
	})

	t.Run("multi device", func(t *testing.T) {
		bob2 := e.connect(bob, "bob-tablet", 0) // 第二台设备：上线后 sync 补到历史
		if m := bob2.next(t); m.Content != "hello bob" {
			t.Fatalf("second device should sync history: %+v", m)
		}
		send(t, ca.Client, bob.UID, "to both devices")
		if cb.next(t).Content != "to both devices" || bob2.next(t).Content != "to both devices" {
			t.Fatal("both bob devices must receive live push")
		}
		ca.next(t)
	})

	t.Run("offline sync", func(t *testing.T) {
		for i := 1; i <= 3; i++ { // carol 不在线
			send(t, ca.Client, carol.UID, fmt.Sprintf("offline-%d", i))
			ca.next(t)
		}
		cc := e.connect(carol, "carol-phone", 0) // 上线：游标 0 → 增量拉取
		for i := 1; i <= 3; i++ {
			m := cc.next(t)
			if m.Content != fmt.Sprintf("offline-%d", i) || m.UserSeq != int64(i) {
				t.Fatalf("offline msg %d out of order/lost: %+v", i, m)
			}
		}
		if cc.Cursor() != 3 {
			t.Fatalf("cursor=%d", cc.Cursor())
		}
		cc.Cancel()
	})

	t.Run("conversations unread and read receipt", func(t *testing.T) {
		var cv struct {
			Latest int64              `json:"latest_user_seq"`
			Convs  []service.ConvView `json:"conversations"`
		}
		e.call("GET", "/api/conversations", carol.Token, nil, &cv)
		if cv.Latest != 3 || len(cv.Convs) != 1 || cv.Convs[0].Unread != 3 || cv.Convs[0].Title != "alice" {
			t.Fatalf("carol convs: %+v", cv)
		}
		convAC := cv.Convs[0].ConvID
		cc := e.connect(carol, "carol-phone2", 3)
		if err := cc.MarkRead(context.Background(), convAC, 3); err != nil {
			t.Fatal(err)
		}
		select {
		case n := <-ca.reads: // 单聊：对方（alice）收到已读回执
			if n.UID != carol.UID || n.Seq != 3 || n.ConvID != convAC {
				t.Fatalf("read notify: %+v", n)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("alice should get read receipt")
		}
		e.call("GET", "/api/conversations", carol.Token, nil, &cv)
		if cv.Convs[0].Unread != 0 {
			t.Fatalf("unread should be 0 after read: %+v", cv.Convs[0])
		}
		var av struct {
			Convs []service.ConvView `json:"conversations"`
		}
		e.call("GET", "/api/conversations", alice.Token, nil, &av)
		for _, c := range av.Convs {
			if c.ConvID == convAC && c.PeerReadSeq != 3 {
				t.Fatalf("peer_read_seq should be 3: %+v", c)
			}
		}
		cc.Cancel()
	})

	t.Run("history pagination", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			send(t, ca.Client, bob.UID, fmt.Sprintf("p%d", i))
			ca.next(t)
			cb.next(t)
		}
		var h struct{ Messages []protocol.Push }
		e.call("GET", "/api/messages?conv="+convAB+"&limit=3", bob.Token, nil, &h)
		if len(h.Messages) != 3 || h.Messages[2].Content != "p4" {
			t.Fatalf("latest page: %+v", h.Messages)
		}
		first := h.Messages[0].Seq
		e.call("GET", fmt.Sprintf("/api/messages?conv=%s&limit=3&before_seq=%d", convAB, first), bob.Token, nil, &h)
		if len(h.Messages) != 3 || h.Messages[2].Seq != first-1 {
			t.Fatalf("older page: %+v", h.Messages)
		}
		if code, _ := e.call("GET", "/api/messages?conv="+convAB, carol.Token, nil, nil); code != 403 {
			t.Fatalf("非成员读取历史应 403, got %d", code)
		}
	})

	t.Run("idempotent http send", func(t *testing.T) {
		in := map[string]any{"to_username": "bob", "content": "once", "client_msg_id": "dup-1"}
		var a1, a2 protocol.SendAck
		e.call("POST", "/api/messages", alice.Token, in, &a1)
		e.call("POST", "/api/messages", alice.Token, in, &a2)
		if a1.MsgID == 0 || a1.MsgID != a2.MsgID {
			t.Fatalf("重发必须幂等: %+v %+v", a1, a2)
		}
		if cb.next(t).Content != "once" {
			t.Fatal("bob should get it")
		}
		cb.expectNone(t) // 只推送一次
		ca.next(t)
	})

	t.Run("group chat", func(t *testing.T) {
		var gv struct {
			ID int64 `json:"id"`
		}
		if code, env := e.call("POST", "/api/groups", alice.Token,
			map[string]any{"name": "team", "member_usernames": []string{"bob", "carol"}}, &gv); code != 200 {
			t.Fatalf("create group: %+v", env)
		}
		cc := e.connect(carol, "carol-g", e.st.LatestUserSeq(carol.UID))
		ack, err := ca.SendGroupText(context.Background(), gv.ID, "hi team")
		if err != nil {
			t.Fatal(err)
		}
		for name, p := range map[string]*peer{"bob": cb, "carol": cc, "alice": ca} {
			m := p.next(t)
			if m.Content != "hi team" || m.ConvID != ack.ConvID || m.FromName != "alice" {
				t.Fatalf("%s group push: %+v", name, m)
			}
		}
		dave := e.newUser("dave")
		cd := e.connect(dave, "dave", 0)
		if _, err := cd.SendGroupText(context.Background(), gv.ID, "let me in"); err == nil || !strings.Contains(err.Error(), "not a member") {
			t.Fatalf("非群成员发消息必须被拒绝: %v", err)
		}
		// 退群后不再收到
		e.call("POST", fmt.Sprintf("/api/groups/%d/leave", gv.ID), bob.Token, nil, nil)
		ca.SendGroupText(context.Background(), gv.ID, "after bob left")
		cc.next(t)
		ca.next(t)
		cb.expectNone(t)
		cc.Cancel()
	})
}

func (p *peer) Cancel() {} // 连接随环境 ctx 一起关闭；此处仅为可读性占位

func TestRequireFriend(t *testing.T) {
	e := newEnv(t, t.TempDir(), true)
	alice, _ := e.newUser("alice"), e.newUser("bob")
	in := map[string]any{"to_username": "bob", "content": "hi"}
	if code, env := e.call("POST", "/api/messages", alice.Token, in, nil); code != 400 || env.Code != protocol.CodeNotFriend {
		t.Fatalf("非好友应被拒绝: %d %+v", code, env)
	}
	e.call("POST", "/api/friends", alice.Token, map[string]any{"username": "bob"}, nil)
	if code, _ := e.call("POST", "/api/messages", alice.Token, in, nil); code != 200 {
		t.Fatal("加好友后应可发送")
	}
}

// 重启后一切仍在：账号、令牌、消息、会话、序号接续。
func TestPersistenceAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	e1 := newEnv(t, dir, false)
	alice, bob := e1.newUser("alice"), e1.newUser("bob")
	var a1 protocol.SendAck
	e1.call("POST", "/api/messages", alice.Token, map[string]any{"to_username": "bob", "content": "survive"}, &a1)
	e1.close() // 模拟停机

	e2 := newEnv(t, dir, false)
	var cv struct {
		Convs []service.ConvView `json:"conversations"`
	}
	if code, _ := e2.call("GET", "/api/conversations", bob.Token, nil, &cv); code != 200 {
		t.Fatal("重启后旧 token 应仍然有效")
	}
	if len(cv.Convs) != 1 || cv.Convs[0].Last == nil || cv.Convs[0].Last.Content != "survive" || cv.Convs[0].Unread != 1 {
		t.Fatalf("重启后会话丢失: %+v", cv)
	}
	var a2 protocol.SendAck
	e2.call("POST", "/api/messages", alice.Token, map[string]any{"to_username": "bob", "content": "after"}, &a2)
	if a2.Seq != a1.Seq+1 || a2.MsgID != a1.MsgID+1 {
		t.Fatalf("重启后序号必须接续: %+v -> %+v", a1, a2)
	}
}

// 登录限流：成功登录不计数；连续失败才会被限制（防暴力破解，又不误伤正常用户）。
func TestLoginRateLimitOnlyCountsFailures(t *testing.T) {
	e := newEnv(t, t.TempDir(), false)
	e.newUser("alice")
	for i := 0; i < 25; i++ { // 25 次成功登录（超过突发额度 20）不应被限流
		if code, _ := e.call("POST", "/api/login", "", map[string]any{"username": "alice", "password": "secret1"}, nil); code != 200 {
			t.Fatalf("successful login #%d got %d", i, code)
		}
	}
	limited := false
	for i := 0; i < 60 && !limited; i++ { // 连续失败 → 最终 429
		code, _ := e.call("POST", "/api/login", "", map[string]any{"username": "alice", "password": "wrong"}, nil)
		limited = code == 429
	}
	if !limited {
		t.Fatal("连续失败登录应被限流")
	}
	if code, _ := e.call("POST", "/api/login", "", map[string]any{"username": "alice", "password": "secret1"}, nil); code != 429 {
		t.Fatalf("被限流期间正确密码也应暂时被拒, got %d", code)
	}
}

// ---------------------------------------------------------------- WebSocket

type wsClient struct {
	nc net.Conn
	br *bufio.Reader
}

func wsDial(t *testing.T, e *env, origin string) (*wsClient, *http.Response) {
	t.Helper()
	nc, err := net.Dial("tcp", strings.TrimPrefix(e.hs.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nc.Close() })
	h := "GET /ws HTTP/1.1\r\nHost: " + strings.TrimPrefix(e.hs.URL, "http://") + "\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n"
	if origin != "" {
		h += "Origin: " + origin + "\r\n"
	}
	nc.Write([]byte(h + "\r\n"))
	br := bufio.NewReader(nc)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &wsClient{nc: nc, br: br}, resp
}

// writeWS 发送带掩码的 WS 帧（客户端 -> 服务端必须掩码）。
func (w *wsClient) writeWS(fin bool, op byte, payload []byte) {
	b := []byte{op}
	if fin {
		b[0] |= 0x80
	}
	n := len(payload)
	switch {
	case n < 126:
		b = append(b, 0x80|byte(n))
	default:
		b = append(b, 0x80|126, byte(n>>8), byte(n))
	}
	mask := []byte{1, 2, 3, 4}
	b = append(b, mask...)
	for i, c := range payload {
		b = append(b, c^mask[i&3])
	}
	w.nc.Write(b)
}

func (w *wsClient) sendFrame(f *protocol.Frame) { w.writeWS(true, 0x2, f.Marshal()) }

// readProto 读取下一个协议帧（跳过 WS 控制帧）。
func (w *wsClient) readProto(t *testing.T) *protocol.Frame {
	t.Helper()
	w.nc.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var h [2]byte
		if _, err := io.ReadFull(w.br, h[:]); err != nil {
			t.Fatalf("ws read: %v", err)
		}
		op, n := h[0]&0x0f, int(h[1]&0x7f)
		if n == 126 {
			var b [2]byte
			io.ReadFull(w.br, b[:])
			n = int(binary.BigEndian.Uint16(b[:]))
		}
		payload := make([]byte, n)
		io.ReadFull(w.br, payload)
		if op == 0xA { // pong
			continue
		}
		if op != 0x2 {
			t.Fatalf("unexpected ws opcode %d", op)
		}
		f, err := protocol.ReadFrame(bytes.NewReader(payload), protocol.DefaultMaxBody)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
}

func TestWebSocketTransport(t *testing.T) {
	e := newEnv(t, t.TempDir(), false)
	alice, bob := e.newUser("alice"), e.newUser("bob")

	// 握手：Accept 值必须符合 RFC 6455 示例（key=dGhlIHNhbXBsZSBub25jZQ== → s3pPLMBiTxaQ9kYGzzhZRbK+xOo=）
	ws, resp := wsDial(t, e, "")
	if resp.StatusCode != 101 || resp.Header.Get("Sec-WebSocket-Accept") != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("handshake: %d %v", resp.StatusCode, resp.Header)
	}

	// 先发一个 ping（服务端必须回 pong），再把认证帧拆成两个分片发送，覆盖控制帧与分片重组。
	ws.writeWS(true, 0x9, []byte("hi"))
	auth := (&protocol.Frame{Cmd: protocol.CmdAuthReq, Seq: 1,
		Body: protocol.Marshal(&protocol.AuthReq{Token: alice.Token, DeviceID: "web-1"})}).Marshal()
	ws.writeWS(false, 0x2, auth[:7])
	ws.writeWS(true, 0x0, auth[7:])
	f := ws.readProto(t)
	var ar protocol.AuthResp
	protocol.Unmarshal(f.Body, &ar)
	if f.Cmd != protocol.CmdAuthResp || ar.Code != 0 {
		t.Fatalf("ws auth: %+v %+v", f, ar)
	}

	// bob 走 TCP，向 WS 上的 alice 发消息：跨传输通信
	cb := e.connect(bob, "bob-tcp", 0)
	send(t, cb.Client, alice.UID, "from tcp to ws")
	push := ws.readProto(t)
	var p protocol.Push
	protocol.Unmarshal(push.Body, &p)
	if push.Cmd != protocol.CmdPush || p.Content != "from tcp to ws" || p.UserSeq != 1 {
		t.Fatalf("ws push: %+v %+v", push, p)
	}
	// WS 回 ACK，并从 WS 发一条给 TCP 的 bob
	ws.sendFrame(&protocol.Frame{Cmd: protocol.CmdPushAck, Seq: push.Seq, Body: protocol.Marshal(&protocol.PushAck{MsgID: p.MsgID})})
	ws.sendFrame(&protocol.Frame{Cmd: protocol.CmdSendReq, Seq: 5, Body: protocol.Marshal(&protocol.SendReq{
		ClientMsgID: "w1", ToUID: bob.UID, MsgType: 1, Content: "from ws to tcp"})})
	var gotAck bool
	for i := 0; i < 3 && !gotAck; i++ {
		if f := ws.readProto(t); f.Cmd == protocol.CmdSendAck && f.Seq == 5 {
			gotAck = true
		}
	}
	if !gotAck {
		t.Fatal("ws client should get SendAck")
	}
	cb.next(t) // 先是自己发的回显
	if m := cb.next(t); m.Content != "from ws to tcp" {
		t.Fatalf("tcp peer got: %+v", m)
	}

	// 跨站 Origin 必须被拒绝
	_, bad := wsDial(t, e, "http://evil.example.com")
	if bad.StatusCode != 403 {
		t.Fatalf("cross-origin ws should be 403, got %d", bad.StatusCode)
	}
}
