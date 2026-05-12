package repository

import (
	"database/sql"

	"github.com/mattn/go-sqlite3"
)

func init() {
	sql.Register("sqlite3_with_fk", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			_, err := conn.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`, nil)
			return err
		},
	})
}
