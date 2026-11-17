# GitHub Pages entry points

The Pages site is a static launcher. It does not contain the Go API, database,
or authenticated dashboard. Each path redirects to the matching interface on
the home server:

- `/admin/`
- `/host/`
- `/guard/`
- `/scanner/`
- `/visitor/`

Set the repository variable `HOME_SERVER_URL` to the public HTTPS URL of the
home-server Caddy endpoint before enabling the Pages workflow.
