# Задание на реализацию сервиса core

Версия 1.0.0 · 20.09.2026 · исполнитель: Разработчик B · reviewer: Разработчик A. Задание самодостаточно вместе с комплектом `handoff-core.zip` (состав — [README](README.md)). Пути указаны от корня репозитория.

## 1. Контекст продукта

«Вовремя» — мини-приложение MAX для малого бизнеса: реестр документов со сроками (лицензии, договоры, сертификаты, медосмотры) и напоминания в MAX. Пользователь открывает мини-приложение из чата с ботом; MAX передаёт подписанные данные запуска `initData`; мини-приложение вызывает публичный API сервиса `core`. Напоминания отправляет сервис `bot`, которому `core` передаёт их по gRPC. Пилот — общепит, 1–3 точки.

## 2. Назначение сервиса

`core` — публичный HTTP API мини-приложения и вся предметная логика: идентификация по `initData`, сессии, справочник, организации, участники и приглашения, документы и периоды, план и планировщик напоминаний, экспорт ICS, CLI для проверяющих и демо-данных.

## 3. Реализуемые требования

FR-01…FR-11, FR-14, FR-15, FR-17…FR-19; NFR-02, NFR-04…NFR-07, NFR-10…NFR-13; BR-02…BR-04. Формулировки и критерии — `docs/requirements/requirements-registry.md`.

## 4. Границы ответственности

Делает: всё, что описано в `openapi.yaml`; хранение в схеме `core`; вызовы `bot` только через `MessagingService`. Не делает: не вызывает MAX Bot API и не знает токен бота (только производный ключ); не читает схему `bot`; не раздаёт статику (это edge); не отправляет сообщения сам.

## 5. Стек и зависимости

Go 1.27 (директива `go 1.26` в корневом `go.mod`, модуль `vovremya`). Библиотеки (последние стабильные версии на 20.09.2026, фиксируются `go.sum`): `github.com/jackc/pgx/v5` (pgxpool), `github.com/pressly/goose/v3`, `google.golang.org/grpc`, `google.golang.org/protobuf`, `github.com/prometheus/client_golang`, `go.opentelemetry.io/otel` и `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`, `golang.org/x/time/rate`, `github.com/google/uuid`, `github.com/caarlos0/env/v11`; только в тестах — `github.com/getkin/kin-openapi` (контрактный тест). HTTP-роутер — `net/http.ServeMux` с шаблонами Go 1.22+ (`"GET /api/v1/documents/{documentId}"`). Генерация: sqlc 1.29+, buf v2.

## 6. Структура каталогов

```text
services/core/
  Dockerfile, sqlc.yaml
  cmd/core/main.go                         composition root и CLI
  migrations/embed.go, 00001_init.sql, 00002_catalog_seed.sql
  internal/domain/errors.go, role.go, account.go, organization.go, membership.go, invite.go, catalog.go,
                  applicability.go, document.go, period.go, deadline.go, reminder_plan.go, start_param.go, plural.go, texts.go
  internal/domain/deadline_test.go, reminder_plan_test.go, applicability_test.go, start_param_test.go, role_test.go
  internal/application/authz.go, auth.go, accounts.go, organizations.go, suggestions.go, documents.go, members.go,
                       invites.go, notifications.go, exports.go, telemetry.go, scheduler.go, retention.go, review.go, seed.go
  internal/application/documents_test.go, scheduler_test.go, invites_test.go
  internal/ports/repositories.go, messaging.go, launch.go, clock.go, tokens.go
  internal/adapters/httpapi/server.go, router.go, middleware.go, auth.go, ratelimit.go, problem.go, dto.go,
                           handlers_session.go, handlers_catalog.go, handlers_organizations.go, handlers_documents.go,
                           handlers_members.go, handlers_invites.go, handlers_exports.go, handlers_telemetry.go,
                           handlers_test.go, contract_test.go
  internal/adapters/postgres/db.go, tx.go, repositories.go, repositories_integration_test.go
  internal/adapters/postgres/queries/accounts.sql, sessions.sql, catalog.sql, organizations.sql, memberships.sql, invites.sql,
                                     documents.sql, periods.sql, reminders.sql, exports.sql, audit.sql, retention.sql
  internal/adapters/postgres/sqlcgen/      генерируется sqlc
  internal/adapters/botgrpc/client.go, fake.go, client_test.go
  internal/adapters/maxlaunch/verifier.go, verifier_test.go
  internal/adapters/ics/calendar.go, calendar_test.go
  internal/infrastructure/config/config.go, logging/logging.go, metrics/metrics.go, tracing/tracing.go,
                          lifecycle/lifecycle.go, cli/cli.go
  internal/archtest/arch_test.go           запрет импорта adapters/infrastructure/pgx/grpc из domain
demo/embed.go, demo/demo-data.json         демо-данные (пакет vovremya/demo)
```

