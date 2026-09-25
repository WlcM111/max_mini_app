# Передача проекта: состояние после реализации Mini App

Версия 1.0.0 · 24.09.2026. Документ для разработчика, который принимает проект без доступа
к переписке.

## 1. Что в репозитории

| Компонент | Состояние | Точка входа |
|---|---|---|
| core-service | реализован, 30 операций OpenAPI 1.1.0 | `services/core/cmd/core` |
| reminders-service | реализован | `services/reminders/cmd/reminders` |
| bot-service | реализован | `services/bot/cmd/bot` |
| MAX Mini App | реализован, 20 экранов | `frontend/src/main.tsx` |
| Развёртывание | Docker Compose: postgres, миграции, три сервиса, edge (собирает интерфейс) | `compose.yaml` |

## 2. С чего начать

```sh
unzip vovremya-full-4_0_0.zip -d vovremya && cd vovremya
docker compose up -d --build && docker compose ps       # всё поднимается одной командой
cd frontend && npm ci && npm test -- --run && npm run build
```

Без Docker: `STACK_ADMIN_DSN=… bash scripts/run_local_stack.sh` поднимает три настоящих сервиса,
`npm --prefix frontend run dev` — интерфейс на `http://localhost:5173/?mockUser=1001`.

## 3. Что читать

| Вопрос | Документ |
|---|---|
| Что за продукт и какие требования | `docs/requirements/product-brief.md`, `requirements-registry.md` |
| Архитектура системы | `docs/ARCHITECTURE_TZ.md`, `docs/architecture/system-overview.md` |
| Как устроен интерфейс | `docs/frontend/frontend-architecture.md`, `docs/frontend/screens-map.md` |
| Что реализовано на этом этапе | `docs/implementation/FRONTEND_REPORT.md` |
| Какие механизмы MAX используются | `docs/frontend/max-api-registry.md` |
| Как клиент общается с backend | `docs/frontend/api-client.md` |
| Результаты проверок | `docs/testing/frontend-test-report.md` |
| Что осталось и чем ограничено | `docs/implementation/known-limitations.md` |
| Как подключить и проверить в MAX | `docs/operations/max-miniapp-setup.md` |

## 4. Незакрытые задачи

| Задача | Что сделать | Чего не хватает |
|---|---|---|
| MAX-01 | сохранить реальные тела webhook в `testdata` | токен и стенд MAX |
| MAX-02 | проверить кнопку `open_app` вместо `link` | то же |
| MAX-03, MAX-04 | подключить Mini App в кабинете и пройти сценарии §3 инструкции | домен с HTTPS |
| T-E2E-01…06 | прогнать браузерные сценарии Playwright | среда с доступом к CDN браузеров |
| C-FE-01 | решить по обновлению до React 19 + max-ui 0.5.0 | решение архитектора (ADR-003) |

## 5. Правила, которые нельзя нарушать

Три backend-сервиса и их границы; публичный контракт `openapi.yaml` как единственный источник
типов клиента; обращения к `window.WebApp` только из `src/platform/max`; сетевые вызовы только
из `src/api/client.ts`; хранение токена без cookie; отсутствие секретов в бандле;
бюджет JS ≤ 200 КБ gzip; тексты статусов и кодов ошибок из нормативных документов.
