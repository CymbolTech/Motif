FROM node:22-alpine AS dependencies

WORKDIR /workspace
RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml nx.json ./
COPY apps/dashboard/package.json apps/dashboard/project.json apps/dashboard/
COPY apps/marketing/package.json apps/marketing/project.json apps/marketing/
RUN pnpm install --frozen-lockfile

FROM dependencies AS build

COPY apps/marketing apps/marketing
ARG NEXT_PUBLIC_DASHBOARD_URL=http://127.0.0.1:4173
ENV NEXT_PUBLIC_DASHBOARD_URL=${NEXT_PUBLIC_DASHBOARD_URL}
# Canonical public origin for metadata, the sitemap and robots.txt.
ARG NEXT_PUBLIC_SITE_URL=
ENV NEXT_PUBLIC_SITE_URL=${NEXT_PUBLIC_SITE_URL}
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm --dir apps/marketing build

FROM node:22-alpine AS runtime

WORKDIR /app
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
ENV PORT=3000
ENV NEXT_TELEMETRY_DISABLED=1
# The standalone output mirrors the workspace layout because tracing starts at the repository root.
COPY --from=build /workspace/apps/marketing/.next/standalone ./
COPY --from=build /workspace/apps/marketing/.next/static ./apps/marketing/.next/static

RUN addgroup -S app && adduser -S -G app app \
  && chown -R app:app /app
USER app

EXPOSE 3000
CMD ["node", "apps/marketing/server.js"]
