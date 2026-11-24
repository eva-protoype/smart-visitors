# Home-server demo

This is the quick, free setup used for the current demo:

- one Linux home PC or laptop running Docker
- one command to start the stack
- one free Cloudflare quick tunnel
- public HTTPS URL, no domain, no Cloudflare account, no router changes
- four interfaces: admin, guard, scanner, and public visitor entry

The tunnel URL is temporary. It changes when the tunnel container is recreated.
The database, photos, passes, and audit logs stay in Docker volumes.

For a permanent named tunnel with a custom domain, use
[`deploy/README.md`](deploy/README.md). This guide is only for the demo.

---

## 1. Prerequisites

- Linux, macOS, or WSL with Docker Engine
- Docker Compose v2 (`docker compose version`)
- `curl`
- About 4 GB RAM; 8 GB is comfortable
- Internet access for the Cloudflare quick tunnel
- The repository cloned on the machine that will stay on

No Cloudflare account, domain, or port forwarding is required.

Quick tunnel is a public URL. Anyone who has the link can reach the login page
and the public visitor entry page. Do not put real visitor data on this demo.

---

## 2. First run

From the repository root:

```bash
chmod +x setup.sh
./setup.sh
```

`setup.sh` does this:

1. Creates a demo `.env` if one does not exist, with random secrets.
2. Starts the Docker stack.
3. Reuses the current quick tunnel if its URL still responds, otherwise
   starts a fresh one.
4. Writes the public URL into `APP_BASE_URL` and `NEXT_PUBLIC_SAAS_URL`.
5. Sets `APP_DOMAIN=localhost` and `APP_SCHEME=http` for the tunnel.
6. Waits for the API, web app, and Caddy to become healthy.
7. Checks the public URL and prints the four interface links.

The first run builds images and can take several minutes. Later runs reuse the
cached images unless the code changed.

If `.env` was just created, the script prints the first-boot admin password
once. It is also stored in `.env` as `BOOTSTRAP_ADMIN_PASSWORD`. Keep that file
private.

### If you already have a `.env`

Make sure these values are set:

```dotenv
POSTGRES_PASSWORD=...
DATABASE_URL=postgres://smart_visitors:...@db:5432/smart_visitors?sslmode=disable
JWT_SECRET=...
MASTER_KEY=...
ML_SERVICE_TOKEN=...
BOOTSTRAP_ADMIN_USER=dev
BOOTSTRAP_ADMIN_PASSWORD=...
```

`setup.sh` will overwrite the public URL values for the quick tunnel. It does
not delete data or change an existing admin password.

---

## 3. Open the four interfaces

`setup.sh` prints a URL like:

```text
https://random-name.trycloudflare.com
```

Use these paths:

| Interface | Path | Login |
|---|---|---|
| Admin dashboard | `/admin` | admin account |
| Guard console | `/guard` | guard account |
| Scanner | `/scanner` | guard account |
| Visitor entry | `/visitor` | public, no account |
| Login page | `/login` |  |
| Host desk (optional) | `/host` | host account |

`/guard` and `/scanner` open the same guard workspace. The scanner is embedded
in the guard console.

The visitor does not get an account. A visitor record is created and a
time-limited QR pass is issued.

### Accounts

On a fresh database, the first admin is created from:

- `BOOTSTRAP_ADMIN_USER` (default `dev`)
- `BOOTSTRAP_ADMIN_PASSWORD`

The bootstrap admin is forced to change the password on first login. The
bootstrap password only applies when the database has no admin yet. Changing
`BOOTSTRAP_ADMIN_PASSWORD` later does **not** change an existing admin.

To create a guard:

1. Log in as admin.
2. Open **Admin** and go to **Users**.
3. Create a user with the **guard** role.
4. Copy the temporary password shown once.
5. Log in as that guard and change the password.

The current demo machine already has its admin and guard accounts. Use the
passwords set during that setup; they are not stored in Git.

---

## 4. Camera scanning

The scanner uses the browser camera plus a bundled `jsQR` decoder. It does not
depend on the browser's `BarcodeDetector` API.

To scan:

1. Open the HTTPS tunnel URL, not a local IP address.
2. Log in as a guard.
3. Open `/scanner` or `/guard`.
4. Click **Open camera** and allow camera permission.
5. Point the camera at the visitor QR pass.

