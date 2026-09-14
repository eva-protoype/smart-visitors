# task.md — Smart Visitors: Step 2 (Phase 2 AI Integrations)

> Single source of truth for Step 2. Created after repo analysis on 2026-09-13.
> Repo root: `/home/anany/Secondary Projects/smart-visitors` | Branch: `main` | HEAD: `bd4dd5f`

---

## 1. Repo analysis (as-found state)

### 1.1 Structure

```
smart-visitors/
├── README.md
├── .gitignore
├── task.md                  # THIS FILE (added in Step 2)
└── backend/
    ├── main.py              # FastAPI app + Base.metadata.create_all + router wiring
    ├── requirement.txt      # fastapi, uvicorn[standard], sqlalchemy, pydantic-settings, email-validator
    └── src/
        ├── user/
        │   ├── models.py    # UserModel + UserRole(visitor/host/guard/admin)
        │   ├── dtos.py      # UserCreateSchema
        │   ├── controller.py# create_user (409 on duplicate username/aadhar/email)
        │   └── router.py    # POST /users/create
        ├── passes/
        │   ├── models.py    # PassModel + PassStatus(active/used/expired/revoked) + AccessLogModel
        │   ├── dtos.py      # PassCreateSchema, PassScanSchema
        │   ├── controller.py# create_visitor_pass, scan_visitor_pass (approve/deny + audit log)
        │   └── router.py    # POST /passes/create, POST /passes/scan
        └── utils/
            ├── db.py        # engine/SessionLocal/Base/get_db (SQLite check_same_thread handled)
            ├── settings.py  # Settings.DB_CONNECTION (default sqlite:///./smart_visitors.db)
            ├── constant.py  # EMPTY (placeholder)
            └── helpers.py   # EMPTY (placeholder)
```

### 1.2 Tech stack (actual)

- FastAPI (Python 3.11+ declared, env here is 3.14), SQLAlchemy >= 2.0 (but code uses legacy `Column` API), Pydantic v2 + `pydantic-settings`, SQLite dev.
- No frontend, no tests, no auth/JWT, no migrations (sync `create_all`), no ML deps yet.

### 1.3 Phase 1 — MVP (completed, verified by reading code)

| Feature (README §Phase 1) | Status | Implementation |
|---|---|---|
| RBAC roles | ✅ Done (model-level only) | `src/user/models.py:UserRole`, `UserModel.role` default `HOST`. No route-level enforcement yet. |
| Pass credential management | ✅ Done | `PassModel.qr_token` auto-uuid, `valid_until`, `host_user_id FK → users.id`. `POST /passes/create` 404s if host missing. |
| Access auditing workflow | ✅ Done | `AccessLogModel(pass_id, gate_id, scan_time, status)`; `scan_visitor_pass()` sets `approved/denied`, transitions `active→used/expired`, logs every attempt. `POST /passes/scan`. |

Existing API surface:

| Method | Path | Input | Output |
|---|---|---|---|
| POST | `/users/create` | `UserCreateSchema{username, aadhar_number, email, role}` | `UserModel` row / 409 |
| POST | `/passes/create` | `PassCreateSchema{visitor_name, visitor_email, host_user_id, valid_until}` | `PassModel` row / 404 |
| POST | `/passes/scan` | `PassScanSchema{qr_token, gate_id=Main Gate}` | `{access: approved\|denied, reason, log_id}` |

Data model (FKs): `users.id (pk)` ← `passes.host_user_id`; `passes.id (pk)` ← `access_logs.pass_id`.

### 1.4 Gaps / issues found (inform Step 2 design)

1. `constant.py`, `helpers.py` are empty — natural home for off-hours windows, gate allowlist, time helpers.
2. `requirement.txt` lacks `scikit-learn/numpy`; filename is singular (`requirement.txt` not `requirements.txt`) — kept as-is to avoid breaking setup docs; new deps appended there.
3. `scan_time` uses naive UTC (`replace(tzinfo=None)`); anomaly features must assume naive UTC consistently.
4. No seed data / no trainable history — anomaly model must work with `<10` rows via rule-based fallback.
5. No input sanitisation layer for future LLM SQL — Text-to-SQL must be read-only + allowlisted.
6. No tests, no persistence dir for ML artifacts — Step 2 adds `backend/artifacts/` (gitignored `*.pkl`) + runtime smoke test.

---

## 2. Step 2 definition

**Step 1 = Phase 1 (MVP). Step 2 = Phase 2 (AI Integrations) per README:**

> - Anomaly Detection Engine: lightweight `IsolationForest` to flag suspicious, off-hours, or rapid multi-gate entry attempts.
> - Natural Language Queries: Text-to-SQL LLM assistant for admins to query access logs in plain English.

### 2.1 Objectives

1. Score every gate scan for suspiciousness without changing the scan path (scan stays synchronous; scoring is a separate read API so MVP latency is untouched).
2. Work on day-0 (few rows) via transparent rules, and improve automatically once history exists via `IsolationForest`.
3. Let admins ask plain-English questions over `users/passes/access_logs` and get back **show-the-SQL + rows + one-line answer**, never write-access.
4. Keep it dependency-light (`scikit-learn`, `numpy` only), SQLite-compatible, no API keys required.

