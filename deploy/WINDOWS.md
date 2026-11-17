# Windows deployment guide

This guide deploys Smart Visitors on a Windows 10/11 home server using Docker
Desktop and Cloudflare Tunnel. The application, database, photos, and audit
records stay on the Windows machine. Cloudflare only provides the public HTTPS
connection.

## What you need

- Windows 10/11 64-bit with virtualization enabled
- 8 GB RAM recommended (4 GB minimum)
- Docker Desktop with the WSL 2 backend
- Git for Windows
- A Cloudflare account only if you want a stable named tunnel
- A domain managed by Cloudflare for a stable hostname, for example
  `visitors.example.com`

The server does not need inbound router ports or a public IP.

## Can Docker be skipped?

Yes, but there is currently no one-command native Windows installer. Docker
packages the five processes and their networking for this project:

- Go API
- PostgreSQL
- Python ML sidecar
- Next.js web app
- Caddy reverse proxy

The practical alternatives are:

1. **WSL 2 without Docker:** install Ubuntu in WSL and run PostgreSQL, Go,
   Python, Node.js, and Caddy there. This is the closest replacement and
   supports the existing Bash scripts, but it still requires Linux tooling.
2. **Fully native Windows:** install PostgreSQL, Go, Python, Node.js, Caddy,
   and `cloudflared` as separate Windows services/processes. This requires
   manual configuration and changing container-only values in `.env`:

```dotenv
DATABASE_URL=postgres://smart_visitors:PASSWORD@127.0.0.1:5432/smart_visitors?sslmode=disable
ML_URL=http://127.0.0.1:8100
API_URL=http://127.0.0.1:8000
PHOTOS_DIR=C:/SmartVisitors/data/photos
AI_ARTIFACT_DIR=C:/SmartVisitors/data/artifacts
```

The ML service must use `API_URL=http://127.0.0.1:8000` and
`ARTIFACTS_DIR=C:/SmartVisitors/data/artifacts`. The Next.js app must be
started with its API URL pointing at the local Go API. Caddy then proxies
`/api` to `127.0.0.1:8000` and the frontend to `127.0.0.1:3000`.

For a reliable home-server deployment, Docker Desktop is recommended. If
Docker licensing, RAM, or policy is the reason you want to avoid it, use WSL 2
and run the Linux services there rather than attempting a fully native setup.
Cloudflare Tunnel works with either approach; the account requirements do not
change.

## 1. Install Windows prerequisites

Open **PowerShell as Administrator** and install Docker Desktop and Git:

```powershell
winget install -e --id Docker.DockerDesktop
winget install -e --id Git.Git
```

Restart Windows if Docker Desktop requests it. Open Docker Desktop and wait
until it says **Docker Desktop is running**. In a new PowerShell window verify:

```powershell
docker version
docker compose version
git --version
```

In Docker Desktop, make sure **Settings -> General -> Use the WSL 2 based
engine** is enabled.

## 2. Download the project

Choose a permanent folder that Docker Desktop can access:

```powershell
New-Item -ItemType Directory -Force C:\SmartVisitors | Out-Null
Set-Location C:\SmartVisitors
git clone https://github.com/eva-protoype/smart-visitors.git
Set-Location .\smart-visitors
```

If GitHub asks for credentials, use a GitHub token or SSH configuration. Do
not put a GitHub token in `.env`.

## 3. Create the unified `.env`

Copy the example file:

```powershell
Copy-Item .env.example .env
notepad .env
```

At minimum, replace every `CHANGE_ME` value. For a local-only first test, keep:

```dotenv
APP_BASE_URL=http://localhost:4000
APP_DOMAIN=localhost
APP_SCHEME=http
NEXT_PUBLIC_SAAS_URL=http://localhost:4000
HTTP_HOST_PORT=4000
CLOUDFLARE_TUNNEL_TOKEN=
```

Generate random values in PowerShell:

```powershell
function New-RandomHex([int]$Bytes = 32) {
  $buffer = New-Object byte[] $Bytes
  [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($buffer)
  ($buffer | ForEach-Object { $_.ToString("x2") }) -join ""
}

New-RandomHex
New-RandomHex
```

Use the two outputs for `JWT_SECRET` and `MASTER_KEY`. Also set long random
values for `POSTGRES_PASSWORD`, `ML_SERVICE_TOKEN`, and
`BOOTSTRAP_ADMIN_PASSWORD`.

Keep `.env` private. It contains the database password, encryption key, and
bootstrap credentials. It is ignored by Git and must never be uploaded to
GitHub.

## 4. Start the server locally

From `C:\SmartVisitors\smart-visitors`:

```powershell
docker compose up -d --build
docker compose ps
```

Wait until `api`, `db`, `ml`, `web`, and `caddy` are healthy or running. Open:

- `http://localhost:4000/login`
- `http://localhost:4000/visitor`
- `http://localhost:4000/scanner`

The first admin login uses `BOOTSTRAP_ADMIN_USER` and
`BOOTSTRAP_ADMIN_PASSWORD` from `.env`. Change the password after signing in.

To see logs:

```powershell
docker compose logs -f api web caddy
```

To stop without deleting data:

```powershell
docker compose down
```

