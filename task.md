
**Smart Visitors** is a centralized visitor entry system for a college. A visitor arrives, registers
(gives details + purpose), and receives a QR pass — time-limited to 24 hours, or consumed on the scan
at the intended entrance. Roles: **visitor** (never an account — a record), **host** (college member
hosting visitors), **admin** (system owner), **guard** (scans passes, handles walk-ins). Admin sees
all activity; hosts see their own visitors (active / ended / upcoming); guards get a scan console
plus a walk-in desk for visitors without phones.

Pass payloads are encrypted at rest and sealed; the QR surface is an opaque token. A pool of
**unassigned plastic sticker QRs** lets the bind a phone-less visitor's walk-in to a physical badge.
Every scan and every sensitive action lands in **two hash-chained audit logs**. An IsolationForest
advisor flags suspicious scan patterns (advisory only). Deployment: **one server, Docker Compose,
five containers**.

### 0.1 Prior conversation decisions (normative)

| # | Decision |
|---|----------|
| D1 | No Aadhaar, no biometrics, no face matching. Identity = name + phone + optional photo + optional free-text `id_note`. |
| D2 | Pass encryption lives at rest, not inside the QR. QR = opaque random token; scanner always hits the server. |
| D3 | Admin can decrypt any pass (escrow), every decrypt is an audit event. |
| D4 | **No external auth system.** No signup. One seeded "developer credential" bootstraps the admin; admin mints host/guard accounts. Go API owns login (JWT). |
| D5 | Modular monolith inside Go API; 5 containers total. Microservice boundaries at package level, not process level. |
| D6 | `aadhar_number` column dropped. Optional `id_note` text instead. Legacy data never displayed. |
| D7 | Exit scans from day one (`direction` on every log). Entry on a one-time pass ends it; exit is informational (headcount). |
| V-record | Visitors are records, never accounts. Passes/QRs are minted by exactly two server flows: host invite and guard walk-in. No public creation endpoints. |
| V-proxy | Proxy posture: forgery and self-injection closed (server-minted tokens, no public routes). Person-swap at the gate is mitigated by photo display + guard judgment + anomaly flags, and is the accepted residual limit of v2 (no biometrics). |

---

## 1. Goals / non-goals / anti-goals

**Goals**
1. A guard can check any visitor in or out in under 3 seconds, always with an Allow/Deny + reason.
2. A host always knows: who is coming, who is on site (mine), who has left.
3. An admin has total activity visibility plus a tamper-evident audit trail.
4. A phone-less visitor is a first-class flow (sticker badge), equal in capability to phone pass.
5. Visitor PII (details payload, photo) is encrypted at rest; only role-checked, audited paths reveal it.
6. Everything runs on one server via `docker compose up`.

**Non-goals**
- Aadhaar/offline e-KYC, biometric/liveness matching, device-bound passkeys.
- Multi-site federation, blockchain/Merkle transparency logs, client-held key ceremonies.
- Landlord "marketing"/payments modules of supastarter (unused packages stay dormant).
- Offline gate scanning (v2 assumes the gate device reaches the API; a fail-closed mode is specified).

**Anti-goals (failures even if technically working)**
- Any public endpoint that mints visitors or passes (proxy creation).
- Guard console displaying more PII than name + photo + host + window + status.
- Plaintext PII in database dumps or application logs.
- A denied scan without a machine-readable reason.

---

## 2. Existing repo disposition

| Path | Action |
|------|--------|
| `backend/main.go`, `internal/{db,config}` | Keep; extend config/db bootstrap (Postgres, migrations, seed). |
| `backend/internal/handlers/users.go`, `passes.go` | Rewrite against v2.1 models (first two phases; logic is superseded). |
| `backend/internal/handlers/ai.go` | Keep text2sql; anomaly routes become sidecar proxies; texts ported where needed. |
| `backend/internal/models/models.go` | Replace with versioned models (new schema). |
| `backend/ml/isolation.py`, `backend/ml/` | Keep; becomes the `ml` FastAPI sidecar, adapted to contract in §11. |
| `backend/src/**`, `backend/main.py`, `requirement.txt` | Legacy Python app — delete at Phase A end. |
| `frontend/` (Streamlit) | Delete at Phase C end (after Next dashboards land). |
| `supanext/` | Keep as dashboard app. Remove auth (better-auth) usage; keep `ui`/`utils`; drop dormant packages from builds (`mail`, `payments`, `storage`, `marketing`, `docs`, `mail-preview`). |
| `docker-compose.yml` | Replace with 5-service spec (§5). |
| `task.md` | Replace with this document. |
| `docs/` | Keep; add `docs/runbook.md` at Phase F. |
| Legacy SQLite DB (`smart_visitors.db`) | Dev artifact; **not migrated**. Fresh Postgres. |
| `users.aadhar_number` | Gone with new schema. text2sql allowlists prevent echoing names/notes of legacy rows anyway. |

