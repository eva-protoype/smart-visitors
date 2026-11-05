package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(connectionURL string) (*sql.DB, error) {
	if !strings.HasPrefix(connectionURL, "postgres://") && !strings.HasPrefix(connectionURL, "postgresql://") {
		return nil, errors.New("DATABASE_URL must use postgres:// or postgresql://")
	}

	database, err := sql.Open("pgx", connectionURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if err := Migrate(ctx, database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}

	return database, nil
}

func Migrate(ctx context.Context, database *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
	id UUID PRIMARY KEY,
	role TEXT NOT NULL CHECK (role IN ('admin','host','guard')),
	username TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	name TEXT NOT NULL,
	email TEXT UNIQUE,
	phone TEXT,
	must_reset_password BOOLEAN NOT NULL DEFAULT false,
	temp_expires_at TIMESTAMPTZ,
	token_version INTEGER NOT NULL DEFAULT 0,
	disabled_at TIMESTAMPTZ,
	expires_at TIMESTAMPTZ,
	created_by UUID REFERENCES users(id),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

CREATE TABLE IF NOT EXISTS visitors (
	id UUID PRIMARY KEY,
	name TEXT NOT NULL,
	phone TEXT,
	photo_ref TEXT,
	id_note TEXT,
	consent_at TIMESTAMPTZ,
	created_by UUID NOT NULL REFERENCES users(id),
	created_via TEXT NOT NULL CHECK (created_via IN ('regular','walk_in')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE visitors ADD COLUMN IF NOT EXISTS name_hash TEXT;
ALTER TABLE visitors ADD COLUMN IF NOT EXISTS phone_hash TEXT;
CREATE INDEX IF NOT EXISTS idx_visitors_phone_hash ON visitors(phone_hash);

CREATE TABLE IF NOT EXISTS gates (
	id UUID PRIMARY KEY,
	name TEXT UNIQUE NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS sticker_batches (
	id UUID PRIMARY KEY,
	count INTEGER NOT NULL CHECK (count > 0),
	created_by UUID NOT NULL REFERENCES users(id),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS stickers (
	id UUID PRIMARY KEY,
	batch_id UUID REFERENCES sticker_batches(id),
	token_hash TEXT UNIQUE NOT NULL,
	status TEXT NOT NULL DEFAULT 'free' CHECK (status IN ('free','assigned','retired')),
	pass_id UUID,
	claimed_by UUID REFERENCES users(id),
	claimed_at TIMESTAMPTZ,
	printed BOOLEAN NOT NULL DEFAULT false,
	retired_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_stickers_status ON stickers(status);

CREATE TABLE IF NOT EXISTS passes (
	id UUID PRIMARY KEY,
	token_hash TEXT UNIQUE NOT NULL,
	status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','used','expired','revoked','shredded')),
	mode TEXT NOT NULL DEFAULT 'one_time' CHECK (mode IN ('one_time','window')),
	visitor_id UUID NOT NULL REFERENCES visitors(id),
	host_id UUID NOT NULL REFERENCES users(id),
	gate_id UUID REFERENCES gates(id),
	purpose TEXT NOT NULL,
	valid_from TIMESTAMPTZ NOT NULL,
	valid_until TIMESTAMPTZ NOT NULL,
	created_via TEXT NOT NULL CHECK (created_via IN ('regular','walk_in')),
	created_by UUID NOT NULL REFERENCES users(id),
	invite_token_hash TEXT UNIQUE,
	sticker_id UUID REFERENCES stickers(id),
	deleted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CHECK (valid_until > valid_from)
);
CREATE INDEX IF NOT EXISTS idx_passes_host_status ON passes(host_id, status);
CREATE INDEX IF NOT EXISTS idx_passes_status_expiry ON passes(status, valid_until);

CREATE TABLE IF NOT EXISTS pass_secrets (
	pass_id UUID PRIMARY KEY REFERENCES passes(id) ON DELETE CASCADE,
	blob BYTEA NOT NULL,
	wrap BYTEA NOT NULL,
	payload_sig BYTEA NOT NULL,
	qr_wrap BYTEA NOT NULL
);

CREATE TABLE IF NOT EXISTS access_logs (
	seq BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	id UUID NOT NULL UNIQUE,
	pass_id UUID REFERENCES passes(id),
	gate_id UUID REFERENCES gates(id),
	guard_id UUID REFERENCES users(id),
	scan_time TIMESTAMPTZ NOT NULL DEFAULT now(),
	direction TEXT NOT NULL CHECK (direction IN ('entry','exit')),
	outcome TEXT NOT NULL CHECK (outcome IN ('approved','denied')),
	reason TEXT NOT NULL,
	prev_hash TEXT NOT NULL,
	hash TEXT NOT NULL,
	anomaly_score DOUBLE PRECISION,
	anomaly_reasons JSONB,
	anomaly_suspicious BOOLEAN
);
CREATE INDEX IF NOT EXISTS idx_access_logs_pass_time ON access_logs(pass_id, scan_time DESC);
CREATE INDEX IF NOT EXISTS idx_access_logs_scan_time ON access_logs(scan_time DESC);

CREATE TABLE IF NOT EXISTS audit_events (
	seq BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	id UUID NOT NULL UNIQUE,
	actor_id UUID REFERENCES users(id),
	action TEXT NOT NULL,
	target TEXT NOT NULL,
	reason TEXT,
	meta JSONB NOT NULL DEFAULT '{}'::jsonb,
	prev_hash TEXT NOT NULL,
	hash TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chain_anchors (
	chain_name TEXT PRIMARY KEY CHECK (chain_name IN ('access_logs','audit_events')),
	seq BIGINT NOT NULL,
	hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS keys (
	name TEXT PRIMARY KEY,
	wrapped BYTEA NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO schema_migrations (version) VALUES (1) ON CONFLICT (version) DO NOTHING;
`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, statement := range strings.Split(schema, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
