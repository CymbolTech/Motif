FROM node:22-alpine AS dependencies

WORKDIR /workspace
RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml nx.json ./
COPY apps/dashboard/package.json apps/dashboard/project.json apps/dashboard/
COPY apps/marketing/package.json apps/marketing/project.json apps/marketing/
RUN pnpm install --frozen-lockfile

FROM dependencies AS build

COPY apps/dashboard apps/dashboard
ARG VITE_DOCS_URL=http://127.0.0.1:4174/docs
ENV VITE_DOCS_URL=${VITE_DOCS_URL}
RUN pnpm --dir apps/dashboard build

FROM nginx:1.27-alpine

COPY --from=build /workspace/apps/dashboard/dist /usr/share/nginx/html
COPY deploy/nginx/dashboard.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
