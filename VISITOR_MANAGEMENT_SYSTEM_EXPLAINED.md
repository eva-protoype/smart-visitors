# Visitor Management System with Anomaly Detection — Explained in Simple English

> This file explains the **Smart Visitors** project in this repo in plain words.
> It covers what the system does, how it is built at a high level, and how each
> small part works at a low level. No prior ML or backend knowledge is assumed.

**Where the code lives:**

- Backend API: `backend/main.py` + `backend/src/`
- Frontend UI: `frontend/app.py`
- Deployment: `docker-compose.yml`, `backend/Dockerfile`, `frontend/Dockerfile`

---

## 1. What is this system? (30-second version)

Think of a **college** with a paper register at the main gate. Parents,
guest lecturers, and job candidates queue up, the guard writes names by hand,
and nobody can search that book later. It is slow, easy to fake, and useless
during an incident enquiry.

This system replaces that register with three simple ideas:

1. **Digital pass instead of paper.** A host (a professor or department staff
   member) creates a time-limited pass for a visitor. The pass has a secret
   code called `qr_token`.
2. **Scan at the gate instead of writing names.** The campus security guard
   types or scans that `qr_token`. The computer instantly answers **allowed**
   or **denied** and writes down what happened.
3. **A helper that watches for strange patterns.** The computer looks at all
   past scans and says things like “this pass was used at 2 AM at 3 different
   campus gates — please check the camera.” It never opens the gate by itself.

There is also a fourth helper: you can **ask questions in normal English**,
like “how many denied entries today?”, and it answers with a number plus the
database query it used, so you can trust it.

---

## 2. A real-life story (in a college)

1. Prof. Priya (a **host**) has invited Ravi Kumar, a guest speaker for
   tomorrow's seminar. She opens the **Users** page once to create her own
   host account, then opens the **Passes** page and enters: visitor name
   “Ravi Kumar”, visitor email, her own user ID, and “valid until tomorrow
   6 PM”. The system returns a `qr_token` like `2e0bc1a6-...`. She emails it
   to Ravi.
2. Ravi arrives at the college **Main Gate**. The security guard opens the
   **Gate Scan** page, pastes the token, selects “Main Gate”, and presses
   Scan. The screen shows **ALLOWED — Access granted**. The pass is now
   marked **used**, so it cannot be reused.
3. At 2 AM, someone tries the same token again at the **Back Gate** (the
   hostel-side entry). The screen shows **DENIED — Pass has already been
   used**. That attempt is still saved.
4. The next morning, the campus admin / warden opens the **Anomaly** page,
   presses **Load flags**, and sees the 2 AM attempt flagged as suspicious
   with reasons: `off-hours entry (02:00)`, `multi-gate use in 24h`.
5. The admin opens **Ask AI** and types “how many denied entries today?”
   The answer is “1 denied entries today.” plus the SQL that was run.

That is the whole system.

---

## 3. Who uses what? (college example)

| Person | Role in code (`UserRole`) | College example |
|---|---|---|
| Visitor | `visitor` | Parent visiting a student, guest lecturer, job candidate. Receives a pass, shows the token. Does not log in. |
| Host / Department office | `host` | Professor or department staff who invites the visitor. Creates passes (`POST /passes/create`). |
| Security guard | `guard` | Guard at the Main Gate / Back Gate / East Wing / Parking entries. Scans passes (`POST /passes/scan`). |
| Campus admin / Warden | `admin` | Reviews logs, anomaly flags, and asks English questions (`/ai/*`). |

Important: roles are **stored** on each user today, but the API does **not**
yet block routes by role. That is a planned Phase 3 task (JWT auth).

---

## 4. High-Level System Design (HLD)

### 4.1 The big picture

```text
                    +-------------------+
                    |  Streamlit UI     |
                    |  frontend/app.py  |
                    |  port 8501        |
                    +--------+----------+
                             | HTTP (requests)
                             | BACKEND_URL=http://backend:8000
                    +--------v----------+
                    |  FastAPI backend  |
                    |  backend/main.py  |
                    |  port 8000        |
                    +---+-----------+---+
                        |           |
              +---------v-+   +-----v-----------+
              | SQLite DB |   | ML artifact file|
              | .db file  |   | .pkl file       |
              +-----------+   +-----------------+
```

