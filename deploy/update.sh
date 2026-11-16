#!/usr/bin/env bash
# Pull the latest prebuilt images (built by GitHub Actions) and restart.
# Intended for hosts reachable by cron instead of SSH deploy (e.g. a home PC).
# Cron example (every 15 min):
#   */15 * * * * /home/you/smart-visitors/deploy/update.sh >> /home/you/smart-visitors/deploy/backups/update.log 2>&1
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -f .env ]; then
	echo "error: .env not found in $(pwd)" >&2
	exit 1
fi

# shellcheck disable=SC1091
set -a
. ./.env
set +a

if [ -z "${IMAGE_PREFIX:-}" ]; then
	echo "error: IMAGE_PREFIX is not set in .env (example: ghcr.io/owner)" >&2
	exit 1
fi

OVERLAYS="-f docker-compose.yml -f docker-compose.prod.yml"
if [ -f docker-compose.tunnel.yml ] && grep -q '^CLOUDFLARE_TUNNEL_TOKEN=.' .env; then
	OVERLAYS="$OVERLAYS -f docker-compose.tunnel.yml"
fi

git pull --ff-only

# shellcheck disable=SC2086
docker compose $OVERLAYS pull
# shellcheck disable=SC2086
docker compose $OVERLAYS up -d --no-build --remove-orphans

docker image prune -f >/dev/null
echo "[update] stack is up"
