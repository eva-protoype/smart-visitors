package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PassCreate struct {
	VisitorName  string `json:"visitor_name"`
	VisitorEmail string `json:"visitor_email"`
	HostUserID   string `json:"host_user_id"`
	ValidUntil   string `json:"valid_until"`
}

func parseValidUntil(s string) (time.Time, bool) {
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func PassesCreate(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p PassCreate
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeErr(w, 422, "Invalid JSON")
			return
		}
		if strings.TrimSpace(p.VisitorName) == "" || strings.TrimSpace(p.VisitorEmail) == "" || strings.TrimSpace(p.HostUserID) == "" || strings.TrimSpace(p.ValidUntil) == "" {
			writeErr(w, 422, "visitor_name, visitor_email, host_user_id, valid_until required")
			return
		}
		if !strings.Contains(p.VisitorEmail, "@") {
			writeErr(w, 422, "value is not a valid email address")
			return
		}
		vu, ok := parseValidUntil(p.ValidUntil)
		if !ok {
			writeErr(w, 422, "Invalid valid_until datetime")
			return
		}
		var cnt int
		if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, p.HostUserID).Scan(&cnt); err != nil || cnt == 0 {
			writeErr(w, 404, "Host user not found")
			return
		}
		id := uuid.NewString()
		qr := uuid.NewString()
		vuStr := vu.Format("2006-01-02T15:04:05")
		if _, err := db.Exec(`INSERT INTO passes(id, visitor_name, visitor_email, qr_token, status, host_user_id, valid_until) VALUES(?,?,?,?,?,?,?)`,
			id, p.VisitorName, p.VisitorEmail, qr, "active", p.HostUserID, vuStr); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{
			"id": id, "visitor_name": p.VisitorName, "visitor_email": p.VisitorEmail,
			"qr_token": qr, "status": "active", "host_user_id": p.HostUserID,
			"valid_until": vu.Format(time.RFC3339),
		})
	}
}

type PassScan struct {
	QRToken string `json:"qr_token"`
	GateID  string `json:"gate_id"`
}

func PassesScan(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p PassScan
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeErr(w, 422, "Invalid JSON")
			return
		}
		if strings.TrimSpace(p.QRToken) == "" {
			writeErr(w, 422, "qr_token required")
			return
		}
		gate := p.GateID
		if gate == "" {
			gate = "Main Gate"
		}
		var passID, status string
		var validUntil sql.NullString
		err := db.QueryRow(`SELECT id, status, valid_until FROM passes WHERE qr_token=?`, p.QRToken).Scan(&passID, &status, &validUntil)
		if err == sql.ErrNoRows {
			writeErr(w, 404, "Pass not found")
			return
		}
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		now := time.Now().UTC()
		scanStr := now.Format("2006-01-02T15:04:05")
		access := "denied"
		reason := ""
		newStatus := status
		switch {
		case status == "revoked":
			reason = "Pass has been revoked"
		case status == "used":
			reason = "Pass has already been used"
		case status == "expired":
			reason = "Pass has expired"
			newStatus = "expired"
		default:
			expired := false
			if validUntil.Valid && validUntil.String != "" {
				if vu, ok := parseValidUntil(validUntil.String); ok && vu.Before(now) {
					expired = true
				}
			}
			if expired {
				newStatus = "expired"
				reason = "Pass has expired"
			} else {
				newStatus = "used"
				access = "approved"
				reason = "Access granted"
			}
		}
		if _, err := db.Exec(`UPDATE passes SET status=? WHERE id=?`, newStatus, passID); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		logID := uuid.NewString()
		if _, err := db.Exec(`INSERT INTO access_logs(id, pass_id, gate_id, scan_time, status) VALUES(?,?,?,?,?)`,
			logID, passID, gate, scanStr, access); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"access": access, "reason": reason, "log_id": logID})
	}
}