## 7. Domain: сущности и инварианты

| Сущность | Инварианты |
|---|---|
| `Account` | `Kind` max или review; для max есть `MaxUserID > 0`; имя 1–128 символов |
| `Role` | порядок viewer < editor < owner; `Allows(min Role) bool` |
| `Organization` | имя 1–100 после `TrimSpace`; категория, регион и признаки существуют в `Catalog`; часовой пояс загружается `time.LoadLocation`; `Version ≥ 1` |
| `Membership` | ровно один owner на организацию; `NotifyLocalTime` — минуты без секунд |
| `Invite` | роль editor или viewer; активно, если не принято, не отозвано и `now < ExpiresAt` (72 ч) |
| `Document` | название 1–200; `ReferenceURL` пусто или `https://` без пробелов ≤ 1024; отступы 0–365, уникальны, ≤ 5; ровно один текущий период |
| `Period` | `ValidUntil ≥ ValidFrom`, если обе заданы; `ValidUntil = nil` — бессрочный |
| `DeadlineStatus` | `StatusOf(validUntil, today)`: nil → no_expiry; `< today` → expired; `≤ today+30` → expiring; иначе valid; `DaysLeft = validUntil − today` |
| `ReminderPlan` | `Plan(periods, recipients, offsets, now, grace)` → множество `(period, account, days, dueAt)`; `dueAt` = (`validUntil − days`) в `notifyLocalTime` пояса организации, переведённое в UTC; только `dueAt ≥ now − grace`; получатели — `kind=max` и `notify_enabled` |
| `StartTarget` | грамматика `doc_<uuid>`, `org_<uuid>`, `inv_<43 base64url>`; иное → none |
| Квоты | 20 организаций на аккаунт, 500 документов и 30 участников на организацию, 50 активных приглашений |

## 8. Use cases и алгоритмы

Общее: каждый изменяющий сценарий — одна транзакция `TxManager.WithinTx`; изменения внутри организации начинаются с `LockOrganization(orgID)` (`SELECT … FOR UPDATE`). Проверка прав — `authz.Require(ctx, orgPublicID, minRole)`: нет членства → `ErrNotFound`, роль ниже → `ErrForbidden`.

**CreateSession(initData, platform, appVersion, ip).** 1) лимит 300/мин на IP; 2) `LaunchVerifier.Verify(initData, now)` (раздел 18); 3) лимит 10/мин на `max_user_id`; 4) транзакция: `UpsertMaxAccount` (обновить имена), токен `vvs_` + base64url(32 случайных байта), вставить сессию (`sha256(токен)`, `expires_at = now + 12 ч`, `source = max_launch`, `launch_query_id`, `platform`), аудит `session.created`; 5) `StartTarget` из `start_param`; 6) 201.

**Authenticate(bearer).** Формат `^vvs_[A-Za-z0-9_-]{43}$` → хеш → сессия с аккаунтом, `revoked_at IS NULL`, `expires_at > now`; иначе `ErrUnauthenticated`. Если `last_seen_at < now − 5 мин` — обновить без ожидания результата.

