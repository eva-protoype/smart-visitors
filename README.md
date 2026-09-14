# Smart Card for Visitors: Centralized Access System

The **Smart Card for Visitors** is a centralized visitor identity verification and registration system[cite: 1]. This backend API handles automated visitor registration, digital pass issuance, hardware gate verification, and access auditing across facilities[cite: 1].

Built for high throughput, the system replaces vulnerable manual logbooks with secure, time-limited digital credentials and transparent entry auditing[cite: 1].

---

## Features Explained for a Non-Technical Person

### What is this, in one line?

Think of it as a **digital visitor book + security pass system** for offices, apartments, or campuses. No paper register. Every visitor gets a temporary QR pass on their phone, and every entry is recorded automatically.

### How does it work day-to-day?

1. **Someone invites a visitor.** Say an employee (a "host") is expecting a guest. They enter the guest's name and visit date in the system.
2. **The visitor gets a QR pass.** This is like a movie ticket — it has an expiry time and works only for that visit. It cannot be reused after entry or after it expires.
3. **The guard scans it at the gate.** The guard just scans the QR. The screen instantly says **Allowed** or **Denied**, with a reason like "already used" or "expired".
4. **Everything is written down automatically.** Who came, which gate, at what time, and whether entry was allowed — all saved. Nobody can erase or fake it later like a paper book.
5. **Different people have different roles in the data model.** Visitors, hosts, guards, and managers (admins) are labeled with a role, but route-level permissions are not enforced yet — that arrives with JWT auth in Phase 3.

### What does the "smart" AI part do? (in detail)

There are two AI helpers. Both are *advisors* — they never open gates by themselves and never change data. They only watch, flag, and answer.

**Helper 1 — Suspicious-entry alarm (anomaly detection).**

- **What it watches:** every gate scan — what time it happened, which gate, whether it was allowed or denied, and how that pass behaved recently (did it appear at several gates in one day? was it scanned many times in one hour?).
- **What counts as suspicious — 5 everyday examples:**
  1. *Late-night entry:* a pass used at 2:14 AM when the building is normally closed (off-hours are 10 PM–6 AM). Flag says: "off-hours entry (02:00)".
  2. *Gate hopping:* the same pass scanned at Main Gate, then Back Gate, then East Wing within 24 hours — as if someone is testing which door opens. Flag says: "gate hopping: 3 gates in 24h".
  3. *Rapid repeats:* the same pass scanned 3+ times within one hour — as if it was shared, copied, or the scanner was being probed. Flag says: "rapid repeat scans: 3 in 1h".
  4. *Denied cluster:* one pass denied several times in a day (expired, already used, or revoked — but someone keeps trying). Flag says: "3 denied in 24h".
  5. *Rare gate:* entry through a gate almost nobody uses (e.g. a service exit at midnight). Flag says: "rare gate: Back Gate".
  Weekend entries get a small extra note ("weekend entry") since offices are usually empty then.
- **What the manager actually sees:** a list ordered most-suspicious-first. Each item shows gate, time, allowed/denied, a score (lower = stranger), and the plain-word reasons above. Example:
  > Back Gate · 02:30 · denied · score −0.68 · SUSPICIOUS · reasons: off-hours entry (02:00), gate hopping: 3 gates in 24h, 2 denied in 24h
