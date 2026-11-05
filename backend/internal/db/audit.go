package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type AuditEvent struct {
	ActorID *string
	Action  string
	Target  string
	Reason  *string
	Meta    map[string]any
}

func auditHash(prev string, seq int64, id string, actorID *string, action, target string, reason *string, meta []byte) string {
	fields := []string{prev, fmt.Sprint(seq), id, "", action, target, ""}
	if actorID != nil {
		fields[3] = *actorID
	}
	if reason != nil {
		fields[6] = *reason
	}

	sum := sha256.Sum256([]byte(strings.Join(fields, "") + string(meta)))
	return hex.EncodeToString(sum[:])
}

func AppendAuditEvent(ctx context.Context, tx *sql.Tx, event AuditEvent) error {
	if _, err := tx.ExecContext(ctx, `LOCK TABLE audit_events IN EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock audit chain: %w", err)
	}

	prev := strings.Repeat("0", 64)
	var previousHash string
	err := tx.QueryRowContext(ctx, `SELECT hash FROM audit_events ORDER BY seq DESC LIMIT 1`).Scan(&previousHash)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read audit chain head: %w", err)
	}
	if err == nil {
		prev = previousHash
	} else {
		err = tx.QueryRowContext(ctx, `SELECT hash FROM chain_anchors WHERE chain_name = 'audit_events'`).Scan(&previousHash)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("read audit chain anchor: %w", err)
		}
		if err == nil {
			prev = previousHash
		}
	}

	if event.Meta == nil {
		event.Meta = map[string]any{}
	}
	meta, err := json.Marshal(event.Meta)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}
	id := uuid.NewString()

	var seq int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO audit_events (id, actor_id, action, target, reason, meta, prev_hash, hash)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, '')
		RETURNING seq`, id, event.ActorID, event.Action, event.Target, event.Reason, string(meta), prev).Scan(&seq); err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}

	hash := auditHash(prev, seq, id, event.ActorID, event.Action, event.Target, event.Reason, meta)
	if _, err := tx.ExecContext(ctx, `UPDATE audit_events SET hash = $1 WHERE seq = $2`, hash, seq); err != nil {
		return fmt.Errorf("seal audit event: %w", err)
	}

	return nil
}

func VerifyAuditChain(ctx context.Context, tx *sql.Tx, limit int) (bool, *int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT seq, id, actor_id::text, action, target, reason, meta, prev_hash, hash
		FROM audit_events ORDER BY seq DESC LIMIT $1`, limit)
	if err != nil {
		return false, nil, fmt.Errorf("read audit chain: %w", err)
	}
	defer rows.Close()

	type row struct {
		seq      int64
		id       string
		actor    sql.NullString
		action   string
		target   string
		reason   sql.NullString
		meta     []byte
		prevHash string
		hash     string
	}
	var chain []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.seq, &item.id, &item.actor, &item.action, &item.target, &item.reason, &item.meta, &item.prevHash, &item.hash); err != nil {
			return false, nil, err
		}
		chain = append(chain, item)
	}
	if err := rows.Err(); err != nil {
		return false, nil, err
	}

	prev := strings.Repeat("0", 64)
	if len(chain) > 0 {
		err := tx.QueryRowContext(ctx, `SELECT hash FROM audit_events WHERE seq < $1 ORDER BY seq DESC LIMIT 1`, chain[len(chain)-1].seq).Scan(&prev)
		if err == sql.ErrNoRows {
			err = tx.QueryRowContext(ctx, `SELECT hash FROM chain_anchors WHERE chain_name = 'audit_events' AND seq < $1`, chain[len(chain)-1].seq).Scan(&prev)
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
		actor, reason := (*string)(nil), (*string)(nil)
		if item.actor.Valid {
			actor = &item.actor.String
		}
		if item.reason.Valid {
			reason = &item.reason.String
		}
		var metadata any
		decoder := json.NewDecoder(strings.NewReader(string(item.meta)))
		decoder.UseNumber()
		if err := decoder.Decode(&metadata); err != nil {
			seq := item.seq
			return false, &seq, nil
		}
		canonicalMeta, err := json.Marshal(metadata)
		if err != nil {
			return false, nil, err
		}
		expected := auditHash(prev, item.seq, item.id, actor, item.action, item.target, reason, canonicalMeta)
		if item.prevHash != prev || item.hash != expected {
			seq := item.seq
			return false, &seq, nil
		}
		prev = item.hash
	}
	return true, nil, nil
}
