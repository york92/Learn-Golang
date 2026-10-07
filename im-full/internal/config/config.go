// Package config 负责加载配置：默认值 < JSON 配置文件 < 环境变量 < 命令行参数。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	TCPAddr  string `json:"tcp_addr"`  // 原生 TCP 长连接（App / CLI）
	HTTPAddr string `json:"http_addr"` // HTTP API + Web 客户端 + WebSocket(/ws)
	DataDir  string `json:"data_dir"`  // 数据目录（WAL 日志）

	AllowRegister bool `json:"allow_register"` // 是否开放自助注册
	RequireFriend bool `json:"require_friend"` // 单聊是否必须先加好友
	TokenTTLHours int  `json:"token_ttl_hours"`
	AuthPerMinute int  `json:"auth_per_minute"` // 每个来源 IP 每分钟允许的「注册 + 登录失败」次数

	HeartbeatSec    int `json:"heartbeat_sec"`     // 下发给客户端的心跳间隔
	MaxConns        int `json:"max_conns"`         // 单机最大连接数
	MaxContentBytes int `json:"max_content_bytes"` // 单条消息内容上限
	FsyncMs         int `json:"fsync_ms"`          // 后台 fsync 间隔；0=每次写都 fsync

	WSAllowAnyOrigin bool `json:"ws_allow_any_origin"` // false=只允许同源页面连 /ws

	LogLevel  string `json:"log_level"`  // debug|info|warn|error
	LogFormat string `json:"log_format"` // text|json
}

func Default() Config {
	return Config{
		TCPAddr: ":9000", HTTPAddr: ":8080", DataDir: "./data",
		AllowRegister: true, RequireFriend: false, TokenTTLHours: 24 * 30, AuthPerMinute: 60,
		HeartbeatSec: 30, MaxConns: 100000, MaxContentBytes: 4096, FsyncMs: 200,
		LogLevel: "info", LogFormat: "text",
	}
}

// Load 读取 JSON 配置文件（path 为空则只用默认值），然后叠加环境变量。
func Load(path string) (Config, error) {
	c := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return c, fmt.Errorf("read config: %w", err)
		}
		if err := json.Unmarshal(b, &c); err != nil {
			return c, fmt.Errorf("parse config %s: %w", path, err)
		}
	}
	c.applyEnv()
	return c, c.Validate()
}

func (c *Config) applyEnv() {
	str := func(k string, dst *string) {
		if v := os.Getenv(k); v != "" {
			*dst = v
		}
	}
	num := func(k string, dst *int) {
		if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
			*dst = v
		}
	}
	flag := func(k string, dst *bool) {
		if v, err := strconv.ParseBool(os.Getenv(k)); err == nil {
			*dst = v
		}
	}
	str("IM_TCP_ADDR", &c.TCPAddr)
	str("IM_HTTP_ADDR", &c.HTTPAddr)
	str("IM_DATA_DIR", &c.DataDir)
	str("IM_LOG_LEVEL", &c.LogLevel)
	str("IM_LOG_FORMAT", &c.LogFormat)
	num("IM_HEARTBEAT_SEC", &c.HeartbeatSec)
	num("IM_MAX_CONNS", &c.MaxConns)
	num("IM_AUTH_PER_MINUTE", &c.AuthPerMinute)
	num("IM_FSYNC_MS", &c.FsyncMs)
	flag("IM_ALLOW_REGISTER", &c.AllowRegister)
	flag("IM_REQUIRE_FRIEND", &c.RequireFriend)
	flag("IM_WS_ALLOW_ANY_ORIGIN", &c.WSAllowAnyOrigin)
}

func (c *Config) Validate() error {
	switch {
	case c.HTTPAddr == "" && c.TCPAddr == "":
		return fmt.Errorf("at least one of tcp_addr / http_addr is required")
	case c.DataDir == "":
		return fmt.Errorf("data_dir is required")
	case c.HeartbeatSec < 1 || c.HeartbeatSec > 300:
		return fmt.Errorf("heartbeat_sec must be in [1,300]")
	case c.TokenTTLHours < 1:
		return fmt.Errorf("token_ttl_hours must be >= 1")
	case c.FsyncMs < 0:
		return fmt.Errorf("fsync_ms must be >= 0")
	}
	return nil
}
