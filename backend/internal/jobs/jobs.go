package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"smart-visitors/internal/config"
	"smart-visitors/internal/db"
)

func ExpirePasses(ctx context.Context, database *sql.DB) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin expiry sweep: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		UPDATE passes SET status = 'expired'
		WHERE status = 'active' AND valid_until < now()
		RETURNING id::text`)
	if err != nil {
		return fmt.Errorf("expire passes: %w", err)
	}
	var expired []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read expired pass: %w", err)
		}
		expired = append(expired, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read expired passes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close expired passes: %w", err)
	}
	for _, id := range expired {
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			Action: "pass.expire", Target: id, Meta: map[string]any{"source": "expiry_sweep"},
		}); err != nil {
			return fmt.Errorf("audit expired pass: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit expiry sweep: %w", err)
	}
	return nil
}

func ShredExpiredPasses(ctx context.Context, database *sql.DB, cfg config.Config) error {
	cutoff := time.Now().AddDate(0, 0, -cfg.RetentionPassDays)
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin pass retention sweep: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT p.id::text, p.visitor_id::text, v.photo_ref
		FROM passes p JOIN visitors v ON v.id = p.visitor_id
		WHERE p.status <> 'shredded' AND p.valid_until < $1
		FOR UPDATE OF p, v SKIP LOCKED`, cutoff)
	if err != nil {
		return fmt.Errorf("select passes for shredding: %w", err)
	}
	type candidate struct {
		passID    string
		visitorID string
		photoRef  sql.NullString
	}
	var candidates []candidate
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.passID, &item.visitorID, &item.photoRef); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read pass for shredding: %w", err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read passes for shredding: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close passes for shredding: %w", err)
	}
	for _, item := range candidates {
		if item.photoRef.Valid {
			if err := removePhoto(cfg.PhotosDir, item.photoRef.String); err != nil {
				return fmt.Errorf("remove visitor photo for pass %s: %w", item.passID, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pass_secrets WHERE pass_id = $1`, item.passID); err != nil {
			return fmt.Errorf("crypto-shred pass %s: %w", item.passID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE visitors SET name = '[shredded]', phone = NULL, id_note = NULL, photo_ref = NULL,
				name_hash = NULL, phone_hash = NULL
			WHERE id = $1`, item.visitorID); err != nil {
			return fmt.Errorf("clear visitor details for pass %s: %w", item.passID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE passes SET status = 'shredded', deleted_at = now(), invite_token_hash = NULL
			WHERE id = $1`, item.passID); err != nil {
			return fmt.Errorf("tombstone pass %s: %w", item.passID, err)
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			Action: "pass.shred", Target: item.passID,
			Meta: map[string]any{"source": "retention_sweep"},
		}); err != nil {
			return fmt.Errorf("audit shredded pass %s: %w", item.passID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit pass retention sweep: %w", err)
	}
	return nil
}

func PurgeLogs(ctx context.Context, database *sql.DB, retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin log retention sweep: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `LOCK TABLE access_logs IN EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock access log chain: %w", err)
	}
	accessDeleted, err := purgeAccessRows(ctx, tx, cutoff)
	if err != nil {
		return err
	}
	if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
		Action: "log.purge", Target: "access_logs",
		Meta: map[string]any{"deleted": accessDeleted, "cutoff": cutoff.UTC().Format(time.RFC3339)},
	}); err != nil {
		return fmt.Errorf("append log purge marker: %w", err)
	}
	_, _, err = purgeAuditRows(ctx, tx, cutoff)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit log retention sweep: %w", err)
	}
	return nil
}

func purgeAccessRows(ctx context.Context, tx *sql.Tx, cutoff time.Time) (int64, error) {
	var sequence int64
	var hash string
	err := tx.QueryRowContext(ctx, `
		SELECT seq, hash FROM access_logs
		WHERE seq < COALESCE(
			(SELECT MIN(seq) FROM access_logs WHERE scan_time >= $1),
			(SELECT COALESCE(MAX(seq) + 1, 0) FROM access_logs)
		)
		ORDER BY seq DESC LIMIT 1`, cutoff).Scan(&sequence, &hash)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("find access log retention anchor: %w", err)
	}
	var deleted int64
	if err == nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chain_anchors (chain_name, seq, hash) VALUES ('access_logs', $1, $2)
			ON CONFLICT (chain_name) DO UPDATE SET seq = EXCLUDED.seq, hash = EXCLUDED.hash
			WHERE chain_anchors.seq < EXCLUDED.seq`, sequence, hash); err != nil {
			return 0, fmt.Errorf("advance access log chain anchor: %w", err)
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM access_logs WHERE seq <= $1`, sequence)
		if err != nil {
			return 0, fmt.Errorf("purge access logs: %w", err)
		}
		deleted, err = result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("count purged access logs: %w", err)
		}
	}
	return deleted, nil
}

func purgeAuditRows(ctx context.Context, tx *sql.Tx, cutoff time.Time) (int64, int64, error) {
	var sequence int64
	var hash string
	err := tx.QueryRowContext(ctx, `
		SELECT seq, hash FROM audit_events
		WHERE seq < COALESCE(
			(SELECT MIN(seq) FROM audit_events WHERE created_at >= $1),
			(SELECT COALESCE(MAX(seq) + 1, 0) FROM audit_events)
		)
		ORDER BY seq DESC LIMIT 1`, cutoff).Scan(&sequence, &hash)
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, fmt.Errorf("find audit retention anchor: %w", err)
	}
	var deleted int64
	if err == nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chain_anchors (chain_name, seq, hash) VALUES ('audit_events', $1, $2)
			ON CONFLICT (chain_name) DO UPDATE SET seq = EXCLUDED.seq, hash = EXCLUDED.hash
			WHERE chain_anchors.seq < EXCLUDED.seq`, sequence, hash); err != nil {
			return 0, 0, fmt.Errorf("advance audit chain anchor: %w", err)
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM audit_events WHERE seq <= $1`, sequence)
		if err != nil {
			return 0, 0, fmt.Errorf("purge audit events: %w", err)
		}
		deleted, err = result.RowsAffected()
		if err != nil {
			return 0, 0, fmt.Errorf("count purged audit events: %w", err)
		}
	}
	return sequence, deleted, nil
}

func removePhoto(directory, ref string) error {
	if ref == "" || filepath.Base(ref) != ref {
		return errors.New("invalid photo reference")
	}
	err := os.Remove(filepath.Join(directory, ref))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
