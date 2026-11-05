package ai

import (
	"strings"
	"testing"
)

func TestVisitorQuestionUsesNonPIIProjection(t *testing.T) {
	query, _, columns, _, err := BuildQuery("how many visitors?", nil, 50)
	if err != nil {
		t.Fatalf("build visitor count: %v", err)
	}
	if !strings.Contains(query, "FROM visitors") || strings.Contains(strings.ToLower(query), "name") {
		t.Fatalf("visitor query contains an unexpected projection: %s", query)
	}
	if len(columns) != 1 || columns[0] != "count" {
		t.Fatalf("unexpected visitor query columns: %v", columns)
	}
}

func TestValidateSQLBlocksPIIAndWrites(t *testing.T) {
	for _, query := range []string{
		"SELECT id_note FROM visitors",
		"SELECT name FROM visitors",
		"DROP TABLE users",
		"SELECT id FROM visitors; DELETE FROM users",
	} {
		if err := ValidateSQL(query); err == nil {
			t.Errorf("expected query to be rejected: %s", query)
		}
	}
}

func TestToPositionalUsesPostgresPlaceholders(t *testing.T) {
	query, args := toPositional("SELECT id FROM passes WHERE status = :status_0 LIMIT :limit_0", map[string]any{
		"status_0": "active",
		"limit_0":  10,
	})
	if query != "SELECT id FROM passes WHERE status = $1 LIMIT $2" {
		t.Fatalf("unexpected PostgreSQL query: %s", query)
	}
	if len(args) != 2 || args[0] != "active" || args[1] != 10 {
		t.Fatalf("unexpected query arguments: %#v", args)
	}
}
