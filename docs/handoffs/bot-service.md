# Задание на реализацию сервиса bot

Версия 1.0.0 · 20.09.2026 · исполнитель: Разработчик A · reviewer: Разработчик B. Комплект — `handoff-bot.zip` (состав — [README](README.md)).

## 1. Контекст продукта

«Вовремя» — мини-приложение MAX для учёта сроков документов малого бизнеса. Мини-приложение по правилам MAX открывается только из чата с ботом; бот также доставляет напоминания, когда мини-приложение закрыто. Своих пользовательских сценариев у бота нет: каждое его сообщение содержит кнопку, открывающую мини-приложение.

## 2. Назначение сервиса

`bot` — единственный держатель токена бота: принимает события MAX (webhook), ведёт состояние диалогов, отправляет сообщения из очереди с соблюдением лимитов MAX, поддерживает подписку webhook и профиль бота, предоставляет `core` gRPC-сервис `MessagingService`.

## 3. Реализуемые требования

FR-08 (доставка), FR-09 (статус канала), FR-12, FR-19 (доставка), BR-05 (бот не меняет данные), NFR-04, NFR-05 (секрет webhook, токен), NFR-07, NFR-10, NFR-12, PLAT-02…PLAT-06.

## 4. Границы ответственности

Делает: webhook `POST /max/webhook`; очередь `bot.outbound_messages`; вызовы Bot API `GET /me`, `GET /subscriptions`, `POST /subscriptions`, `POST /messages`, `PATCH /me/commands`; gRPC-сервер. Не делает: не знает о документах и организациях; не читает схему `core`; не формирует тексты напоминаний (их передаёт core), кроме собственных ответов (`welcome`, `help`, подсказка); не реализует Long Polling (F-42).

## 5. Стек и зависимости

Go 1.27 (директива `go 1.26`), `pgx/v5`, `goose/v3`, `grpc`, `protobuf`, `grpc/health`, `grpc/reflection` (только local), `prometheus/client_golang`, `otel` + `otelgrpc`, `golang.org/x/time/rate`, `google/uuid`, `caarlos0/env/v11`. Клиент Bot API — собственный на `net/http` (ADR-008).

## 6. Структура каталогов

```text
services/bot/
  Dockerfile, sqlc.yaml
  cmd/bot/main.go
  migrations/embed.go, 00001_init.sql
  internal/domain/errors.go, notification.go, recipient.go, update.go, render.go, deeplink.go, backoff.go
  internal/domain/notification_test.go, update_test.go, render_test.go, backoff_test.go
  internal/application/enqueue.go, delivery.go, lease_reaper.go, webhook.go, subscription.go, profile.go, retention.go
  internal/application/enqueue_test.go, delivery_test.go, webhook_test.go
  internal/ports/repositories.go, maxclient.go, clock.go
  internal/adapters/grpcserver/server.go, server_test.go
  internal/adapters/webhook/handler.go, handler_test.go
  internal/adapters/webhook/testdata/bot_started.json, message_created.json, dialog_muted.json
  internal/adapters/maxapi/client.go, models.go, stub.go, client_test.go
  internal/adapters/postgres/db.go, tx.go, repositories.go, repositories_integration_test.go
  internal/adapters/postgres/queries/outbound.sql, recipients.sql, inbound.sql, retention.sql
  internal/adapters/postgres/sqlcgen/     генерируется
  internal/infrastructure/config/config.go, logging/logging.go, metrics/metrics.go, tracing/tracing.go,
                          lifecycle/lifecycle.go, ratelimit/limiter.go, ratelimit/limiter_test.go, cli/cli.go
  internal/archtest/arch_test.go
```

## 7. Domain: сущности и инварианты

| Сущность | Инварианты |
|---|---|
| `Notification` | статусы `queued → sending → sent / retry_wait / failed / expired`; `sending` ⇔ задан `LockedUntil`; `sent` ⇔ задан `SentAt`; попыток ≤ 8; не отправляется после `NotAfter` |
| `Button` | 1–3 на сообщение, по одной в ряду; текст 1–64; `open_app` с payload `^[A-Za-z0-9_-]{0,512}$` или `url` `https://` ≤ 2048 |
| `Recipient` | состояния `unknown, active, muted, stopped, unreachable` (переходы — `docs/architecture/long-operations.md` D8-4); изменение только событием не старее `LastEventTime` |
| `UpdateClassifier` | `Classify(raw []byte) (Update, outcome)`: тип, время (мс → UTC), `max_user_id` из `user.user_id` или `message.sender.user_id`, текст из `message.body.text`; неизвестная форма → `ignored` |
| `Render` | `Render(msg, profile, buttonKind) MaxSendRequest` (spec §8) |
| `Backoff` | `Delay(n) = U(0; min(10 мин, 5 с × 2^(n−1)))` |

## 8. Use cases и алгоритмы

