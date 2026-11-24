#!/usr/bin/env bash
# Start the Smart Visitors home-server demo with a free Cloudflare quick tunnel.
# No domain or Cloudflare account is required.
#
# Usage:
#   ./setup.sh
#   ./setup.sh --new-tunnel
#   ./setup.sh --no-build
#   ./setup.sh --stop
#
# Data is preserved by default. See DEMO.md before deleting volumes.
set -euo pipefail

cd "$(dirname "$0")"

TUNNEL_NAME="${TUNNEL_NAME:-sv-demo-tunnel}"
NEW_TUNNEL=0
NO_BUILD=0
STOP=0

usage() {
	cat <<'USAGE'
Usage: ./setup.sh [options]

Starts the demo stack and a free Cloudflare quick tunnel, then prints the
public entry links.

Options:
  --new-tunnel   Replace the existing quick tunnel (the URL will change)
  --no-build     Reuse existing images instead of rebuilding
  --stop         Stop the tunnel and stack; keep database and photos
  -h, --help     Show this help

This script never deletes Docker volumes. See DEMO.md for a hard reset.
USAGE
}

while [ "$#" -gt 0 ]; do
	case "$1" in
		--new-tunnel) NEW_TUNNEL=1 ;;
		--no-build) NO_BUILD=1 ;;
		--stop) STOP=1 ;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			echo "error: unknown option: $1" >&2
			usage >&2
			exit 2
			;;
	esac
	shift
done

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "error: $1 is required." >&2
		exit 1
	fi
}

need_cmd docker
if ! docker compose version >/dev/null 2>&1; then
	echo "error: Docker Compose v2 is required." >&2
	exit 1
fi

if [ "$STOP" = 1 ]; then
	docker rm -f "$TUNNEL_NAME" >/dev/null 2>&1 || true
	if [ -f .env ]; then
		docker compose down
	else
		echo "[stop] no .env found; stack was not started from this directory"
	fi
	echo "[stop] stack stopped; Docker volumes kept."
	exit 0
fi

need_cmd curl

if [ ! -f .env ]; then
	need_cmd openssl
	echo "[env] .env not found; creating a demo .env from .env.example"
	cp .env.example .env
	chmod 600 .env

	postgres_password="$(openssl rand -hex 16)"
	jwt_secret="$(openssl rand -hex 32)"
	master_key="$(openssl rand -hex 32)"
	ml_token="$(openssl rand -hex 24)"
	admin_password="$(openssl rand -hex 16)"

	sed -i "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${postgres_password}|" .env
	sed -i "s|^DATABASE_URL=.*|DATABASE_URL=postgres://smart_visitors:${postgres_password}@db:5432/smart_visitors?sslmode=disable|" .env
	sed -i "s|^JWT_SECRET=.*|JWT_SECRET=${jwt_secret}|" .env
	sed -i "s|^MASTER_KEY=.*|MASTER_KEY=${master_key}|" .env
	sed -i "s|^ML_SERVICE_TOKEN=.*|ML_SERVICE_TOKEN=${ml_token}|" .env
	sed -i "s|^BOOTSTRAP_ADMIN_PASSWORD=.*|BOOTSTRAP_ADMIN_PASSWORD=${admin_password}|" .env

	echo "[env] first-boot admin: dev / ${admin_password}"
	echo "[env] credentials saved in .env (mode 600)"
fi

for key in POSTGRES_PASSWORD DATABASE_URL JWT_SECRET MASTER_KEY ML_SERVICE_TOKEN BOOTSTRAP_ADMIN_PASSWORD; do
	value="$(grep -E "^${key}=" .env | head -n 1 | cut -d= -f2- || true)"
	case "$value" in
		"" | CHANGE_ME*)
			echo "error: ${key} is missing or still has a CHANGE_ME placeholder in .env" >&2
			exit 1
			;;
	esac
done

set -a
# shellcheck disable=SC1091
. ./.env
set +a

if ! docker compose config -q; then
	echo "error: docker compose could not read .env" >&2
	exit 1
fi

PROJECT="${COMPOSE_PROJECT_NAME:-$(basename "$PWD")}"
NETWORK="${PROJECT}_public"
TUNNEL_URL=""

public_http_code() {
	curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$1" || true
}

