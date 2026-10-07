package gateway

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"example.com/im/internal/protocol"
)

// ---- 内存版 RouteStore（生产换 Redis：HSET route:{uid} {device} {gw|connID} + EXPIRE）

type routeKey struct {
	uid    int64
	device string
}
type routeVal struct {
	gw     string
	connID uint64
	expire time.Time
}

type MemRouteStore struct {
	mu sync.Mutex
	m  map[routeKey]routeVal
}

func NewMemRouteStore() *MemRouteStore { return &MemRouteStore{m: map[routeKey]routeVal{}} }

func (r *MemRouteStore) Bind(_ context.Context, s Session, gw string, connID uint64, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[routeKey{s.UID, s.DeviceID}] = routeVal{gw, connID, time.Now().Add(ttl)}
	return nil
}

// Unbind 只在 connID 匹配时删除（compare-and-delete；Redis 里用 Lua 脚本实现）。
func (r *MemRouteStore) Unbind(_ context.Context, s Session, _ string, connID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := routeKey{s.UID, s.DeviceID}
	if v, ok := r.m[k]; ok && v.connID == connID {
		delete(r.m, k)
	}
	return nil
}

func (r *MemRouteStore) Lookup(uid int64, device string) (gw string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.m[routeKey{uid, device}]
	if !ok || time.Now().After(v.expire) {
		return "", false
	}
	return v.gw, true
}

// ---- 内存版 Backend：演示"逻辑层"该做的几件事

// Pusher 由 *Server 实现；逻辑层通过它把消息送到连接上。
type Pusher interface {
	PushToUser(uid int64, exceptDevice string, body []byte) int
}

type MemBackend struct {
	Pusher Pusher

	mu    sync.Mutex
	seqs  map[string]int64             // convID -> 当前 seq
	dedup map[string]*protocol.SendAck // "uid:clientMsgID" -> 已处理结果（幂等）
	msgID atomic.Int64

	userSeq map[int64]int64 // 每用户收件箱序号

	Delivered atomic.Int64 // 收到的 PushAck 数，测试用
}

func NewMemBackend() *MemBackend {
	return &MemBackend{seqs: map[string]int64{}, userSeq: map[int64]int64{}, dedup: map[string]*protocol.SendAck{}}
}

// Authenticate：演示用 token 格式 "token-<uid>"。生产应校验 JWT / 查会话。
func (b *MemBackend) Authenticate(_ context.Context, token string) (int64, error) {
	s, ok := strings.CutPrefix(token, "token-")
	if !ok {
		return 0, errors.New("bad token")
	}
	return strconv.ParseInt(s, 10, 64)
}

func (b *MemBackend) HandleSend(_ context.Context, s Session, req *protocol.SendReq) (*protocol.SendAck, error) {
	key := fmt.Sprintf("%d:%s", s.UID, req.ClientMsgID)

	b.mu.Lock()
	// ① 幂等：客户端超时重发同一条消息，直接返回上次结果，不重复落库/投递。
	if ack, ok := b.dedup[key]; ok {
		b.mu.Unlock()
		return ack, nil
	}
	// ② 会话内递增 seq（生产：Redis INCR / 号段服务）。
	conv := convID(s.UID, req.ToUID)
	b.seqs[conv]++
	seq := b.seqs[conv]
	msgID := b.msgID.Add(1)
	now := time.Now().UnixMilli()
	ack := &protocol.SendAck{ClientMsgID: req.ClientMsgID, MsgID: msgID, Seq: seq, ServerTime: now}
	b.dedup[key] = ack
	b.userSeq[req.ToUID]++
	toSeq := b.userSeq[req.ToUID]
	b.userSeq[s.UID]++
	fromSeq := b.userSeq[s.UID]
	b.mu.Unlock()

	// ③ 持久化：先落库（收件箱 Timeline），再 ACK、再推送。
	//    顺序不能反：否则推送成功但落库失败，消息就"凭空出现又消失"。
	//    ……此处省略 DB 写入……

	// ④ 推送：接收方所有设备 + 发送方的其他设备（多端同步己发消息）。
	mk := func(us int64) []byte {
		return protocol.Marshal(&protocol.Push{
			MsgID: msgID, UserSeq: us, Seq: seq, ConvID: conv, FromUID: s.UID,
			MsgType: req.MsgType, Content: req.Content, SendTime: now,
		})
	}
	if b.Pusher != nil {
		b.Pusher.PushToUser(req.ToUID, "", mk(toSeq))
		b.Pusher.PushToUser(s.UID, s.DeviceID, mk(fromSeq))
	}
	// 接收方不在线：什么都不用做，消息已在收件箱；另走离线 Push 通道通知即可。
	return ack, nil
}

func (b *MemBackend) HandleSync(_ context.Context, _ Session, _ *protocol.SyncReq) (*protocol.SyncResp, error) {
	return &protocol.SyncResp{}, nil // 内存演示后端不保存历史
}

func (b *MemBackend) HandleRead(_ context.Context, _ Session, r *protocol.ReadReq) (*protocol.ReadAck, error) {
	return &protocol.ReadAck{ConvID: r.ConvID, Seq: r.Seq}, nil
}

func (b *MemBackend) OnPushAck(_ context.Context, _ Session, _ int64) { b.Delivered.Add(1) }

func convID(a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("s_%d_%d", a, b)
}
