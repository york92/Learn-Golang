package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"edu-platform/model"

	// "gorm.io/driver/sqlite"
	"github.com/glebarez/sqlite" // 纯 Go，无需 gcc，Windows/Linux/macOS 均可用
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 全局数据库实例
var DB *gorm.DB

// Init 初始化 SQLite 数据库连接并执行自动迁移
func Init(dsn string) (*gorm.DB, error) {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Info, // 生产用 Warn，调试改 Info
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:      newLogger,
		PrepareStmt: true, // 预编译语句，提升重复查询性能
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// SQLite 推荐单连接，避免 "database is locked"
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// WAL 模式提升并发读性能；启用外键约束
	// db.Exec("PRAGMA journal_mode=WAL;")
	// db.Exec("PRAGMA foreign_keys = ON;")
	// PRAGMA 执行结果必须检查，否则外键约束可能静默失效
	if err := db.Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
		log.Printf("⚠️  PRAGMA journal_mode warning: %v", err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON;").Error; err != nil {
		log.Printf("⚠️  PRAGMA foreign_keys warning: %v", err)
	}

	if err := autoMigrate(db); err != nil {
		// return nil, fmt.Errorf("auto migrate failed: %w", err)
		// 明确 fatal，让启动失败可见，而不是带着坏状态运行
		return nil, fmt.Errorf("auto migrate failed (check gcc/CGO on Windows): %w", err)
	}

	DB = db
	log.Println("✅ Database connected and migrated successfully")
	logTables(db) // 启动时打印已建表列表，方便确认
	return db, nil
}

// logTables 启动时打印所有已建表，用于确认迁移结果
func logTables(db *gorm.DB) {
	var tables []string
	db.Raw("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name").Scan(&tables)
	log.Printf("📋 Tables in DB: %v", tables)
}

// autoMigrate 按外键依赖顺序迁移表
// User → Course → Chapter → Progress
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Course{},
		&model.Chapter{},
		&model.Progress{},
	)
}