---

## 3. Architecture

```
                       ┌──────────────────────────── single server ────────────────────────────┐
 browser ── caddy :80/443 ──┬── web    Next.js (supanext)      :3000   dashboards, visitor page  │
                            ├── api    Go (chi)                 :8000   domain + auth + audit   │
                            │              ├── db     Postgres 16 :5432  (internal net only)     │
                            │              └── ml     FastAPI      :8100  (internal net only)     │
                            └── (sticker printing happens in browser print dialog)                 │
 volumes: dbdata, photos, artifacts                                                        │
 networks: public (caddy↔web, caddy↔api), internal (api↔db↔ml, web never touches db)        │
```

- All browser traffic enters by origin path: `/`→`web`, `/api/*`→`api`. No CORS in prod path.
- `web` never holds a DB. It forwards `/api/*` with the JWT cookie attached.
- `ml` is reachable only from `api` (internal network + HMAC service header).
- Photos/blobs live on disk volumes, never in Postgres; served through the API.

### 3.1 Services (docker compose, final)

| Service | Image/build | Ports | Depends | Health |
|---|---|---|---|---|
| `caddy` | caddy:2 | 80, 443 | web, api | tcp probe |
| `web` | build `./supanext` (standalone Next) | 3000 internal | api (startup, not runtime-critical) | GET /api/health |
| `api` | build `./backend` (Go, multi-stage) | 8000 internal | db healthy, ml healthy | GET /healthz, /readyz(db+master key loaded) |
| `db` | postgres:16-alpine | 5432 internal | — | pg_isready |
| `ml` | build `./backend/ml` (python:3.12-slim + scikit-learn) | 8100 internal | — | GET /health |

`restart: unless-stopped` everywhere. Persistent volumes: `dbdata`, `photos`, `artifacts`.

### 3.2 Environment variables (single root `.env`, gitignored)

| Var | Default | Notes |
|---|---|---|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | — | compose + api DSN |
| `DATABASE_URL` | — | `postgres://...@db:5432/smart_visitors?sslmode=disable` |
| `JWT_SECRET` | — | 32B hex; HS256 |
| `MASTER_KEY` | — | 32B hex; wraps payload DEKs + private keys (see §6) |
| `BOOTSTRAP_ADMIN_USER` | `dev` | "developer credential" |
| `BOOTSTRAP_ADMIN_PASSWORD` | — | required on first boot; forced-change after first login |
| `ISSUER_ED25519_PRIV` | auto-generated | if absent, generated once at seed, master-wrapped, stored in `keys` |
| `PASS_MAX_HOURS` | `24` | hard ceiling on pass validity |
| `RETENTION_PASS_DAYS` | `30` | crypto-shred + purge after pass end |
| `RETENTION_LOG_DAYS` | `90` | access_logs/audit_events purge |
| `TOKEN_TTL_HOURS` | `12` | JWT lifetime |
| `RATE_LIMIT_SCAN` | `30/min/IP` | per-scanner |
| `RATE_LIMIT_PUBLIC` | `10/h/IP` | invite fill, visitor page |
| `RATE_LIMIT_LOGIN` | `5/h/user+IP` | brute force |
| `ML_URL`, `ML_SERVICE_TOKEN` | — | internal HMAC |
| `APP_BASE_URL` | — | absolute URL for invite links |
| `OFF_HOUR_START/END` | `22` / `6` | anomaly feature window |
| `ANOMALY_CONTAMINATION` | `0.1` | passed to sidecar train |
| `PHOTOS_DIR` | `/data/photos` | volume |
| `LOG_LEVEL` | `info` | — |

---

## 4. Identity & access

### 4.1 Account model

`users` table holds **only** `admin | host | guard`. Visitors live in `visitors` (records).
No signup route exists anywhere in the codebase.

**Bootstrap ("developer credential"):**
At API start, if `users` is empty: create one admin from `BOOTSTRAP_ADMIN_*`. This admin has
`must_reset_password = true` (forces a credential change on first login — the env password is
a bootstrap, not the ongoing password). Log an audit event `user.bootstrap`.

**Account minting:** Admin-only `POST /users`. Fields: `role`, `name`, `username` (unique),
`email?`/`phone?`, optional `expires_at?` for guard seasonal accounts. Server returns a
**generated temp password (shown once)** with `temp_password_expires_at = now+24h` and
`must_reset_password = true`. Login with a temp password works only until reset.

**Login:** `POST /auth/login {username, password}` → 200 `{token, expires_at, user}` or
standard error. Argon2id hashes. Rate limit `RATE_LIMIT_LOGIN`. A disabled user gets `403
FORBIDDEN` with code `ACCOUNT_DISABLED`.

