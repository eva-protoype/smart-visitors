package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHashChainsDetectTampering(t *testing.T) {
	database := openTestChainDB(t)
	defer database.Close()
	if _, err := database.Exec(`TRUNCATE access_logs, audit_events, chain_anchors RESTART IDENTITY`); err != nil {
		t.Fatalf("clear test chains: %v", err)
	}

	ctx := context.Background()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin chain fixture: %v", err)
	}
	for i := 0; i < 200; i++ {
		reason := fmt.Sprintf("scan-%d", i)
		if _, err := AppendAccessLog(ctx, tx, AccessLog{
			ScanTime:  time.Date(2026, 1, 1, 12, 0, i, 123456000, time.UTC),
			Direction: "entry", Outcome: "denied", Reason: reason,
		}); err != nil {
			_ = tx.Rollback()
			t.Fatalf("append access log %d: %v", i, err)
		}
		if err := AppendAuditEvent(ctx, tx, AuditEvent{
			Action: "login.failure", Target: fmt.Sprintf("fixture-%d", i),
			Meta: map[string]any{"ordinal": i, "label": reason},
		}); err != nil {
			_ = tx.Rollback()
			t.Fatalf("append audit event %d: %v", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit chain fixture: %v", err)
	}

	verify := func(t *testing.T, run func(*sql.Tx) (bool, *int64, error), expectedBroken int64) {
		t.Helper()
		verifyTx, err := database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatalf("begin chain verification: %v", err)
		}
		defer verifyTx.Rollback()
		ok, brokenAt, err := run(verifyTx)
		if err != nil {
			t.Fatalf("verify chain: %v", err)
		}
		if ok || brokenAt == nil || *brokenAt != expectedBroken {
			t.Fatalf("expected first broken sequence %d; got ok=%t seq=%v", expectedBroken, ok, brokenAt)
		}
	}

	if _, err := database.Exec(`UPDATE access_logs SET reason = 'tampered' WHERE seq = 100`); err != nil {
		t.Fatalf("tamper access log fixture: %v", err)
	}
	verify(t, func(tx *sql.Tx) (bool, *int64, error) {
		return VerifyAccessChain(ctx, tx, 1000)
	}, 100)

	if _, err := database.Exec(`UPDATE audit_events SET reason = 'tampered' WHERE seq = 101`); err != nil {
		t.Fatalf("tamper audit fixture: %v", err)
	}
	verify(t, func(tx *sql.Tx) (bool, *int64, error) {
		return VerifyAuditChain(ctx, tx, 1000)
	}, 101)
}

func TestChainAnchorsPreserveRetainedSuffix(t *testing.T) {
	database := openTestChainDB(t)
	defer database.Close()
	if _, err := database.Exec(`TRUNCATE access_logs, audit_events, chain_anchors RESTART IDENTITY`); err != nil {
		t.Fatalf("clear test chains: %v", err)
	}

	ctx := context.Background()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin chain fixture: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := AppendAccessLog(ctx, tx, AccessLog{
			ScanTime:  time.Date(2026, 1, 1, 12, 0, i, 0, time.UTC),
			Direction: "entry", Outcome: "denied", Reason: fmt.Sprintf("scan-%d", i),
		}); err != nil {
			_ = tx.Rollback()
			t.Fatalf("append access log %d: %v", i, err)
		}
		if err := AppendAuditEvent(ctx, tx, AuditEvent{
			Action: "login.failure", Target: fmt.Sprintf("fixture-%d", i),
			Meta: map[string]any{"ordinal": i},
		}); err != nil {
			_ = tx.Rollback()
			t.Fatalf("append audit event %d: %v", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit chain fixture: %v", err)
	}

	for _, chain := range []struct {
		name   string
		table  string
		verify func(*sql.Tx) (bool, *int64, error)
	}{
		{"access_logs", "access_logs", func(tx *sql.Tx) (bool, *int64, error) {
			return VerifyAccessChain(ctx, tx, 100)
		}},
		{"audit_events", "audit_events", func(tx *sql.Tx) (bool, *int64, error) {
			return VerifyAuditChain(ctx, tx, 100)
		}},
	} {
		var anchorHash string
		if err := database.QueryRow(`SELECT hash FROM ` + chain.table + ` WHERE seq = 3`).Scan(&anchorHash); err != nil {
			t.Fatalf("read %s anchor hash: %v", chain.name, err)
		}
		if _, err := database.Exec(`
			INSERT INTO chain_anchors (chain_name, seq, hash) VALUES ($1, 3, $2)`,
			chain.name, anchorHash); err != nil {
			t.Fatalf("write %s anchor: %v", chain.name, err)
		}
		if _, err := database.Exec(`DELETE FROM ` + chain.table + ` WHERE seq <= 3`); err != nil {
			t.Fatalf("purge %s prefix: %v", chain.name, err)
		}
		verifyTx, err := database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatalf("begin %s chain verification: %v", chain.name, err)
		}
		ok, brokenAt, err := chain.verify(verifyTx)
		_ = verifyTx.Rollback()
		if err != nil || !ok || brokenAt != nil {
			t.Fatalf("expected valid retained %s suffix; got ok=%t seq=%v err=%v",
				chain.name, ok, brokenAt, err)
		}
	}
}

func openTestChainDB(t *testing.T) *sql.DB {
	t.Helper()
	connectionURL := os.Getenv("TEST_DATABASE_URL")
	if connectionURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	parsed, err := url.Parse(connectionURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	if !strings.HasSuffix(parsed.Path, "_test") {
		t.Fatal("TEST_DATABASE_URL must point to a database ending in _test")
	}
	database, err := Open(connectionURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	return database
}