**GetMe.** Членства с названиями организаций (по `joined_at`); для `kind=max` — `GetRecipientStatus` с deadline 2 с (ошибка → `unavailable`, `bot_chat_url` из кэша профиля или null); для review — `unknown`, null; лимиты — константы.

**DeleteMe.** Транзакция: организации, где пользователь владелец, удаляются каскадно; аккаунт удаляется (сессии, членства, напоминания, экспорты — каскадом); аудит `account.deleted` с `target_public_id` аккаунта. 204.

**GetCatalog.** Справочник загружается из БД при старте в неизменяемую структуру; ETag — `"` + первые 16 hex SHA-256 от JSON + `"`; совпадение `If-None-Match` → 304.

**CreateOrganization.** Валидация; транзакция: существует `public_id` → тот же `created_by` → 200 с текущим состоянием, иначе `ErrConflictIDReused`; членств у аккаунта ≥ 20 → `ErrQuotaExceeded`; вставка организации, признаков, членства owner (уведомления включены, 09:00), аудит. 201.

**UpdateOrganization.** editor; блокировка; `version ≠ expected_version` → `ErrConflictVersion`; изменения; признаки заменяются целиком; `version+1`; при смене часового пояса — перепланирование всех текущих периодов организации. **DeleteOrganization.** owner; аудит (до удаления) и удаление каскадом.

**ListSuggestions.** viewer; применимые типы: существует правило типа, у которого категория NULL или равна категории организации и все признаки правила есть у организации; исключить типы, уже присутствующие среди документов; порядок `sort_order`.

**CreateDocument / CreateDocumentsBatch.** editor; валидация всех элементов до транзакции; транзакция: блокировка; для каждого элемента: `public_id` существует в той же организации с тем же `created_by` → вернуть текущий, иначе при существовании → `ErrConflictIDReused`; количество документов + новые > 500 → `ErrQuotaExceeded`; отступы: из запроса, иначе из типа, иначе [30, 7, 1]; вставить документ, период (`public_id` генерирует сервер, `is_current = true`), отступы; перепланировать напоминания периода. Пакет — всё или ничего; 201 (пакет) либо 201/200 (одиночный).

**ListDocuments.** viewer; `today` — текущая дата в поясе организации; запрос:

```sql
SELECT d.public_id, d.document_type_code, d.title, d.responsible_label, p.valid_until,
       (SELECT min(r.due_at) FROM core.reminders r
         WHERE r.period_id = p.id AND r.account_id = @account_id AND r.status = 'planned') AS next_reminder_at
FROM core.documents d
JOIN core.document_periods p ON p.document_id = d.id AND p.is_current
WHERE d.organization_id = @organization_id
  AND (sqlc.narg(status)::text IS NULL
       OR (sqlc.narg(status) = 'expired'   AND p.valid_until < @today::date)
       OR (sqlc.narg(status) = 'expiring'  AND p.valid_until BETWEEN @today::date AND @today::date + 30)
       OR (sqlc.narg(status) = 'valid'     AND p.valid_until > @today::date + 30)
       OR (sqlc.narg(status) = 'no_expiry' AND p.valid_until IS NULL))
  AND (sqlc.narg(q)::text IS NULL OR d.title ILIKE '%' || sqlc.narg(q) || '%' ESCAPE '\')
  AND (coalesce(p.valid_until, 'infinity'::date), d.public_id) > (@after_until::date, @after_id::uuid)
ORDER BY coalesce(p.valid_until, 'infinity'::date), d.public_id
LIMIT @page_size;
```

