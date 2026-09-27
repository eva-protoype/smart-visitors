package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Open parses DB_CONNECTION like sqlite:///./path.db or sqlite:////abs/path.db
// and returns a *sql.DB. Only sqlite is supported (matches Python behavior).
func Open(conn string) (*sql.DB, string, error) {
	path := "./smart_visitors.db"
	if strings.HasPrefix(conn, "sqlite:///") {
		path = strings.TrimPrefix(conn, "sqlite:///")
		if path == "" || path == ":memory:" {
			path = ":memory:"
		}
	} else if conn != "" {
		// Allow raw path fallback
		path = conn
	}
	var dsn string
	if path == ":memory:" {
		dsn = ":memory:"
	} else {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, "", err
		}
		if dir := filepath.Dir(abs); dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}
		dsn = abs
		path = abs
	}
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, "", err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		return nil, "", err
	}
	if err := Migrate(sqlDB); err != nil {
		return nil, "", err
	}
	return sqlDB, path, nil
}

func Migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			aadhar_number TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			role TEXT NOT NULL DEFAULT 'host'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_aadhar ON users(aadhar_number)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE TABLE IF NOT EXISTS passes (
			id TEXT PRIMARY KEY,
			visitor_name TEXT NOT NULL,
			visitor_email TEXT NOT NULL,
			qr_token TEXT UNIQUE,
			status TEXT NOT NULL DEFAULT 'active',
			host_user_id TEXT REFERENCES users(id),
			valid_until DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_passes_qr ON passes(qr_token)`,
		`CREATE TABLE IF NOT EXISTS access_logs (
			id TEXT PRIMARY KEY,
			pass_id TEXT NOT NULL REFERENCES passes(id),
			gate_id TEXT NOT NULL DEFAULT 'Main Gate',
			scan_time DATETIME NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_pass ON access_logs(pass_id)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_time ON access_logs(scan_time)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