There are only two running programs (containers):

- **frontend** — what humans click. It has no database. Every button press
  just calls the backend over HTTP.
- **backend** — the brain. It owns the database and the AI logic.

Both are started with one command:

```bash
docker compose up --build
```

`docker-compose.yml` wires them together: the frontend talks to
`http://backend:8000`, your browser talks to the frontend on
`http://localhost:8501` and optionally to the backend docs on
`http://localhost:8000/docs`.

### 4.2 Why this shape?

- **FastAPI** because each gate scan must be fast and independent. Guards at
  many campus gates can scan at the same time.
- **SQLite** for local development because it is one file, zero setup.
  Production can swap `DB_CONNECTION` to Postgres without changing the code
  shape (SQLAlchemy handles both).
- **Streamlit** because the UI is mostly forms + tables. No JavaScript
  framework is needed.
- **scikit-learn only for one model** (`IsolationForest`). Everything else is
  plain Python + regex, so the system works offline with no API keys.

### 4.3 Main flows (happy paths)

**Flow A — Issue a pass (professor invites a visitor):**

```text
Professor/host -> Frontend (Passes page) -> POST /passes/create -> DB writes passes row -> returns qr_token
```

**Flow B — Scan at gate (security guard checks the visitor):**

```text
Security guard -> Frontend (Gate Scan) -> POST /passes/scan -> DB updates pass + inserts access_logs row -> returns allowed/denied
```

**Flow C — Find suspicious entries (separate from scanning):**

```text
Campus admin -> Frontend (Anomaly) -> POST /ai/anomaly/train (once in a while)
Campus admin -> GET /ai/anomaly/flags -> backend scores recent logs -> returns sorted list
```

Scanning never waits for the AI. That is deliberate: the college gate must
stay fast even if the model is slow or missing.

**Flow D — Ask a question:**

```text
Campus admin -> Frontend (Ask AI) -> POST /ai/query {question} -> backend builds safe SQL -> runs it -> returns {answer, sql, rows}
```

### 4.4 What the system does NOT do (on purpose)

- The AI never opens gates and never edits data. It only reads and suggests.
- There is no face recognition, phone tracking, or live video. Only the scan
  log (`who scanned which pass, where, when, result`) is used.
- There is no auto-retraining scheduler. A human presses “Train”.

---

## 5. Low-Level System Design (LLD)

### 5.1 Code map (which file does what)

```text
backend/
  main.py                  # creates FastAPI app, creates tables, plugs in 3 routers
  requirement.txt          # fastapi, uvicorn, sqlalchemy, pydantic-settings, email-validator, scikit-learn, numpy
  Dockerfile               # python:3.11-slim + uvicorn main:app on port 8000
  src/
    user/
      models.py            # UserModel + UserRole enum
      dtos.py              # UserCreateSchema (input validation)
      controller.py        # create_user, 409 on duplicate
      router.py            # POST /users/create
    passes/
      models.py            # PassModel, PassStatus enum, AccessLogModel
      dtos.py              # PassCreateSchema, PassScanSchema
      controller.py        # create_visitor_pass, scan_visitor_pass
      router.py            # POST /passes/create, POST /passes/scan
    ai/
      anomaly.py           # 7 features, IsolationForest train/score, rule fallback, pickle save/load
      text2sql.py          # English -> safe parametrised SELECT -> answer
      dtos.py              # AnomalyFlag, TrainResponse, NLQueryRequest/Response
      controller.py        # thin glue between router and anomaly.py/text2sql.py
      router.py            # POST /ai/anomaly/train, GET /ai/anomaly/flags,
                           # GET /ai/anomaly/score/{log_id}, POST /ai/query
    utils/
      constant.py          # OFF_HOUR_START/END, KNOWN_GATES, thresholds, allowlists
      helpers.py           # is_off_hours(), hours_between(), safe_limit()
      settings.py          # DB_CONNECTION, ANOMALY_CONTAMINATION, AI_ARTIFACT_DIR, ...
      db.py                # SQLAlchemy engine, SessionLocal, Base, get_db()
frontend/
  app.py                   # 6 Streamlit pages, calls backend with requests
  requirements.txt         # streamlit, requests, pandas
  Dockerfile               # streamlit run app.py on port 8501
```

