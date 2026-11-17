# Split frontends on GitHub Pages — plan

> Status: proposal for review — Nov 17 2026.
> Scope: host admin, guard, scanner and visitor UIs as separate static sites on
> `*.github.io`, talking to the existing Go API over HTTPS.
> Current single-origin deployment (see `deploy/README.md`) stays live until each
> static app reaches parity.

---

## 0. TL;DR

| Question | Answer |
|---|---|
| Can GitHub Pages host these four UIs? | **Yes, but only as static single-page apps.** Pages cannot run Next.js SSR, server actions, route handlers or cookies. |
| Can it host the current app unchanged? | **No.** `getGoSession()` runs server-side in `app/(authenticated)/layout.tsx`; auth, redirects and the image proxy are server features. |
| Do the requested hostnames work? | Each `https://<name>.github.io` requires a GitHub **account or org named `<name>`** plus a repo named `<name>.github.io`. They are independent accounts. |
| Are the names available? | See §2 — `smart-visitor` is taken, `smart-vistor` (typo) is free, `sm-scanner`, `sm-guard`, `sm-visitor`, `smart-visitors` appear free. |
| Is it a good first move? | No. It is a ~2-week refactor with new security surface (CORS + bearer tokens). Recommended: one org with path-based sites first (or keep the current single origin). |
| What must change in the backend? | CORS allowlist, real client IP behind Cloudflare, configurable visitor/scanner base URLs. Bearer auth already works. Details §5. |

---

## 1. What GitHub Pages actually provides

| Capability | Available | Notes |
|---|---|---|
| Static HTML/CSS/JS | ✅ | Vite/React/Next static export all fine |
| HTTPS | ✅ | automatic, required for camera access |
| Custom 404 page | ✅ | used for SPA deep-link fallback |
| Custom domain (CNAME) | ✅ | one domain per Pages site |
| Server-side rendering | ❌ | no Node process |
| Set-Cookie / httpOnly sessions | ❌ | auth must move to bearer tokens |
| API / database | ❌ | API stays on the home PC + tunnel |
| SPA URL rewrites | ❌ | use hash routes or `404.html` fallback |
| Custom response headers | ❌ | CSP must be a `<meta>` tag; no `frame-ancestors` |
| Private repos on free plan | ❌ | Pages needs a public repo (or a paid plan) |
| Limits | — | ~1 GB site, ~100 GB/month bandwidth, 10 builds/hour — far above this project's needs |

**Critical detail — `github.io` is on the Public Suffix List.** Browsers treat
`sm-guard.github.io` and `sm-visitor.github.io` as *different sites*. Cookies cannot be
shared across them, and Safari/Firefox block third-party cookies. Therefore:

- every static app authenticates with `Authorization: Bearer <JWT>` (the Go API already
  returns the token in the login response);
- no cross-site cookie flows are designed or relied on;
- the existing `sv_session` cookie remains only for the single-origin tunnel app.

---

## 2. Hostname options

### 2.1 Availability check (GitHub API, Nov 17 2026)

| Requested | Status | Comment |
|---|---|---|
| `smart-vistor.github.io` | free | typo; fix the name before creating anything |
| `smart-visitor.github.io` | **taken** | existing GitHub organisation `smart-visitor` |
| `sm-scanner.github.io` | free | usable |
| `sm-guard.github.io` | free | usable |
| `sm-visitor.github.io` | free | usable |
| `smart-visitors.github.io` | free | best admin/root candidate |
| `sm-host.github.io` | free | host dashboard has no hostname in the request yet |

Availability is not a reservation — names can be taken before creation, and GitHub can
reserve names.

### 2.2 Four deployment shapes

**A. One org, path-based sites — recommended**

Create one org/account `smart-visitors` (or reuse `eva-protoype`) and one repo
`smart-visitors.github.io` that publishes four folders:

```
https://smart-visitors.github.io/admin/
https://smart-visitors.github.io/guard/
https://smart-visitors.github.io/scanner/
https://smart-visitors.github.io/visitor/
```

- one account, one Pages site, one CI workflow, one build;
- shared code stays in one monorepo;
- URLs are not the exact four hostnames requested.

**B. Four separate orgs — exact hostnames**