### 2.2 Scope IN / OUT

IN:
- `src/ai/` module: `anomaly.py`, `text2sql.py`, `dtos.py`, `controller.py`, `router.py` (+ `__init__.py`).
- Constants/helpers fill-in, settings additions, `main.py` wiring, `requirement.txt` additions.
- 4 new endpoints (see §3). Disk persistence of IsolationForest via `pickle`.

OUT (explicit non-goals for Step 2):
- No JWT/RBAC enforcement, no frontend, no Postgres migration, no real LLM API call (hook left as `LLM_PROVIDER=none`; rule parser is the default so it works offline).
- No retraining scheduler/cron — training is on-demand `POST /ai/anomaly/train`.
- No PII redaction beyond not echoing `aadhar_number` in Text-to-SQL outputs.

### 2.3 Subtasks

- [x] 2.0 Repo analysis + this `task.md`
- [ ] 2.1 `utils/constant.py`: `OFF_HOUR_START=22`, `OFF_HOUR_END=6`, `KNOWN_GATES`, `ANOMALY_*` thresholds
- [ ] 2.2 `utils/helpers.py`: `is_off_hours(dt)`, `hours_between(a,b)`, `safe_limit(n)`
- [ ] 2.3 `utils/settings.py`: `ANOMALY_CONTAMINATION=0.1`, `AI_ARTIFACT_DIR=./artifacts`, `AI_MAX_QUERY_ROWS=50`, `LLM_PROVIDER=none`
- [ ] 2.4 `src/ai/anomaly.py`: feature builder + `IsolationForest` train/score + rule fallback + explanations + pickle persistence
- [ ] 2.5 `src/ai/text2sql.py`: NL pattern matcher → parametrised SELECT (allowlisted tables/columns) → executor → answer formatter
- [ ] 2.6 `src/ai/{dtos,controller,router}.py`: schemas + orchestration + routes
- [ ] 2.7 Wire `ai_router` in `backend/main.py`; append `scikit-learn`, `numpy` to `requirement.txt`
- [ ] 2.8 Verify: `uv run` import check + seed demo data + `POST /passes/scan` + `POST /ai/anomaly/train` + `GET /ai/anomaly/flags` + `POST /ai/query` smoke tests

---

## 3. API contracts (Step 2 additions, prefix `/ai`)

### 3.1 `POST /ai/anomaly/train`

Trains (or retrains) IsolationForest on all `access_logs`. Works with ≥10 logs; with fewer rows returns `model: "rules"` and does not persist.

Request: `{}` (empty JSON).
Response 200:
```json
{ "model": "isolation_forest", "n_samples": 42, "n_features": 7, "contamination": 0.1, "artifact": "artifacts/isolation_forest.pkl" }
```

### 3.2 `GET /ai/anomaly/flags?limit=20&only_suspicious=true`

Scores recent logs (most recent `limit`, max 200). Each item carries score + reasons so guards can audit.

Response 200:
```json
{
  "model": "isolation_forest",
  "count": 3,
  "flags": [
    { "log_id": "…", "pass_id": "…", "gate_id": "Back Gate", "scan_time": "2026-09-13T02:14:00",
      "status": "approved", "anomaly_score": -0.21, "is_suspicious": true,
      "reasons": ["off-hours entry (02:00)", "gate hopping: 3 gates in 24h"] }
  ]
}
```
Convention: lower `anomaly_score` (IsolationForest `score_samples`, negative = stranger) ⇒ more suspicious. Rule-fallback emits `anomaly_score` in `[-1, 0]` for comparability.

### 3.3 `GET /ai/anomaly/score/{log_id}`

Scores one log. 404 if unknown id. Same item shape as §3.2 (single object).

### 3.4 `POST /ai/query`

Admin NL → safe read-only SQL.

Request:
```json
{ "question": "how many denied entries today at Main Gate?", "limit": 20 }
```
Response 200:
```json
{
  "sql": "SELECT COUNT(*) AS denied_today FROM access_logs WHERE status='denied' AND date(scan_time)=date('now') AND gate_id=:gate_0",
  "params": { "gate_0": "Main Gate" },
  "columns": ["denied_today"],
  "rows": [[3]],
  "answer": "3 denied entries today at Main Gate.",
  "warnings": []
}
```

Supported question families (case-insensitive, extensible in `text2sql.py:PATTERNS`):
1. counts by status (`how many denied/approved … [today|yesterday|last N days] [at GATE]`)
2. recent logs (`show last N scans / denied entries`)
3. per-gate breakdown (`entries by gate`, `top gates`)
4. pass status (`how many active/expired/used/revoked passes`, `passes expiring …`)
5. user counts by role (`how many visitors/hosts/guards/admins`)
6. fallback: unrecognised → `400 {detail: "Could not understand question…", examples:[…]}` (never raw-SQL passthrough).

