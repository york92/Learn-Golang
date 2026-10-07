// imserver：IM 服务端单可执行文件 —— TCP 长连接 + WebSocket + HTTP API + Web 客户端。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/im/internal/api"
	"example.com/im/internal/config"
	"example.com/im/internal/gateway"
	"example.com/im/internal/service"
	"example.com/im/internal/store"
)

var version = "dev" // 构建时用 -ldflags "-X main.version=..." 注入

func main() {
	cfgPath := flag.String("config", "", "配置文件路径（JSON），不填则使用默认值")
	tcp := flag.String("tcp", "", "覆盖 tcp_addr")
	httpAddr := flag.String("http", "", "覆盖 http_addr")
	data := flag.String("data", "", "覆盖 data_dir")
	seed := flag.Bool("seed", false, "启动时创建演示账号 alice/bob/carol（密码 123456）、好友关系和一个群")
	showVer := flag.Bool("version", false, "打印版本并退出")
	flag.Parse()
	if *showVer {
		fmt.Println("imserver", version)
		return
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}
	if *tcp != "" {
		cfg.TCPAddr = *tcp
	}
	if *httpAddr != "" {
		cfg.HTTPAddr = *httpAddr
	}
	if *data != "" {
		cfg.DataDir = *data
	}

	log := newLogger(cfg)
	st, err := store.Open(cfg.DataDir, store.Options{FsyncInterval: time.Duration(cfg.FsyncMs) * time.Millisecond})
	if err != nil {
		log.Error("open store failed", "dir", cfg.DataDir, "err", err)
		os.Exit(1)
	}
	defer st.Close()

	svc := service.New(st, service.Config{
		AllowRegister: cfg.AllowRegister, RequireFriend: cfg.RequireFriend,
		TokenTTL: time.Duration(cfg.TokenTTLHours) * time.Hour, MaxContent: cfg.MaxContentBytes,
	})
	gw := gateway.NewServer(gateway.Config{
		GatewayID: "gw-1", MaxConns: int64(cfg.MaxConns),
		HeartbeatInterval: time.Duration(cfg.HeartbeatSec) * time.Second,
	}, svc, gateway.NewMemRouteStore())
	gw.SetLogger(log)
	svc.SetPusher(gw)

	if *seed {
		seedDemo(svc, log)
	}

	errc := make(chan error, 2)
	var httpSrv *http.Server
	if cfg.TCPAddr != "" {
		ln, err := net.Listen("tcp", cfg.TCPAddr)
		if err != nil {
			log.Error("listen tcp failed", "addr", cfg.TCPAddr, "err", err)
			os.Exit(1)
		}
		go func() { errc <- gw.Serve(ln) }()
	}
	if cfg.HTTPAddr != "" {
		httpSrv = &http.Server{Addr: cfg.HTTPAddr, Handler: api.New(svc, gw, cfg.WSAllowAnyOrigin, cfg.AuthPerMinute, log),
			ReadHeaderTimeout: 10 * time.Second}
		ln, err := net.Listen("tcp", cfg.HTTPAddr)
		if err != nil {
			log.Error("listen http failed", "addr", cfg.HTTPAddr, "err", err)
			os.Exit(1)
		}
		go func() {
			if err := httpSrv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}()
	}

	fmt.Printf("\n  imserver %s 已启动\n", version)
	if cfg.HTTPAddr != "" {
		fmt.Printf("  Web 客户端 : http://localhost%s\n  WebSocket  : ws://localhost%s/ws\n", port(cfg.HTTPAddr), port(cfg.HTTPAddr))
	}
	if cfg.TCPAddr != "" {
		fmt.Printf("  TCP 长连接 : localhost%s\n", port(cfg.TCPAddr))
	}
	fmt.Printf("  数据目录   : %s\n  按 Ctrl+C 优雅退出\n\n", cfg.DataDir)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-errc:
		log.Error("server failed", "err", err)
		os.Exit(1)
	case <-ctx.Done():
	}

	log.Info("shutting down...")
	sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if httpSrv != nil {
		_ = httpSrv.Shutdown(sctx) // WebSocket 是被 Hijack 的连接，由网关负责收尾
	}
	_ = gw.Shutdown(sctx) // 下发 Kick 带随机重连延迟，等连接排空
	_ = st.Close()        // 最后 fsync 落盘
	log.Info("bye")
}

func port(addr string) string {
	if _, p, err := net.SplitHostPort(addr); err == nil {
		return ":" + p
	}
	return addr
}

func newLogger(cfg config.Config) *slog.Logger {
	var lvl slog.Level
	_ = lvl.UnmarshalText([]byte(cfg.LogLevel))
	opts := &slog.HandlerOptions{Level: lvl}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

// seedDemo 创建演示数据（幂等：已存在则跳过）。
func seedDemo(svc *service.Service, log *slog.Logger) {
	var uids []int64
	for _, n := range []string{"alice", "bob", "carol"} {
		u, ok := svc.St.FindUser(n)
		if !ok {
			nu, err := svc.St.CreateUser(n, n, "123456")
			if err != nil {
				log.Warn("seed user failed", "user", n, "err", err)
				return
			}
			u = *nu
			log.Info("seeded user", "username", n, "password", "123456", "uid", u.UID)
		}
		uids = append(uids, u.UID)
	}
	_ = svc.St.AddFriend(uids[0], uids[1])
	_ = svc.St.AddFriend(uids[0], uids[2])
	_ = svc.St.AddFriend(uids[1], uids[2])
	if len(svc.St.GroupsOf(uids[0])) == 0 {
		if _, err := svc.St.CreateGroup(uids[0], "演示群", uids[1:]); err == nil {
			log.Info("seeded group", "name", "演示群")
		}
	}
}