Request handling always follows the same 3-layer pattern:

```text
router.py (HTTP in/out) -> controller.py (orchestration) -> models.py / anomaly.py / text2sql.py (real work)
```

Validation happens at the edge with Pydantic (`dtos.py`), so bad input is
rejected before it touches the database.

### 5.2 Database design (3 tables)

All IDs are random UUID strings. Times are **naive UTC** (no timezone info);
every part of the code must treat them as UTC or off-hours flags will shift.

**Table `users`** (`backend/src/user/models.py:UserModel`):

| Column | Meaning | Rule |
|---|---|---|
| `id` | Random ID | Primary key |
| `username` | Login name | Unique, required |
| `aadhar_number` | ID number | Unique, required, **never returned by Ask AI** |
| `email` | Email | Unique, required |
| `role` | `visitor/host/guard/admin` | Defaults to `host` |

**Table `passes`** (`backend/src/passes/models.py:PassModel`):

| Column | Meaning | Rule |
|---|---|---|
| `id` | Random ID | Primary key |
| `visitor_name`, `visitor_email` | Who the pass is for | Required |
| `qr_token` | Secret scan code | Auto UUID, unique, indexed |
| `status` | `active/used/expired/revoked` | Starts `active` |
| `host_user_id` | Which host invited them | Foreign key to `users.id` |
| `valid_until` | Expiry date-time | Compared against scan time |

**Table `access_logs`** (`backend/src/passes/models.py:AccessLogModel`):

| Column | Meaning |
|---|---|
| `id` | Random ID |
| `pass_id` | Which pass was scanned (foreign key to `passes.id`) |
| `gate_id` | Which campus gate (`Main Gate` = college entrance, `Back Gate` = hostel side, `East Wing` = department block, `Parking` = parking lot; free text, defaults to `Main Gate`) |
| `scan_time` | When (naive UTC, defaults to now) |
| `status` | `approved` or `denied` |

Relationships in plain words:

```text
one user (host) -> many passes
one pass -> many access_logs (every scan attempt, allowed or denied, is saved)
```

### 5.3 API contracts (exact shapes)

**`POST /users/create`** — `backend/src/user/router.py`, `controller.py`:

```json
// request
{"username": "priya_prof", "aadhar_number": "1234-5678-9012", "email": "priya@example.com", "role": "host"}
// success 200 -> the created user row (includes generated "id")
// duplicate username/aadhar/email -> 409 "User already exists"
```

**`POST /passes/create`** — `backend/src/passes/router.py`:

```json
// request
{"visitor_name": "Ravi Kumar", "visitor_email": "ravi@example.com",
 "host_user_id": "<id from above>", "valid_until": "2026-09-15T18:00:00"}
// success 200 -> pass row with "qr_token" and "status": "active"
// unknown host_user_id -> 404 "Host user not found"
```

**`POST /passes/scan`** — `backend/src/passes/controller.py:scan_visitor_pass`:

```json
// request
{"qr_token": "<from pass>", "gate_id": "Main Gate"}
// response (always writes an access_logs row, except unknown token)
{"access": "approved", "reason": "Access granted", "log_id": "<log id>"}
```

Decision order (first match wins):

1. Token not found -> `404 Pass not found` (no log row, nothing to attach it to).
2. `status == revoked` -> denied, “Pass has been revoked”.
3. `status == used` -> denied, “Pass has already been used”.
4. `status == expired` OR `valid_until < now` -> set `expired`, denied, “Pass has expired”.
5. Else -> set `used`, approved, “Access granted”.

Note the pass is **single-use**: the first good scan flips `active -> used`.

**`POST /ai/anomaly/train`** — `backend/src/ai/anomaly.py:train_model`:

