#!/usr/bin/env bash
# Sync deploy/github-pages/index.html into the org landing repo and push.
# The root repo is published at https://project-dawnguard.github.io/
#
# Usage: deploy/github/sync-landing.sh
set -euo pipefail

cd "$(dirname "$0")/../.."
ORG="${ORG:-project-DawnGuard}"
HOST="$(echo "$ORG" | tr '[:upper:]' '[:lower:]').github.io"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

gh repo clone "$ORG/$ORG.github.io" "$TMP/repo" -- --depth 1 >/dev/null

cp deploy/github-pages/index.html "$TMP/repo/index.html"
touch "$TMP/repo/.nojekyll"

cd "$TMP/repo"
git config user.name >/dev/null 2>&1 || git config user.name "Smart Visitors Bot"
git config user.email >/dev/null 2>&1 || git config user.email "bot@users.noreply.github.com"

git add index.html .nojekyll
if git diff --cached --quiet; then
	echo "[sync] landing page already up to date"
	exit 0
fi

git commit -q -m "chore: sync landing page"
git push -q origin main
echo "[sync] published https://$HOST/"