**EnqueueNotification.** 1) Валидация (`docs/contracts/grpc-contract.md` §1) → `INVALID_ARGUMENT` с описанием поля. 2) Глубина очереди (кэш, обновляется каждые 5 с) > `BOT_QUEUE_LIMIT` → `RESOURCE_EXHAUSTED`. 3) `request_hash` = SHA-256 детерминированной сериализации запроса без `idempotency_key`. 4) Транзакция: `INSERT … ON CONFLICT (idempotency_key) DO NOTHING RETURNING id`; вставлено → кнопки, `QUEUED`, `duplicate=false`; не вставлено → чтение существующей строки: хеш совпал → текущий статус и `duplicate=true`; иначе `ALREADY_EXISTS`.

**GetNotificationStatus / GetRecipientStatus / GetBotProfile.** Чтение по ключу (нет → `NOT_FOUND`); получатель (нет строки → `UNKNOWN`, `bot_chat_url` из профиля или пусто); профиль из памяти (не загружен → `UNAVAILABLE`).

**DeliveryWorker** (`BOT_WORKERS` горутин, опрос каждые 500 мс):

```sql
UPDATE bot.outbound_messages SET status = 'sending', locked_until = now() + @lease::interval,
       attempts = attempts + 1, updated_at = now()
WHERE id IN (SELECT id FROM bot.outbound_messages
             WHERE status IN ('queued', 'retry_wait') AND next_attempt_at <= now()
             ORDER BY next_attempt_at LIMIT @batch FOR UPDATE SKIP LOCKED)
RETURNING *;
```

Для каждой строки: `now > not_after` → `expired`; получатель `stopped` → `failed`, `recipient_stopped`; есть кнопки `open_app`, а профиль не загружен → `retry_wait` через 30 с без увеличения счётчика (`attempts − 1`); иначе ожидание глобального лимитера и лимитера получателя (контекст с lease), `Render`, `MaxClient.SendMessage` (таймаут 10 с), классификация по spec §8: `sent` (`sent_at`, `max_message_id`, `recipients.last_delivery_at`), `retry_wait` (`next_attempt_at = now + Delay(attempts)`; при `attempts ≥ 8` → `failed`), `failed` (403/404 → получатель `unreachable`). Все финальные обновления — `WHERE id = $1 AND status = 'sending'`, `locked_until = NULL`.

**LeaseReaper** (каждые 30 с): `UPDATE … SET status='retry_wait', locked_until=NULL, next_attempt_at=now(), updated_at=now() WHERE status='sending' AND locked_until < now()`.

**Webhook** (`POST /max/webhook`, spec §7): 1) секрет в `X-Max-Bot-Api-Secret` (`subtle.ConstantTimeCompare`) иначе 401; 2) тело ≤ 256 KB иначе 413; 3) `dedupe_key = sha256(тело)`; 4) `Classify`; 5) транзакция: `INSERT bot.inbound_updates … ON CONFLICT DO NOTHING`; конфликт → 200; при `applied` — upsert получателя с условием времени; при ответе — постановка сообщения с ключом `reply:<hex dedupe_key>` и `not_after = now + 1 ч`; подсказка на произвольный текст — только если за последние 10 минут этому получателю не ставилось сообщение вида `help` (проверка по `outbound_recipient_idx`); 6) `COMMIT` → `200 {}`; ошибка БД → 503. Таблица событий → эффектов — spec §7.

**SubscriptionKeeper**, **ProfileService**, `PATCH /me/commands` (`[{"name":"start","description":"Открыть приложение"},{"name":"help","description":"Как пользоваться"}]`, ошибка — только warning, формат поля проверяется в MAX-01) — spec §9, §10.

**RetentionJob** (каждый час): `inbound_updates` старше 7 суток; сообщения в `sent/failed/expired` старше 30 суток по `updated_at` (кнопки удаляются каскадом); получатели `stopped` без событий 30 суток.

**Режим stub:** `MaxClient` — реализация в памяти: `SendMessage` пишет в лог и кольцевой буфер (500 записей) и возвращает `mid = stub-<n>`; `GetMe` → `BOT_STUB_USERNAME`; подписка не выполняется; admin-эндпоинт `GET /debug/stub/messages` отдаёт буфер JSON-массивом `{recipient, text, buttons, created_at}`.

## 9. Используемые контракты

`api/proto/vovremya/bot/v1/messaging.proto` 1.0.0 (сервер); webhook и Bot API — `docs/max/max-integration-spec.md` §7–§10, §15; факты — `docs/max/max-platform-facts.md`.

## 10. Определения типов

gRPC — в `.proto`. Модели Bot API (`models.go`): `SendMessageBody{Text string; Notify bool; Attachments []Attachment}`, `Attachment{Type "inline_keyboard"; Payload{Buttons [][]Button}}`, `Button{Type "link"|"open_app"; Text; URL; WebApp; Payload}`, `SendMessageResult{Message{Body{Mid}}}`, `Me{UserID int64; Name; Username; IsBot bool}`, `SubscriptionList{Subscriptions []{URL string}}`, `SubscribeBody{URL; UpdateTypes []string; Secret}`, `SimpleResult{Success bool; Message string}`. Неизвестные поля ответа игнорируются.

