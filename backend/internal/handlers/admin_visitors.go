package handlers

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"net/http"
	"strings"
	"time"

	"smart-visitors/internal/config"
)

type adminVisitorRow struct {
	VisitorID  string    `json:"visitor_id"`
	Name       string    `json:"name"`
	Phone      *string   `json:"phone,omitempty"`
	HostName   string    `json:"host_name"`
	HostUser   string    `json:"host_username"`
	Purpose    string    `json:"purpose"`
	PassStatus string    `json:"pass_status"`
	GateName   string    `json:"gate_name,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ValidFrom  time.Time `json:"valid_from"`
	ValidUntil time.Time `json:"valid_until"`
}

func AdminVisitors(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to, err := visitorExportWindow(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		rows, err := loadAdminVisitors(r, database, cfg, from, to)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load visitor records.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": rows, "from": from.UTC(), "to": to.UTC(),
		})
	}
}

func AdminVisitorsCSV(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to, err := visitorExportWindow(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		rows, err := loadAdminVisitors(r, database, cfg, from, to)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not export visitor records.")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="visitor-records.csv"`)
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{
			"visitor_id", "visitor_name", "phone", "host_name", "host_username",
			"purpose", "pass_status", "gate", "created_at", "valid_from", "valid_until",
		})
		for _, row := range rows {
			phone := ""
			if row.Phone != nil {
				phone = *row.Phone
			}
			_ = writer.Write([]string{
				row.VisitorID, row.Name, phone, row.HostName, row.HostUser,
				row.Purpose, row.PassStatus, row.GateName,
				row.CreatedAt.UTC().Format(time.RFC3339),
				row.ValidFrom.UTC().Format(time.RFC3339),
				row.ValidUntil.UTC().Format(time.RFC3339),
			})
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return
		}
	}
}

func visitorExportWindow(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30)
	to := now.Add(time.Second)
	var err error
	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		from, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("from must be an RFC3339 date-time.")
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		to, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("to must be an RFC3339 date-time.")
		}
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, errors.New("to must be later than from.")
	}
	return from.UTC(), to.UTC(), nil
}

func loadAdminVisitors(r *http.Request, database *sql.DB, cfg config.Config, from, to time.Time) ([]adminVisitorRow, error) {
	rows, err := database.QueryContext(r.Context(), `
		SELECT v.id::text, v.name, v.phone, h.name, h.username, p.purpose,
		       p.status, COALESCE(g.name, ''), p.created_at, p.valid_from, p.valid_until
		FROM passes p
		JOIN visitors v ON v.id = p.visitor_id
		JOIN users h ON h.id = p.host_id
		LEFT JOIN gates g ON g.id = p.gate_id
		WHERE p.created_at >= $1 AND p.created_at < $2
		ORDER BY p.created_at DESC
		LIMIT $3`, from, to, 5000)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]adminVisitorRow, 0)
	for rows.Next() {
		var item adminVisitorRow
		var storedName string
		var storedPhone sql.NullString
		if err := rows.Scan(
			&item.VisitorID, &storedName, &storedPhone, &item.HostName, &item.HostUser,
			&item.Purpose, &item.PassStatus, &item.GateName, &item.CreatedAt,
			&item.ValidFrom, &item.ValidUntil,
		); err != nil {
			return nil, err
		}
		item.Name, err = decryptVisitorName(cfg.MasterKey, item.VisitorID, storedName)
		if err != nil {
			return nil, err
		}
		if storedPhone.Valid {
			phone, err := openVisitorValue(cfg.MasterKey, item.VisitorID, "phone", storedPhone.String)
			if err != nil {
				return nil, err
			}
			item.Phone = &phone
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
