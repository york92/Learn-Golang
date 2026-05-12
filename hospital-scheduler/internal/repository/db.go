package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3" // side-effect: registers "sqlite3"
)

// DB wraps *sql.DB
type DB struct {
	*sql.DB
}

// Open opens (or creates) a SQLite database.
// Uses the custom "sqlite3_with_fk" driver registered in driver.go
// so that PRAGMA foreign_keys=ON is guaranteed on every connection.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite3_with_fk", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Single writer — avoids SQLITE_BUSY, and our ConnectHook runs once.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{db}, nil
}

// Migrate executes a multi-statement SQL script one statement at a time.
// go-sqlite3 does not support multiple statements in a single Exec call.
func (db *DB) Migrate(script string) error {
	for _, stmt := range splitSQL(script) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate [%.80s]: %w", strings.TrimSpace(stmt), err)
		}
	}
	return nil
}

// splitSQL splits a SQL script by semicolons, respecting strings and comments.
func splitSQL(script string) []string {
	var stmts []string
	var buf strings.Builder
	inStr   := false
	inLine  := false
	inBlock := false

	runes := []rune(script)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		if inLine {
			if ch == '\n' { inLine = false; buf.WriteRune(ch) }
			continue
		}
		if inBlock {
			if ch == '*' && i+1 < len(runes) && runes[i+1] == '/' { inBlock = false; i++ }
			continue
		}
		if !inStr && ch == '-' && i+1 < len(runes) && runes[i+1] == '-' { inLine = true; continue }
		if !inStr && ch == '/' && i+1 < len(runes) && runes[i+1] == '*' { inBlock = true; i++; continue }
		if ch == '\'' { inStr = !inStr }

		if ch == ';' && !inStr {
			if s := strings.TrimSpace(buf.String()); s != "" { stmts = append(stmts, s) }
			buf.Reset()
			continue
		}
		buf.WriteRune(ch)
	}
	if s := strings.TrimSpace(buf.String()); s != "" { stmts = append(stmts, s) }
	return stmts
}

// WithTx runs fn inside a transaction, rolling back on error.
func (db *DB) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
