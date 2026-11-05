package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AccessLog struct {
	PassID    *string
	GateID    *string
	GuardID   *string
	ScanTime  time.Time
	Direction string
	Outcome   string
	Reason    string
}

func accessHash(prev string, sequence int64, id string, scanTime time.Time, gateID, passID, guardID *string, direction, outcome, reason string) string {
	fields := []string{
		prev, fmt.Sprint(sequence), id, scanTime.UTC().Format(time.RFC3339Nano),
		stringValue(gateID), stringValue(passID), stringValue(guardID),
		direction, outcome, reason,
	}
	sum := sha256.Sum256([]byte(strings.Join(fields, "")))
	return hex.EncodeToString(sum[:])
}

func AppendAccessLog(ctx context.Context, tx *sql.Tx, entry AccessLog) (string, error) {
	if entry.Direction != "entry" && entry.Direction != "exit" {
		return "", errors.New("invalid scan direction")
	}
	if entry.Outcome != "approved" && entry.Outcome != "denied" {
		return "", errors.New("invalid scan outcome")
	}
	if entry.ScanTime.IsZero() {
		entry.ScanTime = time.Now().UTC()
	}
	entry.ScanTime = time.UnixMicro(entry.ScanTime.UTC().UnixMicro()).UTC()

	if _, err := tx.ExecContext(ctx, `LOCK TABLE access_logs IN EXCLUSIVE MODE`); err != nil {
		return "", fmt.Errorf("lock access log chain: %w", err)
	}
	prev := strings.Repeat("0", 64)
	var previousHash string
	err := tx.QueryRowContext(ctx, `SELECT hash FROM access_logs ORDER BY seq DESC LIMIT 1`).Scan(&previousHash)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("read access log chain head: %w", err)
	}
	if err == nil {
		prev = previousHash
	} else {
		err = tx.QueryRowContext(ctx, `SELECT hash FROM chain_anchors WHERE chain_name = 'access_logs'`).Scan(&previousHash)
		if err != nil && err != sql.ErrNoRows {
			return "", fmt.Errorf("read access log anchor: %w", err)
		}
		if err == nil {
			prev = previousHash
		}
	}

	id := uuid.NewString()
	var sequence int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO access_logs (id, pass_id, gate_id, guard_id, scan_time, direction, outcome, reason, prev_hash, hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, '')
		RETURNING seq`, id, entry.PassID, entry.GateID, entry.GuardID, entry.ScanTime,
		entry.Direction, entry.Outcome, entry.Reason, prev).Scan(&sequence); err != nil {
		return "", fmt.Errorf("append access log: %w", err)
	}

	hash := accessHash(prev, sequence, id, entry.ScanTime, entry.GateID, entry.PassID, entry.GuardID, entry.Direction, entry.Outcome, entry.Reason)
	if _, err := tx.ExecContext(ctx, `UPDATE access_logs SET hash = $1 WHERE seq = $2`, hash, sequence); err != nil {
		return "", fmt.Errorf("seal access log: %w", err)
	}
	return id, nil
}

func VerifyAccessChain(ctx context.Context, tx *sql.Tx, limit int) (bool, *int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT seq, id, pass_id::text, gate_id::text, guard_id::text, scan_time,
		       direction, outcome, reason, prev_hash, hash
		FROM access_logs ORDER BY seq DESC LIMIT $1`, limit)
	if err != nil {
		return false, nil, fmt.Errorf("read access log chain: %w", err)
	}
	defer rows.Close()

	type row struct {
		sequence  int64
		id        string
		passID    sql.NullString
		gateID    sql.NullString
		guardID   sql.NullString
		scanTime  time.Time
		direction string
		outcome   string
		reason    string
		prevHash  string
		hash      string
	}
	var chain []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.sequence, &item.id, &item.passID, &item.gateID, &item.guardID,
			&item.scanTime, &item.direction, &item.outcome, &item.reason, &item.prevHash, &item.hash); err != nil {
			return false, nil, err
		}
		chain = append(chain, item)
	}
	if err := rows.Err(); err != nil {
		return false, nil, err
	}

	prev := strings.Repeat("0", 64)
	if len(chain) > 0 {
		err := tx.QueryRowContext(ctx, `SELECT hash FROM access_logs WHERE seq < $1 ORDER BY seq DESC LIMIT 1`, chain[len(chain)-1].sequence).Scan(&prev)
		if err == sql.ErrNoRows {
			err = tx.QueryRowContext(ctx, `SELECT hash FROM chain_anchors WHERE chain_name = 'access_logs' AND seq < $1`, chain[len(chain)-1].sequence).Scan(&prev)
		}
		if err != nil && err != sql.ErrNoRows {
			return false, nil, err
		}
		if err == sql.ErrNoRows {
			prev = strings.Repeat("0", 64)
		}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		item := chain[i]
		expected := accessHash(prev, item.sequence, item.id, item.scanTime,
			nullStringPointer(item.gateID), nullStringPointer(item.passID), nullStringPointer(item.guardID),
			item.direction, item.outcome, item.reason)
		if item.prevHash != prev || item.hash != expected {
			sequence := item.sequence
			return false, &sequence, nil
		}
		prev = item.hash
	}
	return true, nil, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
