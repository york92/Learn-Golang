package gateway

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 最小可用的 WebSocket（RFC 6455）服务端实现，仅依赖标准库。
//
// 思路：握手后把 WebSocket 包装成一个 net.Conn，对上层表现为"字节流"：
//   - 读：把收到的每个 binary/text 消息的 payload 拼成字节流
//   - 写：每次 Write 发送一个 binary 消息
//
// 这样 Conn / 协议帧 / 心跳 / 限流等全部逻辑与 TCP 接入完全复用。
// 浏览器端每个 WS 消息里放一个（或多个）完整协议帧即可。

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const (
	opCont   = 0x0
	opText   = 0x1
	opBinary = 0x2
	opClose  = 0x8
	opPing   = 0x9
	opPong   = 0xA
)

// HandleWS 是 /ws 的 http.Handler：完成升级握手后阻塞，直到连接结束。
// allowAnyOrigin=false 时只允许与 Host 同源的浏览器页面连接（防跨站劫持）。
func (s *Server) HandleWS(allowAnyOrigin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
			!strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") ||
			r.Header.Get("Sec-WebSocket-Version") != "13" {
			http.Error(w, "websocket upgrade required", http.StatusBadRequest)
			return
		}
		key := r.Header.Get("Sec-WebSocket-Key")
		if key == "" {
			http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && !allowAnyOrigin {
			if u, err := url.Parse(o); err != nil || !strings.EqualFold(u.Host, r.Host) {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		nc, brw, err := hj.Hijack()
		if err != nil {
			return
		}
		sum := sha1.Sum([]byte(key + wsGUID))
		resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n"
		_ = nc.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := nc.Write([]byte(resp)); err != nil {
			_ = nc.Close()
			return
		}
		_ = nc.SetDeadline(time.Time{})
		s.ServeConn(&wsConn{Conn: nc, br: brw.Reader, maxMsg: s.cfg.MaxBody * 4})
	}
}

type wsConn struct {
	net.Conn // SetDeadline / RemoteAddr 等直接透传
	br       *bufio.Reader
	maxMsg   int
	rbuf     []byte // 当前消息尚未被读走的部分

	wmu sync.Mutex // 写（数据帧 + pong + close）互斥
}

var errWSProtocol = errors.New("websocket: protocol error")

func (c *wsConn) Read(p []byte) (int, error) {
	for len(c.rbuf) == 0 {
		payload, err := c.readMessage()
		if err != nil {
			return 0, err
		}
		c.rbuf = payload
	}
	n := copy(p, c.rbuf)
	c.rbuf = c.rbuf[n:]
	return n, nil
}

// readMessage 读取一个完整的数据消息（处理分片、ping/pong/close）。
func (c *wsConn) readMessage() ([]byte, error) {
	var msg []byte
	started := false
	for {
		var h [2]byte
		if _, err := io.ReadFull(c.br, h[:]); err != nil {
			return nil, err
		}
		fin := h[0]&0x80 != 0
		if h[0]&0x70 != 0 { // RSV 位：未协商扩展，必须为 0
			return nil, errWSProtocol
		}
		op := h[0] & 0x0f
		if h[1]&0x80 == 0 { // 客户端 -> 服务端必须带掩码
			return nil, errWSProtocol
		}
		n := uint64(h[1] & 0x7f)
		switch n {
		case 126:
			var b [2]byte
			if _, err := io.ReadFull(c.br, b[:]); err != nil {
				return nil, err
			}
			n = uint64(binary.BigEndian.Uint16(b[:]))
		case 127:
			var b [8]byte
			if _, err := io.ReadFull(c.br, b[:]); err != nil {
				return nil, err
			}
			n = binary.BigEndian.Uint64(b[:])
		}
		// 先校验长度再分配，防内存攻击（与 TCP 帧同理）。
		if n > uint64(c.maxMsg) || (op < 0x8 && uint64(len(msg))+n > uint64(c.maxMsg)) {
			return nil, errors.New("websocket: message too large")
		}
		var mask [4]byte
		if _, err := io.ReadFull(c.br, mask[:]); err != nil {
			return nil, err
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(c.br, payload); err != nil {
			return nil, err
		}
		for i := range payload {
			payload[i] ^= mask[i&3]
		}

		switch op {
		case opPing:
			_ = c.writeFrame(opPong, payload)
		case opPong:
		case opClose:
			_ = c.writeFrame(opClose, payload)
			return nil, io.EOF
		case opText, opBinary:
			if started {
				return nil, errWSProtocol
			}
			started, msg = true, payload
			if fin {
				return msg, nil
			}
		case opCont:
			if !started {
				return nil, errWSProtocol
			}
			msg = append(msg, payload...)
			if fin {
				return msg, nil
			}
		default:
			return nil, errWSProtocol
		}
	}
}

func (c *wsConn) Write(p []byte) (int, error) {
	if err := c.writeFrame(opBinary, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *wsConn) writeFrame(op byte, payload []byte) error {
	hdr := make([]byte, 0, 10)
	hdr = append(hdr, 0x80|op)
	switch n := len(payload); {
	case n < 126:
		hdr = append(hdr, byte(n))
	case n <= 0xffff:
		hdr = append(hdr, 126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err := (&netBuffers{hdr, payload}).WriteTo(c.Conn)
	return err
}

type netBuffers [2][]byte

func (b *netBuffers) WriteTo(w io.Writer) (int64, error) {
	nb := net.Buffers{b[0], b[1]}
	return nb.WriteTo(w)
}

// Close 尽力发一个 close 帧再关底层连接。
// 用 TryLock：若写协程正卡在写上，就不等了，直接关底层连接把它踢醒。
func (c *wsConn) Close() error {
	if c.wmu.TryLock() {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		_, _ = c.Conn.Write([]byte{0x80 | opClose, 0})
		c.wmu.Unlock()
	}
	return c.Conn.Close()
}