## 11. Схема БД и миграции

`services/bot/migrations/00001_init.sql`, goose, таблица версий `bot.goose_db_version`, команда `bot migrate up` с `BOT_MIGRATE_DATABASE_URL`.

## 12. Транзакции и конкурентность

Захват очереди `FOR UPDATE SKIP LOCKED`; обработка сообщения вне транзакции захвата; финальное обновление условно по статусу; webhook — одна транзакция; уникальность ключей защищает от гонок постановки.

## 13. Вызовы других сервисов

Только MAX Bot API (spec §8–§10). В core ничего не вызывает.

## 14. Ошибки и отказы

gRPC-коды — grpc-contract §1. Недоступна БД → gRPC `UNAVAILABLE`, webhook 503, `/readyz` 503. MAX недоступен → `retry_wait`. 401 от MAX → `retry_wait`, лог error не чаще раза в минуту. Паника в обработчике — перехват, 500/`INTERNAL`, лог с `request_id`.

## 15. Идемпотентность, таймауты, повторы

`idempotency_key` + `request_hash`; webhook — `sha256(тело)`; ответы — `reply:<hash>`. Таймауты: gRPC-обработчики — контекст клиента; MAX — 10 с; webhook — 10 с; остановка — `GracefulStop` 5 с, HTTP `Shutdown` 20 с, воркеры завершают текущее сообщение. Повторы доставки — формула `Backoff`.

## 16. Конфигурация

Переменные `BOT_*` — `docs/operations/configuration.md`. При `live` обязательны `BOT_MAX_TOKEN`, `BOT_WEBHOOK_PUBLIC_URL`, `BOT_WEBHOOK_SECRET`. HTTP-клиент MAX: пул доверия = `x509.SystemCertPool()` + PEM из `BOT_MAX_EXTRA_CA_FILE`; отсутствие файла в `live` — ошибка запуска.

## 17. Логи, метрики, health

`docs/architecture/observability.md`; метрики bot: `vovremya_http_*` (webhook), `vovremya_grpc_server_handled_total`, `vovremya_outbound_queue_depth`, `vovremya_outbound_send_total`, `vovremya_max_api_*`, `vovremya_max_webhook_updates_total`, `vovremya_max_subscription_ok`. Токен и секрет не логируются; текст сообщений пользователей не логируется.

## 18. Интеграция с MAX

Полностью — `docs/max/max-integration-spec.md` §7–§12, §15, §16.

## 19. Примеры и фикстуры

`docs/contracts/examples.md` §4 (curl webhook, grpcurl); образцы событий `testdata/*.json` заполняются в MAX-01 реальными телами из логов стенда с заменой имён на «Тест».

## 20. Тесты и критерии приёмки

| Группа | Что покрыть | Критерий |
|---|---|---|
| T-DOM | переходы `Notification` и `Recipient`, `Classify` на образцах, `Render` для `link` и `open_app`, `Backoff` в пределах | все ветки |
| T-GRPC | валидация, повтор (`duplicate`), конфликт (`ALREADY_EXISTS`), `RESOURCE_EXHAUSTED`, `UNAVAILABLE` профиля | коды совпадают с контрактом |
| T-BOT | воркер с фейковым `MaxClient`: 200, 400, 401, 403, 404, 429, 500, таймаут; истечение lease; `not_after`; `recipient_stopped`; лимит 1 сообщение/600 мс на получателя и 20 rps глобально | статусы и тайминги в допуске 10 % |
| T-IDEM | повтор webhook; повтор Enqueue | одна строка, один ответ |
| T-FAIL | MAX 5xx/429 | `retry_wait` с ростом задержки |
| T-REPO | запросы sqlc на postgres-test | зелёный `make test-integration` |

Приёмка: AC-MAX-05…AC-MAX-08, AC-MAX-12, AC-04.

## 21. Порядок реализации

BOT-01: каркас, миграция, gRPC-сервер со всеми RPC, `stub` → BOT-02: воркер, lease, лимиты, клиент MAX с сертификатами Минцифры → BOT-03: webhook, состояния, ответы, подписка, профиль, команды, очистка.

## 22. Команды

```sh
docker compose up -d --build bot
docker compose exec bot wget -qO- http://127.0.0.1:8081/debug/stub/messages
go test ./services/bot/...
make gen-proto && make gen-sql
make test-integration
```

## 23. Решения, которые нельзя менять самостоятельно

gRPC-контракт; только webhook (без Long Polling); кнопка `link` по умолчанию; лимиты 20 rps и 600 мс; очередь в PostgreSQL; ключи идемпотентности; бот без сценариев изменения данных; хранение событий без текста сообщений.

## 24. Definition of Done

Все RPC и webhook реализованы; T-DOM, T-GRPC, T-BOT, T-IDEM, T-FAIL, интеграционные тесты зелёные; в `live` на стенде: профиль загружен, подписка создана, AC-MAX-05…08 и AC-MAX-12 пройдены; `docker compose up` даёт `healthy`; review Разработчика B.
