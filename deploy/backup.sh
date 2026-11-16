#!/usr/bin/env bash
# Back up Postgres + visitor photos. Keeps 14 days locally.
# Cron example (daily 02:30):
#   30 2 * * * /home/you/smart-visitors/deploy/backup.sh >> /home/you/smart-visitors/deploy/backups/backup.log 2>&1
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

STAMP="$(date +%F-%H%M)"
OUT_DIR="deploy/backups"
mkdir -p "$OUT_DIR"

echo "[backup] postgres -> $OUT_DIR/db-$STAMP.sql.gz"
docker compose exec -T db \
	pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --clean --if-exists \
	| gzip >"$OUT_DIR/db-$STAMP.sql.gz"

echo "[backup] photos -> $OUT_DIR/photos-$STAMP.tar.gz"
docker compose exec -T api tar czf - -C /data/photos . \
	>"$OUT_DIR/photos-$STAMP.tar.gz"

# MASTER_KEY cannot be recovered if lost. Back up .env separately, offline:
#   gpg -c --output smart-visitors-env.gpg .env
# Store that file somewhere safe (not in this repo).

find "$OUT_DIR" -maxdepth 1 -type f \( -name 'db-*.sql.gz' -o -name 'photos-*.tar.gz' \) \
	-mtime +14 -delete

echo "[backup] done"
