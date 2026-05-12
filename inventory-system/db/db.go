package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// Init 初始化 SQLite，开启 WAL 模式并创建表结构
func Init(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("%s?_foreign_keys=on", dbPath))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite 并发写是串行的，限制连接池避免 SQLITE_BUSY
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := applyPragmas(db); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	log.Println("[DB] SQLite initialized, WAL mode active")
	return db, nil
}

func applyPragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",   // 写前日志，读写不互阻
		"PRAGMA synchronous=NORMAL", // WAL 模式下 NORMAL 已足够安全
		"PRAGMA busy_timeout=5000",  // 锁等待 5 秒而非立即报错
		"PRAGMA cache_size=-64000",  // 64MB 页缓存
		"PRAGMA temp_store=MEMORY",  // 临时表放内存
		"PRAGMA foreign_keys=ON",    // 开启外键约束
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return nil
}

func migrate(db *sql.DB) error {
	ddl := `
CREATE TABLE IF NOT EXISTS products (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT    NOT NULL,
    spec              TEXT    NOT NULL DEFAULT '',
    price             REAL    NOT NULL DEFAULT 0,
    stock             INTEGER NOT NULL DEFAULT 0,
    warning_threshold INTEGER NOT NULL DEFAULT 0,
    version           INTEGER NOT NULL DEFAULT 0,
    deleted           INTEGER NOT NULL DEFAULT 0,
    created_at        DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
);

CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);
CREATE INDEX IF NOT EXISTS idx_products_deleted ON products(deleted);

CREATE TABLE IF NOT EXISTS stock_logs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id   INTEGER NOT NULL,
    change_type  TEXT    NOT NULL CHECK(change_type IN ('IN','OUT','LOSS')),
    quantity     INTEGER NOT NULL CHECK(quantity > 0),
    before_stock INTEGER NOT NULL,
    after_stock  INTEGER NOT NULL,
    remark       TEXT    NOT NULL DEFAULT '',
    operator     TEXT    NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
    FOREIGN KEY (product_id) REFERENCES products(id)
);

CREATE INDEX IF NOT EXISTS idx_logs_product_id  ON stock_logs(product_id);
CREATE INDEX IF NOT EXISTS idx_logs_created_at  ON stock_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_logs_change_type ON stock_logs(change_type);

CREATE TABLE IF NOT EXISTS stock_warnings (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id    INTEGER NOT NULL,
    current_stock INTEGER NOT NULL,
    threshold     INTEGER NOT NULL,
    is_resolved   INTEGER NOT NULL DEFAULT 0,
    triggered_at  DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
    resolved_at   DATETIME,
    FOREIGN KEY (product_id) REFERENCES products(id)
);

CREATE INDEX IF NOT EXISTS idx_warnings_product_id  ON stock_warnings(product_id);
CREATE INDEX IF NOT EXISTS idx_warnings_is_resolved ON stock_warnings(is_resolved);
`
	_, err := db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
