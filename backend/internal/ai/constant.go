package ai

// Mirrors backend/src/utils/constant.py verbatim.
const (
	OffHourStart = 22
	OffHourEnd   = 6

	MinTrainSamples         = 10
	RuleSuspiciousThreshold = -0.3
	NFeatures               = 7
	ArtifactName            = "isolation_forest.pkl"
)

var KnownGates = []string{"Main Gate", "Back Gate", "East Wing", "Parking"}

var AllowedTables = map[string]bool{
	"access_logs": true,
	"passes":      true,
	"users":       true,
}

var AllowedColumns = map[string]map[string]bool{
	"access_logs": {"id": true, "pass_id": true, "gate_id": true, "scan_time": true, "status": true},
	"passes":      {"id": true, "visitor_name": true, "visitor_email": true, "qr_token": true, "status": true, "host_user_id": true, "valid_until": true},
	"users":       {"id": true, "username": true, "email": true, "role": true},
}

var BlockedSQLKeywords = []string{
	"insert", "update", "delete", "drop", "alter", "create",
	"replace", "truncate", "pragma", "attach", "detach",
	"vacuum", "grant", "revoke", "--", ";",
}

var ExampleQuestions = []string{
	"how many denied entries today?",
	"how many approved scans last 7 days at Main Gate?",
	"show last 10 denied scans",
	"entries by gate",
	"how many active passes?",
	"how many visitors?",
}
