# Словарь конфигурации

Версия 1.0.0 · 20.09.2026. Решение — ADR-015. «Секрет: да» означает, что значение не логируется, может задаваться файлом через `<ИМЯ>_FILE`, а при `APP_ENV=prod` не может начинаться с `devonly` (иначе сервис завершается с сообщением `insecure default secret`). Столбец «По умолчанию» — значение в коде сервиса, если переменная не задана; значения в `compose.yaml` указаны отдельно.

## Общие

| Переменная | Сервисы | По умолчанию | Prod | Секрет | Назначение |
|---|---|---|---|---|---|
| `APP_ENV` | core, bot | `local` | `prod` | нет | режим; `prod` включает проверку секретов |
| `APP_VERSION` | сборка всех образов | `dev` | тег релиза `1.0.0` | нет | версия в логах и тег образов |
| `LOG_LEVEL` | core, bot | `info` | `info` | нет | `debug`, `info`, `warn`, `error` |

## PostgreSQL (контейнер `postgres`)

| Переменная | Compose по умолчанию | Prod | Секрет | Назначение |
|---|---|---|---|---|
| `POSTGRES_PASSWORD` | `devonly_pg_admin` | 32 символа `[A-Za-z0-9]` | да | суперпользователь `vovremya_admin` (только init) |
| `PG_CORE_MIGRATOR_PASSWORD` | `devonly_core_migrator` | 32 символа | да | роль `core_migrator` |
| `PG_CORE_APP_PASSWORD` | `devonly_core_app` | 32 символа | да | роль `core_app` |
| `PG_BOT_MIGRATOR_PASSWORD` | `devonly_bot_migrator` | 32 символа | да | роль `bot_migrator` |
| `PG_BOT_APP_PASSWORD` | `devonly_bot_app` | 32 символа | да | роль `bot_app` |

## core

