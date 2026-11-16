# GitHub Pages showcase

`index.html` is a static single-file landing page. It does **not** run the app —
the live system stays on the home machine behind the Cloudflare Tunnel. GitHub Pages
only hosts static files, so this page is the showcase + link.

## Before publishing

1. Start the tunnel and copy the public URL (named tunnel or `deploy/quick-tunnel.sh`).
2. Replace `__LIVE_URL__` in `index.html` with that URL (three occurrences, or use
   `sed -i 's|__LIVE_URL__|https://visitors.example.com|g' index.html`).
3. Update the repo links if the project moves.

## Option A — project Pages (this repo)

1. Repo → Settings → Pages → Source: **GitHub Actions**.
2. Push `main`; `.github/workflows/pages.yml` uploads `deploy/github-pages/`.
3. URL: `https://<owner>.github.io/smart-visitors/`.

## Option B — exact `smart-visitors.github.io`

That hostname is reserved for a GitHub **account or organisation named
`smart-visitors`**. If you control it, create a repo named
`smart-visitors.github.io` and publish the contents of this folder as the repo root:

```bash
cd deploy/github-pages
git init -b main
git add .
git commit -m "Smart Visitors landing page"
git remote add origin git@github.com:smart-visitors/smart-visitors.github.io.git
git push -u origin main
```

Then Settings → Pages → Deploy from branch → `main` / `/ (root)`.

If the account is not named `smart-visitors`, the address will be
`https://<your-account>.github.io/smart-visitors/` instead. No workaround can claim
another account's `github.io` hostname.
