#!/usr/bin/env bash
# Throwaway public demo URL via a Cloudflare quick tunnel.
# No domain or account needed. URL is random and dies when this process stops.
#
# Usage: deploy/quick-tunnel.sh
# Stop: Ctrl+C
set -euo pipefail

cd "$(dirname "$0")/.."

PROJECT="${COMPOSE_PROJECT_NAME:-$(basename "$PWD")}"
NETWORK="${PROJECT}_public"

if ! docker network inspect "$NETWORK" >/dev/null 2>&1; then
	echo "error: docker network $NETWORK not found — start the stack first" >&2
	exit 1
fi

echo "[tunnel] public URL appears below (https://<random>.trycloudflare.com)"
exec docker run --rm --network "$NETWORK" cloudflare/cloudflared:latest \
	tunnel --no-autoupdate --url http://caddy:80 --http-host-header localhost
