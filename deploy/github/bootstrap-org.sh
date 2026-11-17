#!/usr/bin/env bash
# Create the GitHub Pages repos under the project-DawnGuard org.
#
# Prerequisite: the organisation must already exist (GitHub has no API to create
# orgs). Create it once at https://github.com/account/organizations/new with the
# exact name project-DawnGuard, then run:
#
#   deploy/github/bootstrap-org.sh
#
# Idempotent: existing repos are skipped. Pages is enabled with the GitHub Actions
# build source; the first successful deploy comes from each repo's workflow.
set -euo pipefail

ORG="${ORG:-project-DawnGuard}"
HOST="$(echo "$ORG" | tr '[:upper:]' '[:lower:]').github.io"

command -v gh >/dev/null || { echo "error: gh CLI not installed" >&2; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "error: run 'gh auth login' first" >&2; exit 1; }

if ! gh api "orgs/$ORG" >/dev/null 2>&1; then
	echo "error: org '$ORG' not found (or no access)." >&2
	echo "Create it at https://github.com/account/organizations/new (Free plan)," >&2
	echo "then re-run this script." >&2
	exit 1
fi

create_repo() {
	local name="$1" desc="$2" homepage="$3"
	if gh repo view "$ORG/$name" >/dev/null 2>&1; then
		echo "[skip] $ORG/$name already exists"
	else
		echo "[create] $ORG/$name"
		gh repo create "$ORG/$name" --public --add-readme --description "$desc"
	fi

	gh repo edit "$ORG/$name" \
		--homepage "$homepage" \
		--add-topic smart-visitors \
		--add-topic visitor-management >/dev/null 2>&1 || true

	# Pages with Actions as the source. Safe to re-run.
	if gh api "repos/$ORG/$name/pages" >/dev/null 2>&1; then
		echo "[ok] Pages already enabled for $ORG/$name"
	else
		gh api -X POST "repos/$ORG/$name/pages" -f build_type=workflow >/dev/null 2>&1 ||
			echo "[info] could not enable Pages for $ORG/$name (enable it in Settings -> Pages)"
	fi
}

# Root Pages site: landing/showcase.
create_repo "$ORG.github.io" \
	"Smart Visitors — project landing page and showcase" \
	"https://$HOST/"

# One project site per role. URLs are https://$HOST/<repo>/
create_repo "admin" \
	"Smart Visitors — admin console (static SPA)" \
	"https://$HOST/admin/"

create_repo "guard" \
	"Smart Visitors — guard console (static SPA)" \
	"https://$HOST/guard/"

create_repo "scanner" \
	"Smart Visitors — gate QR scanner kiosk (static PWA)" \
	"https://$HOST/scanner/"

create_repo "visitor" \
	"Smart Visitors — visitor invite and pass pages (public)" \
	"https://$HOST/visitor/"

# Optional, uncomment when the host dashboard URL is decided.
# create_repo "host" \
# 	"Smart Visitors — host dashboard (static SPA)" \
# 	"https://$HOST/host/"

echo
echo "Repos are under https://github.com/$ORG"
echo "Public URLs (after each repo's first Pages deploy):"
echo "  https://$HOST/"
echo "  https://$HOST/admin/"
echo "  https://$HOST/guard/"
echo "  https://$HOST/scanner/"
echo "  https://$HOST/visitor/"
echo
echo "Next: set CORS_ALLOWED_ORIGINS on the API to https://$HOST"
echo "(project paths share one origin, so one entry covers all four apps)."
