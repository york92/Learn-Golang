package gateway

import (
	"context"
	"time"

	"example.com/im/internal/protocol"
)

// Session 是已认证连接的身份。
type Session struct {
	UID      int64
	DeviceID string
}

// Backend 是网关依赖的"逻辑层"抽象。
// 网关只做连接与转发；真正的业务（鉴权、落库、生成 seq、路由）都在 Backend 里。
// 生产中它是 gRPC / HTTP 客户端；这里提供内存实现用于演示和测试。
type Backend interface {
	// Authenticate 校验 token，返回 uid。
	Authenticate(ctx context.Context, token string) (uid int64, err error)
	// HandleSend 处理上行消息：幂等 → 落库 → 分配 seq → 投递，返回 ServerAck。
	HandleSend(ctx context.Context, s Session, req *protocol.SendReq) (*protocol.SendAck, error)
	// HandleSync 增量同步：返回该用户收件箱中 user_seq > req.Since 的消息。
	HandleSync(ctx context.Context, s Session, req *protocol.SyncReq) (*protocol.SyncResp, error)
	// HandleRead 上报某会话已读位置。
	HandleRead(ctx context.Context, s Session, req *protocol.ReadReq) (*protocol.ReadAck, error)
	// OnPushAck 客户端确认收到某条推送，逻辑层据此推进"已投递"状态。
	OnPushAck(ctx context.Context, s Session, msgID int64)
}

// RouteStore 是全局路由表：uid+device -> 所在网关。生产用 Redis（HSET + EXPIRE）。
type RouteStore interface {
	Bind(ctx context.Context, s Session, gatewayID string, connID uint64, ttl time.Duration) error
	Unbind(ctx context.Context, s Session, gatewayID string, connID uint64) error
}
