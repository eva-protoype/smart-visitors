package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"smart-visitors/internal/db"
)

func AuditEventsList(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := queryLimit(r, 50)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "limit must be between 1 and 100.")
			return
		}
		cursor, err := parseCursor(r.URL.Query().Get("cursor"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "cursor must be a positive sequence number.")
			return
		}
		rows, err := database.QueryContext(r.Context(), `
			SELECT seq, id::text, actor_id::text, action, target, reason, meta, prev_hash, hash, created_at
			FROM audit_events
			WHERE ($1 = '' OR actor_id::text = $1) AND ($2 = '' OR action = $2)
			  AND ($3 = 0 OR seq < $3)
			ORDER BY seq DESC LIMIT $4`,
			r.URL.Query().Get("actor"), r.URL.Query().Get("action"), cursor, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load audit events.")
			return
		}
		defer rows.Close()
		events := make([]map[string]any, 0)
		for rows.Next() {
			var seq int64
			var id, action, target, prevHash, hash string
			var actor, reason sql.NullString
			var meta []byte
			var createdAt time.Time
			if err := rows.Scan(&seq, &id, &actor, &action, &target, &reason, &meta, &prevHash, &hash, &createdAt); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load audit events.")
				return
			}
			var metadata any
			if err := json.Unmarshal(meta, &metadata); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load audit events.")
				return
			}
			events = append(events, map[string]any{
				"seq": seq, "id": id, "actor_id": nullString(actor),
				"action": action, "target": target, "reason": nullString(reason),
				"meta": metadata, "prev_hash": prevHash, "hash": hash, "created_at": createdAt,
			})
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load audit events.")
			return
		}
		var nextCursor any
		if len(events) == limit {
			nextCursor = events[len(events)-1]["seq"]
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": events, "next_cursor": nextCursor})
	}
}

func AccessLogsList(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := queryLimit(r, 50)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "limit must be between 1 and 100.")
			return
		}
		cursor, err := parseCursor(r.URL.Query().Get("cursor"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "cursor must be a positive sequence number.")
			return
		}
		ctx := r.Context()
		tx, err := database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load access logs.")
			return
		}
		defer tx.Rollback()
		rows, err := tx.QueryContext(ctx, `
			SELECT seq, id::text, pass_id::text, gate_id::text, guard_id::text,
			       scan_time, direction, outcome, reason, prev_hash, hash,
			       anomaly_score, anomaly_reasons
			FROM access_logs
			WHERE ($1 = '' OR pass_id::text = $1)
			  AND ($2 = '' OR gate_id::text = $2)
			  AND ($3 = '' OR outcome = $3)
			  AND ($4 = 0 OR seq < $4)
			ORDER BY seq DESC LIMIT $5`,
			r.URL.Query().Get("pass"), r.URL.Query().Get("gate"),
			r.URL.Query().Get("outcome"), cursor, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load access logs.")
			return
		}
		items := make([]map[string]any, 0)
		var nextCursor any
		for rows.Next() {
			var seq int64
			var id, direction, outcome, reason, prevHash, hash string
			var passID, gateID, guardID sql.NullString
			var scanTime time.Time
			var anomalyScore sql.NullFloat64
			var anomalyReasons []byte
			if err := rows.Scan(&seq, &id, &passID, &gateID, &guardID, &scanTime,
				&direction, &outcome, &reason, &prevHash, &hash, &anomalyScore, &anomalyReasons); err != nil {
				_ = rows.Close()
				writeErr(w, http.StatusInternalServerError, "Could not load access logs.")
				return
			}
			var reasons any
			if len(anomalyReasons) > 0 {
				if err := json.Unmarshal(anomalyReasons, &reasons); err != nil {
					_ = rows.Close()
					writeErr(w, http.StatusInternalServerError, "Could not load access logs.")
					return
				}
			}
			items = append(items, map[string]any{
				"seq": seq, "id": id, "pass_id": nullString(passID), "gate_id": nullString(gateID),
				"guard_id": nullString(guardID), "scan_time": scanTime.UTC(), "direction": direction,
				"outcome": outcome, "reason": reason, "prev_hash": prevHash, "hash": hash,
				"anomaly_score": nullableFloat(anomalyScore), "anomaly_reasons": reasons,
			})
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			writeErr(w, http.StatusInternalServerError, "Could not load access logs.")
			return
		}
		_ = rows.Close()
		if len(items) == limit {
			nextCursor = items[len(items)-1]["seq"]
		}
		chainOK, brokenAt, err := db.VerifyAccessChain(ctx, tx, 100000)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not verify access log chain.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": items, "next_cursor": nextCursor,
			"integrity": map[string]any{"ok": chainOK, "broken_at_seq": brokenAt},
		})
	}
}

func VerifyLogs(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tx, err := database.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not verify audit chains.")
			return
		}
		defer tx.Rollback()

		accessOK, accessBrokenAt, err := db.VerifyAccessChain(r.Context(), tx, 100000)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not verify access log chain.")
			return
		}
		auditOK, auditBrokenAt, err := db.VerifyAuditChain(r.Context(), tx, 100000)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not verify audit event chain.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           accessOK && auditOK,
			"access_logs":  map[string]any{"ok": accessOK, "broken_at_seq": accessBrokenAt},
			"audit_events": map[string]any{"ok": auditOK, "broken_at_seq": auditBrokenAt},
		})
	}
}

func parseCursor(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	cursor, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cursor < 1 {
		return 0, strconv.ErrSyntax
	}
	return cursor, nil
}

func nullableFloat(value sql.NullFloat64) any {
	if !value.Valid {
		return nil
	}
	return value.Float64
}

func queryLimit(r *http.Request, fallback int) (int, error) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return fallback, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 100 {
		return 0, strconv.ErrSyntax
	}
	return limit, nil
}