Первая страница: `after_until = '-infinity'`, `after_id = 00000000-0000-0000-0000-000000000000`; `q` экранирует `\`, `%`, `_`; запрашивается `limit + 1` строк, лишняя даёт `next_cursor`.

**GetDocument.** viewer; документ, текущий и до 20 последних периодов, отступы, шаги продления и `data_status` из справочника, `next_reminder_at` текущего пользователя, `can_edit = role ≥ editor`.

**UpdateDocument.** editor; блокировка; проверка версии; применение полей (null очищает); `valid_from`/`valid_until` меняют текущий период; `reminder_offsets_days` заменяет набор; `version+1`, `updated_by`; перепланирование при изменении дат или отступов.

**RenewDocument.** editor; блокировка; период с этим `id` уже есть у документа → 200, у другого документа → `ErrConflictIDReused`; валидация дат; старый период `is_current = false` (сначала), новый вставляется текущим; перепланирование нового периода и отмена `planned` старого; `version+1`. 201.

**DeleteDocument.** editor; аудит `document.deleted`; удаление каскадом. 204.

**Перепланирование `Replan(scope)`.** Вход — множества периодов и аккаунтов в области изменения. 1) `desired = domain.Plan(...)`; 2) для каждого элемента — upsert:

```sql
INSERT INTO core.reminders (period_id, account_id, days_before, due_at, status, next_attempt_at)
VALUES (@period_id, @account_id, @days_before, @due_at, 'planned', @due_at)
ON CONFLICT (period_id, account_id, days_before) DO UPDATE
SET due_at = EXCLUDED.due_at, next_attempt_at = EXCLUDED.due_at, status = 'planned',
    attempts = 0, last_error_code = NULL, updated_at = now()
WHERE core.reminders.status <> 'handed_off';
```

3) строки `planned` в области, отсутствующие в `desired` (сравнение по ключу `period:account:days` через `<> ALL(@keep::text[])`), → `cancelled`.

**Members.** `ListMembers` — viewer. `UpdateMemberRole` — owner; цель — участник и не owner; роль editor/viewer. `RemoveMember`: если цель — сам пользователь: owner → `ErrForbidden` («удалите организацию»), иначе удалить членство; если цель — другой: требуется owner, цель не owner. В обоих случаях `planned`-напоминания цели по периодам организации → `cancelled`; аудит `member.removed`.

**Notification settings.** GET — своё членство. PUT — обновить `notify_enabled`, `notify_local_time`; `Replan(все текущие периоды организации, {я})`.

**Invites.** `CreateInvite` — owner; повтор `id` → `ErrConflictIDReused`; участников ≥ 30 или активных приглашений ≥ 50 → `ErrQuotaExceeded`; профиль бота из кэша (TTL 10 мин) или `GetBotProfile` (ошибка без кэша → `ErrDependencyUnavailable`); токен — base64url(32 байта) (43 символа); `link_url` = шаблон с `{payload}` = `inv_<токен>`; `share_text` = «Приглашаю вести сроки документов «{название}» в приложении «Вовремя»»; вставить (`sha256(токен)`, `expires_at = now + 72 ч`), аудит. `PreviewInvite` — лимит 10/мин на аккаунт; не найдено, отозвано или принято другим → `ErrInviteInvalid` (404); истекло → `ErrInviteExpired`; пользователь уже участник → `ErrAlreadyMember`. `AcceptInvite` — те же проверки внутри транзакции с блокировкой организации и приглашения; квота 30; вставить членство, отметить принятие, `Replan(текущие периоды, {новый участник})`, аудит; после фиксации — `EnqueueNotification` владельцу: ключ `mj:<invite_uuid>`, `kind = MEMBER_JOINED`, текст из spec §15, кнопка `org_<uuid>`, `not_after = now + 24 ч`; ошибка только логируется. `RevokeInvite` — owner; активное → `revoked_at`; уже отозванное → 204; принятое → `ErrInviteInvalid` (409).

**Exports.** `CreateCalendarExport` — viewer, лимит 10/мин на аккаунт; токен 43 символа; вставка с `expires_at = now + 10 мин`; ответ `download_url = CORE_PUBLIC_BASE_URL + "/api/v1/downloads/" + токен`, `file_name = "vovremya-" + первые 8 символов UUID организации + ".ics"`. `DownloadCalendar`: `UPDATE … SET download_count = download_count + 1 WHERE token_hash = $1 AND expires_at > now() AND download_count < 3 RETURNING organization_id`; нет строки: существует — `ErrLinkGone` (410), нет — 404. ICS (RFC 5545, CRLF): `BEGIN:VCALENDAR`, `VERSION:2.0`, `PRODID:-//Vovremya//RU`; для каждого документа с `valid_until`: `VEVENT` c `UID:<period_uuid>@vovremya`, `DTSTAMP`, `DTSTART;VALUE=DATE`, `SUMMARY:Срок: <название>` (экранирование `\`, `;`, `,`, перевод строки), `VALARM` с `TRIGGER:-P<N>D` для каждого отступа. Заголовки `Content-Type: text/calendar; charset=utf-8`, `Content-Disposition: attachment; filename="…"`, `Cache-Control: no-store`.