Create orgs `sm-guard`, `sm-scanner`, `sm-visitor` and `smart-visitors` (admin), each
with a `<org>.github.io` repo. To keep shared API/auth code in one monorepo, CI must
push each build to the corresponding repo (deploy key / PAT), or the shared package
must be published to npm/GitHub Packages. Four Pages settings, four workflows, four
places to debug.

**C. Custom domains (best branding, not github.io)**

If a domain is acceptable (`admin.example.com`, etc.), one static host can serve all
four with CNAMEs, or Cloudflare can route subdomains. Costs ~₹700–1000/year. This also
avoids lookalike-github.io phishing concerns.

**D. Hybrid — current state**

Keep the single-origin app on the tunnel; use one GitHub Pages site only as a
showcase/landing page. Zero refactor, zero CORS work. Already implemented in
`deploy/github-pages/`.

> Recommendation: **A now, C later if branding matters.** Only choose B if the exact
> `*.github.io` hostnames are a hard requirement.

---

### 2.3 Chosen shape (Nov 17): org `project-DawnGuard`

The org exists as the single account; four **project** Pages sites plus a root landing
site, all on one origin:

| Repo | URL | Purpose |
|---|---|---|
| `project-DawnGuard.github.io` | `https://project-dawnguard.github.io/` | landing/showcase |
| `admin` | `https://project-dawnguard.github.io/admin/` | admin console |
| `guard` | `https://project-dawnguard.github.io/guard/` | guard console |
| `scanner` | `https://project-dawnguard.github.io/scanner/` | gate scanner kiosk |
| `visitor` | `https://project-dawnguard.github.io/visitor/` | invite + pass pages |
| `host` (optional) | `https://project-dawnguard.github.io/host/` | host dashboard, if split out |

Because all four paths share the origin `https://project-dawnguard.github.io`, CORS
needs **one** allowlist entry, not four. Repos are created by
`deploy/github/bootstrap-org.sh`; GitHub has no API to create the org itself, so the
org must be created once in the web UI.

---

## 3. Target architecture

```
admin.github.io    ─┐
guard.github.io    ─┤   HTTPS + JSON + Authorization: Bearer JWT
scanner.github.io  ─┼──────────────▶  https://api.example.com (Go, home PC)
visitor.github.io  ─┘                        │
                                             ├─ Postgres (internal)
                                             └─ ML sidecar (internal)
```

- Pages serves immutable static assets from GitHub's CDN.
- The API stays exactly where it is now (Caddy + Go behind the Cloudflare Tunnel).
- No static app contains secrets: API base URL is public, tokens are user-scoped.
- Visitor invite links and pass links are generated by the API and must point at the
  visitor app (`VISITOR_BASE_URL`), not at `APP_BASE_URL`.

Request path for a guard scan:

```
scanner app (camera) → decode QR locally → POST /api/scan
  Authorization: Bearer <token>
  Origin: https://sm-scanner.github.io
→ Go API verifies JWT + role, logs scan, returns result card
```

---

## 4. Authentication & session model

### 4.1 Login

1. App calls `POST /api/auth/login` with `{username, password}`.
2. API returns `{token, expires_at, user}` (already implemented) **and** continues to set
   the cookie for the single-origin app. Static apps ignore the cookie.
3. App stores the token:
   - admin / guard: `sessionStorage` (dies with the tab; a shared front desk PC should
     log in per shift);
   - scanner kiosk: `localStorage` with a visible "remember this device" toggle — a
     stolen kiosk token is the main residual risk (see §8); 12 h TTL limits it;
   - visitor: no token — public endpoints only.
4. Every request sends `Authorization: Bearer <token>`.
5. On `401`, clear the token and show the login screen. Decode `exp` client-side to
   pre-empt expiry and show a countdown.

### 4.2 Authorization

- Client-side role gate is UX only; the API remains the source of truth.
- Each app ships only its role's screens, but a host who opens the admin bundle still
  gets `403` from admin endpoints.
- Existing `token_version` invalidation (password reset / disable) keeps working; old
  tokens die server-side.

### 4.3 Optional hardening — kiosk device tokens (phase P2.5)