if docker inspect "$TUNNEL_NAME" >/dev/null 2>&1; then
	running="$(docker inspect -f '{{.State.Running}}' "$TUNNEL_NAME" 2>/dev/null || true)"
	existing_url="$(docker logs "$TUNNEL_NAME" 2>&1 | grep -Eo 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -n 1 || true)"
	if [ "$NEW_TUNNEL" = 0 ] && [ "$running" = "true" ] && [ -n "$existing_url" ]; then
		existing_code="$(public_http_code "${existing_url}/login")"
		case "$existing_code" in
			"" | 000)
				echo "[tunnel] existing URL is no longer reachable; replacing"
				docker rm -f "$TUNNEL_NAME" >/dev/null 2>&1 || true
				;;
			*)
				TUNNEL_URL="$existing_url"
				echo "[tunnel] reusing ${TUNNEL_URL} (HTTP ${existing_code})"
				;;
		esac
	else
		echo "[tunnel] replacing existing tunnel container"
		docker rm -f "$TUNNEL_NAME" >/dev/null 2>&1 || true
	fi
fi

if ! docker network inspect "$NETWORK" >/dev/null 2>&1; then
	echo "[stack] starting db to create the Docker network"
	docker compose up -d db
fi

if [ -z "$TUNNEL_URL" ]; then
	echo "[tunnel] starting Cloudflare quick tunnel"
	docker run -d \
		--name "$TUNNEL_NAME" \
		--restart unless-stopped \
		--network "$NETWORK" \
		cloudflare/cloudflared:latest \
		tunnel --no-autoupdate --url http://caddy:80 --http-host-header localhost >/dev/null

	for _ in $(seq 1 60); do
		TUNNEL_URL="$(docker logs "$TUNNEL_NAME" 2>&1 | grep -Eo 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -n 1 || true)"
		if [ -n "$TUNNEL_URL" ]; then
			break
		fi
		sleep 2
	done

	if [ -z "$TUNNEL_URL" ]; then
		echo "error: the quick tunnel did not print a public URL" >&2
		docker logs "$TUNNEL_NAME" 2>&1 | tail -n 60 >&2 || true
		exit 1
	fi
fi

set_env() {
	local key="$1"
	local value="$2"
	if grep -q "^${key}=" .env; then
		sed -i "s|^${key}=.*|${key}=${value}|" .env
	else
		printf '%s=%s\n' "$key" "$value" >>.env
	fi
}

# Cloudflared sends "Host: localhost", so Caddy must accept that host.
set_env APP_DOMAIN localhost
set_env APP_SCHEME http
set_env APP_BASE_URL "$TUNNEL_URL"
set_env NEXT_PUBLIC_SAAS_URL "$TUNNEL_URL"

echo "[stack] starting services"
if [ "$NO_BUILD" = 1 ]; then
	docker compose up -d
else
	echo "[stack] building images first; this can take several minutes"
	docker compose up -d --build
fi

wait_for_service() {
	local service="$1"
	local container
	container="$(docker compose ps -q "$service" 2>/dev/null || true)"
	if [ -z "$container" ]; then
		echo "error: ${service} is not running" >&2
		return 1
	fi

	local status=""
	for _ in $(seq 1 90); do
		status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)"
		case "$status" in
			healthy | running) return 0 ;;
		esac
		sleep 2
	done

	echo "error: ${service} did not become healthy (last status: ${status:-unknown})" >&2
	docker compose logs --tail=60 "$service" >&2 || true
	return 1
}

wait_for_service api
wait_for_service web
wait_for_service caddy

echo "[check] verifying the public URL"
public_code=""
for _ in $(seq 1 45); do
	public_code="$(public_http_code "${TUNNEL_URL}/login")"
	case "$public_code" in
		2* | 3* | 4*) break ;;
	esac
	sleep 2
done

case "$public_code" in
	"" | 000)
		echo "error: ${TUNNEL_URL} did not resolve from this machine" >&2
		docker logs "$TUNNEL_NAME" 2>&1 | tail -n 60 >&2 || true
		exit 1
		;;
	5*)
		echo "warning: ${TUNNEL_URL}/login returned HTTP ${public_code}" >&2
		echo "warning: check 'docker logs ${TUNNEL_NAME}' and 'docker compose logs caddy'" >&2
		;;
esac

echo
echo "Smart Visitors demo is up"
echo "Public:  ${TUNNEL_URL}"
echo "Login:   ${TUNNEL_URL}/login"
echo "Admin:   ${TUNNEL_URL}/admin"
echo "Guard:   ${TUNNEL_URL}/guard"
echo "Scanner: ${TUNNEL_URL}/scanner"
echo "Visitor: ${TUNNEL_URL}/visitor"
echo
echo "Tunnel container: ${TUNNEL_NAME}"
echo "Tunnel logs:      docker logs -f ${TUNNEL_NAME}"
echo "Stop without deleting data: ./setup.sh --stop"
