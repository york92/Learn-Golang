package gateway

import (
	"time"

	"example.com/im/internal/protocol"
)

type Config struct {
	Addr      string // 监听地址
	GatewayID string // 本网关对外标识（写进路由表，供逻辑层定位）

	MaxBody     int           // 单帧 body 上限
	MaxConns    int64         // 单机连接数上限（含未认证）
	AuthTimeout time.Duration // 建连后必须在此时间内完成认证，防"占坑"攻击

	HeartbeatInterval time.Duration // 告诉客户端的心跳间隔
	IdleTimeout       time.Duration // 超过此时间没收到任何数据就断开（通常 2.5~3 倍心跳）
	WriteTimeout      time.Duration // 单次写超时，防慢消费者拖住写协程
	SendQueueSize     int           // 每连接发送队列长度，满了视为慢消费者，直接踢

	RatePerSec float64 // 每连接上行限速（令牌桶）
	Burst      int

	PushRetryInterval time.Duration // 推送未收到 ACK 的重传间隔
	PushMaxRetry      int           // 最大重传次数，超出后放弃（客户端上线靠 sync 拉取兜底）

	RouteTTL     time.Duration // 路由表 TTL
	RouteRefresh time.Duration // 路由续期间隔
}

func (c *Config) setDefaults() {
	if c.MaxBody <= 0 {
		c.MaxBody = protocol.DefaultMaxBody
	}
	if c.MaxConns <= 0 {
		c.MaxConns = 1_000_000
	}
	if c.AuthTimeout <= 0 {
		c.AuthTimeout = 10 * time.Second
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 30 * time.Second
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = 3 * c.HeartbeatInterval
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.SendQueueSize <= 0 {
		c.SendQueueSize = 64
	}
	if c.RatePerSec <= 0 {
		c.RatePerSec = 20
	}
	if c.Burst <= 0 {
		c.Burst = 40
	}
	if c.PushRetryInterval <= 0 {
		c.PushRetryInterval = 3 * time.Second
	}
	if c.PushMaxRetry <= 0 {
		c.PushMaxRetry = 3
	}
	if c.RouteTTL <= 0 {
		c.RouteTTL = 3 * time.Minute
	}
	if c.RouteRefresh <= 0 {
		c.RouteRefresh = time.Minute
	}
}