Safety contract (hard rules):
- Only single `SELECT …` (regex + `sqlparse`-free keyword blocklist: no INSERT/UPDATE/DELETE/DROP/ALTER/PRAGMA/ATTACH).
- Tables allowlist: `access_logs`, `passes`, `users`. Columns allowlist per table; `users.aadhar_number` is **never** selected.
- `LIMIT` always applied (`min(request.limit, AI_MAX_QUERY_ROWS)`).
- Parametrised (`:name`) — no f-string interpolation of user text into SQL.
- Read-only execution via the request-scoped `get_db` session, no commit.

---

## 4. ML design — anomaly detection

Features per `access_log` row (7 dims, see `anomaly.py:build_features`):
1. `hour` (0–23, from naive-UTC `scan_time`)
2. `is_off_hours` (1 if hour ≥22 or <6)
3. `is_weekend` (Sat/Sun)
4. `gate_rarity` (1 − gate frequency in training set; unseen gate = 1.0)
5. `pass_velocity_1h` (scans of same `pass_id` within ±1h window, capped at 10)
6. `gate_hopping_24h` (distinct gates for same `pass_id` in 24h, capped at 10)
7. `is_denied` (1 if `status=denied`)

Model: `sklearn.ensemble.IsolationForest(contamination=settings.ANOMALY_CONTAMINATION, random_state=42)`.
Threshold: `score < percentile(contamination)` on training scores ⇒ suspicious (persisted with model).
Fallback (n < 10): rule score = `−(0.45·off_hours + 0.25·min(velocity,3)/3 + 0.2·min(hopping−1,3)/3 + 0.1·denied)`; suspicious if ≤ −0.3. Reasons always emitted (`off-hours entry`, `rapid repeat scans: N in 1h`, `gate hopping`, `denied entry`, `rare gate`).
Persistence: `pickle` dict `{model, gate_freq, threshold, n_samples, trained_at}` → `artifacts/isolation_forest.pkl`; loaded lazily, missing file ⇒ rules.
Explainability: every flag carries human `reasons[]`; no black-box boolean alone.

---

## 5. File changes (Step 2)

| File | Change |
|---|---|
| `task.md` (root) | NEW — this spec |
| `backend/requirement.txt` | ADD `scikit-learn>=1.3`, `numpy>=1.26` |
| `backend/src/utils/constant.py` | FILL — off-hours, gates, thresholds, allowlists |
| `backend/src/utils/helpers.py` | FILL — `is_off_hours`, `hours_between`, `safe_limit` |
| `backend/src/utils/settings.py` | ADD `ANOMALY_CONTAMINATION`, `AI_ARTIFACT_DIR`, `AI_MAX_QUERY_ROWS`, `LLM_PROVIDER` |
| `backend/src/ai/__init__.py` | NEW — empty |
| `backend/src/ai/dtos.py` | NEW — `AnomalyFlag`, `AnomalyFlagsResponse`, `TrainResponse`, `NLQueryRequest/Response` |
| `backend/src/ai/anomaly.py` | NEW — features, train, score, persistence |
| `backend/src/ai/text2sql.py` | NEW — patterns, SQL builder, executor, answer formatter |
| `backend/src/ai/controller.py` | NEW — thin orchestration over `anomaly.py` + `text2sql.py` |
| `backend/src/ai/router.py` | NEW — 4 routes (§3) |
| `backend/main.py` | EDIT — `app.include_router(ai_router)` |
| `backend/artifacts/` | RUNTIME dir (gitignored `*.pkl`) |

No changes to `user/` or `passes/` logic.

---

## 6. Acceptance criteria

- [ ] `POST /ai/anomaly/train` → 200 on empty DB (`model: rules`) and on ≥10 seeded logs (`model: isolation_forest`, artifact file exists).
- [ ] `GET /ai/anomaly/flags` → 200, items contain `anomaly_score`, `is_suspicious`, `reasons[]`; an off-hours + gate-hopping seed is flagged suspicious.
- [ ] `POST /ai/query` → 200 with `sql+rows+answer` for ≥5 canonical questions (§3.4 families); `users.aadhar_number` never appears; destructive/unparsable input → 400, never executes.
- [ ] Existing routes unaffected: `/users/create`, `/passes/create`, `/passes/scan` still pass smoke test.
- [ ] `uv run python -c "from src.ai.router import router"` imports cleanly; no new `ruff`/syntax errors.

Verify (from `backend/`):
```bash
uv sync || uv pip install -r requirement.txt
uv run uvicorn main:app --reload
# smoke:
curl -X POST localhost:8000/ai/anomaly/train -H 'Content-Type: application/json' -d '{}'
curl 'localhost:8000/ai/anomaly/flags?limit=20'
curl -X POST localhost:8000/ai/query -H 'Content-Type: application/json' -d '{"question":"how many denied entries today?"}'
```

## 7. Risks & next steps (out of scope, noted)

- Timezone-naive datetimes skew off-hours flags if servers move TZ → migrate to TZ-aware (Phase 3).
- No auth on `/ai/query` → gate behind ADMIN role once JWT lands (Phase 3).
- Rule parser covers common questions only → plug `LLM_PROVIDER=openai|local` generating SQL **through the same allowlist validator** (Phase 3).
- No scheduled retraining → add nightly job + precision/recall eval once labels exist (Phase 3).