**ClientEvents.** Лимит 60/мин на IP; валидация; `code` вне белого списка (`not_in_max`, `launch_invalid`, `launch_expired`, `network`, `server_5xx`, `bridge_download_failed`, `bridge_share_failed`, `bridge_qr_failed`, `bridge_unsupported`) → `other`; метрика и лог info. 202.

**ReminderScheduler** (каждые 15 с): 1) захват без длинной транзакции:

```sql
UPDATE core.reminders r SET next_attempt_at = now() + @lease::interval, updated_at = now()
WHERE r.id IN (SELECT id FROM core.reminders WHERE status = 'planned' AND next_attempt_at <= now()
               ORDER BY next_attempt_at LIMIT @batch FOR UPDATE SKIP LOCKED)
RETURNING r.id, r.period_id, r.account_id, r.days_before, r.due_at, r.attempts;
```

2) загрузка данных (период, документ, организация, `max_user_id` получателя); 3) для каждого: `now > due_at + 24 ч` → `skipped`; иначе `EnqueueNotification` (ключ `rem:<period_uuid>:<account_uuid>:<days>`, `kind = REMINDER`, текст из spec §15 с датой `DD.MM.YYYY` и `PluralDays`, кнопка «Открыть документ» → `doc_<uuid>`, `not_after = due_at + 24 ч`); OK → `UPDATE … SET status='handed_off', handed_off_at=now(), bot_notification_id=… WHERE id=… AND status='planned'`; повторяемая ошибка → `attempts+1`, `next_attempt_at = now + min(5 мин, 10 с × 2^attempts) + U(0; 5 с)`, `last_error_code`; неповторяемая → `skipped`, `grpc_invalid`. Не более 10 одновременных вызовов gRPC.

**RetentionJob** (каждый час) — правила `docs/database/data-model.md` §8.

**CLI.** `review-token issue --login <^[a-z][a-z0-9_]{2,31}$> --role editor|viewer --ttl <≤ 720h>`: upsert аккаунта review (имя «Проверяющий»), демо-организация `0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01` должна существовать (иначе код выхода 2 и текст «сначала выполните seed-demo»), upsert членства, сессия `source = review_cli`, аудит, печать токена. `review-token revoke --login` — `revoked_at` всем сессиям. `seed-demo` — идемпотентно загружает `demo/demo-data.json`: владелец — review-аккаунт `demo_owner`; даты документов задаются смещением `valid_until_offset_days` от текущей даты.

## 9. Используемые контракты

`openapi.yaml` 1.0.0 (сервер, полностью); `api/proto/vovremya/bot/v1/messaging.proto` 1.0.0 (клиент: `EnqueueNotification`, `GetRecipientStatus`, `GetBotProfile`); сводки — `docs/contracts/http-api.md`, `docs/contracts/grpc-contract.md`.

## 10. Определения типов

Все типы запросов и ответов — `components.schemas` в `openapi.yaml`; соответствие Go-типам и таблицам — `docs/contracts/data-schemas.md`; типы gRPC — `messaging.proto`.

## 11. Схема БД и миграции

