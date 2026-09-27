package ai

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type APIError struct {
	Code   int
	Detail string
}

func (e *APIError) Error() string { return e.Detail }

func httpErr(code int, detail string) *APIError { return &APIError{Code: code, Detail: detail} }

// ---------- gate + date ----------

var roleMap = [][2]string{
	{"visitors", "visitor"}, {"visitor", "visitor"},
	{"hosts", "host"}, {"host", "host"},
	{"guards", "guard"}, {"guard", "guard"},
	{"admins", "admin"}, {"admin", "admin"},
}

var passStatuses = []string{"active", "used", "expired", "revoked"}

var expiryRe = regexp.MustCompile(`\bexpiring\b|\bexpires?\b|valid until`)
var expiredRe = regexp.MustCompile(`\bexpired\b`)

func FindGate(question string) *string {
	q := strings.ToLower(question)
	for _, gate := range KnownGates {
		if strings.Contains(q, strings.ToLower(gate)) {
			g := gate
			return &g
		}
	}
	re := regexp.MustCompile(`(?:at|for|in|gate)\s+([a-z][a-z\s]+)`)
	m := re.FindStringSubmatch(q)
	if m != nil {
		cand := strings.TrimSpace(m[1])
		// title-case words
		words := strings.Fields(cand)
		for i, w := range words {
			if len(w) > 0 {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
		candidate := strings.Join(words, " ")
		if candidate == "" {
			return nil
		}
		lc := strings.ToLower(candidate)
		if lc == "today" || lc == "yesterday" || lc == "days" {
			return nil
		}
		for _, gate := range KnownGates {
			if candidate == gate {
				g := gate
				return &g
			}
		}
		// Unknown gate name: return nil so caller warns + searches all gates
		// (matches Python: candidate not in KNOWN_GATES -> None).
		_ = candidate
		return nil
	}
	return nil
}

func DateFilter(question string) (frag string, params map[string]any, label string) {
	q := strings.ToLower(question)
	re := regexp.MustCompile(`last\s+(\d+)\s+days?`)
	if m := re.FindStringSubmatch(q); m != nil {
		days, _ := strconv.Atoi(m[1])
		if days < 1 {
			days = 1
		}
		if days > 365 {
			days = 365
		}
		return " AND date(scan_time) >= date('now', :days_ago)",
			map[string]any{"days_ago": fmt.Sprintf("-%d days", days)},
			fmt.Sprintf("last %d days", days)
	}
	if strings.Contains(q, "yesterday") {
		return " AND date(scan_time) = date('now', '-1 day')", map[string]any{}, "yesterday"
	}
	if strings.Contains(q, "today") {
		return " AND date(scan_time) = date('now')", map[string]any{}, "today"
	}
	return "", map[string]any{}, ""
}

// ---------- validator ----------

func ValidateSQL(s string) *APIError {
	t := strings.TrimSpace(strings.ToLower(s))
	if !strings.HasPrefix(t, "select") {
		return httpErr(http.StatusBadRequest, "Only SELECT queries are allowed")
	}
	for _, kw := range BlockedSQLKeywords {
		var found bool
		if kw == "--" || kw == ";" {
			found = strings.Contains(t, kw)
		} else {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(kw) + `\b`)
			found = re.MatchString(t)
		}
		if found {
			return httpErr(http.StatusBadRequest, "Blocked keyword in query: "+kw)
		}
	}
	reTables := regexp.MustCompile(`(?:from|join)\s+([a-z_]+)`)
	for _, m := range reTables.FindAllStringSubmatch(t, -1) {
		if !AllowedTables[m[1]] {
			return httpErr(http.StatusBadRequest, "Table not allowed: "+m[1])
		}
	}
	// Column check: strip COUNT(*) then reject * then check idents.
	parts := strings.SplitN(t, "from", 2)
	selectPart := parts[0]
	reCount := regexp.MustCompile(`\bcount\s*\(\s*\*\s*\)`)
	selectPart = reCount.ReplaceAllString(selectPart, "")
	if strings.Contains(selectPart, "*") {
		return httpErr(http.StatusBadRequest, "Wildcard SELECT is not allowed")
	}
	allowedCols := map[string]bool{}
	for _, cols := range AllowedColumns {
		for c := range cols {
			allowedCols[c] = true
		}
	}
	reAs := regexp.MustCompile(`\bas\s+[a-z_][a-z0-9_]*`)
	projected := reAs.ReplaceAllString(selectPart, "")
	skip := map[string]bool{"select": true, "distinct": true, "all": true, "count": true, "as": true, "asc": true, "desc": true}
	reIdent := regexp.MustCompile(`[a-z_][a-z0-9_]*`)
	for _, m := range reIdent.FindAllString(projected, -1) {
		if skip[m] {
			continue
		}
		if !allowedCols[m] {
			return httpErr(http.StatusBadRequest, "Column not allowed: "+m)
		}
	}
	if strings.Contains(t, "aadhar") {
		return httpErr(http.StatusBadRequest, "aadhar_number is never queryable")
	}
	return nil
}

func ClampQueryLimit(limit *int, maxRows int) int {
	n := 20
	if limit != nil {
		n = *limit
	}
	if n < 1 {
		n = 1
	}
	if n > maxRows {
		n = maxRows
	}
	return n
}

// BuildQuery returns sql, params, columns, scope or APIError.
func BuildQuery(question string, limit *int, maxRows int) (string, map[string]any, []string, string, *APIError) {
	if strings.TrimSpace(question) == "" {
		return "", nil, nil, "", httpErr(http.StatusBadRequest, "Question must not be empty")
	}
	q := strings.ToLower(strings.TrimSpace(question))
	lim := ClampQueryLimit(limit, maxRows)
	gate := FindGate(question)
	dateSQL, dateParams, dateLabel := DateFilter(question)
	gateSQL := ""
	gateParams := map[string]any{}
	if gate != nil {
		gateSQL = " AND gate_id = :gate_0"
		gateParams["gate_0"] = *gate
	}
	whereSuffix := dateSQL + gateSQL
	params := map[string]any{}
	for k, v := range dateParams {
		params[k] = v
	}
	for k, v := range gateParams {
		params[k] = v
	}

	// 1. users by role
	for _, rm := range roleMap {
		plural, role := rm[0], rm[1]
		re := regexp.MustCompile(`how many\s+` + regexp.QuoteMeta(plural) + `\b`)
		if re.MatchString(q) {
			return "SELECT COUNT(*) AS count FROM users WHERE role = :role_0 LIMIT 1",
				map[string]any{"role_0": role}, []string{"count"}, plural + " count", nil
		}
	}
	// 2. pass status
	for _, st := range passStatuses {
		re := regexp.MustCompile(`how many\s+` + st + `\s+passes?`)
		if re.MatchString(q) {
			return "SELECT COUNT(*) AS count FROM passes WHERE status = :status_0 LIMIT 1",
				map[string]any{"status_0": st}, []string{"count"}, st + " passes", nil
		}
	}
	reTotalPass := regexp.MustCompile(`how many\s+passes?`)
	if reTotalPass.MatchString(q) && !expiryRe.MatchString(q) {
		return "SELECT COUNT(*) AS count FROM passes LIMIT 1", map[string]any{}, []string{"count"}, "total passes", nil
	}
	// 3. entries by gate
	reGate := regexp.MustCompile(`(entries by gate|by gate|top gates|breakdown by gate|per gate)`)
	if reGate.MatchString(q) {
		p := map[string]any{}
		for k, v := range dateParams {
			p[k] = v
		}
		p["limit_0"] = lim
		return "SELECT gate_id, COUNT(*) AS entries FROM access_logs" + dateSQL + " GROUP BY gate_id ORDER BY entries DESC LIMIT :limit_0",
			p, []string{"gate_id", "entries"}, "entries by gate", nil
	}
	// 4. counts by scan status
	reCount := regexp.MustCompile(`how many\s+(denied|approved|total)?\s*(entries|entry|scans?|logs?|attempts?)?`)
	if m := reCount.FindStringSubmatch(q); m != nil && (strings.Contains(q, "denied") || strings.Contains(q, "approved") || strings.Contains(q, "entr") || strings.Contains(q, "scan") || strings.Contains(q, "log") || strings.Contains(q, "attempt")) {
		status := m[1]
		if status == "denied" || status == "approved" {
			scope := status + " entries"
			if dateLabel != "" {
				scope += " " + dateLabel
			}
			if gate != nil {
				scope += " at " + *gate
			}
			p := map[string]any{"status_0": status}
			for k, v := range params {
				p[k] = v
			}
			return "SELECT COUNT(*) AS count FROM access_logs WHERE status = :status_0" + whereSuffix + " LIMIT 1",
				p, []string{"count"}, scope, nil
		}
		scope := "entries"
		if dateLabel != "" {
			scope += " " + dateLabel
		}
		if gate != nil {
			scope += " at " + *gate
		}
		where := whereSuffix
		if strings.HasPrefix(where, " AND ") {
			where = where[5:]
		}
		s := "SELECT COUNT(*) AS count FROM access_logs"
		if where != "" {
			s += " WHERE " + where
		}
		return s + " LIMIT 1", params, []string{"count"}, scope, nil
	}
	// 5. recent logs
	hasShow := strings.Contains(q, "show") || strings.Contains(q, "list") || strings.Contains(q, "recent") || strings.Contains(q, "last")
	hasLog := strings.Contains(q, "scan") || strings.Contains(q, "log") || strings.Contains(q, "entr") || strings.Contains(q, "denied") || strings.Contains(q, "approved") || strings.Contains(q, "attempt") || strings.Contains(q, "recent") || strings.Contains(q, "last")
	if hasShow && hasLog {
		reN := regexp.MustCompile(`(last|recent|show|list)\s+(\d+)`)
		n := lim
		if m := reN.FindStringSubmatch(q); m != nil {
			if v, err := strconv.Atoi(m[2]); err == nil {
				nn := v
				if nn < 1 {
					nn = 1
				}
				if nn > maxRows {
					nn = maxRows
				}
				n = nn
			}
		}
		cols := []string{"id", "pass_id", "gate_id", "scan_time", "status"}
		if strings.Contains(q, "denied") || strings.Contains(q, "approved") {
			sw := "denied"
			if strings.Contains(q, "approved") && !strings.Contains(q, "denied") {
				sw = "approved"
			} else if strings.Contains(q, "denied") {
				sw = "denied"
			}
			scope := fmt.Sprintf("last %d %s scans", n, sw)
			p := map[string]any{"status_0": sw}
			for k, v := range params {
				p[k] = v
			}
			p["limit_0"] = n
			return "SELECT " + strings.Join(cols, ", ") + " FROM access_logs WHERE status = :status_0" + whereSuffix + " ORDER BY scan_time DESC LIMIT :limit_0",
				p, cols, scope, nil
		}
		scope := fmt.Sprintf("last %d scans", n)
		where := whereSuffix
		if strings.HasPrefix(where, " AND ") {
			where = where[5:]
		}
		s := "SELECT " + strings.Join(cols, ", ") + " FROM access_logs"
		if where != "" {
			s += " WHERE " + where
		}
		p := map[string]any{}
		for k, v := range params {
			p[k] = v
		}
		p["limit_0"] = n
		return s + " ORDER BY scan_time DESC LIMIT :limit_0", p, cols, scope, nil
	}
	// 6. expired vs expiring
	if expiredRe.MatchString(q) {
		return "SELECT id, visitor_name, visitor_email, status, valid_until FROM passes WHERE status = 'expired' ORDER BY valid_until DESC LIMIT :limit_0",
			map[string]any{"limit_0": lim},
			[]string{"id", "visitor_name", "visitor_email", "status", "valid_until"},
			"expired passes", nil
	}
	if expiryRe.MatchString(q) {
		return "SELECT id, visitor_name, visitor_email, status, valid_until FROM passes WHERE status = 'active' ORDER BY valid_until ASC LIMIT :limit_0",
			map[string]any{"limit_0": lim},
			[]string{"id", "visitor_name", "visitor_email", "status", "valid_until"},
			"active passes by expiry", nil
	}
	return "", nil, nil, "", httpErr(http.StatusBadRequest, "Could not understand question. Try e.g.: "+ExampleQuestions[0])
}

// RunQuery executes validated SQL with positional args derived from :params.
func RunQuery(db *sql.DB, question string, limit *int, maxRows int) (string, map[string]any, []string, [][]any, string, []string, *APIError) {
	s, params, columns, scope, apiErr := BuildQuery(question, limit, maxRows)
	if apiErr != nil {
		return "", nil, nil, nil, "", nil, apiErr
	}
	if verr := ValidateSQL(s); verr != nil {
		return "", nil, nil, nil, "", nil, verr
	}
	execSQL, args := toPositional(s, params)
	rows, err := db.Query(execSQL, args...)
	if err != nil {
		return "", nil, nil, nil, "", nil, httpErr(http.StatusBadRequest, err.Error())
	}
	defer rows.Close()
	fetchN := ClampQueryLimit(limit, maxRows)
	var out [][]any
	count := 0
	cols, _ := rows.Columns()
	_ = cols
	for rows.Next() && count < fetchN {
		vals := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			break
		}
		row := make([]any, len(columns))
		for i, v := range vals {
			switch t := v.(type) {
			case []byte:
				row[i] = string(t)
			case nil:
				row[i] = nil
			default:
				row[i] = t
			}
		}
		out = append(out, row)
		count++
	}
	var answer string
	if len(columns) == 1 && columns[0] == "count" && len(out) > 0 {
		answer = fmt.Sprintf("%v %s.", out[0][0], scope)
	} else if len(out) == 0 {
		answer = fmt.Sprintf("No results for: %s.", scope)
	} else {
		answer = fmt.Sprintf("%d row(s) for: %s.", len(out), scope)
	}
	var warnings []string
	if FindGate(question) == nil && regexp.MustCompile(`\bat\b`).MatchString(strings.ToLower(question)) {
		warnings = append(warnings, "Gate name not recognised; searched across all gates.")
	}
	if out == nil {
		out = [][]any{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	if params == nil {
		params = map[string]any{}
	}
	return s, params, columns, out, answer, warnings, nil
}

// toPositional rewrites :name to ? in order of first appearance.
func toPositional(s string, params map[string]any) (string, []any) {
	re := regexp.MustCompile(`:[a-z_0-9]+`)
	var args []any
	out := re.ReplaceAllStringFunc(s, func(m string) string {
		key := m[1:]
		if v, ok := params[key]; ok {
			args = append(args, v)
			return "?"
		}
		args = append(args, nil)
		return "?"
	})
	return out, args
}