| Переменная | По умолчанию | Секрет | Назначение |
|---|---|---|---|
| `CORE_HTTP_ADDR` | `:8080` | нет | публичный API (через edge) |
| `CORE_ADMIN_ADDR` | `:8081` | нет | `/healthz`, `/readyz`, `/metrics` |
| `CORE_DATABASE_URL` | обязательна | да | DSN роли `core_app` |
| `CORE_MIGRATE_DATABASE_URL` | обязательна для `migrate` | да | DSN роли `core_migrator` |
| `CORE_DB_MAX_CONNS` | `20` | нет | размер пула |
| `CORE_MAX_WEBAPP_SECRET_HEX` | обязательна | да | `hex(HMAC_SHA256("WebAppData", токен))`, 64 hex; в prod запрещено dev-значение `e45f5316…d663` |
| `CORE_LAUNCH_MAX_AGE` | `1h` | нет | срок `auth_date` |
| `CORE_LAUNCH_FUTURE_SKEW` | `60s` | нет | допуск будущего `auth_date` |
| `CORE_SESSION_TTL` | `12h` | нет | срок сессии |
| `CORE_BOT_GRPC_ADDR` | `bot:9090` | нет | адрес bot |
| `CORE_BOT_RPC_TIMEOUT` | `2s` | нет | deadline gRPC |
| `CORE_BOT_PROFILE_CACHE_TTL` | `10m` | нет | кэш `GetBotProfile` |
| `CORE_PUBLIC_BASE_URL` | обязательна | нет | базовый адрес ссылок скачивания ICS |
| `CORE_TRUSTED_PROXY_CIDRS` | `10.77.1.0/24` | нет | от кого принимать `X-Forwarded-For` |
| `CORE_HTTP_MAX_INFLIGHT` | `256` | нет | предел одновременных запросов |
| `CORE_HANDLER_TIMEOUT` | `5s` | нет | бюджет обработчика; операциям ассистента он расширяется: текст и профиль — `CORE_GIGACHAT_TIMEOUT` + 5 с, фото — 3 × `CORE_GIGACHAT_TIMEOUT` + 5 с (ADR-033) |
| `CORE_RATE_ACCOUNT_RPS`, `CORE_RATE_ACCOUNT_BURST` | `10`, `30` | нет | лимит на аккаунт |
| `CORE_RATE_SESSION_PER_USER_MIN`, `CORE_RATE_SESSION_PER_IP_MIN` | `10`, `300` | нет | лимиты `POST /sessions` |
| `CORE_SCHEDULER_INTERVAL`, `CORE_SCHEDULER_BATCH`, `CORE_SCHEDULER_LEASE` | `15s`, `200`, `2m` | нет | планировщик напоминаний |
| `CORE_REMINDER_GRACE` | `24h` | нет | допустимое опоздание напоминания |
| `CORE_INVITE_TTL` | `72h` | нет | срок приглашения |
| `CORE_EXPORT_TTL` | `10m` | нет | срок ссылки ICS |
| `CORE_RETENTION_INTERVAL` | `1h` | нет | период очистки |
| `CORE_SHUTDOWN_TIMEOUT` | `25s` | нет | общий срок остановки |
| `CORE_RELAY_INTERVAL` | `2s` | нет | период доставки событий outbox в reminders-service |
| `CORE_RELAY_CONCURRENCY` | `4` | нет | одновременные доставки outbox (1…4) |
| `CORE_OUTBOX_BATCH` | `100` | нет | событий outbox за один проход |
| `CORE_OUTBOX_LEASE` | `60s` | нет | аренда захваченного события outbox |
| `CORE_OUTBOX_RETENTION` | `168h` | нет | срок хранения доставленных событий outbox |
| `CORE_SESSION_RETENTION` | `720h` | нет | срок хранения истёкших и отозванных сессий |
| `CORE_AUDIT_RETENTION` | `4320h` | нет | срок хранения журнала аудита |
| `CORE_RATE_INVITE_PER_MIN` | `10` | нет | приглашений в минуту на аккаунт |
| `CORE_RATE_CLIENT_EVENTS_PER_MIN` | `60` | нет | технических событий мини-приложения в минуту |
| `CORE_GIGACHAT_AUTH_KEY` | пусто | **да, секрет** | ключ авторизации GigaChat API: Base64(Client ID:Client Secret). Пусто — ассистент выключен (ADR-032). Поддерживается `CORE_GIGACHAT_AUTH_KEY_FILE` |
| `CORE_GIGACHAT_SCOPE` | `GIGACHAT_API_PERS` | нет | область доступа: PERS, B2B или CORP |
| `CORE_GIGACHAT_MODEL` | `GigaChat-Pro` | нет | модель генерации |
| `CORE_GIGACHAT_BASE_URL` | `https://gigachat.devices.sberbank.ru/api/v1` | нет | адрес API; в prod только https |
| `CORE_GIGACHAT_OAUTH_URL` | `https://ngw.devices.sberbank.ru:9443/api/v2/oauth` | нет | выдача токена доступа (живёт 30 минут) |
| `CORE_GIGACHAT_CA_FILE` | `/etc/vovremya/ca/russian_trusted_ca_bundle.pem` | нет | бандл НУЦ Минцифры для TLS с доменами Сбера |
| `CORE_GIGACHAT_TIMEOUT` | `8s` | нет | предел ожидания ответа модели; распознавание фото — до трёх таких интервалов |
| `CORE_GIGACHAT_MAX_INPUT_CHARS` | `2000` | нет | предел длины текста пользователя |
| `CORE_GIGACHAT_DAILY_TOKEN_BUDGET` | `200000` | нет | суточный бюджет токенов; 0 — без ограничения |
| `CORE_RATE_ASSISTANT_PER_MIN` | `10` | нет | обращений к ассистенту в минуту на аккаунт |

## bot