`services/core/migrations/00001_init.sql`, `00002_catalog_seed.sql` (goose, встраиваются через `embed`, таблица версий `core.goose_db_version`, `goose.SetTableName`). Команда `core migrate up` использует `CORE_MIGRATE_DATABASE_URL`. Модель, индексы, нормализация — `docs/database/data-model.md`.

## 12. Транзакции и конкурентность

READ COMMITTED; блокировка строки организации в каждом изменении внутри организации; оптимистичная блокировка по `version`; планировщик — `SKIP LOCKED` и lease; финальные обновления напоминаний — с условием `status = 'planned'`. Таблица сценариев — data-model §7.

## 13. Вызовы других сервисов

| Вызов | Когда | Ожидаемый ответ | При ошибке |
|---|---|---|---|
| `EnqueueNotification` | планировщик; принятие приглашения | `status = QUEUED`, `duplicate` | backoff (планировщик); лог (member_joined) |
| `GetRecipientStatus` | `GET /me` | `state`, `bot_chat_url` | `state = unavailable` |
| `GetBotProfile` | создание приглашения (кэш 10 мин) | `open_app_link_template`, `chat_url` | 503 `DEPENDENCY_UNAVAILABLE` без кэша |

## 14. Ошибки и отказы

| Ошибка domain/application | HTTP | `code` |
|---|---|---|
| `ErrValidation{Fields}` | 400 | VALIDATION_FAILED |
| `ErrUnauthenticated` | 401 | UNAUTHENTICATED |
| `ErrLaunchInvalid`, `ErrLaunchExpired` | 401 | LAUNCH_DATA_INVALID, LAUNCH_DATA_EXPIRED |
| `ErrForbidden` | 403 | FORBIDDEN |
| `ErrNotFound` | 404 | NOT_FOUND |
| `ErrInviteInvalid` | 404 / 409 | INVITE_INVALID |
| `ErrConflictVersion`, `ErrConflictIDReused`, `ErrQuotaExceeded`, `ErrInviteExpired`, `ErrAlreadyMember` | 409 | соответствующий код |
| `ErrLinkGone` | 410 | LINK_GONE |
| `ErrRateLimited` | 429 + `Retry-After` | RATE_LIMITED |
| перегрузка семафора | 503 + `Retry-After: 1` | OVERLOADED |
| `ErrDependencyUnavailable`; недоступна БД | 503 | DEPENDENCY_UNAVAILABLE |
| прочее | 500 | INTERNAL (детали только в логе) |

## 15. Идемпотентность, таймауты, повторы

Клиентские UUID (раздел 8); ключ напоминания `rem:…`; ключ сообщения о новом участнике `mj:…`. Таймауты HTTP-сервера: чтение заголовков 5 с, тела 10 с, запись 15 с, idle 60 с; бюджет обработчика 5 с; gRPC 2 с; при остановке — `Shutdown` 20 с. Повторы — только планировщик (формула выше); HTTP-обработчики не повторяют запросы к БД.

## 16. Конфигурация

Переменные `CORE_*`, `APP_ENV`, `LOG_LEVEL` — `docs/operations/configuration.md`. Загрузка — `caarlos0/env` в `config.Config`; секреты поддерживают `_FILE`; при `APP_ENV=prod` секрет с префиксом `devonly` или dev-ключ `e45f53166d1da778700f29abbf89f6ac247864cb97356f927707e56f0066d663` → выход с кодом 1 и сообщением `insecure default secret`.

## 17. Логи, метрики, health

Поля логов и метрики — `docs/architecture/observability.md`; обязательные метрики core: `vovremya_http_*`, `vovremya_session_create_total`, `vovremya_client_events_total`, `vovremya_grpc_client_requests_total`, `vovremya_db_*`, `vovremya_reminders_*`, `vovremya_app_errors_total`. Admin-сервер `:8081`: `/healthz`, `/readyz` (ping БД 500 мс), `/metrics`. `core healthcheck` — GET `/readyz`, код выхода 0/1.

## 18. Интеграция с MAX

