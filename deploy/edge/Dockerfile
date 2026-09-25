# syntax=docker/dockerfile:1.7
FROM node:22-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY frontend/ ./
ARG VITE_MOCK_BRIDGE=false
ARG VITE_APP_VERSION=dev
ENV VITE_MOCK_BRIDGE=${VITE_MOCK_BRIDGE} \
    VITE_MOCK_BOT_TOKEN=devonly-local-bot-token \
    VITE_APP_VERSION=${VITE_APP_VERSION}
RUN npm run build
# prod-сборка не должна содержать dev-токен имитации
RUN if [ "$VITE_MOCK_BRIDGE" != "true" ] && grep -r "devonly" dist; then echo "dev secret leaked into bundle" && exit 1; fi

FROM caddy:2.10-alpine
COPY deploy/edge/Caddyfile /etc/caddy/Caddyfile
COPY --from=web /web/dist /srv/app
EXPOSE 80 443 8088