Add `POST /api/auth/device-token` (admin-issued, role=`guard|scanner`, 7-day TTL,
`device_id`, revocable, audit event) so a wall-mounted scanner does not need a human
password every day. Not required for the first release; the kiosk can log in per shift.

### 4.4 CSRF

Bearer tokens are not attached automatically by browsers, so classic CSRF does not
apply. CORS is still restricted to the exact app origins.

---

## 5. Backend changes required

Existing facts verified in the repo:

| Fact | Location |
|---|---|
| Bearer auth already supported | `backend/internal/auth/auth.go:221-225` |
| No CORS middleware anywhere | `backend/` — none found |
| Rate limiting uses `r.RemoteAddr` | `backend/internal/handlers/passes.go:1118` (`clientIP`) |
| One base URL builds all public links | `APP_BASE_URL` in `passes.go:257,420`, `self_entry.go:196` |
| Login returns a token in JSON | `backend/internal/handlers/auth.go` |

### 5.1 CORS middleware — new `backend/internal/http/CORS` (or `internal/middleware`)

```go
// env: CORS_ALLOWED_ORIGINS=https://sm-guard.github.io,https://sm-scanner.github.io,...
func CORS(allowed map[string]bool) func(http.Handler) http.Handler
```

Requirements:

- exact-match `Origin` against the allowlist; never `*` for authenticated routes;
- `Vary: Origin`;
- allow methods `GET, POST, PATCH, PUT, DELETE, OPTIONS`;
- allow headers `Authorization, Content-Type`;
- `Access-Control-Max-Age: 600`;
- answer `OPTIONS` with `204` **before** auth middleware;
- no `Access-Control-Allow-Credentials` (bearer only);
- reject unknown origins by omitting CORS headers (browser blocks), optionally `403`
  for non-GET.

Wire in `backend/main.go` around the `/api` router. Keep `/healthz`, `/readyz` simple.

### 5.2 Real client IP behind the tunnel

Currently `clientIP()` returns the `cloudflared` container address for every request, so
`RATE_LIMIT_SCAN`, `RATE_LIMIT_LOGIN` and audit IPs are effectively global. Required:

- config `TRUST_PROXY_HEADERS=true` and `TRUSTED_PROXY_CIDRS=172.16.0.0/12`;
- if the immediate peer is trusted, take the first valid address from
  `CF-Connecting-IP`, else the first untrusted hop in `X-Forwarded-For`;
- fall back to `RemoteAddr`;
- unit tests for spoofed headers from untrusted peers;
- use it for rate-limit keys and scan/audit metadata.

Without this, public static apps make the rate limiter trivially incorrect.

### 5.3 Configurable public URLs

Add config values (default to `APP_BASE_URL` for backwards compatibility):

```
VISITOR_BASE_URL=https://sm-visitor.github.io
SCANNER_BASE_URL=https://sm-scanner.github.io
ADMIN_BASE_URL=https://smart-visitors.github.io
GUARD_BASE_URL=https://sm-guard.github.io
```

Change link builders:

| File | Current | New |
|---|---|---|
| `handlers/passes.go:257` | `APP_BASE_URL + "/fill/" + token` | `VISITOR_BASE_URL + "/#/fill/" + token` (hash routing) |
| `handlers/passes.go:420` | `APP_BASE_URL + "/p/" + rawQRToken` | `VISITOR_BASE_URL + "/#/p/" + rawQRToken` |
| `handlers/self_entry.go:196` | same | same |

If path-based deployment (Option A), set `VISITOR_BASE_URL=https://smart-visitors.github.io/visitor`
and keep the same suffix logic.

### 5.4 Media over CORS

- `GET /api/visitors/{id}/photo` returns an image; the static app cannot use `<img src>`
  with an Authorization header → fetch as `Blob` and use `URL.createObjectURL`.
- `GET /api/passes/{id}/qr.png` same pattern, or render the QR client-side from the raw
  token (the visitor app already receives it) to avoid an authenticated image request.
- `POST /api/fill/{token}/photo` is multipart; preflight must allow `Content-Type` and
  the origin. Keep the existing 2 MB / resize limits.

### 5.5 Caching

Add `Cache-Control: no-store` to authenticated responses and public pass-detail
responses. Static assets caching is handled by Pages.

