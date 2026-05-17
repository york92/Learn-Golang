package config

import (
	"log"
	"os"
)

// AppConfig 全局应用配置
var AppConfig = &Config{}

// Config 应用配置结构体
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
}

// ServerConfig HTTP 服务配置
type ServerConfig struct {
	Port string
	Mode string // gin 运行模式: debug / release / test
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	DSN string // SQLite 文件路径
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret     string
	ExpireHours int
}

// Init 初始化配置（优先读取环境变量，否则使用默认值）
func Init() {
	AppConfig.Server = ServerConfig{
		Port: getEnv("SERVER_PORT", "8080"),
		Mode: getEnv("GIN_MODE", "debug"),
	}

	AppConfig.Database = DatabaseConfig{
		DSN: getEnv("DB_DSN", "./edu_platform.db"),
	}

	AppConfig.JWT = JWTConfig{
		Secret:      getEnv("JWT_SECRET", "edu-platform-secret-key-change-in-prod"),
		ExpireHours: 72,
	}

	log.Printf("✅ Config loaded — port:%s mode:%s dsn:%s",
		AppConfig.Server.Port,
		AppConfig.Server.Mode,
		AppConfig.Database.DSN,
	)
}

// getEnv 读取环境变量，不存在则返回默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
