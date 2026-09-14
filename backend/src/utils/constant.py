# Step 2 (Phase 2 AI) shared constants.
# Kept in utils so passes/user/ai modules share one source of truth.

OFF_HOUR_START = 22  # 22:00 inclusive
OFF_HOUR_END = 6  # 06:00 exclusive

KNOWN_GATES = ["Main Gate", "Back Gate", "East Wing", "Parking"]

# Minimum access_logs rows needed to fit IsolationForest; below this use rules.
MIN_TRAIN_SAMPLES = 10

# Rule-fallback score <= this is suspicious (scores are in [-1, 0]).
RULE_SUSPICIOUS_THRESHOLD = -0.3

# Text-to-SQL safety allowlists. users.aadhar_number is deliberately excluded.
ALLOWED_TABLES = ("access_logs", "passes", "users")

ALLOWED_COLUMNS = {
    "access_logs": ("id", "pass_id", "gate_id", "scan_time", "status"),
    "passes": ("id", "visitor_name", "visitor_email", "qr_token", "status", "host_user_id", "valid_until"),
    "users": ("id", "username", "email", "role"),
}

BLOCKED_SQL_KEYWORDS = (
    "insert",
    "update",
    "delete",
    "drop",
    "alter",
    "create",
    "replace",
    "truncate",
    "pragma",
    "attach",
    "detach",
    "vacuum",
    "grant",
    "revoke",
    "--",
    ";",
)

EXAMPLE_QUESTIONS = [
    "how many denied entries today?",
    "how many approved scans last 7 days at Main Gate?",
    "show last 10 denied scans",
    "entries by gate",
    "how many active passes?",
    "how many visitors?",
]
