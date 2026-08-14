FROM node:24-bookworm-slim AS build

WORKDIR /app
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json ./web/package.json
RUN pnpm install --frozen-lockfile
COPY web ./web
WORKDIR /app/web
ENV NEXT_TELEMETRY_DISABLED=1
ARG API_INTERNAL_BASE_URL=http://api:8080
ENV API_INTERNAL_BASE_URL=${API_INTERNAL_BASE_URL}
RUN pnpm build

FROM node:24-bookworm-slim AS runtime
WORKDIR /app
COPY --from=build --chown=node:node /app/web/.next/standalone ./
COPY --from=build --chown=node:node /app/web/.next/static ./web/.next/static
WORKDIR /app/web
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
USER node
EXPOSE 3000
CMD ["node", "server.js"]
