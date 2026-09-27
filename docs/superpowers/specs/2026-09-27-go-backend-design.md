# Smart Visitors Go Backend Design

> Approved: replace backend in place. Go monolith + Python exec helper.

**Goal:** Same 7 endpoints, same SQLite schema, frontend untouched.

**Architecture:** Go owns HTTP+DB+text2sql+rules. Python `ml/isolation.py` owns IsolationForest fit/score via stdin/stdout JSON. Pickle bundle stays compatible. Missing python/sklearn degrades to rules, never 500.

**Tech Stack:** Go 1.21+, chi router, modernc.org/sqlite (pure Go, no cgo), google/uuid, Python3 + sklearn + numpy for ML only.

**Spec sources:** task.md §3-§4, README Phase2, VISITOR_MANAGEMENT_SYSTEM_EXPLAINED.md §5.

## Layout
```
backend/
  main.go                # chi router, migrate, mount
  go.mod / go.sum
  Dockerfile             # Go build + python sklearn runtime
  ml/isolation.py        # train/score helper, no FastAPI
  internal/config/       # env: DB_CONNECTION, ANOMALY_*, AI_*, LLM_PROVIDER
  internal/db/           # sqlite open, migrate 3 tables
  internal/models/       # User, Pass, AccessLog structs
  internal/ai/           # anomaly rules + text2sql port
  internal/handlers/     # users, passes, ai HTTP
```

## Contracts (verbatim)
- POST /users/create -> 200 row / 409 duplicate username/aadhar/email
- POST /passes/create -> 200 row / 404 host missing
- POST /passes/scan {qr_token, gate_id=Main Gate} -> {access, reason, log_id}, decision order: 404 unknown, revoked, used, expired/valid_until<now, else approved single-use active->used
- POST /ai/anomaly/train {} -> {model: isolation_forest|rules, n_samples, n_features:7, contamination, artifact}
- GET /ai/anomaly/flags?limit&only_suspicious -> {model, count, flags[]{log_id,pass_id,gate_id,scan_time,status,anomaly_score,is_suspicious,reasons[]}}, sorted lowest-score-first, limit clamp 1-200
- GET /ai/anomaly/score/{log_id} -> single flag / 404
- POST /ai/query {question, limit} -> {sql,params,columns,rows,answer,warnings}, 6 families first-match-wins, validator order: SELECT-only, blocklist, allowlist tables, no SELECT *, no aadhar, limit min(req,50), parametrised
- GET /openapi.json stub + GET /docs minimal (frontend healthcheck)

## ML parity
Features 7 dims exact: hour/23, off_hours(22-6), weekend, gate_rarity, velocity_1h/10, hopping_24h/10, is_denied. Rule score = -(0.45*off+0.25*min(v,3)/3+0.20*min(max(h*10-1,0),3)/3+0.10*denied)-0.05 weekend, suspicious <= -0.3. Reasons exact strings. Threshold = contamination percentile. Artifact `artifacts/isolation_forest.pkl`, delete stale on <10 rows.

## Safety
Naive UTC everywhere. No aadhar in query outputs. No auth (Phase3). No cron (on-demand train).