```json
// request: {} (empty JSON)
// few rows (<10): {"model": "rules", "n_samples": 1, "n_features": 7, "contamination": 0.1, "artifact": null}
// enough rows:    {"model": "isolation_forest", "n_samples": 42, ..., "artifact": "artifacts/isolation_forest.pkl"}
```

It always returns HTTP 200, never an error on a fresh database.

**`GET /ai/anomaly/flags?limit=20&only_suspicious=false`**:

```json
{"model": "isolation_forest", "count": 2, "flags": [
  {"log_id": "...", "pass_id": "...", "gate_id": "Back Gate",
   "scan_time": "2026-09-13T02:30:00", "status": "denied",
   "anomaly_score": -0.675, "is_suspicious": true,
   "reasons": ["off-hours entry (02:00)", "gate hopping: 3 gates in 24h"]}
]}
```

`limit` is clamped to 1–200 (`safe_limit`). Results are sorted
most-suspicious-first (lowest score first).

**`GET /ai/anomaly/score/{log_id}`** — same single-item shape; `404` if unknown.

**`POST /ai/query`** — `backend/src/ai/text2sql.py:run_query`:

```json
// request
{"question": "how many denied entries today at Main Gate?", "limit": 20}
// response
{"sql": "SELECT COUNT(*) ...", "params": {"status_0": "denied", "gate_0": "Main Gate"},
 "columns": ["count"], "rows": [[3]], "answer": "3 denied entries today at Main Gate.", "warnings": []}
```

### 5.4 Anomaly detection — how the “strange or not?” score works

File: `backend/src/ai/anomaly.py`. Settings: `backend/src/utils/settings.py`.
Constants: `backend/src/utils/constant.py`.

#### Step 1 — Turn each scan into 7 numbers

`extract_features()` looks at one log row **plus its siblings** (other scans
of the same pass) and builds:

| # | Feature | Simple meaning | Example |
|---|---|---|---|
| 1 | `hour_norm` | Time of day, 0–1 (`hour / 23`) | 2 AM -> 0.087 |
| 2 | `is_off_hours` | 1 if hour ≥ 22 or < 6 | 2 AM -> 1 |
| 3 | `is_weekend` | 1 if Saturday/Sunday | Sunday -> 1 |
| 4 | `gate_rarity` | `1 − gate frequency`. Rare gates score near 1 | A service exit used once in 100 scans -> 0.99 |
| 5 | `velocity_1h` | Scans of same pass within ±1 hour, capped at 10, divided by 10 | 3 scans in an hour -> 0.3 |
| 6 | `hopping_24h` | Distinct gates for same pass in 24h, capped at 10, divided by 10 | 3 gates -> 0.3 |
| 7 | `is_denied` | 1 if this scan was denied | Denied -> 1 |

Alongside the numbers it collects **human reasons** (independent of the
numeric verdict):

- `off-hours entry (HH:00)` when #2 fires
- `weekend entry`
- `rapid repeat scans: N in 1h` when velocity ≥ 3
- `gate hopping: N gates in 24h` (≥3) or `multi-gate use in 24h` (== 2)
- `denied entry` or `N denied in 24h`
- `rare gate: X` when rarity ≥ 0.9

#### Step 2a — Day 0: transparent rules (fewer than 10 logs)

`MIN_TRAIN_SAMPLES = 10`. With fewer rows there is not enough history to
learn from, so `rule_score()` is used:

```text
score = -(0.45 * off_hours + 0.25 * min(velocity,3)/3
          + 0.20 * min(hopping-1,3)/3 + 0.10 * denied) - 0.05 if weekend
```

Scores live in `[-1, 0]`. Suspicious if `score <= -0.3`
(`RULE_SUSPICIOUS_THRESHOLD`). Weights say: night-time matters most (0.45),
then rapid repeats (0.25), then gate hopping (0.20), then denied (0.10).

Example: a 2 AM denied scan with nothing else:
`-(0.45 + 0.10) = -0.55` -> suspicious. A normal 10 AM approved scan:
`-0.0 = 0.0` -> normal.

#### Step 2b — Later: IsolationForest (10+ logs)