**Sessions:** stateless JWT HS256. Claims: `{sub (user id), role, name, iat, exp: iat+12h}`.
Frontend stores it in an httpOnly, SameSite=Strict cookie; logout clears it. Password change /
account disable bumps `token_version` on the user row and the JWT carries it — old tokens die
immediately.

**RBAC matrix (role-enforced in Go middleware):**

| Capability | admin | host | guard | public |
|---|---|---|---|---|
| Mint users / batch stickers / gates CRUD | ✅ | — | — | — |
| Create visits, invite visitors | ✅ (any host's) | ✅ (own) | — | — |
| Revoke pass | ✅ | own | — | — |
| Extend window / reassign sticker / force-expire (override events) | ✅ | — | — | — |
| Decrypt details | ✅ (escrow, audited) | own (audited) | — | — |
| Walk-in create + sticker claim | — | — | ✅ | — |
| Scan (`/scan`) | ✅ | — | ✅ | — |
| View audit log / run NL query / admin dashboards | ✅ | — | — | — |
| Fill invite link, view visitor page | — | — | — | ✅ (tokenized, rate-limited) |

### 4.2 Visitors (`visitors` table — records, not accounts)

Minted in exactly two flows; both write `created_by` (host id or guard id) and `created_via`
(`regular | walk_in`). Fields: `name` (required), `phone?`, `photo_ref?`, `id_note?`
(free text, optional — D6 replaces Aadhaar), `consent_at` timestamp (set when an invite link is
filled or a guard records consent for walk-ins; required before a pass activates).

---

## 5. Crypto model (D2/D3)

### 5.1 Keys

| Key | Algo | Storage |
|---|---|---|
| Master key | 32B symmetric | env `MASTER_KEY`, never in DB |
| Pass DEK | random 32B, one per pass | wrapped (below); raw never stored |
| Issuer signing key | Ed25519 | generated at seed if absent; master-wrapped in `keys` table; signs pass tokens |
| Future: host-held wrap keys | X25519 (WebCrypto) | wrap format is generic; reserved slot, not used in v2 |

### 5.2 Sealed pass payload

```
payload_json = {"visitor":{"name","phone?","id_note?"},"purpose","host_id","gate_id?",
                "created_via","issued_at"}
blob   = XChaCha20-Poly1305(DEK, payload_json)              // aead: nonce prefixed
wrap   = XChaCha20-Poly1305(MASTER_KEY, "dek:"+pass_id, DEK) // AAD binds wrap to pass_id
pass_secrets(pass_id, blob, wrap, payload_sig)
payload_sig = Ed25519_sign(issuer, sha256(payload_json))    // integrity receipt for future offline use
```

- **Activation rule:** a pass is `active` only after either (a) the invite link was filled
  (visitor-entered/confirmed details + consent) or (b) a guard recorded details + consent on a
  walk-in. Until then `status = draft`.
- **Decrypt:** `GET /passes/{id}/details`. Server unwraps, verifies role per §4.1, writes
  `audit_events{action:"pass.decrypt", target:pass_id}`, returns payload. Host sees own; admin sees
  any. Guard has **no** decrypt route.
- **Crypto-shredding (retention sweep):** at `valid_until + RETENTION_PASS_DAYS`, delete
  `pass_secrets*.wrap` and `blob` and the photo file; the row keeps metadata + hash receipts.
- **Upgrade path (explicitly non-goal now):** wrap format already accepts a second wrapped copy
  (`wrap_host`); a client-held host key can be introduced without schema or flow changes.

### 5.3 What the QR contains

QR encodes `sv1:<token>` where `token` is 128-bit random, URL-safe. At rest only
`sha256(token)` is stored (peppered with `MASTER_KEY` via HMAC to survive DB theft:
`token_hash = HMAC-SHA256(MASTER_KEY, token)`). Server holds the decision logic; nothing in the
QR is sensitive. The plastic sticker QR encodes exactly the same format.

---

## 6. Data model (Postgres DDL)

```sql
CREATE TYPE user_role   AS ENUM ('admin','host','guard');
CREATE TYPE pass_status AS ENUM ('draft','active','used','expired','revoked','shredded');
CREATE TYPE pass_mode   AS ENUM ('one_time','window');
CREATE TYPE scan_dir    AS ENUM ('entry','exit');
CREATE TYPE scan_out    AS ENUM ('approved','denied');
CREATE TYPE sticker_st  AS ENUM ('free','assigned','retired');
CREATE TYPE visit_src   AS ENUM ('regular','walk_in');

CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  role          user_role NOT NULL,
  username      TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,              -- argon2id
  name          TEXT NOT NULL,
  email         TEXT UNIQUE,
  phone         TEXT,
  must_reset_password BOOLEAN NOT NULL DEFAULT false,
  temp_expires_at     TIMESTAMPTZ,
  token_version       INTEGER NOT NULL DEFAULT 0,
  disabled_at         TIMESTAMPTZ,
  created_by    UUID REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE visitors (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL,
  phone      TEXT,
  photo_ref  TEXT,                          -- filename in photos volume
  id_note    TEXT,                          -- free text, optional (D6)
  consent_at TIMESTAMPTZ,
  created_by UUID NOT NULL REFERENCES users(id),
  created_via visit_src NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE gates (
  id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name   TEXT UNIQUE NOT NULL,
  active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE sticker_batches (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  count INTEGER NOT NULL, created_by UUID NOT NULL REFERENCES users(id), created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE stickers (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  batch_id     UUID REFERENCES sticker_batches(id),
  token_hash   TEXT UNIQUE NOT NULL,         -- HMAC-SHA256(MASTER_KEY, token)
  status       sticker_st NOT NULL DEFAULT 'free',
  pass_id      UUID,
  claimed_by   UUID REFERENCES users(id), claimed_at TIMESTAMPTZ,
  printed      BOOLEAN NOT NULL DEFAULT false,
  retired_at   TIMESTAMPTZ
);
CREATE INDEX ON stickers(status);

CREATE TABLE passes (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash  TEXT UNIQUE NOT NULL,
  status      pass_status NOT NULL DEFAULT 'draft',
  mode        pass_mode NOT NULL DEFAULT 'one_time',
  visitor_id  UUID NOT NULL REFERENCES visitors(id),
  host_id     UUID NOT NULL REFERENCES users(id),
  gate_id     UUID REFERENCES gates(id),    -- intended entrance; NULL = any gate
  purpose     TEXT NOT NULL,                -- display copy; PII-detail copy lives inside the blob
  valid_from  TIMESTAMPTZ NOT NULL,
  valid_until TIMESTAMPTZ NOT NULL,         -- ≤ valid_from + PASS_MAX_HOURS (enforced)
  created_via visit_src NOT NULL,
  created_by  UUID NOT NULL REFERENCES users(id),
  invite_token_hash TEXT UNIQUE,            -- set while draft (regular flow)
  sticker_id  UUID REFERENCES stickers(id), -- set for walk_in
  deleted_at  TIMESTAMPTZ                   -- retention tombstone
);
CREATE INDEX ON passes(host_id, status); CREATE INDEX ON passes(status, valid_until);

CREATE TABLE pass_secrets (
  pass_id     UUID PRIMARY KEY REFERENCES passes(id) ON DELETE CASCADE,
  blob        BYTEA NOT NULL,               -- XChaCha payload
  wrap        BYTEA NOT NULL,               -- master-wrapped DEK (AAD = pass_id)
  payload_sig BYTEA NOT NULL
);

CREATE TABLE access_logs (
  seq       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  id        UUID NOT NULL DEFAULT gen_random_uuid(),
  pass_id   UUID NOT NULL REFERENCES passes(id),
  gate_id   UUID NOT NULL REFERENCES gates(id),
  guard_id  UUID REFERENCES users(id),       -- NULL if automated scan
  scan_time TIMESTAMPTZ NOT NULL DEFAULT now(),
  direction scan_dir NOT NULL,
  outcome   scan_out NOT NULL,
  reason    TEXT NOT NULL,                   -- machine code, see §9
  prev_hash TEXT NOT NULL,
  hash      TEXT NOT NULL
);

CREATE TABLE audit_events (
  seq       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  id        UUID NOT NULL DEFAULT gen_random_uuid(),
  actor_id  UUID REFERENCES users(id),
  action    TEXT NOT NULL,                   -- enum in §10.2
  target    TEXT NOT NULL,
  reason    TEXT,
  meta      JSONB NOT NULL DEFAULT '{}',
  prev_hash TEXT NOT NULL,
  hash      TEXT NOT NULL
);

CREATE TABLE keys (                          -- master-wrapped key storage
  name TEXT PRIMARY KEY, wrapped BYTEA NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 7. Pass lifecycle

### 7.1 Status machine

```
draft ──(invite filled | walk-in detail+consent recorded)──▶ active
active ──(entry scan, mode=one_time, correct gate)──▶        used
active ──(valid_until passes)──▶                             expired   (sweep, every 5 min)
active/used ──(revoke)──▶                                    revoked
any ──(retention sweep)──▶                                   shredded  (metadata retained)
```

- `mode=one_time` (default): pass **ends** at entry scan at `gate_id` (or any gate if NULL). Exit
  scan afterwards is informational and does not change status.
- `mode=window`: pass stays `active` through the window; entry/exit scans toggle an implied
  on-site state (last direction per pass). Used for recurring contractor-style visits.
- Validity ceiling: `valid_until ≤ valid_from + PASS_MAX_HOURS` — enforced at creation, incl.
  admin overrides (an override may only *extend within* the ceiling; the ceiling is absolute).

### 7.2 Invite link flow (regular path)

1. Host `POST /visits {visitor:{name,phone?}, purpose, gate_id?, valid_from, valid_until, mode?}`
   → creates draft pass + `visitors` record (details here are host-entered **hints**; the sealed
   blob is written at activation) → returns `{pass_id, invite_url}` where
   `invite_url = APP_BASE_URL + /fill/<random 128-bit>` (`invite_token_hash` stored HMAC'd).
2. Visitor opens link → public form (rate-limited): confirm/correct name/phone, optional
   `id_note`, optional **photo upload** (webcam or file, JPEG ≤ 2MB after server-side resize), and
   a consent checkbox with a purpose notice. Submit → server writes final payload into the blob,
   sets `consent_at`, flips draft → active, mints QR.
3. Link becomes dead (single-use, `410 TOKEN_USED`).
4. Visitor sees/stores their QR page: `/p/<token>` (public, tokenized).

### 7.3 Walk-in flow (no phone) — guard desk

1. Guard console → Walk-in tab: search host by username (by role, host list), fill
   name/phone?/purpose/`id_note?`, capture photo from webcam (optional but default-on),
   consent noted by guard (`consent_note + guard id`).
2. Guard scans a **free sticker** from the pool (camera into guard console) →
   `POST /stickers/claim {sticker_token, ...walkin payload}` — single atomic call: creates
   `visitors` row (created_by=guard), draft pass → active, binds `sticker_id`, transitions sticker
   `free→assigned`. Response includes printable confirm card.
3. Plastic sticker in ID sleeve = the visitor's badge for the day. If the walk-in needs a window
   other than next-24h the guard sets it in the form (within ceiling).
4. If the visitor later provides a phone, the guard may hit "send link" → displays the `/p/{token}`
   URL for SMS (manual sending is fine; SMTP is optional, out of scope by default).

### 7.4 Gate scan flow — decision order (exact)

`POST /scan {token, gate_id, direction}`
Validate shape → lookup `HMAC(token)`.

| # | Condition | outcome | reason code | Side effects |
|---|---|---|---|---|
| 1 | malformed token | denied | `BAD_TOKEN` | log |
| 2 | unknown token (pass & sticker) | denied | `UNKNOWN_TOKEN` | log |
| 3 | sticker unbound | denied | `STICKER_UNBOUND` | log |
| 4 | pass `revoked` | denied | `PASS_REVOKED` | log |
| 5 | pass `expired`/window not started | denied | `NOT_IN_WINDOW` | log |
| 6 | wrong gate (pass.gate_id set ≠ scan gate) | denied | `WRONG_GATE` | log |
| 7 | entry + already `used` | denied | `ALREADY_USED` | log |
| 8 | exit + (direction exit, any status ≥ active) | approved | `OK` | log exit; on-site=off |
| 9 | entry + `active` | approved | `OK` | if one_time → status `used` |
| 10 | anything unexpected | denied | `INTERNAL` | log + alert |

Response card shown to guard (max fields, per anti-goals): `outcome`, `reason` (humanized),
`name`, `photo` (if present — delivered inline as base64 in the same response, already resized
to 480px; no second call), `host name`, `validity string`, `direction`, `pass mode note`
("this badge closes at entry"). Card round-trip latency budget: p95 < 300 ms.

When the API is unreachable, gate fails **closed**: scanner shows banner; guards use the manual
paper fallback sheet (§22) and the incident becomes a signed `override`-style audit event on
recovery ("offline register", admin transcribes).

### 7.5 Expiry & visits dashboards' scopes

- sweep every 5 min: `active` with `valid_until < now` → `expired` (each transition appended to the
  chain as a system event, actor NULL).
- Host dashboard scopes: `upcoming` = active, `valid_from > now` (or any `draft`); `on_site` =
  last direction = entry; `ended` = `used|expired|revoked` today. Admin sees all, all hosts.

---

## 8. API reference

All under `/api`. JSON only. Auth by `Authorization: Bearer <JWT>` (web relays its cookie).
Standard error body: `{"error":{"code","message","details?"}}`.
Header `X-Request-ID` per request; logged without bodies.

| # | Method+Path | Roles | Notes |
|---|---|---|---|
| 1 | `POST /auth/login` | public | rate-limited; returns JWT + expiry |
| 2 | `POST /auth/logout` | any | clears cookie; token_version bump optional |
| 3 | `GET  /me` | any | self info; `must_reset_password` flag |
| 4 | `POST /me/password` | any | body `{old,new}`; bumps token_version; clears must_reset |
| 5 | `POST /users` | admin | mint host/guard; returns temp password once |
| 6 | `GET  /users?role=&q=` | admin | paginated |
| 7 | `POST /users/{id}/resetpassword` | admin | new temp password; bumps token_version |
| 8 | `POST /users/{id}/disable` | admin | sets disabled_at; bumps token_version |
| 9 | `POST /visits` | admin(any), host(own) | §7.2; caps per §7.1; `409 OVERLAP_WARN` if same visitor has active window (bounds-check only) |
| 10 | `GET  /visits?scope=upcoming|active|ended&q=` | admin/host | host → own only |
| 11 | `GET  /fill/{token}` | public | invite form metadata + random challenge for upload |
| 12 | `POST /fill/{token}` | public | final details + consent + optional photo; rate-limited `RATE_LIMIT_PUBLIC`; 409 if already activated |
| 13 | `GET  /passes/{id}/qr.png` | host(own/admin) | QR PNG, `sv1:<token>` |
| 14 | `GET  /passes/{id}/details` | host(own)/admin | DECRYPT; audit-logged |
| 15 | `POST /passes/{id}/revoke` | host(own)/admin | reason required |
| 16 | `POST /passes/{id}/extend` | admin | override event; within ceiling |
| 17 | `GET  /p/{token}` | public | visitor's own pass page data (masked: no photo, no PII beyond name/host/purpose/status/countdown) |
| 18 | `GET  /visitors/{id}/photo` | host(own pass)/admin/guard(active pass) | streams JPEG |
| 19 | `POST /walkins` | guard | §7.3 (without sticker binding: optional phone-mint variant) |
| 20 | `POST /stickers/claim` | guard | §7.3 with binding |
| 21 | `POST /stickers/batches` | admin | `{count}` ≤ 200/batch; returns printable sheet payload |
| 22 | `GET  /stickers?status=` | admin | pool management |
| 23 | `POST /stickers/{id}/retire` | admin | override event; required reason |
| 24 | `POST /scan` | guard/admin | §7.4; rate-limited |
| 25 | `GET  /logs?pass=&gate=&outcome=&limit=` | admin | chain-view + integrity info |
| 26 | `GET  /logs/verify` | admin | runs §9 chain check, returns `ok|broken_at_seq` |
| 27 | `GET  /events?actor=&action=&limit=` | admin | audit list |
| 28 | `POST /gates` `GET /gates` `PATCH /gates/{id}` | admin | CRUD |
| 29 | `GET  /dash/kpis` | admin/host | counts by scope (§12 dashboards) |
| 30 | `POST /ai/query` | admin | existing text2sql (allowlists intact) |
| 31 | `GET  /ai/anomaly/flags` | admin | proxy to sidecar |
| 32 | `POST /ai/anomaly/train` | admin | proxy |
| 33 | `GET  /internal/scans/{pass_id}/history` | ml (HMAC) | for sidecar feature calc |
| 34 | `POST /internal/logs/{log_id}/anomaly` | ml (HMAC) | write back score+reasons |
| 35 | `GET  /healthz` `GET /readyz` | public | readiness = db + master key + issuer key |

All admin/host/guard endpoints require `token_version` match.

---

## 9. Hash chain spec (access_logs + audit_events)

- Per-table single chain ordered by `seq`. Genesis `prev_hash = "0"*64`.
- Canonical input = `prev_hash || seq || id || scan_time(UTC RFC3339) || gate_id || pass_id ||
  guard_id||direction||outcome||reason` (audit: `actor_id||action||target||reason||meta_json_canonical`).
- `hash = hex(sha256(canonical_input))`. `meta_json_canonical`: keys sorted, no whitespace.
- `GET /logs/verify` recomputes all rows (bounded: last 100k), reports first mismatching `seq`.
- Nobody, including admin, *edits* logs; corrections are appended events. Deletion only via
  retention purge (of whole expired windows), which also logs a purge marker event.

---

## 10. Audit events (audit_events.actions)

`user.bootstrap, user.create, user.reset, user.disable, login.success, login.failure,
me.password_change, gate.create, gate.update, sticker.batch.generate, sticker.claim,
sticker.retire, visit.create, pass.activate, pass.revoke, pass.extend, pass.decrypt,
visitor.photo.set, walkin.create, config.flag, log.purge, offline.register`.
Every write to `passes/visitors/stickers/gates/users` creates exactly one audit event.
RLS not used; audit is enforced in code paths only — document as known cover-limit (page
constraints, solo build).

---

## 11. Anomaly sidecar (`ml`) — contract

- FastAPI (`python:3.12-slim` + scikit-learn), port 8100, reads **nothing** from db:
  it calls back `GET /internal/scans/{pass_id}/history` (HMAC `ML_SERVICE_TOKEN`.
- Endpoints:
  - `GET  /health`
  - `POST /train` (admin only, proxied through api): retrains IsolationForest (existing 7
    features from `extract_features`, sklearn, `contamination`), persists pickle in
    `artifacts`; returns `{model, n_samples, threshold, trained_at}` (degrades silently below 10
    samples to deterministic rules; existing fallback logic retained).
  - `POST /score {log context}` → `{score, is_suspicious, reasons[]}`; used by `internal/logs`
    writeback.
- Go flow (fully async, never blocks the decision at `/scan`): log committed → goroutine POSTs
  log context `{log_id, pass_id, gate_id, scan_time, direction, outcome}` to ml `/score` → ml
  calls back `GET /internal/scans/{pass_id}/history` (only read a pass's own history) → ml
  computes the 7 features + score → returns `{score, is_suspicious, reasons[]}` → Go appends
  `anomaly_score`/`anomaly_reasons` onto that log row (columns added by migration:
  `anomaly_score REAL NULL`, `anomaly_reasons JSONB NULL`). Sidecar down ⇒ score stays NULL,
  decision unaffected.
- NL queries (text2sql) remain **in Go** with existing allowlists, exactly as today.

---

## 12. Dashboards (supanext, `web`)

### 12.1 Integration notes
- Remove `better-auth` + `packages/auth` imports; API is Go JWT in an httpOnly cookie; client
  wrapper attaches it to `fetch`.
- Keep `packages/ui` (shadcn), `packages/utils`, i18n as-is. tRPC removed; use React Query +
  plain fetch against `/api`.
- Remove dormant packages (`marketing`, `payments`, `storage`, `docs`, `mail-preview`) from build.

### 12.2 Admin
- KPI strip: entries today, on-site now, upcoming, anomalies count, chain integrity badge
  (`/logs/verify`), stickers free.
- Pass list: filter by host/gate/status/date; actions: decrypt drawer (with pending-confirm),
  revoke (reason prompt), extend (reason + within-ceiling validation), view photo.
- Users: mint host/guard, reset password (temp password shown once), disable.
- Gate and sticker-pool management (generate batch → printable sheet, retire).
- Audit viewer (two tabs: scans/events) + English-query box (text2sql result grid).
- Anomaly list + train button.

### 12.3 Host
- KPI strip: on-site now / upcoming / ended today.
- "New visit" wizard (purpose, gate, window, visitor name/phone hint) → invite URL with copy
  button (+ optional email later).
- Pass cards per state with live countdown; decrypt drawer → shows name/phone/photo/`id_note`;
  revoke; per-pass scan feed.
- "Walk-ins assigned to me" section (guard-created, auto-attached to this host).

### 12.4 Guard console (mobile-first, dark, huge tap targets)
- Scan tab: camera QR reader (jsQR via browser), directional switch (entry/exit), result card:
  big green/red, reason, name, photo, host, "badge closes at entry" note.
- Walk-in form (§7.3) + free-sticker scanner widget.
- "Recent scans" (this gate, this shift).
- Offline banner: API unchecked = fail closed + instructions to use paper fallback sheet.

### 12.5 Visitor page `/p/{token}`
No login. Shows: name, host name, purpose, QR, status + countdown, gate name, "show this at the
gate" note. Link is a secret ("don't share") warning. Rate-limited.

### 12.6 Sticker printing
After batch generation: printable A4 sheet of QRs (grid), each with short human code
(`B-0042-XK` = batch-sequence-random3) + "unassigned" watermark. Browser print → paper → affix
to plastic sleeves. Admin marks `printed` post-hoc.

---

## 13. Jobs (in Go, alongside API, no extra service)

- every 5 min: expiry sweep (§7.5).
- daily 03:30 local: retention sweep — for passes ended > `RETENTION_PASS_DAYS`: delete
  `pass_secrets` rows (crypto-shred), delete photo file, status → `shredded`, tombstone write.
- daily 04:00: log purge pass (`RETENTION_LOG_DAYS`) with purge marker audit event.

---

## 14. Observability

- `/healthz` liveness; `/readyz` includes DB ping, master key load test, issuer key presence.
- Every request logged: id, path, role, latency. **No tokens, no bodies, no PII in logs.**
- Anomaly assistant never blocks the scan path (async only).
- Errors: rate-limit headers on 429; unique `X-Request-ID` in response body header.

---

## 15. Testing plan

**Unit/Integration (Go):**
- status-machine table-driven tests per §7.1 (incl. ceiling enforcement).
- chain tamper test: recompute verify on seeded 200-row fixture (+ row N mutated) expect
  `broken_at_seq=N`.
- rate limits; login failures lockout; temp-password expiry; token_version invalidation.

**Attack suite (must all behave as specified):**

| Attack | Expected |
|---|---|
| Replay an approved token at second entry | denied `ALREADY_USED` (one_time) |
| Use pass at wrong gate | denied `WRONG_GATE` |
| Post-expiry scan | denied `NOT_IN_WINDOW` |
| Revoked pass scan | denied `PASS_REVOKED` |
| Unbound sticker scan | denied `STICKER_UNBOUND` |
| Random/forged token (`sv1:AAAA...`) | denied `UNKNOWN_TOKEN` |
| Re-post same invite token | `410 TOKEN_USED` |
| Ramp 1000 fill POSTs/min | 429 before DB pressure |
| Guard decrypts pass via API | 403 (no route) |
| Host decrypts foreign pass | 403 + audit `forbidden_attempt` |
| text2sql `DROP TABLE users;` | 400, allowlist intact |
| DB theft: read `pass_secrets.blob` | undecipherable without `MASTER_KEY` |
| Same sticker re-claim | `409 STICKER_ASSIGNED` |
| Chain row NULLed of past seq | verify flags first mismatch |

**Perf targets:** p95 `/scan` < 300 ms, p95 `/p/{token}` < 500 ms, QR PNG < 100 ms, anomaly
async p50 < 3 s (never on decision path).

**E2E smoke (script `scripts/smoke.sh`):** admin login → mint host → host login → create visit →
fill invite → fetch QR → guard scan in → guard scan out (window mode) → admin decrypt → verify
chain → run 3 walkthrough queries.

---

## 16. Build order (solo)

| Phase | Scope | Acceptance |
|---|---|---|
| A | Bootstrap, JWT+RBAC middleware, users/gates/audit scaffolds; delete legacy `src/` python; DDL migration; bootstrap-admin path | smoke: login admin, mint host+guard; RBAC denies host hitting `/users`; chain tables exist with genesis |
| B | visitors/visits/passes/secrets/invite/fill/scan + chains; `-p/{token}`; QR route | full API flow via curl; attack suite §15 rows 1–10 pass; p95 met on scan |
| C | Admin + Host + Guard dashboards + visitor page in supanext (Streamlit deleted) | All role pages reachable with correct visibility; decrypt drawer writes audit rows |
| D | Walk-in + sticker claim + printable batch sheets | Phone-less visitor round-trip works on one device; sticker statuses obey lifecycle |
| E | ml sidecar hookup (train + flags), text2sql surfaced admin-only, perms on `/internal/*` | flags populate after seed; admin sees suspicious feed; internal HMAC rejects others |
| F | Retention jobs, rate-limit tuning, offline-fallback sheet + paper register, `/healthz`+`readyz`, `docs/runbook.md`, README regen, compose prod profile | `docker compose up` from clean; retention dry-run sweeps; runbook complete |

Phases A–B are already a deployable v0 demo; C–D make it a real pass system; E–F are the
production-hardening tail.

---

## 17. Acceptance checklist (overall)

- [ ] Compose up from clean repo → seeded admin login works.
- [ ] Admin mints host with temp password; first login forces change; old JWT dies after reset.
- [ ] Host creates visit; visitor fills link; QR works at gate (entry approved → `used`).
- [ ] Walk-in flow: sticker free → claimed → binds pass → scans at entry, one-time ends.
- [ ] Admin decrypt drawer works and appends `pass.decrypt` event; guard cannot decrypt (403).
- [ ] Both chains verify clean after ~200 seeded events; mutation test marks broken seq.
- [ ] text2sql never surfaces `id_note` unless its column is allowlisted explicitly (it is not).
- [ ] No PII appears in server logs or errors; attack suite green; offline mode fails closed.
- [ ] `users` table has no visitor rows; `visitors` never authenticates (no login route).

---

## 18. Appendix — canonical behaviors & shapes

- **TS time format:** RFC3339 UTC everywhere in API bodies.
- **IDs:** UUIDv4 in responses; opaque except users (which admins see).
- **Pagination:** `?limit<=100&cursor=<seq|id>` returning `{items, next_cursor}`.
- **Photo upload:** client→`POST /fill/{token}/photo` returns storage key; server resizes
  (sharp-equivalent) to 480px JPEG q80; deterministic filename `{visitor_id}.jpg` in `photos`.
- **Reason lexicon (humanized):**
  `BAD_TOKEN → "this code isn't recognised"`, `UNKNOWN_TOKEN → "unknown pass — check with admin"`,
  `NOT_IN_WINDOW → "pass not active yet/expired"`, `WRONG_GATE → "this pass is for {gate}"`,
  `ALREADY_USED → "already entered — badge closed"`, `PASS_REVOKED → "cancelled by host/admin"`,
  `STICKER_UNBOUND → "sticker not yet issued"`, `INTERNAL → "system error — try again / call admin"`.
- **supabase/GCP/etc: none.** Everything is Postgres + volumes on one box.
- **Backups:** `pg_dump` cron + volume rsync (documented in runbook; compose profile `backups`
  optional).

EOF OF PLAN