### 5.6 Rate limits

Apply per real IP + username (login already does per username+IP). Scan/visitor public
endpoints must remain per real IP after §5.2. Cloudflare WAF rate limiting is optional
defence in depth, not a replacement.

---

## 6. Frontend plan

### 6.1 Why not reuse the Next.js app as-is

The current `frontend/apps/saas` app uses:

- `output: "standalone"` (server build);
- server layouts that call `getGoSession()` and `redirect()`;
- server-side auth cookies;
- `next-intl` middleware;
- an image proxy route handler;
- a large amount of dead better-auth / oRPC / payments code that still type-checks.

Static export would force every page to be client-side anyway. Four small Vite apps are
cleaner, faster to build and cheaper to host than carving four exports out of Next.

### 6.2 Proposed structure

```
frontend/
  packages/web-core/          # shared, framework-agnostic
    src/api.ts                # fetch wrapper: base URL, bearer, errors, blob fetch
    src/auth.ts               # token storage, expiry, login/logout, role guard
    src/qr.ts                 # jsQR camera hook + QR PNG generation
    src/types.ts              # DTOs mirrored from the Go API
  apps/admin-web/             # Vite + React + TS + TanStack Query + Tailwind
  apps/guard-web/
  apps/scanner-web/
  apps/visitor-web/
  apps/host-web/              # optional, see §6.3
```

Reuse `@repo/ui` tokens/components where they do not depend on Next. If extraction is
slow, copy the handful of shadcn components used; do not import the Next app.

### 6.3 App scopes

| App | URL (option A) | Screens | Auth |
|---|---|---|---|
| Admin | `/admin/` | KPI strip, passes table + decrypt drawer, users, gates, sticker batches, audit/scan logs, chain verify, anomalies, NL query | admin |
| Guard | `/guard/` | Login, walk-in wizard, sticker claim, host lookup, recent scans | guard |
| Scanner | `/scanner/` | Full-screen camera, direction toggle, result card, recent scans, PWA/offline banner | guard (kiosk) |
| Visitor | `/visitor/` | Invite form (`#/fill/<token>`), photo + consent, pass page (`#/p/<token>`), countdown | public |
| Host | `/host/` (proposed) | Invite wizard, own visitors (upcoming/on-site/ended), revoke, scan feed | host |

The request did not assign the host dashboard a hostname — `sm-host.github.io` is free;
otherwise add `/host/`. Decide before phase P3.

**"QR scanner at host side" is ambiguous.** The plan assumes `scanner` is a gate/kiosk
scanner used by guards. If it should be a host-side arrival verifier, its auth role
becomes `host` and the endpoint surface changes (read-only pass lookup + acknowledge
arrival), which needs a new API endpoint. Resolve in §12.

### 6.4 Routing on Pages

Pages serves `404.html` for unknown paths. Two workable options:

1. **Hash routing (recommended for visitor links):**
   `https://sm-visitor.github.io/#/fill/<token>` and `#/p/<token>`.
   No server config, works everywhere. API link builders must produce these.
2. **404 fallback:** copy `index.html` to `404.html`; the SPA reads `location.pathname`.
   Cleaner URLs, but every route returns HTTP 404 (bad for previews/SEO; irrelevant for
   an internal tool).

Admin/guard/scanner are single-screen apps; they can use plain paths with the 404 trick
or no routing at all.

### 6.5 QR scanning

- Library: `jsQR` (small, no wasm) or `@zxing/browser`; neither is currently installed.
- `navigator.mediaDevices.getUserMedia({ video: { facingMode: "environment" } })` —
  requires HTTPS (Pages provides it) and a user gesture.
- Decode locally; send only `{token, gate_id, direction}` to `/api/scan`.
- Guard against duplicate scans: debounce 2 s per token, show the result card until the
  next scan.
- Manual fallback: type the short code / paste `sv1:` token when the camera fails.
- iOS Safari: test permission prompt, orientation changes, and low-light; a torch toggle
  is optional (`track.applyConstraints({ advanced: [{ torch: true }] })` where supported).

### 6.6 Static-app security

