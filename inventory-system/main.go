package main

import (
	"context"
	"errors"
	"inventory/config"
	"inventory/db"
	"inventory/handler"
	"inventory/pkg/notify"
	"inventory/repository"
	"inventory/service"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()

	// ── 数据库 ──────────────────────────────────────────
	database, err := db.Init(cfg.DBPath)
	if err != nil {
		log.Fatalf("DB init failed: %v", err)
	}
	defer database.Close()

	// ── 依赖注入 ─────────────────────────────────────────
	productRepo := repository.NewProductRepo(database)
	logRepo     := repository.NewLogRepo(database)
	warningRepo := repository.NewWarningRepo(database)

	notifier       := notify.NewLogNotifier()
	productService := service.NewProductService(productRepo)
	stockService   := service.NewStockService(database, productRepo, logRepo, warningRepo, notifier)
	warningService := service.NewWarningService(warningRepo)

	productHandler := handler.NewProductHandler(productService)
	stockHandler   := handler.NewStockHandler(stockService)
	warningHandler := handler.NewWarningHandler(warningService)

	// ── 路由（Go 1.22 新路由语法）────────────────────────
	mux := http.NewServeMux()

	// 商品 CRUD
	mux.HandleFunc("POST /api/products",           productHandler.Create)
	mux.HandleFunc("GET /api/products",            productHandler.List)
	mux.HandleFunc("GET /api/products/{id}",       productHandler.GetByID)
	mux.HandleFunc("PUT /api/products/{id}",       productHandler.Update)
	mux.HandleFunc("DELETE /api/products/{id}",    productHandler.Delete)

	// 库存操作
	mux.HandleFunc("POST /api/stock/in",           stockHandler.StockIn)
	mux.HandleFunc("POST /api/stock/out",          stockHandler.StockOut)
	mux.HandleFunc("POST /api/stock/loss",         stockHandler.StockLoss)

	// 流水查询
	mux.HandleFunc("GET /api/logs",                stockHandler.ListLogs)

	// 预警
	mux.HandleFunc("GET /api/warnings",                     warningHandler.List)
	mux.HandleFunc("PUT /api/warnings/{id}/resolve",        warningHandler.Resolve)

	// 健康检查
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// ── HTTP Server ──────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🚀 库存管理系统启动，监听端口 :%s", cfg.ServerPort)
		log.Printf("   数据库路径：%s", cfg.DBPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("服务器已安全关闭")
}

// loggingMiddleware 简单的请求日志中间件
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("[HTTP] %s %s → %d (%s)", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
