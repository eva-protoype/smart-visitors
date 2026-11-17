#!/usr/bin/env bash
set -euo pipefail

ORG="${ORG:-project-DawnGuard}"
HOME_SERVER_URL="${HOME_SERVER_URL:-}"

if [ -z "$HOME_SERVER_URL" ]; then
	echo "error: set HOME_SERVER_URL to the public HTTPS home-server URL" >&2
	exit 1
fi

case "$HOME_SERVER_URL" in
	https://*) ;;
	*) echo "error: HOME_SERVER_URL must use HTTPS" >&2; exit 1 ;;
esac

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

write_launcher() {
	local role="$1"
	local title="$2"
	local target="$3"
	local repo_dir="$TMP/$role"

	gh repo clone "$ORG/$role" "$repo_dir" -- --depth 1 >/dev/null
	mkdir -p "$repo_dir/site"
	cat >"$repo_dir/site/index.html" <<EOF
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="refresh" content="0;url=$HOME_SERVER_URL/$target">
  <title>Smart Visitors — $title</title>
</head>
<body>
  <p>Opening the $title…</p>
  <p><a href="$HOME_SERVER_URL/$target">Continue to Smart Visitors</a></p>
</body>
</html>
EOF
	(
		cd "$repo_dir"
		git add site/index.html
		if git diff --cached --quiet; then
			return
		fi
		git config user.name "Smart Visitors"
		git config user.email "anany.mishra.real@gmail.com"
		git commit -m "chore: connect Pages entry to home server"
		git push origin main
	)
}

write_launcher admin "admin dashboard" admin
write_launcher guard "guard console" guard
write_launcher scanner "gate scanner" scanner
write_launcher visitor "visitor entry" visitor

landing_dir="$TMP/landing"
gh repo clone "$ORG/$ORG.github.io" "$landing_dir" -- --depth 1 >/dev/null
cat >"$landing_dir/index.html" <<EOF
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Smart Visitors</title>
</head>
<body>
  <h1>Smart Visitors</h1>
  <p>Choose an interface:</p>
  <ul>
    <li><a href="https://$ORG.github.io/admin/">Admin</a></li>
    <li><a href="https://$ORG.github.io/guard/">Guard</a></li>
    <li><a href="https://$ORG.github.io/scanner/">Scanner</a></li>
    <li><a href="https://$ORG.github.io/visitor/">Visitor</a></li>
  </ul>
</body>
</html>
EOF
(
	cd "$landing_dir"
	git add index.html
	if ! git diff --cached --quiet; then
		git config user.name "Smart Visitors"
		git config user.email "anany.mishra.real@gmail.com"
		git commit -m "chore: refresh Smart Visitors entry links"
		git push origin main
	fi
)

echo "[sync] organization Pages sites updated for $HOME_SERVER_URL"
