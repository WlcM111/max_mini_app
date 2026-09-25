# Задание на платформу: Compose, образы, edge, стенд

Версия 1.0.0 · 20.09.2026 · исполнитель: Разработчик B · reviewer: Разработчик A. Комплект — `handoff-infra.zip`.

| Тема | Нормативный источник | Ключевые требования |
|---|---|---|
| Compose | `compose.yaml`, `docs/architecture/deployment.md` | сервисы, сети `edge`/`data (internal)`, healthcheck, лимиты, профили `monitoring`, `test`, `loadtest`; одна команда `docker compose up -d --build` |
| Образы Go | `services/core/Dockerfile`, `services/bot/Dockerfile` | `golang:1.27-alpine` → `alpine:3.22`, пользователь 10001, BuildKit-кэш модулей и сборки, бинарник с командой `healthcheck` |
| Edge | `deploy/edge/Dockerfile`, `deploy/edge/Caddyfile`, ADR-011 | сборка frontend на `node:22-alpine`, запрет `devonly` в prod-бандле, CSP, `X-Request-Id`, маршруты `/api/*`, `POST /max/webhook`, SPA |
| PostgreSQL | `deploy/postgres/init/01-init.sh`, ADR-006 | роли, схемы, привилегии по умолчанию, том в `/var/lib/postgresql` |
| Конфигурация | `.env.example`, `docs/operations/configuration.md`, `scripts/generate_prod_env.py` | dev-значения `devonly_`, prod-`.env` с правами 600 |
| Стенд | `docs/operations/compose-and-runbook.md` §2 | VPS, ufw, DNS, ACME, сертификаты Минцифры |
| Резервные копии | runbook §4 | cron 03:00, 7 копий, копия вне хоста |
| Сборка ≤ 5 минут | `scripts/measure_build.sh` | базовые образы загружаются заранее и не учитываются |

| Задача | Шаги | Definition of Done |
|---|---|---|
| INF-01 (20.09) | арендовать VPS, настроить ufw и DNS, установить Docker, развернуть репозиторий со страницей-заглушкой edge | `https://<домен>/` отвечает 200 с доверенным сертификатом |
| INF-02 (22.09) | скачать сертификаты Минцифры, сверить отпечатки, закоммитить, заполнить runbook §6 | bot в режиме `live` успешно вызывает `GET /me` |
| INT-03 (22–28.09) | ежедневно 21:00: `git pull && docker compose up -d --build`, дамп БД и копия вне хоста, запись в журнал интеграции | журнал за каждый день |
| REL-01 (28.09) | тег `v1.0.0`, prod-развёртывание, `measure_build.sh` на чистой машине, `run_data_api_checks.py` против стенда | чек-лист `docs/operations/submission.md` §4 |

Решения, которые нельзя менять самостоятельно: состав сервисов compose, сети и их изоляция, запуск миграций отдельными контейнерами, одна команда запуска, запрет рабочих секретов в репозитории.