| Переменная | По умолчанию | Секрет | Назначение |
|---|---|---|---|
| `BOT_MODE` | `stub` | нет | `stub` или `live` |
| `BOT_HTTP_ADDR`, `BOT_ADMIN_ADDR`, `BOT_GRPC_ADDR` | `:8080`, `:8081`, `:9090` | нет | webhook, admin, gRPC |
| `BOT_DATABASE_URL`, `BOT_MIGRATE_DATABASE_URL` | обязательны | да | DSN `bot_app`, `bot_migrator` |
| `BOT_DB_MAX_CONNS` | `8` | нет | размер пула |
| `BOT_MAX_API_BASE_URL` | `https://botapi.max.ru` | нет | Bot API (на боевом стенде — этот адрес, на нём зарегистрирована подписка на webhook) |
| `BOT_MAX_TOKEN` | обязательна при `live` | да | токен бота (compose берёт из `MAX_BOT_TOKEN`) |
| `BOT_MAX_EXTRA_CA_FILE` | `/etc/vovremya/ca/russian_trusted_ca_bundle.pem` | нет | сертификаты Минцифры |
| `BOT_MAX_REQUEST_TIMEOUT` | `10s` | нет | таймаут вызова MAX |
| `BOT_REMINDERS_GRPC_ADDR` | `reminders:9091` | нет | адрес reminders-service: нажатие «Напомнить через неделю» ставит повтор в его план (ADR-036) |
| `BOT_REMINDERS_RPC_TIMEOUT` | `5s` | нет | срок вызова reminders-service при нажатии кнопки |
| `BOT_WEBHOOK_PUBLIC_URL` | обязательна при `live` | нет | `https://<домен>/max/webhook` |
| `BOT_WEBHOOK_SECRET` | обязательна | да | 5–256 символов `[A-Za-z0-9_-]`, в prod — 64 |
| `BOT_WEBHOOK_UPDATE_TYPES` | `bot_started,bot_stopped,dialog_removed,dialog_muted,dialog_unmuted,message_created,message_callback` | нет | подписка; `message_callback` нужен кнопке «Напомнить через неделю» (ADR-036) |
| `BOT_SUBSCRIPTION_CHECK_INTERVAL` | `10m` | нет | проверка подписки |
| `BOT_GLOBAL_RPS` | `20` | нет | лимит вызовов Bot API (официальный предел 30) |
| `BOT_PER_RECIPIENT_INTERVAL` | `600ms` | нет | пауза между сообщениями одному получателю |
| `BOT_WORKERS`, `BOT_WORKER_BATCH`, `BOT_WORKER_POLL_INTERVAL` | `4`, `50`, `500ms` | нет | доставка |
| `BOT_LEASE` | `60s` | нет | lease отправки |
| `BOT_MAX_ATTEMPTS`, `BOT_RETRY_BASE`, `BOT_RETRY_MAX` | `8`, `5s`, `10m` | нет | повторы |
| `BOT_QUEUE_LIMIT` | `50000` | нет | backpressure |
| `BOT_STUB_USERNAME` | `vovremya_local_bot` | нет | ник в режиме `stub` |
| `BOT_OPEN_APP_BUTTON_KIND` | `link` | нет | `link` или `open_app` |
| `BOT_SHUTDOWN_TIMEOUT` | `25s` | нет | срок остановки |
| `BOT_WEBHOOK_MAX_BODY_BYTES` | `262144` | нет | предел тела webhook; больше — 413 |
| `BOT_WEBHOOK_TIMEOUT` | `10s` | нет | срок обработки одного события webhook |
| `BOT_HANDLER_TIMEOUT` | `10s` | нет | срок обработки вызова gRPC |
| `BOT_PROFILE_RETRY_INTERVAL` | `30s` | нет | повтор загрузки профиля бота и отсрочка сообщений с диплинком до его появления |
| `BOT_LEASE_REAP_INTERVAL` | `30s` | нет | период возврата сообщений с истёкшей арендой |
| `BOT_QUEUE_DEPTH_INTERVAL` | `5s` | нет | период обновления глубины очереди для backpressure и метрики |
| `BOT_RETENTION_INTERVAL` | `1h` | нет | период очистки |
| `BOT_INBOUND_TTL` | `168h` | нет | хранение журнала событий webhook |
| `BOT_FINALIZED_TTL` | `720h` | нет | хранение завершённых сообщений |
| `BOT_STOPPED_RECIPIENT_TTL` | `720h` | нет | хранение остановленных диалогов |
| `BOT_GRPC_STOP_TIMEOUT` | `5s` | нет | ожидание graceful stop gRPC перед принудительной остановкой |
| `BOT_HTTP_STOP_TIMEOUT` | `20s` | нет | ожидание завершения запросов webhook при остановке |
| `BOT_DB_MAX_CONNS` | `8` | нет | размер пула PostgreSQL |

Команда `bot migrate up` требует только `BOT_MIGRATE_DATABASE_URL`; команда
`bot healthcheck` — только `BOT_ADMIN_ADDR`. При `APP_ENV=prod` запуск с
`BOT_MODE=stub` запрещён (ADR-031), `BOT_WEBHOOK_SECRET` должен быть не короче
64 символов, а `BOT_MAX_API_BASE_URL` — только `https://`.

## edge и сборка frontend

| Переменная | Compose по умолчанию | Prod | Назначение |
|---|---|---|---|
| `EDGE_SITE_ADDRESS` | `:80` | доменное имя | адрес сайта Caddy; домен включает ACME |
| `EDGE_HTTP_PORT`, `EDGE_HTTPS_PORT` | `8080`, `8443` | `80`, `443` | порты хоста |
| `ACME_EMAIL` | `devonly@localhost` | почта команды | уведомления ACME |
| `EDGE_FRAME_ANCESTORS` | `'self' https://max.ru https://*.max.ru` | по итогам MAX-03 | CSP `frame-ancestors` |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | `https://<домен>` | передаётся в `CORE_PUBLIC_BASE_URL` |
| `MAX_BOT_TOKEN` | пусто | токен организаторов | передаётся в `BOT_MAX_TOKEN` |
| `VITE_MOCK_BRIDGE` | `true` | `false` | имитация MAX Bridge |
| `VITE_MOCK_BOT_TOKEN` | `devonly-local-bot-token` (задан в Dockerfile edge) | не попадает в prod-бандл | подпись имитированного initData |
| `VITE_APP_VERSION` | `dev` | `1.0.0` | версия в телеметрии и экране «Аккаунт» |
| `K6_BASE_URL`, `K6_BOT_TOKEN` | задаются в сервисе `k6` | не используются | профиль `loadtest`: адрес edge и dev-токен для подписи initData в сценарии k6 |
| `VITE_MSW` | не задана | не задана | только `npm run dev`: MSW вместо API |

