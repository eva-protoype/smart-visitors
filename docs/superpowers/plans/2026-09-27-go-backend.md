# Go Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace FastAPI with Go in backend/, keep Python only for IsolationForest.

**Architecture:** Go chi monolith + `ml/isolation.py` exec helper for fit/score.

**Tech Stack:** Go 1.21, chi v5, modernc.org/sqlite, google/uuid, Python sklearn/numpy.

**Spec:** docs/superpowers/specs/2026-09-27-go-backend-design.md

## Global Constraints
- Same 7 endpoint contracts + status codes as task.md §3.
- SQLite schema identical (users, passes, access_logs, UUID strings, naive UTC).
- Rule weights/thresholds verbatim: 0.45/0.25/0.20/0.10, -0.05 weekend, -0.3 threshold, MIN 10 rows.
- Text2sql validator order verbatim, limit cap 50, flags limit cap 200.
- Python only in ml/isolation.py; Go degrades to rules if helper fails.

## Review Focus
- Off-hours wrap 22-6 across midnight must flag 02:00 but not 10:00.
- Single-use pass: second scan denied used, still logs row.
- Unknown gate word after "at" warns but searches all gates.
- Stale pickle deleted when DB reset below 10 rows.
- /openapi.json returns 200 for Streamlit healthcheck.

---
### Task 1: Config + DB + models
**Files:**
- Create: `backend/go.mod`, `backend/main.go`, `backend/internal/config/config.go`, `backend/internal/db/db.go`, `backend/internal/models/models.go`
**Interfaces:**
- Consumes: env vars DB_CONNECTION, ANOMALY_CONTAMINATION, AI_ARTIFACT_DIR, AI_MAX_QUERY_ROWS, LLM_PROVIDER
- Produces: `config.Load() Config`, `db.Open(path) *sql.DB + Migrate(db)`, models User/Pass/AccessLog
- [ ] Step 1: Init go.mod chi, sqlite, uuid; write config with defaults matching settings.py
- [ ] Step 2: Implement sqlite open (parse sqlite:///./x.db -> path, mkdir data dir), migrate 3 tables with indexes
- [ ] Step 3: Run `go build ./...` Expected: PASS
- [ ] Step 4: Commit

### Task 2: Users + Passes handlers
**Files:**
- Create: `backend/internal/handlers/users.go`, `backend/internal/handlers/passes.go`
- Modify: `backend/main.go: routes`
**Interfaces:**
- Consumes: db, models
- Produces: `POST /users/create`, `POST /passes/create`, `POST /passes/scan`
- [ ] Step 1: Users create with 409 on username/aadhar/email collision, email validate, role default host
- [ ] Step 2: Passes create 404 host missing, uuid qr_token, valid_until parse RFC3339
- [ ] Step 3: Scan decision order verbatim + always log except unknown token 404
- [ ] Step 4: Smoke via curl Expected: create->create pass->scan approved->scan denied used
- [ ] Step 5: Commit

### Task 3: AI rules + text2sql port
**Files:**
- Create: `backend/internal/ai/anomaly.go`, `backend/internal/ai/text2sql.go`
**Interfaces:**
- Consumes: db rows
- Produces: `ExtractFeatures(), RuleScore(), BuildQuery(), ValidateSQL(), RunQuery()`
- [ ] Step 1: Port is_off_hours, hours_between, safe_limit, 7 features, reasons strings exact
- [ ] Step 2: Port 6 NL families first-match-wins + date/gate extraction + validator order
- [ ] Step 3: Unit check via `go vet` + manual query tests
- [ ] Step 4: Commit

### Task 4: Python ML helper + Go bridge
**Files:**
- Create: `backend/ml/isolation.py`, `backend/internal/ai/forest.go`
**Interfaces:**
- Consumes: feature matrix JSON via stdin
- Produces: `Train(features) {threshold}`, `Score(features) {scores}`, pickle bundle read/write
- [ ] Step 1: isolation.py train/score subcommands using sklearn, pickle {model,gate_freq,threshold,...}
- [ ] Step 2: Go exec python3 with timeout, fallback rules on error/missing sklearn
- [ ] Step 3: Verify train with >=10 rows yields isolation_forest artifact
- [ ] Step 4: Commit

### Task 5: AI HTTP + openapi + docker
**Files:**
- Create: `backend/internal/handlers/ai.go`
- Modify: `backend/main.go`, `backend/Dockerfile`, `backend/.dockerignore`
**Interfaces:**
- Produces: `POST /ai/anomaly/train`, `GET /ai/anomaly/flags`, `GET /ai/anomaly/score/{id}`, `POST /ai/query`, `GET /openapi.json`
- [ ] Step 1: Wire AI routes + sorting + clamping
- [ ] Step 2: Dockerfile multi-stage Go + python3 sklearn runtime
- [ ] Step 3: Full smoke: all task.md §6 curls + frontend healthcheck
- [ ] Step 4: Commit
