# supastarter for Next.js

supastarter is the ultimate starter kit for production-ready, scalable SaaS applications.

## Smart Visitors

The SaaS app is customized as the Smart Visitors dashboard. Run its commands from this
directory; they load environment variables from the repository-level `.env` file.
Configure `BACKEND_URL`, `DATABASE_URL`, `BETTER_AUTH_SECRET`, and
`NEXT_PUBLIC_SAAS_URL` there. For local development, start the template database with
`docker compose -f docker-compose.yml up -d postgres` from this directory, then run
`pnpm --filter @repo/database generate`, `pnpm --filter @repo/database push`, and
`pnpm dev`. Start the Smart Visitors API with `docker compose up -d backend` from the
repository root. Do not create an app-level `.env.local`.

## Helpful links

- [📘 Documentation](https://supastarter.dev/docs/nextjs)
- [🚀 Demo](https://demo.supastarter.dev)