Docker volumes contain the database and uploaded files. Do not use
`docker compose down -v` unless you intentionally want to erase them.

## 5. Public access: choose one Cloudflare option

### Option A: Quick Tunnel (no Cloudflare account and no card)

This is the fastest option for testing. It creates a temporary random
`trycloudflare.com` hostname and changes when the process stops.

Install `cloudflared`:

```powershell
winget install -e --id Cloudflare.cloudflared
```

Start the tunnel in a second PowerShell window:

```powershell
cloudflared tunnel --no-autoupdate --url http://localhost:4000
```

Copy the displayed `https://....trycloudflare.com` URL and update `.env`:

```dotenv
APP_BASE_URL=https://YOUR-RANDOM-NAME.trycloudflare.com
APP_DOMAIN=YOUR-RANDOM-NAME.trycloudflare.com
APP_SCHEME=http
NEXT_PUBLIC_SAAS_URL=https://YOUR-RANDOM-NAME.trycloudflare.com
CLOUDFLARE_TUNNEL_TOKEN=
```

Restart the web container so the public URL is included in generated links:

```powershell
docker compose up -d --build
```

Leave the `cloudflared` PowerShell window open. Closing it ends the public
connection. This option is suitable for demos, not permanent organization
Pages redirects.

### Option B: Named Tunnel (stable URL)

Use this for a real deployment. A named tunnel requires a Cloudflare account
and normally a domain added to that account.

1. Sign in to the Cloudflare dashboard.
2. Open **Zero Trust -> Networks -> Tunnels -> Create a tunnel**.
3. Choose **Cloudflared**, name it `smart-visitors`, and copy the tunnel token.
4. Add a public hostname such as `visitors.example.com`.
5. Set the service to `http://caddy:80` when `cloudflared` runs in Docker.
6. Put the token and hostname in `.env`:

```dotenv
APP_DOMAIN=visitors.example.com
APP_SCHEME=http
APP_BASE_URL=https://visitors.example.com
NEXT_PUBLIC_SAAS_URL=https://visitors.example.com
CLOUDFLARE_TUNNEL_TOKEN=PASTE_THE_TUNNEL_TOKEN_HERE
```

Start the stack with the tunnel overlay:

```powershell
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d --build
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml ps
```

The tunnel container is configured to restart automatically with Docker
Desktop. Test it:

```powershell
curl.exe -I https://visitors.example.com/login
```

Do not expose database or API ports directly to the internet. Cloudflare
terminates HTTPS; the Docker network uses HTTP internally, so
`APP_SCHEME=http` is intentional.

## 6. Cloudflare account question

You do **not** need the same Cloudflare account for a Quick Tunnel because
Quick Tunnels do not require an account or a domain.

For a **named tunnel**, use the Cloudflare account that owns the domain and
hostname. It may be the same account you used previously, but it does not
have to be the same email account as GitHub or the `project-DawnGuard`
organization. The tunnel token belongs to that Cloudflare account and should
only be stored in the private `.env`.

GitHub Pages and Cloudflare are separate services:

- GitHub Pages provides the organization entry URLs.
- Cloudflare points those URLs to the Windows home server.
- Docker and the database remain on the Windows machine.

## 7. Organization interface URLs

After a stable hostname is working, update the organization Pages redirects
with the stable server URL. From Git Bash or WSL:

```bash
HOME_SERVER_URL=https://visitors.example.com \
  ./deploy/github/sync-org-pages.sh
```

The resulting entry URLs are:

- `https://project-dawnguard.github.io/admin/`
- `https://project-dawnguard.github.io/guard/`
- `https://project-dawnguard.github.io/scanner/`
- `https://project-dawnguard.github.io/visitor/`

Do not point these Pages repositories at a temporary `trycloudflare.com` URL
unless temporary links are acceptable.

## 8. Updates and backups

Pull updates and rebuild:

```powershell
git pull --ff-only
docker compose -f docker-compose.yml -f docker-compose.tunnel.yml up -d --build --remove-orphans
```

Back up the database and uploaded files from Git Bash or WSL:

```bash
./deploy/backup.sh
```

Store the backup and a secure copy of `.env` outside the Windows machine.
Without `MASTER_KEY`, encrypted visitor data cannot be recovered.

## 9. Windows troubleshooting

| Problem | Fix |
|---|---|
| Docker commands fail | Start Docker Desktop and wait for its engine to become ready. |
| Camera is missing | Use the HTTPS Cloudflare URL, not `http://192.168...`; allow camera permission. |
| Tunnel returns 502 | Check `docker compose ps` and use `http://caddy:80` for the Docker tunnel service. |
| Login redirects repeatedly | Keep `APP_SCHEME=http` behind Cloudflare and use one public hostname everywhere. |
| Generated QR links use localhost | Update `APP_BASE_URL` and `NEXT_PUBLIC_SAAS_URL`, then rebuild `web`. |
| Port 4000 is busy | Change `HTTP_HOST_PORT` and use the same port in the Quick Tunnel command. |
| Data disappeared | Do not run `docker compose down -v`; inspect Docker volumes before cleanup. |

The Bash launcher `deploy/home-server.sh` is intended for Linux, WSL, or Git
Bash. On native PowerShell, use the Docker Compose commands in this guide.