- CSP via `<meta http-equiv="Content-Security-Policy">`:
  `default-src 'self'; connect-src https://api.example.com; img-src 'self' blob: data:; script-src 'self'; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'`.
- No third-party scripts, analytics or fonts (supply-chain/XSS surface).
- React escapes visitor fields; never use `dangerouslySetInnerHTML` for API data.
- Never put `MASTER_KEY`, DB URLs or service tokens in `VITE_*` env (all bundles are
  public). Only `VITE_API_BASE_URL` is baked in.
- Tokens in `sessionStorage` are readable by any XSS; short TTL + strict CSP is the
  mitigation, not storage choice.

---

## 7. CI/CD and repo layout

### Option A — one org, one Pages site (recommended)

```
smart-visitors/smart-visitors.github.io   ← source repo (or a folder in this monorepo)
  dist/admin/ guard/ scanner/ visitor/
```

Workflow:

1. build all four Vite apps with `VITE_API_BASE_URL`;
2. copy to `dist/<app>/`;
3. `actions/upload-pages-artifact` + `actions/deploy-pages`.

Keep the existing `deploy.yml` for API images unchanged. Add the Pages origin to
`CORS_ALLOWED_ORIGINS` in `.env` and restart the API.

### Option B — four orgs, four repos

- Shared code as a published package (`@smart-visitors/web-core` on GitHub Packages) or
  duplicated source; otherwise builds cannot share the API client.
- Each repo has its own `pages.yml`; a monorepo deploy needs a PAT/deploy key to push
  to the other repos.
- Four sets of repository variables for `VITE_API_BASE_URL` and Pages settings.
- Higher maintenance for cosmetic hostname differences.

### Environments

- `main` → production Pages + production API.
- PR previews: run the Vite build and publish screenshots; GitHub Pages has no native
  preview URLs. Not a blocker for this project.
- Keep the current single-origin app deployable as rollback.

---

## 8. Security / risk register

| Risk | Impact | Mitigation | Residual |
|---|---|---|---|
| XSS steals bearer token from session/localStorage | Impersonation until expiry | Strict CSP, no third-party scripts, React escaping, dependency review, 12 h TTL, token_version revoke | Low–medium |
| CORS misconfiguration | Any site calls API with a user's token (token still needed) | Exact origin allowlist, no `*`, no credentials, integration tests per origin | Low |
| Third-party cookie blocking | Login broken if cookies were used | Bearer only; no cookie dependency | None by design |
| `github.io` lookalike phishing | Credential theft | Publish fixed URLs, train staff, custom domain later, 2FA on GitHub | Medium |
| Public source code | Reveals endpoints/roles | API enforces authz; no secrets in bundles | Accepted |
| Kiosk token theft (localStorage) | Gate scans under guard identity | Device token rotation/revocation, physical control, short TTL | Medium |
| Rate-limit bypass behind tunnel | Brute force / spam | Trusted-proxy client IP extraction (§5.2), Cloudflare rate rules | Low |
| API single point of failure (home PC) | All four UIs unusable | Fail-closed UX, paper fallback, monitoring, UPS | Accepted for now |
| GitHub org/account compromise | Defacement, malicious JS served to staff | 2FA, minimal org membership, no deploy keys with broad scope | Low |
| Visitor pass token leakage via URL | Someone else views/uses pass | Capability URL treated as secret; short validity; one-time entry; rate limits | Accepted |

---

## 9. Testing plan

### Backend

- CORS unit tests: allowed origin echoes; unknown origin gets no CORS headers; `OPTIONS`
  returns 204 without auth; `Authorization` allowed.
- Client IP tests: `CF-Connecting-IP` honored only from trusted peer; spoofed header from
  an untrusted peer ignored; malformed headers fall back safely.
- Bearer tests per role: admin/guard token on wrong-role endpoints → 403; expired token →
  401; token_version bump invalidates.
- Link builder tests: invite/pass URLs use `VISITOR_BASE_URL` with hash routes.
- Rate-limit integration: two clients with different forwarded IPs do not share a bucket.

### Frontend / E2E (Playwright, one suite per app)

- visitor: open invite link → fill → consent → photo → pass page countdown → API errors
  (used/expired token).