- **How the alarm learns:** when the building is new and there are fewer than 10 past entries, it uses fixed common-sense rules (the examples above). Once 10+ entries exist, a staff member presses "train" and the system studies the building's own history to learn what is *unusual here specifically* (a factory with night shifts learns differently from a 9-to-5 office). Retraining is manual — press it again every few weeks so it stays current.
- **What it does NOT do:** it does not block the gate (the guard's scan decision is separate and instant), it does not accuse anyone (a flag means "please check the camera/register", not "this person is guilty"), and it does not use faces, phones, or tracking — only the scan log.

**Helper 2 — Ask questions in English (natural-language queries).**

- **How you use it:** type a plain question, get a one-line answer plus a small table. You also always see the database query it ran, so anyone can double-check it.
- **Questions it understands — try these:**
  1. "how many denied entries today?" → *"3 denied entries today."*
  2. "how many approved scans last 7 days at Main Gate?" → *"41 approved entries last 7 days at Main Gate."*
  3. "how many denied entries yesterday at Back Gate?" → same idea, different day/place.
  4. "show last 10 scans" / "show last 5 denied scans" → the newest rows, newest first.
  5. "entries by gate" → a breakdown like Main Gate: 50, Back Gate: 6, East Wing: 2.
  6. "how many active / expired / used / revoked passes?" → pass stock-take.
  7. "which passes are expiring?" → active passes sorted by expiry, soonest first.
  8. "how many visitors / hosts / guards / admins?" → headcount by role.
- **Dates and places it gets:** words like *today*, *yesterday*, *last N days*, and gate names (*Main Gate, Back Gate, East Wing, Parking*). If you write "at" plus a name it does not know, it tells you ("Gate name not recognised; searched across all gates") instead of guessing silently.
- **Safety in plain words:** it can only *look*, never touch — no deleting, editing, or creating entries. It shows at most 50 rows at a time. It never displays Aadhar numbers. If it does not understand a question (or the question asks for something destructive like "delete the logs"), it says so honestly with an example to try, instead of making something up.

### Who uses what?

- **Reception / host:** creates visitor passes.
- **Guard:** scans passes at the gate.
- **Manager / admin:** checks logs, sees suspicious flags, asks questions in plain English.

---

## Features Explained for a Semi-Technical Reader

### System overview

FastAPI backend + SQLAlchemy (SQLite locally, Postgres in prod). Three core tables: `users`, `passes`, `access_logs`. A Streamlit UI (`frontend/app.py`, via Docker Compose) covers users, passes, gate scans, anomaly flags, and plain-English queries. No auth yet — role-based route enforcement is still to come.

### Phase 1 (MVP) — what exists

| Feature | What it does | Key endpoint |
|---|---|---|
| **User + RBAC roles** | Creates `visitor / host / guard / admin` users. Uniqueness enforced on `username`, `aadhar_number`, `email` (409 on duplicate). Roles are stored on the model; route-level enforcement is not yet added. | `POST /users/create` |
| **Pass issuance** | Creates a time-boxed pass: `visitor_name`, `visitor_email`, `host_user_id (FK → users.id)`, `valid_until`, auto-generated `qr_token (uuid)`. 404 if the host does not exist. Status lifecycle: `active → used / expired`, plus manual `revoked`. | `POST /passes/create` |
| **Gate scan + audit** | Looks up `qr_token`, decides `approved / denied` (`revoked`, `used`, `expired`, or `valid_until < now`), transitions pass status, and **always writes an `access_logs` row** (`pass_id, gate_id, scan_time (naive UTC), status`). | `POST /passes/scan` |

Typical flow: `create user (host) → create pass → scan qr_token → read {access, reason, log_id}`.

### Phase 2 (AI) — added in Step 2 (in detail)

**1. Anomaly detection (`/ai/anomaly/*`, code in `backend/src/ai/anomaly.py`).**

*How it works, end to end:* each `access_logs` row is converted to numbers, a model learns the shape of "normal", and every scan gets a score plus human reasons.

- **The 7 features (built in `extract_features()` per log row):**
  | # | Feature | How it is computed |
  |---|---|---|
  | 1 | `hour_norm` | `scan_time.hour / 23` (naive UTC, same clock as the scan path) |
  | 2 | `is_off_hours` | 1 if hour ≥ 22 or < 6 (`OFF_HOUR_START/END` in `utils/constant.py`) |
  | 3 | `is_weekend` | 1 if Saturday/Sunday |
  | 4 | `gate_rarity` | `1 − gate frequency` across all logs (an unseen gate scores 1.0) |
  | 5 | `velocity_1h` | scans of the same `pass_id` within ±1h, capped at 10, divided by 10 |
  | 6 | `hopping_24h` | distinct gates for the same `pass_id` within 24h, capped at 10, divided by 10 |
  | 7 | `is_denied` | 1 if `status == "denied"` |
- **Training (`POST /ai/anomaly/train`, `train_model()`):**
  - Needs ≥ `MIN_TRAIN_SAMPLES` (10) rows; fewer returns `{"model": "rules", "artifact": null}` with HTTP 200 — the API never errors on a fresh database.
  - With enough data it fits `sklearn.ensemble.IsolationForest(contamination=settings.ANOMALY_CONTAMINATION=0.1, random_state=42)`, computes `score_samples` on the training set, and stores the decision threshold at the `contamination` percentile (i.e. roughly the strangest 10% of history counts as suspicious).
  - Persists a pickle bundle `{model, gate_freq, threshold, n_samples, contamination, trained_at}` to `AI_ARTIFACT_DIR/isolation_forest.pkl` (default `./artifacts/`, gitignored). If `scikit-learn` is missing it degrades to rules mode instead of crashing.
  - Retraining is on-demand (no cron yet) — call it after busy days so the baseline follows the facility (e.g. a new night shift stops looking anomalous).
- **Scoring (`GET /ai/anomaly/flags`, `GET /ai/anomaly/score/{log_id}`, `score_log_row()`):**
  - With a bundle: `score = model.score_samples([features])`; suspicious if `score < threshold`. Without one: `rule_score()` in `[-1, 0]` — `−(0.45·off_hours + 0.25·min(velocity,3)/3 + 0.20·min(hopping−1,3)/3 + 0.10·denied)` minus 0.05 on weekends; suspicious if ≤ `RULE_SUSPICIOUS_THRESHOLD` (−0.3).
  - Reasons are emitted independently of the numeric verdict: off-hours (`off-hours entry (HH:00)`), weekend, `rapid repeat scans: N in 1h` (velocity ≥ 3), `gate hopping: N gates in 24h` (hopping ≥ 3) or `multi-gate use in 24h` (== 2), denied (`denied entry` / `N denied in 24h`), `rare gate: X` (rarity ≥ 0.9). If the forest flags a row but no rule fires, the reason is `"statistical outlier for this facility"` so the flag is never a bare boolean.
  - `GET /flags` takes `limit` (clamped to 1–200) and `only_suspicious`, sorts most-suspicious-first, and reports which backend served the batch (`"model": "isolation_forest" | "rules"`).
  - Example item: `{log_id, pass_id, gate_id: "Back Gate", scan_time: "2026-09-13T02:30:00", status: "denied", anomaly_score: −0.675, is_suspicious: true, reasons: ["off-hours entry (02:00)", "gate hopping: 3 gates in 24h", "2 denied in 24h"]}`.
  - Scoring is a separate read path — the synchronous `POST /passes/scan` decision never waits on the model.
- **Operational notes / limits:** naive-UTC timestamps (TZ migration is a Phase 3 item — server TZ moves skew off-hours flags); `contamination` tunes sensitivity (raise toward 0.2 for a high-security site, lower toward 0.05 to cut noise); sibling queries per row are O(pass history), fine for this scale.

**2. Natural-language queries (`POST /ai/query`, code in `backend/src/ai/text2sql.py`).**

- **Pipeline:** `run_query()` → `build_query()` (regex pattern match → parametrised SQL + params + columns + answer scope) → `validate_sql()` → `db.execute(text(sql), params)` → one-line `answer` + `warnings`. An LLM may propose SQL in Phase 3 (`LLM_PROVIDER` setting), but it must pass the same validator before execution.
- **The 6 question families (matched in order; first match wins):**
  1. *Users by role* — `how many visitors/hosts/guards/admins?` → `SELECT COUNT(*) FROM users WHERE role = :role_0`.
  2. *Pass status* — `how many active/used/expired/revoked passes?` (or bare `how many passes?`) → `... FROM passes WHERE status = :status_0`.
  3. *Entries by gate* — `entries by gate | top gates | per gate` (+ optional date words) → `SELECT gate_id, COUNT(*) ... GROUP BY gate_id ORDER BY entries DESC`.
  4. *Counts by scan status* — `how many denied/approved/total entries|scans|logs [today|yesterday|last N days] [at GATE]` → `SELECT COUNT(*) FROM access_logs WHERE status = :status_0 [AND date(scan_time)...] [AND gate_id = :gate_0]`.
  5. *Recent logs* — `show|list|last|recent [N] [denied|approved] scans|logs|entries` → `SELECT id, pass_id, gate_id, scan_time, status ... ORDER BY scan_time DESC LIMIT :limit_0`.
  6. *Expiring passes* — contains `expiring|expire|valid until` → active passes `ORDER BY valid_until ASC`.
  Anything else → HTTP 400 with an example question; never a raw-SQL passthrough.
- **Date/gate extraction:** `_date_filter()` handles `last N days` (clamped 1–365, `date(scan_time) >= date('now', :days_ago)`), `yesterday`, `today` (SQLite `date()` semantics); `_find_gate()` matches `KNOWN_GATES` (`Main Gate, Back Gate, East Wing, Parking`) case-insensitively and ignores date words captured after "at/for/in".
- **Safety validator (`validate_sql()`), in order:** must start with `SELECT`; blocklist scan for `INSERT/UPDATE/DELETE/DROP/ALTER/CREATE/.../PRAGMA/ATTACH` plus `--` and `;` (statement chaining); every `FROM/JOIN` table must be in `ALLOWED_TABLES` (`access_logs, passes, users`); the string `aadhar` anywhere rejects (Aadhar is not even in `ALLOWED_COLUMNS`); caller limit is clamped via `min(request.limit, AI_MAX_QUERY_ROWS=50)`.
- **Response shape:** `{sql, params, columns, rows, answer, warnings}` — SQL is echoed for auditability; `answer` is `"<n> <scope>."` for counts, `"<k> row(s) for: <scope>."` for listings, `"No results for: <scope>."` when empty; `warnings` currently carries the unrecognised-gate notice.
- **Limits to know:** regex coverage (not a full LLM — paraphrases outside the families 400); SQLite date functions (will need dialect handling for Postgres); no JOIN questions yet; no per-role gating on the endpoint itself (ADMIN-only auth is a Phase 3 item).

---

##  Tech Stack

* **Core Framework:** [FastAPI](https://fastapi.tiangolo.com/) (Python 3.11+) for high-concurrency, asynchronous API routes[cite: 1].
* **Database & ORM:** SQLite (dev) / PostgreSQL (prod) with SQLAlchemy[cite: 1].
* **Data Validation:** Pydantic v2 for strict type checking and request payload validation[cite: 1].
* **Machine Learning (WIP):** `scikit-learn` for anomaly detection & LLM integration for Text-to-SQL analytics.

##  Core Features

### Phase 1: MVP Setup (Completed)
* **Role-Based Access Control (RBAC):** Distinct database permissions for visitors, security guards, host personnel, and system administrators[cite: 1].
* **Pass Credential Management:** Secure generation of time-limited QR visitor passes[cite: 1].
* **Access Auditing Workflow:** Persistent logging of every gate scan attempt, capturing entry outcomes and validation statuses[cite: 1].

### Phase 2: AI Integrations (In Progress)
* **Anomaly Detection Engine:** A lightweight `IsolationForest` model to flag suspicious, off-hours, or rapid multi-gate entry attempts.
* **Natural Language Queries:** A Text-to-SQL LLM assistant allowing admins to query access logs in plain English.

##  Local Development Setup

### 1. Clone the repository
```bash
git clone [https://github.com/eva-protoype/smart-visitors.git](https://github.com/eva-protoype/smart-visitors.git)
cd smart-visitors/backend

### virtual environment setup
python3 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -r requirements.txt