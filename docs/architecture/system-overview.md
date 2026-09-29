# Общая архитектура

Версия 1.0.0 · 20.09.2026

## D1. Контекст системы

```mermaid
flowchart LR
  owner["Владелец бизнеса"]
  staff["Сотрудник: редактор или наблюдатель"]
  maxc["Клиент MAX: iOS, Android, desktop, web"]
  maxp["Платформа MAX: Bot API и webhook"]
  sys["Система Вовремя"]
  jury["Проверяющие хакатона"]
  owner --> maxc
  staff --> maxc
  maxc -->|"HTTPS: мини-приложение и /api/v1"| sys
  maxp -->|"HTTPS webhook: события бота"| sys
  sys -->|"HTTPS Bot API: сообщения"| maxp
  maxp -->|"сообщения бота с кнопками"| maxc
  jury -->|"HTTPS /api/v1 по DATA-API.yaml"| sys
```

## D2. Компоненты и микросервисы

```mermaid
flowchart TB
  subgraph device["Устройство пользователя"]
    fe["Mini App: React 18, MAX UI, MAX Bridge"]
  end
  subgraph host["Хост Docker Compose"]
    edge["edge: Caddy 2.10, TLS, статика, прокси"]
    core["core: Go, публичный HTTP API, предметная логика, планировщик"]
    bot["bot: Go, webhook, очередь доставки, лимиты MAX"]
    pg[("PostgreSQL 18: схемы core и bot")]
  end
  maxapi["MAX Bot API: botapi.max.ru"]
  fe -->|"HTTPS JSON /api/v1, Bearer"| edge
  edge -->|"HTTP /api/*"| core
  edge -->|"HTTP POST /max/webhook"| bot
  core -->|"gRPC vovremya.bot.v1.MessagingService"| bot
  core -->|"SQL, роль core_app, схема core"| pg
  bot -->|"SQL, роль bot_app, схема bot"| pg
  bot -->|"HTTPS, Authorization: токен бота"| maxapi
  maxapi -->|"HTTPS POST, X-Max-Bot-Api-Secret"| edge
```

| Компонент | Технология | Ответственность | Владелец |
|---|---|---|---|
| Mini App | React 18.3, TypeScript 5, Vite 7, @maxhub/max-ui, TanStack Query 5, React Router 7 | интерфейс, bootstrap, MAX Bridge | A |
| edge | Caddy 2.10 | TLS (ACME), статика, маршрутизация `/api/*` и `/max/webhook`, заголовки безопасности, `X-Request-Id` | B |
| core | Go 1.27 (минимум 1.26), pgx 5, sqlc, goose, grpc-go | сессии, организации, участники, приглашения, справочник, документы, напоминания, экспорт | B |
| bot | Go 1.27 (минимум 1.26), pgx 5, grpc-go | webhook, состояние диалогов, очередь сообщений, подписка, профиль бота | A |
| PostgreSQL | 18 (образ `postgres:18-alpine`) | хранение, схемы с раздельными ролями | B (core), A (bot) |

## D3. Путь MAX → Mini App → backend

```mermaid
flowchart LR
  u["Пользователь"] -->|"1. кнопка бота или диплинк startapp"| mc["Клиент MAX"]
  mc -->|"2. открывает https://домен/ c фрагментом WebAppData"| fe["Mini App"]
  fe -->|"3. читает initData"| br["MAX Bridge window.WebApp"]
  fe -->|"4. POST /api/v1/sessions"| edge["edge"]
  edge -->|"5. прокси"| core["core"]
  core -->|"6. HMAC-подпись и срок 1 ч"| ver["maxlaunch verifier"]
  core -->|"7. upsert account, insert session"| pg[("PostgreSQL")]
  core -->|"8. 201: token и start"| fe
  fe -->|"9. GET /me и /catalog с Bearer"| edge
  fe -->|"10. экран по start: документ, организация, приглашение или дашборд"| u
```

Сквозные сценарии со всеми этапами (MAX → Mini App → public API → backend → PostgreSQL/bot → ответ → UI) — [sequences.md](sequences.md). Развёртывание — [deployment.md](deployment.md).
