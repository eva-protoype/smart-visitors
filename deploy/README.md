# Deploying Smart Visitors

Two supported shapes:

| Shape | Public URL | Cost | Best for |
|---|---|---|---|
| **Home / college PC + Cloudflare Tunnel** | `https://visitors.yourdomain.com` (or random `*.trycloudflare.com` for a demo) | free + optional domain | no public IP, NAT/CGNAT, no router config |
| **VM + Docker Compose + GHCR images** | `https://visitors.yourdomain.com` via Caddy | ~₹350–1000/mo | always-on, no machine at home |

GitHub Pages **cannot** host this app (Next.js SSR, cookies, Go API, Postgres). It can host a static showcase page that links to the live app — see [GitHub Pages](#github-pages-showcase).

---

## 1. Home PC + Cloudflare Tunnel

```
browser ──https──▶ Cloudflare edge ──tunnel──▶ cloudflared ──http──▶ caddy ──▶ web / api
                                                                       ├──▶ db  (internal)
                                                                       └──▶ ml  (internal)
```

No inbound ports, no port forwarding, no public IP. TLS terminates at Cloudflare;
`cloudflared` only makes **outbound** connections.

### 1.1 Prerequisites

- Linux/macOS/Windows machine that can stay on (8 GB RAM is plenty; 4 GB works)
- Docker + Docker Compose v2
- Optional but recommended: a domain on a **free Cloudflare plan** (₹700–1000/yr)
- The repo cloned on that machine

### 1.2 Create the named tunnel

1. Cloudflare dashboard → **Zero Trust → Networks → Tunnels → Create a tunnel**
2. Type **Cloudflared**, name it `smart-visitors`
3. Copy the **token** (`eyJhIjoi...`)
4. **Public hostname** tab → Add:
   - Subdomain: `visitors`
   - Domain: `yourdomain.com`
   - Service: **HTTP** → `caddy:80`
   - (optional) HTTP Host Header: leave empty when `APP_DOMAIN` matches the public hostname
5. DNS record is created automatically.

> `caddy:80` resolves because `cloudflared` runs on the same Docker network as Caddy.
> If the hostname is set to `visitors.yourdomain.com`, then `APP_DOMAIN` in `.env` must be
> `visitors.yourdomain.com` — Caddy matches the incoming `Host` header against it.

### 1.3 Configure `.env`

```dotenv
# public origin
APP_DOMAIN=visitors.yourdomain.com
APP_SCHEME=http
APP_BASE_URL=https://visitors.yourdomain.com
NEXT_PUBLIC_SAAS_URL=https://visitors.yourdomain.com

# multi-origin static apps (GitHub Pages)
CORS_ALLOWED_ORIGINS=https://project-dawnguard.github.io
TRUST_PROXY_HEADERS=true
TRUSTED_PROXY_CIDRS=172.16.0.0/12
# leave empty until the visitor Pages app ships; defaults to APP_BASE_URL
VISITOR_BASE_URL=
VISITOR_HASH_ROUTES=false

# tunnel
CLOUDFLARE_TUNNEL_TOKEN=eyJhIjoi...

# keep this machine's ports off the LAN as well
HTTP_HOST_PORT=4000
```

Leave `APP_SCHEME=http`: Cloudflare terminates TLS, so Caddy must **not** redirect
HTTP → HTTPS at the origin. The Go API detects `X-Forwarded-Proto: https` from
Cloudflare and marks the session cookie `Secure` automatically.

### 1.4 Start

```bash
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d --build
```

Check:

```bash
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml ps
curl -I https://visitors.yourdomain.com/login
```

Browser camera access for the guard console requires HTTPS — the tunnel provides it.

### 1.5 Cloudflare settings

- **SSL/TLS → Overview:** mode **Full** (origin is HTTP behind the tunnel; Flexible also works but Full is clearer)
- **SSL/TLS → Edge Certificates:** *Always Use HTTPS* on
- **Caching → Cache Rules:** bypass cache for `/api/*`, `/p/*`, `/fill/*`
- **Speed:** leave Brotli on; Rocket Loader off (it can break Next.js hydration)
- Optional: **Zero Trust → Access** policy in front of `/login` if only staff should reach the dashboards. Do **not** put Access in front of `/p/*`, `/fill/*`, or `/api/scan` — visitors and guards need them.

### 1.6 Quick demo URL (no domain)

```bash
deploy/quick-tunnel.sh
```

Prints a random `https://<something>.trycloudflare.com` URL. It changes every restart,
so it is for demos only — invite links will still point at `APP_BASE_URL` (`localhost`),
so only login/dashboards are fully usable. Stop with Ctrl+C.

---

## 2. VM + prebuilt images (optional)

Any Ubuntu VM works (Hetzner CX22 ≈ €3.79, DO 1–2 GB, Oracle free ARM). Compose is the
same; only the image source changes.

1. Push to `main` → GitHub Actions builds and pushes:
   - `ghcr.io/<owner>/smart-visitors-api:latest`
   - `ghcr.io/<owner>/smart-visitors-web:latest`
   - `ghcr.io/<owner>/smart-visitors-ml:latest`
2. First run only: make each package **public** (repo → Packages → package → Settings →
   Danger Zone → Change visibility) **or** log in on the VM with a PAT that has
   `read:packages`:
   ```bash
   echo "$GHCR_READ_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
   ```
3. On the VM:
   ```dotenv
   IMAGE_PREFIX=ghcr.io/eva-protoype
   IMAGE_TAG=latest
   ```
4. Deploy:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.prod.yml pull
   docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --no-build
   ```

### Automatic SSH deploy (VM only)

Repo → Settings → Secrets and variables → Actions:

| Kind | Name | Value |
|---|---|---|
| secret | `DEPLOY_HOST` | VM IP / hostname |
| secret | `DEPLOY_USER` | `ubuntu` (or your user) |
| secret | `DEPLOY_SSH_KEY` | private key (ed25519) |
| secret | `DEPLOY_PATH` | `/home/ubuntu/smart-visitors` |
| variable | `DEPLOY_ENABLED` | `true` |

The deploy job pulls `main`, loads `docker-compose.prod.yml`, and adds
`docker-compose.tunnel.yml` automatically when `CLOUDFLARE_TUNNEL_TOKEN` is set.

### Home PC without public SSH

Use cron instead — it pulls images itself:

```cron
*/15 * * * * /home/you/smart-visitors/deploy/update.sh >> /home/you/smart-visitors/deploy/backups/update.log 2>&1
```

Prefer building locally? Skip the CI images entirely:

```bash
git pull && docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d --build
```

---

## 3. First-boot security checklist

- [ ] `JWT_SECRET` and `MASTER_KEY` are fresh 64-hex values, not the dev ones:
      `openssl rand -hex 32`
- [ ] `POSTGRES_PASSWORD`, `ML_SERVICE_TOKEN` are long random values
- [ ] Logged in as `dev` and changed the bootstrap password immediately
- [ ] `.env` is **not** committed; `git check-ignore .env` prints it
- [ ] A copy of `.env` (especially `MASTER_KEY`) is stored offline, encrypted
      (`gpg -c .env`). Losing `MASTER_KEY` permanently destroys encrypted pass payloads.
- [ ] Firewall: only local shell access; no inbound 80/443 needed with the tunnel.
      If the box is on a LAN, `HTTP_HOST_PORT` binding is only reachable locally/on LAN.
- [ ] Backups running (below); restore tested once.

## 4. Backups

```bash
deploy/backup.sh
```

Writes `deploy/backups/db-<stamp>.sql.gz` + `photos-<stamp>.tar.gz`, keeps 14 days.
Cron:

```cron
30 2 * * * /home/you/smart-visitors/deploy/backup.sh >> /home/you/smart-visitors/deploy/backups/backup.log 2>&1
```

Copy backups off the machine (rsync / rclone / Backblaze B2 free 10 GB). Volume
snapshots alone are not backups.

## 5. Updates

- **Home PC, local build:** `git pull && docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d --build`
- **Home PC, GHCR:** `deploy/update.sh` (cron every 15 min is fine)
- **VM:** automatic on push to `main` if `DEPLOY_ENABLED=true`; otherwise the pull/up
  commands above.

`docker image prune -f` occasionally; the ml image is the big one (~423 MB).

## 6. Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Tunnel 502 / 1033 | `cloudflared` not on the `public` network, or caddy unhealthy | `docker compose ... ps`; service must be `http://caddy:80` |
| 404 from Caddy only on the public hostname | `APP_DOMAIN` ≠ incoming Host | set `APP_DOMAIN` to the public hostname, restart caddy |
| Login works but redirect loop | Caddy is forcing HTTPS at the origin | keep `APP_SCHEME=http` with the tunnel |
| Cookie not set / not sent | origin and app are on different hostnames, or browser blocks `Secure` | use one hostname; tunnel gives HTTPS, so `Secure` is correct |
| Camera QR scanner does nothing | page opened over `http://` | open the `https://` tunnel URL |
| `up` rebuilds every time | local build mode | use `docker-compose.prod.yml` + `--no-build` |
| ml container OOM on 1 GB VM | sklearn sidecar | 2 GB RAM, or temporarily disable ml (also remove it from `api`'s `depends_on`) |

## 7. GitHub Pages sites

All static sites live under the `project-DawnGuard` org on one origin:

| Repo | URL | Role |
|---|---|---|
| `project-DawnGuard.github.io` | `https://project-dawnguard.github.io/` | admin console (landing page until P4) |
| `guard` | `https://project-dawnguard.github.io/guard/` | guard console |
| `scanner` | `https://project-dawnguard.github.io/scanner/` | host-side arrival verifier |
| `visitor` | `https://project-dawnguard.github.io/visitor/` | invite + pass pages |
| `admin` | `https://project-dawnguard.github.io/admin/` | temporary placeholder; retire after root cutover |
| `landing` (planned) | `https://project-dawnguard.github.io/landing/` | landing page after P4 |

There is no host dashboard: only admin sees visitor status. Hosts use the scanner app
to verify arrivals.

Provisioning:

```bash
# one-time, after the org exists in the GitHub web UI
deploy/github/bootstrap-org.sh

# push the landing page after editing deploy/github-pages/index.html
deploy/github/sync-landing.sh
```

`bootstrap-org.sh` is idempotent: it creates missing repos, enables Pages and skips
anything that already exists. The role repos currently serve placeholder pages; the
static apps replace them in phases P1–P4 of
[`docs/github-pages-split-plan.md`](../docs/github-pages-split-plan.md).

Notes:

- The org root uses the legacy branch build; role sites use GitHub Actions
  (`site/index.html` + `.github/workflows/pages.yml` in each repo).
- Every role path shares **one origin**, so CORS needs only
  `https://project-dawnguard.github.io`.
- The landing page CTA points at the current quick-tunnel URL. Quick tunnels change on
  every restart — update `__LIVE_URL__`/the URL and re-run `sync-landing.sh`, or move to
  a named tunnel in §1.2.
- This is showcase hosting only; the real app keeps running on the home PC.

Verify the API accepts the Pages origin and rejects others:

```bash
# allowed: should return 204 with Access-Control-Allow-Origin
curl -i -X OPTIONS https://YOUR-TUNNEL/api/auth/login \
  -H 'Origin: https://project-dawnguard.github.io' \
  -H 'Access-Control-Request-Method: POST'

# unknown: should return 403 with no Access-Control-Allow-Origin
curl -i -X OPTIONS https://YOUR-TUNNEL/api/auth/login \
  -H 'Origin: https://evil.example' \
  -H 'Access-Control-Request-Method: POST'
```

`TRUSTED_PROXY_CIDRS` must contain the Docker bridge subnet that Caddy uses, not the
whole private range. Check it with `docker network inspect smart-visitors_public`.
Wrong values make every request look like it came from the same IP, or let clients spoof
their rate-limit key.
