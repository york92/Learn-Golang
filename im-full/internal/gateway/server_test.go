package gateway_test

import (
	"bufio"
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"example.com/im/internal/client"
	"example.com/im/internal/gateway"
	"example.com/im/internal/protocol"
)

func startServer(t *testing.T, cfg gateway.Config) (*gateway.Server, *gateway.MemBackend, *gateway.MemRouteStore, string) {
	t.Helper()
	be := gateway.NewMemBackend()
	rs := gateway.NewMemRouteStore()
	srv := gateway.NewServer(cfg, be, rs)
	be.Pusher = srv
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	})
	return srv, be, rs, ln.Addr().String()
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// 端到端：alice -> bob，收到推送，ServerAck 带 seq，PushAck 回到后端。
func TestEndToEnd(t *testing.T) {
	_, be, _, addr := startServer(t, gateway.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan *protocol.Push, 4)
	bob := client.New(client.Options{Addr: addr, Token: "token-2", DeviceID: "bob-phone",
		OnPush: func(p *protocol.Push) { got <- p }})
	alice := client.New(client.Options{Addr: addr, Token: "token-1", DeviceID: "alice-phone"})
	go bob.Run(ctx)
	go alice.Run(ctx)

	var ack *protocol.SendAck
	waitFor(t, "alice connected & send ok", func() bool {
		var err error
		ack, err = alice.SendText(ctx, 2, "hi bob")
		return err == nil
	})
	if ack.Seq != 1 || ack.MsgID == 0 {
		t.Fatalf("unexpected ack %+v", ack)
	}
	select {
	case p := <-got:
		if p.Content != "hi bob" || p.FromUID != 1 || p.Seq != 1 {
			t.Fatalf("unexpected push %+v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("bob did not receive push")
	}
	waitFor(t, "push ack reaches backend", func() bool { return be.Delivered.Load() == 1 })
}

// 用原始 TCP 手写客户端，验证：不 ACK 就重传；ACK 之后停止重传。
func TestPushRetransmitUntilAck(t *testing.T) {
	cfg := gateway.Config{PushRetryInterval: 100 * time.Millisecond, PushMaxRetry: 5}
	srv, _, _, addr := startServer(t, cfg)

	nc, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	br := bufio.NewReader(nc)
	protocol.WriteFrame(nc, &protocol.Frame{Cmd: protocol.CmdAuthReq, Seq: 1,
		Body: protocol.Marshal(&protocol.AuthReq{Token: "token-9", DeviceID: "d1"})})
	if f, err := protocol.ReadFrame(br, 1<<16); err != nil || f.Cmd != protocol.CmdAuthResp {
		t.Fatalf("auth: %v %+v", err, f)
	}

	srv.PushToUser(9, "", protocol.Marshal(&protocol.Push{MsgID: 77, Content: "x"}))

	first, err := protocol.ReadFrame(br, 1<<16)
	if err != nil || first.Cmd != protocol.CmdPush {
		t.Fatalf("first push: %v", err)
	}
	// 故意不 ACK，应当收到同一 seq 的重传
	_ = nc.SetReadDeadline(time.Now().Add(time.Second))
	again, err := protocol.ReadFrame(br, 1<<16)
	if err != nil || again.Cmd != protocol.CmdPush || again.Seq != first.Seq {
		t.Fatalf("expected retransmit, got %v %+v", err, again)
	}
	// ACK 之后不应再有重传
	protocol.WriteFrame(nc, &protocol.Frame{Cmd: protocol.CmdPushAck, Seq: first.Seq,
		Body: protocol.Marshal(&protocol.PushAck{MsgID: 77})})
	time.Sleep(50 * time.Millisecond)
	// 排空 ACK 前可能已在途的重传
	_ = nc.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	for {
		if _, err := protocol.ReadFrame(br, 1<<16); err != nil {
			break
		}
	}
	_ = nc.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	if f, err := protocol.ReadFrame(br, 1<<16); err == nil {
		t.Fatalf("unexpected frame after ack: %+v", f)
	}
}

// 幂等：同一个 client_msg_id 重发，得到相同 msg_id/seq，对端只收到一次推送。
func TestSendIdempotent(t *testing.T) {
	_, _, _, addr := startServer(t, gateway.Config{})
	pushes := make(chan *protocol.Push, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bob := client.New(client.Options{Addr: addr, Token: "token-2", DeviceID: "b",
		OnPush: func(p *protocol.Push) { pushes <- p }})
	go bob.Run(ctx)

	nc, _ := net.Dial("tcp", addr)
	defer nc.Close()
	br := bufio.NewReader(nc)
	protocol.WriteFrame(nc, &protocol.Frame{Cmd: protocol.CmdAuthReq, Seq: 1,
		Body: protocol.Marshal(&protocol.AuthReq{Token: "token-1", DeviceID: "a"})})
	protocol.ReadFrame(br, 1<<16)

	send := func(seq uint32) protocol.SendAck {
		protocol.WriteFrame(nc, &protocol.Frame{Cmd: protocol.CmdSendReq, Seq: seq,
			Body: protocol.Marshal(&protocol.SendReq{ClientMsgID: "same-id", ToUID: 2, MsgType: 1, Content: "once"})})
		f, err := protocol.ReadFrame(br, 1<<16)
		if err != nil || f.Cmd != protocol.CmdSendAck {
			t.Fatalf("ack: %v %+v", err, f)
		}
		var a protocol.SendAck
		protocol.Unmarshal(f.Body, &a)
		return a
	}
	waitFor(t, "bob online", func() bool { return len(pushes) == 0 })
	time.Sleep(100 * time.Millisecond)
	a1, a2 := send(2), send(3)
	if a1.MsgID != a2.MsgID || a1.Seq != a2.Seq {
		t.Fatalf("not idempotent: %+v vs %+v", a1, a2)
	}
	time.Sleep(300 * time.Millisecond)
	if len(pushes) != 1 {
		t.Fatalf("want exactly 1 push, got %d", len(pushes))
	}
}

// 空闲超时：认证后不发心跳，应被服务端断开。
func TestIdleTimeout(t *testing.T) {
	_, _, _, addr := startServer(t, gateway.Config{HeartbeatInterval: 20 * time.Millisecond, IdleTimeout: 150 * time.Millisecond})
	nc, _ := net.Dial("tcp", addr)
	defer nc.Close()
	br := bufio.NewReader(nc)
	protocol.WriteFrame(nc, &protocol.Frame{Cmd: protocol.CmdAuthReq, Seq: 1,
		Body: protocol.Marshal(&protocol.AuthReq{Token: "token-5", DeviceID: "d"})})
	protocol.ReadFrame(br, 1<<16)
	_ = nc.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := protocol.ReadFrame(br, 1<<16); err == nil {
		t.Fatal("expected connection to be closed by idle timeout")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("server did not close idle connection in time")
	}
}

// 认证超时 + 认证失败。
func TestAuthTimeoutAndFailure(t *testing.T) {
	srv, _, _, addr := startServer(t, gateway.Config{AuthTimeout: 150 * time.Millisecond})

	silent, _ := net.Dial("tcp", addr) // 只建连不说话
	defer silent.Close()
	_ = silent.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := bufio.NewReader(silent).ReadByte(); err == nil {
		t.Fatal("expected close")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("silent connection was not dropped by auth timeout")
	}

	bad, _ := net.Dial("tcp", addr)
	defer bad.Close()
	protocol.WriteFrame(bad, &protocol.Frame{Cmd: protocol.CmdAuthReq, Seq: 1,
		Body: protocol.Marshal(&protocol.AuthReq{Token: "garbage", DeviceID: "d"})})
	f, err := protocol.ReadFrame(bufio.NewReader(bad), 1<<16)
	if err != nil {
		t.Fatal(err)
	}
	var r protocol.AuthResp
	protocol.Unmarshal(f.Body, &r)
	if r.Code != protocol.CodeAuthFailed {
		t.Fatalf("want auth failed, got %+v", r)
	}
	if srv.OnlineCount() != 0 {
		t.Fatal("unauthenticated connection must not be registered")
	}
}

// 同账号同设备重复登录：旧连接被踢，新连接保留，路由指向新连接。
func TestKickOldDevice(t *testing.T) {
	srv, _, rs, addr := startServer(t, gateway.Config{GatewayID: "gw-1"})
	login := func() (net.Conn, *bufio.Reader) {
		nc, _ := net.Dial("tcp", addr)
		br := bufio.NewReader(nc)
		protocol.WriteFrame(nc, &protocol.Frame{Cmd: protocol.CmdAuthReq, Seq: 1,
			Body: protocol.Marshal(&protocol.AuthReq{Token: "token-3", DeviceID: "phone"})})
		protocol.ReadFrame(br, 1<<16)
		return nc, br
	}
	old, oldBr := login()
	defer old.Close()
	nw, _ := login()
	defer nw.Close()

	f, err := protocol.ReadFrame(oldBr, 1<<16)
	if err != nil || f.Cmd != protocol.CmdKick {
		t.Fatalf("old conn should receive Kick, got %v %+v", err, f)
	}
	waitFor(t, "old conn cleaned, new conn kept", func() bool { return srv.OnlineCount() == 1 })
	time.Sleep(50 * time.Millisecond)
	if gw, ok := rs.Lookup(3, "phone"); !ok || gw != "gw-1" {
		t.Fatalf("route must survive old conn cleanup, got %q %v", gw, ok)
	}
}

// 并发压力（配合 -race）：多个发送者并发给一个接收者发消息，全部送达且无重复。
func TestConcurrentFanIn(t *testing.T) {
	_, _, _, addr := startServer(t, gateway.Config{RatePerSec: 1000, Burst: 1000})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const senders, perSender = 8, 20
	var mu sync.Mutex
	recv := map[int64]bool{}
	bob := client.New(client.Options{Addr: addr, Token: "token-100", DeviceID: "b",
		OnPush: func(p *protocol.Push) { mu.Lock(); recv[p.MsgID] = true; mu.Unlock() }})
	go bob.Run(ctx)

	var wg sync.WaitGroup
	for i := 1; i <= senders; i++ {
		c := client.New(client.Options{Addr: addr, Token: "token-" + string(rune('0'+i)), DeviceID: "d"})
		go c.Run(ctx)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < perSender; n++ {
				for {
					if _, err := c.SendText(ctx, 100, "m"); err == nil {
						break
					}
					time.Sleep(20 * time.Millisecond)
				}
			}
		}()
	}
	wg.Wait()
	waitFor(t, "all pushes received", func() bool { mu.Lock(); defer mu.Unlock(); return len(recv) == senders*perSender })
}
