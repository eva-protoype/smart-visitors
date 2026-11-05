FROM node:22-bookworm AS build
WORKDIR /workspace
RUN corepack enable
COPY . .
RUN pnpm install --frozen-lockfile
RUN pnpm --filter ./apps/saas... build

FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production \
    PORT=3000 \
    HOSTNAME=0.0.0.0
COPY --from=build /workspace/apps/saas/.next/standalone ./
COPY --from=build /workspace/apps/saas/.next/static ./apps/saas/.next/static
EXPOSE 3000
CMD ["sh", "-c", "if [ -f apps/saas/server.js ]; then node apps/saas/server.js; else node server.js; fi"]
