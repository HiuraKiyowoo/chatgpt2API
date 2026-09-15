package core

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB (sqlite, pure-Go driver modernc.org/sqlite).
type DB struct {
	*sql.DB
}

func OpenDB(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // sqlite single-writer
	if err := db.Ping(); err != nil {
		return nil, err
	}
	d := &DB{db}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			label TEXT NOT NULL DEFAULT '',
			credential_type TEXT NOT NULL DEFAULT 'access_token',
			credential_enc TEXT NOT NULL,
			access_token_enc TEXT NOT NULL DEFAULT '',
			cookies_enc TEXT NOT NULL DEFAULT '',
			cf_clearance TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			oai_client_version TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'invalid',
			proxy_url TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '',
			cooldown_until INTEGER NOT NULL DEFAULT 0,
			in_flight INTEGER NOT NULL DEFAULT 0,
			request_count INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			error_count INTEGER NOT NULL DEFAULT 0,
			last_used_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			key_hash TEXT NOT NULL UNIQUE,
			key_preview TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL DEFAULT 0,
			last_used_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'admin',
			created_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL,
			level TEXT NOT NULL DEFAULT 'info',
			source TEXT NOT NULL DEFAULT 'api',
			message TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_ts ON logs(ts DESC)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// kolom email ditunda di DB lama: ALTER idempotent (error 'duplicate column' diabaikan)
	if _, err := d.Exec(`ALTER TABLE accounts ADD COLUMN email TEXT NOT NULL DEFAULT ''`); err != nil &&
		!strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("migrate email: %w", err)
	}
	return nil
}

func Now() int64 { return time.Now().Unix() }

func Truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