The token can also be pasted manually if a camera is unavailable.

Camera access requires HTTPS or `localhost`. The Cloudflare quick tunnel
provides HTTPS.

---

## 5. The quick tunnel URL

The free quick tunnel URL is random and temporary. It changes when:

- the `sv-demo-tunnel` container is recreated
- the machine reboots and Docker removes the container
- Cloudflare retires the free quick tunnel (this happened after a few hours)
- you run `./setup.sh --new-tunnel`

`./setup.sh` checks the current URL first. If it still responds, the script
reuses it. If it no longer resolves, the script starts a new tunnel, updates
`.env`, and prints the new URL. Reload the app after it finishes. Anyone using
the old link will need the new one.

To force a new URL even when the current one still works:

```bash
./setup.sh --new-tunnel
```

If the GitHub Pages entry links are configured, update them too:

```bash
TUNNEL_URL="$(docker logs sv-demo-tunnel 2>&1 | grep -Eo 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -n 1)"
HOME_SERVER_URL="$TUNNEL_URL" ./deploy/github/sync-org-pages.sh
```

That optional step needs the GitHub CLI (`gh`) authenticated for the
`project-DawnGuard` organization.

For a stable URL, use a named Cloudflare Tunnel instead:
[`deploy/README.md`](deploy/README.md).

---

## 6. Start, stop, logs, and updates

```bash
./setup.sh                 # start or restart, rebuild changed images
./setup.sh --no-build      # start without rebuilding
./setup.sh --new-tunnel    # recreate the quick tunnel, new URL
./setup.sh --stop          # stop stack and tunnel; keep data
```

Logs:

```bash
docker logs -f sv-demo-tunnel
docker compose logs -f api web caddy
docker compose ps
```

After pulling code changes:

```bash
git pull --ff-only
./setup.sh
```

`--no-build` is only for a quick restart; it does not pick up source changes.

---

## 7. Data, backups, and reset

Normal stop and start keeps data:

```bash
./setup.sh --stop
./setup.sh
```

Back up the database and photos:

```bash
./deploy/backup.sh
```

Backups go to `deploy/backups/` and are ignored by Git. Also keep an encrypted
copy of `.env`; `MASTER_KEY` is required to recover encrypted pass data.

### Hard reset (destructive)

This erases the database, photos, passes, audit logs, and generated keys:

```bash
./deploy/backup.sh
docker compose down -v
docker rm -f sv-demo-tunnel
./setup.sh
```

Do not run `docker compose down -v` unless you intend to lose the demo data.
After a hard reset, the admin is bootstrapped again from `.env`.

Changing `BOOTSTRAP_ADMIN_PASSWORD` alone does nothing once an admin exists.
Use the admin password-change screen, or reset the database intentionally.

---

## 8. Troubleshooting

| Symptom | Check |
|---|---|
| `permission denied: ./setup.sh` | `chmod +x setup.sh` |
| Tunnel shows 502 | Wait for Caddy/web; run `docker logs -f sv-demo-tunnel` and `docker compose ps` |
| 404 only on the public URL | `APP_DOMAIN` must be `localhost` for the quick tunnel; `setup.sh` sets it |
| Login redirect loop | Keep `APP_SCHEME=http` behind the tunnel; do not make Caddy redirect to HTTPS |
| Camera says unsupported | Open the HTTPS URL; allow camera permission; hard refresh with `Ctrl+Shift+R` |
| Camera opens but does not scan | Improve lighting, hold the QR steady, or paste the token manually |
| Login returns 429 | Demo rate limit is 5 attempts per hour; wait or restart `api` in the demo |
| Port 4000 is busy | Change `HTTP_HOST_PORT` in `.env`; the tunnel still reaches Caddy inside Docker |
| Build fails | Check free disk space and RAM; run `docker compose build` to see the full error |
| URL changed | Run `./setup.sh` to reuse the current tunnel, or `./setup.sh --new-tunnel` and share the new link |

---

## 9. What this setup does not cover

- Custom domains or permanent named tunnels: use `deploy/README.md`
- Windows-native commands: use `deploy/WINDOWS.md`
- Production hardening, TLS certificates, or Cloudflare Access policies
- Automatic account provisioning beyond the first admin
- Real visitor data or Aadhaar/biometric data

This is a demo setup, not a production deployment.