Проверка `initData` — `docs/max/max-integration-spec.md` §4 (алгоритм), §5 (грамматика `start_param`); тест-векторы TV-1…TV-6 — `docs/max/test-vectors.md`. Ключ — `CORE_MAX_WEBAPP_SECRET_HEX` (32 байта). Реализация — чистая функция `Verify(initData string, now time.Time) (LaunchIdentity, error)` без обращения к сети.

## 19. Примеры и фикстуры

`docs/contracts/examples.md` (curl-сценарии), `demo/demo-data.json` (12 документов демо-организации, UUID документа КЭП — `3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02`), `docs/max/test-vectors.md`, `scripts/sign_initdata.py`.

## 20. Тесты и критерии приёмки

| Группа | Что обязательно покрыть | Критерий |
|---|---|---|
| T-DOM | `StatusOf` на границах 0, 30, 31 день и `nil`; `Plan` для поясов Europe/Moscow, Asia/Vladivostok, Europe/Kaliningrad, прошедших отступов и выключенных уведомлений; применимость для 3 профилей; грамматика `start_param`; `Role.Allows`; `PluralDays` для 1, 2, 5, 11, 21 | 100 % веток указанных функций |
| T-DOM-AUTH | TV-1…TV-6, плюс `+` в имени пользователя | все ожидаемые результаты |
| T-APP | создание, повтор, конфликт id, квоты, продление, перепланирование, приглашения, удаление аккаунта (с `FakeMessagingGateway` и репозиториями в памяти или postgres-test) | все сценарии раздела 8 |
| T-REPO, T-PG | запросы sqlc на postgres-test; изоляция схем; ограничения (один owner, один текущий период) | зелёный `make test-integration` |
| T-API | статусы и `code` для каждой операции OpenAPI; `DisallowUnknownFields`; роли | ответ валиден по OpenAPI (kin-openapi) |
| T-CONC | два параллельных PATCH → один 409; 501-й документ при параллельных вставках → `QUOTA_EXCEEDED`; два планировщика не передают одно напоминание дважды | детерминированно 20 прогонов подряд |
| T-FAIL | bot недоступен: CRUD 2xx, `/me` `unavailable`, приглашение 503, напоминание остаётся `planned` | ожидаемые ответы |
| T-ARCH | зависимости domain | тест зелёный |

Приёмка сервиса: AC-01…AC-08, AC-10, AC-11 (`docs/testing/acceptance.md`).

## 21. Порядок реализации

CORE-01 каркас → CORE-02 сессии и проверка initData → CORE-03 справочник и организации → CORE-04 документы → CORE-05 план и планировщик (сначала с `FakeMessagingGateway`, затем с `bot`) → CORE-06 участники, приглашения, настройки, удаление → CORE-07 экспорт, телеметрия, CLI.

## 22. Команды

```sh
docker compose up -d --build                 # вся система локально
docker compose up -d --build core            # пересобрать только core
make gen-sql                                 # код sqlc после изменения queries/*.sql
go test ./services/core/...                  # модульные тесты
make test-integration                        # тесты с PostgreSQL (профиль test)
docker compose exec core /app/core seed-demo
```

## 23. Решения, которые нельзя менять самостоятельно

Состав сервисов и gRPC-контракт; пути, коды и схемы OpenAPI; алгоритм проверки `initData` и производный ключ; серверные непрозрачные сессии; схема `core` и роли БД; идемпотентность по клиентским UUID; очередь напоминаний в PostgreSQL без брокера; лимиты и квоты; формат ключей идемпотентности. Изменение — через ADR и review Разработчика A.

## 24. Definition of Done

Все операции OpenAPI реализованы; контрактный тест, T-DOM, T-APP, T-API, T-CONC, T-FAIL и `make test-integration` зелёные; `go vet` без замечаний; логи содержат обязательные поля; метрики отдаются; `docker compose up -d --build` даёт `healthy`; AC-01…AC-08, AC-10, AC-11 пройдены на стенде; нет секретов в логах; review Разработчика A получен.