`POST /ai/anomaly/train` (`train_model()`):

1. Loads all logs oldest-first, computes gate frequencies, builds the 7-dim
   vectors into a NumPy array.
2. Fits `sklearn.ensemble.IsolationForest(contamination=0.1, random_state=42)`.
   `contamination=0.1` (`ANOMALY_CONTAMINATION`) means “assume roughly the
   strangest 10% of history is suspicious”. Raise toward 0.2 for a
   high-security site, lower toward 0.05 to reduce noise.
3. Computes the decision threshold as the 10th percentile of training scores
   and pickles `{model, gate_freq, threshold, n_samples, contamination,
   trained_at}` to `AI_ARTIFACT_DIR/isolation_forest.pkl`
   (default `./artifacts/`, persisted via the `backend-data` Docker volume).
4. If scikit-learn is missing, it degrades to `"model": "rules"` instead of
   crashing.

Scoring (`score_log_row()` + `_score_with_bundle()`):

- With a bundle: `score = model.score_samples([features])`; suspicious if
  `score < threshold`. Lower (more negative) = stranger.
- Without a bundle (or on error): rule score above.
- If the forest says suspicious but no rule fired, the reason becomes
  `"statistical outlier for this facility"` so a flag is never a bare boolean.
- `score_recent()` loads the bundle once, scores the newest `limit` rows,
  optionally keeps only suspicious ones, sorts lowest-score-first.

Key design point: scoring is a **separate read path**. `POST /passes/scan`
never calls the model, so gate latency is untouched.

### 5.5 Ask-in-English (Text-to-SQL) — how questions become safe SQL

File: `backend/src/ai/text2sql.py`. Pipeline in `run_query()`:

```text
question -> build_query() (regex match) -> validate_sql() -> db.execute(text(sql), params) -> answer + warnings
```

**The 6 question families** (first match wins in `build_query()`):

1. **Users by role** — “how many visitors/hosts/guards/admins?”
   `SELECT COUNT(*) FROM users WHERE role = :role_0`
2. **Pass status** — “how many active/used/expired/revoked passes?”
   `... FROM passes WHERE status = :status_0`
3. **Entries by gate** — “entries by gate / top gates / per gate”
   `SELECT gate_id, COUNT(*) ... GROUP BY gate_id ORDER BY entries DESC`
4. **Counts by scan status** — “how many denied/approved/total entries/scans
   [today|yesterday|last N days] [at GATE]”
5. **Recent logs** — “show|list|last|recent [N] [denied|approved] scans”
   `... ORDER BY scan_time DESC LIMIT :limit_0`
6. **Expiring passes** — contains “expiring/expire/valid until”
   active passes `ORDER BY valid_until ASC`.

Anything else -> HTTP 400 with an example question. It never passes raw user
text to SQL.

**Date and gate parsing:**

- `_date_filter()`: `last N days` (clamped 1–365), `yesterday`, `today`,
  using SQLite `date()` functions.
- `_find_gate()`: matches `KNOWN_GATES` (`Main Gate` = college entrance,
  `Back Gate` = hostel side, `East Wing` = department block, `Parking` =
  parking lot) case-insensitively. If the question contains “at …” with an
  unknown name, the query runs across all gates **plus** a warning:
  “Gate name not recognised; searched across all gates.”

**Safety validator** (`validate_sql()`, in order):

1. Must start with `SELECT`.
2. Blocklist scan: `INSERT/UPDATE/DELETE/DROP/ALTER/CREATE/.../PRAGMA/ATTACH`,
   plus `--` and `;` (no statement chaining).
3. Every `FROM/JOIN` table must be in `ALLOWED_TABLES`
   (`access_logs, passes, users`).
4. The string `aadhar` anywhere rejects the query (Aadhar is not even in
   `ALLOWED_COLUMNS`).
5. Limit is always clamped: `min(request.limit, AI_MAX_QUERY_ROWS=50)`.
6. All values are bound parameters (`:name`), never pasted into the SQL string.
7. Execution uses the request-scoped read session; no commit happens.

