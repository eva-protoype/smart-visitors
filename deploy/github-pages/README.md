# GitHub Pages entry points

These files are templates for the Pages repositories in the
`project-DawnGuard` organization. They do not contain the Go API, database, or
authenticated dashboard. Each site redirects to the matching interface on the
home server:

- `https://project-dawnguard.github.io/admin/`
- `https://project-dawnguard.github.io/guard/`
- `https://project-dawnguard.github.io/scanner/`
- `https://project-dawnguard.github.io/visitor/`

Run `deploy/github/sync-org-pages.sh` with `HOME_SERVER_URL` set to the public
HTTPS URL of the home server to update the organization sites.
