package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

var validRoles = map[string]bool{"visitor": true, "host": true, "guard": true, "admin": true}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, detail string) {
	writeJSON(w, code, map[string]string{"detail": detail})
}

type UserCreate struct {
	Username     string `json:"username"`
	AadharNumber string `json:"aadhar_number"`
	Email        string `json:"email"`
	Role         string `json:"role"`
}

func UsersCreate(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p UserCreate
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeErr(w, 422, "Invalid JSON")
			return
		}
		if strings.TrimSpace(p.Username) == "" || strings.TrimSpace(p.AadharNumber) == "" || strings.TrimSpace(p.Email) == "" {
			writeErr(w, 422, "username, aadhar_number, email required")
			return
		}
		if !strings.Contains(p.Email, "@") {
			writeErr(w, 422, "value is not a valid email address")
			return
		}
		role := p.Role
		if role == "" {
			role = "host"
		}
		if !validRoles[role] {
			writeErr(w, 422, "Invalid role")
			return
		}
		var exists int
		err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE username=? OR aadhar_number=? OR email=?`,
			p.Username, p.AadharNumber, p.Email).Scan(&exists)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		if exists > 0 {
			writeErr(w, 409, "User already exists")
			return
		}
		id := uuid.NewString()
		if _, err := db.Exec(`INSERT INTO users(id, username, aadhar_number, email, role) VALUES(?,?,?,?,?)`,
			id, p.Username, p.AadharNumber, p.Email, role); err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				writeErr(w, 409, "User already exists")
				return
			}
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{
			"id": id, "username": p.Username, "aadhar_number": p.AadharNumber,
			"email": p.Email, "role": role,
		})
	}
}
