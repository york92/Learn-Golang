package main

import (
	"log"

	"edu-platform/config"
	"edu-platform/database"
	"edu-platform/router"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 加载配置（环境变量 > 默认值）
	config.Init()

	// 2. 初始化数据库（连接 + AutoMigrate）
	_, err := database.Init(config.AppConfig.Database.DSN)
	if err != nil {
		log.Fatalf("❌ Database initialization failed: %v", err)
	}

	// 3. 设置 Gin 运行模式
	gin.SetMode(config.AppConfig.Server.Mode)

	// 4. 创建 Gin 引擎（不带默认中间件，由 router 统一注册）
	r := gin.New()

	// 5. 注册路由与中间件
	router.Register(r, database.DB)

	// 6. 启动 HTTP 服务
	addr := ":" + config.AppConfig.Server.Port
	log.Printf("🚀 Server starting on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}