- scanner: mocked camera stream → decode fixture QR → approve/deny card paths.
- guard: walk-in form → claim sticker → recent scans.
- admin: login → KPI → decrypt (audit written) → revoke.
- cross-origin photo/QR blob fetch renders.
- deep links (`#/fill/...`, `#/p/...`) survive refresh.
- expired token → login screen, no infinite redirect loop.
- CSP: injected inline `<script>` is blocked; no console errors on load.

### Real devices

- Android Chrome + iOS Safari camera scan over HTTPS; permission denied path; manual
  code fallback; screen rotation; low light.
- Test on the actual college network and on mobile data (tunnel path).

### Security

- `grep -R` the built bundles for `MASTER_KEY`, DB URLs, service tokens.
- CORS check from an evil origin (`https://evil.example`) returns no allow headers.
- Dependency audit (`pnpm audit`) and pinned lockfile.
- Verify `github.io` repo has 2FA and no unnecessary collaborators.

---

## 10. Phased delivery

| Phase | Work | Acceptance | Estimate |
|---|---|---|---|
| **P0** Backend foundations | CORS middleware, trusted proxy IP, configurable base URLs, tests, deploy to tunnel | Preflight from each planned origin passes; evil origin blocked; rate limits keyed per real IP; invite links point at visitor base URL | 1–1.5 d |
| **P1** Visitor static app | `web-core` API/auth client, Vite visitor app, fill + pass pages, photo upload, deploy Pages | Full invite→pass round-trip from a phone; no cookies used; link builders correct | 2 d |
| **P2** Scanner app | Camera scanner, result card, direction toggle, PWA shell, deploy | Guard scans entry/exit at the gate; camera works on Android + iOS; manual fallback | 2 d |
| **P3** Guard app + host decision | Walk-in wizard, sticker claim, recent scans; host app if required | Phone-less walk-in fully created and scanned from the static app | 2–3 d |
| **P4** Admin app | Port admin dashboards to static client; decrypt/revoke/users/gates/stickers/audit/NL query | Admin parity with current app; audit rows still written; role checks enforced | 3–4 d |
| **P5** Hardening | CSP, device tokens, dependency audit, real-device matrix, runbook | Security checklist green; rollback path documented | 1–2 d |
| **P6** Cutover | Point links at Pages, monitor for a week, retire old routes if clean | No 4xx/5xx regression; staff using static URLs | 0.5 d |

Total ≈ **11–14 developer-days** plus real-device testing. The existing single-origin
app remains the fallback throughout.

---

## 11. Costs and limits

- Four GitHub Pages sites (or one): free for public repos; bandwidth/build limits far
  above this project.
- Four orgs: free, but four accounts to secure and maintain. Org creation does not cost
  money; it costs administration.
- Domain (optional): ₹700–1000/year for custom subdomains.
- API hosting unchanged (home PC + free tunnel, or ~₹350–1000/month VM).
- GitHub Actions: free for public repos; Pages deployment is a few seconds.

---

## 12. Decisions needed before P1

1. ~~URL shape~~ **Decided:** one org `project-DawnGuard` with project Pages sites
   (`/admin/`, `/guard/`, `/scanner/`, `/visitor/`) plus a root landing site. Revisit
   only if exact `*.github.io` hostnames are still required.
2. **Admin location:** `/admin/` (current plan) or the org root `https://project-dawnguard.github.io/`?
3. **Host dashboard:** `/host/`, `sm-host.github.io` (separate org — not planned), or
   leave hosts on the single-origin app for now?
4. **What "scanner" means:** gate kiosk (assumed) or host-side arrival verification? The
   latter changes the role and needs a new API endpoint.
5. **Domain budget:** is a real domain acceptable later for branding? It removes the
   lookalike-github.io risk and enables `admin.`/`guard.` subdomains.

---

## 13. Alternative that avoids all of this

Keep the current single-origin app (Caddy routes `/`, `/login`, `/p/*`, `/fill/*`) on the
tunnel. Use one GitHub Pages site only as a showcase/landing page (already built). This
keeps cookies same-origin, needs no CORS, no token storage, no SPA rewrite, no four
accounts, and no 2-week detour. Revisit the split only if the four distinct URLs become
a real requirement (branding, separate staff portals, or kiosk isolation).