**Response shape** (`NLQueryResponse`): `{sql, params, columns, rows, answer,
warnings}`. SQL is echoed so anyone can audit it. `answer` is a one-liner like
`"3 denied entries today at Main Gate."` or `"5 row(s) for: last 10 scans."`.

#### Why does the Streamlit Ask AI page show SQL + JSON? (FAQ)

If you press **Ask** and see, below the green answer line, a grey SQL block
followed by a JSON block like `{"params": {...}, "columns": [...]}`, that is
**not an error — it is on purpose**. Here is why, in simple words:

1. **The backend always returns the full receipt.** `POST /ai/query` responds
   with `NLQueryResponse` (`backend/src/ai/dtos.py`): the one-line `answer`,
   the table (`columns` + `rows`), **and** the proof of how the answer was
   computed (`sql`, `params`, `warnings`). The frontend just displays every
   field it receives.
2. **The frontend code asks for it.** In `frontend/app.py` (Ask AI section):
   `st.code(data.get("sql", ""), language="sql")` draws the SQL block, and
   `st.json({"params": data["params"], "columns": data["columns"]})` draws the
   JSON block with the bound values (e.g. `{"status_0": "denied"}`) and the
   column names. Then `st.dataframe(...)` draws the result table underneath.
3. **Trust over magic.** A plain-English answer from a computer is easy to
   doubt (“did it count the right gate? the right day?”). Showing the SQL and
   its parameters lets a warden or auditor verify the answer without reading
   any code — the same “show your work” idea as the anomaly `reasons[]`.
   The `params` box also proves your words were put in safe labeled boxes
   (`:gate_0`), never pasted into the SQL text.

In short: **green line = the answer, SQL block = how it was computed,
JSON block = the exact values plugged in, table = the raw rows.** If you
prefer a cleaner look for non-technical staff, you can hide the SQL + JSON
inside a collapsed `st.expander("Show technical details")` — the data stays
available, but the page shows only the answer and the table by default.

### 5.6 Frontend design (what each page calls)

File: `frontend/app.py`. Helpers `api_get()` / `api_post()` wrap `requests`
with timeouts and truncate server errors to 500 chars. The sidebar holds the
backend URL (default from `BACKEND_URL` env) and a health check against
`/openapi.json`.

| Page | Calls | Notes |
|---|---|---|
| Dashboard | `POST /ai/query` (denied today, approved 7d, pass stock), `GET /ai/anomaly/flags?limit=5` | Shows the 5-step flow reminder |
| Users | `POST /users/create`, `POST /ai/query` per role | No list-users endpoint exists, so headcount uses NL queries |
| Passes | `POST /passes/create`, `POST /ai/query` (stock/expiring) | Shows `qr_token` in a copyable block |
| Gate Scan | `POST /passes/scan`, `POST /ai/query` (recent N) | Big ALLOWED/DENIED banner + `log_id` |
| Anomaly | `POST /ai/anomaly/train`, `GET /ai/anomaly/score/{id}`, `GET /ai/anomaly/flags` | Table + per-flag expander with reasons |
| Ask AI | `POST /ai/query` | Shows answer, warnings, SQL code block, params JSON, table (SQL + JSON are shown deliberately for auditability — see §5.5 FAQ) |

### 5.7 Configuration knobs

`backend/src/utils/settings.py` (overridable by environment / `.env`):

| Setting | Default | What it changes |
|---|---|---|
| `DB_CONNECTION` | `sqlite:///./smart_visitors.db` | Swap to Postgres in prod |
| `ANOMALY_CONTAMINATION` | `0.1` | Forest sensitivity (strangest 10% flagged) |
| `AI_ARTIFACT_DIR` | `./artifacts` | Where `isolation_forest.pkl` is saved |
| `AI_MAX_QUERY_ROWS` | `50` | Hard cap for Ask AI rows |
| `LLM_PROVIDER` | `none` | Hook for Phase 3 LLM SQL (must still pass `validate_sql`) |