## Порты

| Порт | Где | Доступ |
|---|---|---|
| 8080 (local), 80/443 (prod) | хост → edge | публичный |
| 8080 | core, bot внутри сети | только edge |
| 8081 | core, bot | внутри сетей compose (Prometheus, healthcheck) |
| 9090 | bot gRPC | только сеть `edge` (core) |
| 5432 | postgres | только сеть `data` |
| 127.0.0.1:9091 | Prometheus (профиль monitoring) | локально |
| 127.0.0.1:55432 | postgres-test (профиль test) | локально |


## reminders-service

| Переменная | Назначение | Тип | По умолчанию | Обязательна | Безопасная передача |
|---|---|---|---|---|---|
| APP_ENV | режим (local, prod) | строка | local | нет | — |
| APP_VERSION | версия сборки для логов | строка | dev | нет | — |
| LOG_LEVEL | уровень логирования | строка | info | нет | — |
| REMINDERS_GRPC_ADDR | адрес gRPC-сервера | host:port | :9091 | нет | только внутренняя сеть |
| REMINDERS_ADMIN_ADDR | адрес служебного HTTP | host:port | :8081 | нет | только внутренняя сеть |
| REMINDERS_DATABASE_URL | DSN роли приложения | строка | — | да | файл через REMINDERS_DATABASE_URL_FILE |
| REMINDERS_MIGRATE_DATABASE_URL | DSN роли-владельца схемы | строка | значение предыдущего | для миграций | файл через _FILE |
| REMINDERS_DB_MAX_CONNS | предел соединений пула | целое | 10 | нет | — |
| REMINDERS_BOT_GRPC_ADDR | адрес bot-service | host:port | bot:9090 | нет | внутренняя сеть |
| REMINDERS_BOT_RPC_TIMEOUT | срок вызова bot-service | длительность | 5s | нет | — |
| REMINDERS_SCHEDULER_INTERVAL | период опроса плана | длительность | 15s | нет | — |
| REMINDERS_SCHEDULER_BATCH | размер пакета захвата (1..1000) | целое | 200 | нет | — |
| REMINDERS_SCHEDULER_LEASE | аренда захваченной строки | длительность | 2m | нет | не меньше BOT_RPC_TIMEOUT |
| REMINDERS_SCHEDULER_CONCURRENCY | одновременные вызовы bot (1..64) | целое | 4 | нет | — |
| REMINDERS_GRACE | допустимое опоздание напоминания | длительность | 24h | нет | — |
| REMINDERS_RETRY_BASE | базовая выдержка повтора | длительность | 10s | нет | — |
| REMINDERS_RETRY_MAX | предел выдержки | длительность | 5m | нет | — |
| REMINDERS_RETRY_JITTER | случайная добавка | длительность | 5s | нет | — |
| REMINDERS_RETENTION_INTERVAL | период очистки | длительность | 1h | нет | — |
| REMINDERS_INBOX_TTL | срок хранения журнала событий | длительность | 168h | нет | — |
| REMINDERS_FINALIZED_TTL | срок хранения неактивных напоминаний | длительность | 720h | нет | — |
| REMINDERS_HANDLER_TIMEOUT | предел обработки запроса без срока клиента | длительность | 10s | нет | — |
| REMINDERS_SHUTDOWN_TIMEOUT | срок остановки | длительность | 25s | нет | — |
| REMINDERS_MAX_BATCH_EVENTS | предел событий в пакете (1..1000) | целое | 200 | нет | — |
| REMINDERS_TEST_DATABASE_URL | DSN для интеграционных тестов | строка | — | только тесты | локальная среда |

## core-service: переменные взаимодействия с reminders

| Переменная | Назначение | По умолчанию |
|---|---|---|
| CORE_REMINDERS_GRPC_ADDR | адрес reminders-service | reminders:9091 |
| CORE_REMINDERS_RPC_TIMEOUT | срок запросов плана | 2s |
| CORE_REMINDERS_SYNC_FLUSH_TIMEOUT | предел синхронной попытки доставки события (AC-05) | 300ms |

Проверка значений выполняется при старте: неизвестный APP_ENV, отсутствие DSN, выход параметров
за допустимые границы и аренда меньше срока вызова bot приводят к отказу запуска с описанием причины.
