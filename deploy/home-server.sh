#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v docker >/dev/null 2>&1; then
	echo "error: Docker is required." >&2
	exit 1
fi

if [ ! -f .env ]; then
	echo "error: copy .env.example to .env and configure it first." >&2
	exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
	echo "error: Docker Compose v2 is required." >&2
	exit 1
fi

if [ "${PULL_LATEST:-0}" = "1" ]; then
	git pull --ff-only
fi

set -a
# shellcheck disable=SC1091
. ./.env
set +a

COMPOSE_FILES=(-f docker-compose.yml)
if [ -n "${CLOUDFLARE_TUNNEL_TOKEN:-}" ]; then
	COMPOSE_FILES+=(-f docker-compose.tunnel.yml)
fi

docker compose "${COMPOSE_FILES[@]}" up -d --build --remove-orphans
docker compose "${COMPOSE_FILES[@]}" ps

echo
echo "Smart Visitors is live at ${APP_BASE_URL:-http://localhost:4000}"
echo "Admin:   ${APP_BASE_URL:-http://localhost:4000}/admin"
echo "Host:    ${APP_BASE_URL:-http://localhost:4000}/host"
echo "Guard:   ${APP_BASE_URL:-http://localhost:4000}/guard"
echo "Scanner: ${APP_BASE_URL:-http://localhost:4000}/scanner"
echo "Visitor: ${APP_BASE_URL:-http://localhost:4000}/visitor"