`backend/src/utils/constant.py`: `OFF_HOUR_START=22`, `OFF_HOUR_END=6`,
`KNOWN_GATES` (4 campus gates: Main Gate, Back Gate, East Wing, Parking),
`MIN_TRAIN_SAMPLES=10`,
`RULE_SUSPICIOUS_THRESHOLD=-0.3`, table/column allowlists, SQL blocklist,
example questions.

`backend/src/utils/helpers.py`: `is_off_hours()` (handles the overnight
22→6 wrap), `hours_between()` (absolute hours), `safe_limit()` (clamp to
`[1, maximum]`).

`backend/src/utils/db.py`: SQLite gets `check_same_thread=False`; one
`get_db()` session per request, always closed.

---

## 6. Worked example — a college at night (numbers included)

Setup: 12 past scans, mostly weekday 10 AM approvals at the college Main Gate
(parents and guests arriving for office hours). New scan: the same guest pass
at the Back Gate (hostel side), 2:30 AM Sunday, denied, 3rd scan of that pass
in an hour, seen at 3 campus gates in 24h — as if someone is testing which
college entry opens after hours.

Features: `[2/23≈0.087, 1, 1, ~0.9, 0.3, 0.3, 1]`.
Rule score: `-(0.45 + 0.25·1 + 0.20·0.667 + 0.10) − 0.05 ≈ −0.98` → suspicious.
Reasons: `off-hours entry (02:00)`, `weekend entry`, `rapid repeat scans:
3 in 1h`, `gate hopping: 3 gates in 24h`, `N denied in 24h`, possibly
`rare gate: Back Gate`. After `POST /ai/anomaly/train`, the forest learns
this campus's baseline instead, but the reasons stay the same shape.

---

## 7. Limits, risks, and what comes next (Phase 3)

1. **Naive-UTC times** — if the server timezone moves, off-hours flags shift.
   Fix: store timezone-aware timestamps.
2. **No auth yet** — anyone can call any endpoint. Fix: JWT + require `admin`
    (campus admin) for `/ai/*`, `host` (professor) for pass creation, `guard`
    (security guard) for scanning.
3. **Regex English only** — paraphrases outside the 6 families get HTTP 400.
   Fix: optional LLM (`LLM_PROVIDER=openai|local`) whose SQL must still pass
   `validate_sql()`.
4. **Manual retraining** — no nightly job. Fix: scheduled retrain + track
   precision/recall once labels exist.
5. **No Postgres full-text / JOIN questions yet** — SQLite `date()` syntax is
   assumed; JOIN questions are not supported.

---

## 8. How to run and try it

```bash
docker compose up --build
# frontend: http://localhost:8501
# backend docs: http://localhost:8000/docs
```

Click-through test (5 minutes, college scenario):

1. Users page → create a professor as host (e.g. `priya_prof`) → copy its `id`.
2. Passes page → create a visitor pass for a guest speaker with that host ID → copy `qr_token`.
3. Gate Scan → scan it at Main Gate (college entrance) → expect ALLOWED; scan again → DENIED.
4. Anomaly → Load flags (try `only_suspicious` off first).
5. Ask AI → “how many denied entries today?” → check the echoed SQL.
6. Optional: after 10+ scans, press **Train / retrain model** and reload flags
   to see `model: isolation_forest`.

---

## 9. Glossary (plain words)

- **Pass** — a temporary ticket for one visit. Has an expiry time and a
  status (`active → used/expired`, or manually `revoked`).
- **qr_token** — the secret code on the pass. Whoever holds it can try to enter
  once. Think “movie ticket barcode”.
- **Access log** — one saved line per scan attempt: which pass, which gate,
  when, allowed or denied. Never deleted by the app.
- **Anomaly score** — a number where lower means stranger. Rules emit
  `[-1, 0]`; IsolationForest emits its own negative scores. Only the ordering
  matters, not the exact value.
- **Contamination** — the fraction of history you assume is weird (0.1 = 10%).
  It sets the forest’s threshold.
- **Allowlist / blocklist** — explicit lists of what Ask AI may touch
  (3 tables, safe columns) and what it may never run (DELETE, DROP, …).
- **Parametrised query** — user words go in labeled boxes (`:gate_0`), never
  pasted into the SQL text, so injection is impossible.
