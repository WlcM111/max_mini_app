# Отчёт реализации третьего микросервиса: core-service

Версия 3.0.0 · 24.09.2026 · этап 3 · предыдущие этапы: `FIRST_SERVICE_REPORT.md` (reminders-service),
`SECOND_SERVICE_REPORT.md` (bot-service).

## 1. Что реализовано

**core-service** — публичный HTTP API мини-приложения «Вовремя» и вся предметная логика:
идентификация по данным запуска MAX, сессии, справочник, организации и участники,
документы и периоды, приглашения, экспорт календаря, телеметрия, доставка изменений
в reminders-service через transactional outbox.

Объём: 7 565 строк Go (включая тесты) в `services/core`, 21 путь OpenAPI, 3 миграции,
9 репозиториев PostgreSQL, 2 gRPC-клиента, 2 фоновых компонента.

| Модуль (v2 §4) | Где | Состояние |
|---|---|---|
| identity | `internal/app/sessions.go`, `adapters/maxlaunch` | реализован |
| catalog | `internal/app/app.go`, `adapters/postgres/catalog.go` | реализован |
| workspace | `internal/app/organizations.go`, `members.go`, `invites.go` | реализован |
| documents | `internal/app/documents.go` | реализован |
| exports | `internal/app/exports.go`, `adapters/ics` | реализован |
| audit | `adapters/postgres/misc.go` (`AuditRepo`) | реализован |
| telemetry | `internal/app/accounts.go` (`AcceptClientEvents`), `internal/app/metrics.go` | реализован |
| outbox | `internal/app/outbox.go`, `adapters/postgres/misc.go`, `adapters/remindersgrpc` | реализован |

## 2. Реализованные операции публичного API

Все 21 путь и 30 операций `openapi.yaml` 1.1.0: сессии (`POST /sessions`,
`DELETE /sessions/current`), аккаунт (`GET /me`, `DELETE /me`), справочник (`GET /catalog`
с ETag и 304), организации (создание с идемпотентностью по `id`, чтение со сводкой статусов,
изменение с оптимистичной блокировкой, удаление, подсказки типов документов), документы
(реестр с фильтрами, поиском и курсором, создание одиночное и пакетное, карточка, изменение,
продление, удаление), участники (список, смена роли, исключение и выход, настройки напоминаний),
приглашения (создание со ссылкой-диплинком, список, отзыв, предпросмотр, принятие),
экспорт календаря (создание одноразовой ссылки, скачивание ICS без Bearer), телеметрия
(`POST /client-events`).

CLI: `serve`, `migrate up|down`, `healthcheck`, `review-token issue|revoke`, `seed-demo`.

## 3. Инженерные решения этапа

| Решение | Основание |
|---|---|
| SQL написан вручную поверх pgx, каталог `sqlcgen` и цель `make gen-sql` удалены | оба ранее реализованных сервиса используют тот же подход (решение этапа 1, `FIRST_SERVICE_REPORT.md` §3); sqlc в среде сборки отсутствует, единый стиль важнее |
| Состояние плана `reminders_state` вычисляется по собственному outbox и результату чтения плана | требование v2 §2.3: `actual` — событий в очереди нет и план прочитан; `pending` — событие ещё не доставлено; `unavailable` — reminders-service не ответил |
| Событие `membership_state` пишется при создании участия, смене роли и изменении настроек уведомлений | роль не входит в контракт `MembershipState`, но версия агрегата должна расти монотонно |
| Отвергнутое получателем событие (`REJECTED`) переводится в `failed` с повтором через 24 часа | повтор без изменения данных бесполезен, но строка сохраняется для разбора (`docs/architecture/consistency.md` §5) |
| Метрика `vovremya_app_errors_total` переиспользуется из общего каркаса | объявлена в `internal/platform/metrics` (observability §2), повторная регистрация невозможна |

## 4. Изменения нормативных документов

| Документ | Изменение | Основание |
|---|---|---|
| `services/core/migrations/00001_init.sql` | удалена таблица `core.reminders`, добавлена `core.outbox_events`, добавлено поле `memberships.version` | handoff `core-service-v2.md` §1, §2.1, §2.4 |
| `services/core/migrations/00001_init.sql` | `REVOKE … FROM core_app` обёрнут в условный блок | дефект **D-7** (см. §6) |
| `services/core/migrations/00003_migration_history_privileges.sql` | новая миграция: снятие прав на `core.goose_db_version` с `core_app` | ADR-014, требование v2 §7 |
| `openapi.yaml` | версия 1.1.0: поле `reminders_state`, ответ `default` со схемой `Problem` у всех операций, `409` у `revokeInvite` | handoff `core-service-v2.md` §3 |
| `compose.yaml` | сервис `core` получил собственный адрес `10.77.2.20`, псевдоним `core-api`, зависимости от `bot` и `reminders` | дефект **D-8** (см. §6) |
| `deploy/edge/Dockerfile` | образ собирается без каталога `frontend/`, отдаёт страницу-заглушку; прежняя сборка сохранена как `Dockerfile.frontend` | дефект **D-9** (см. §6) |

Protobuf-контракты, пути и коды ошибок OpenAPI, схемы данных reminders и bot не изменялись.

## 5. Транзакции, идемпотентность, конкурентность

Каждый изменяющий сценарий — одна транзакция `TxManager.WithinTx`; изменения внутри
организации начинаются с `SELECT … FOR UPDATE` строки организации. Событие outbox пишется
в той же транзакции — попытка записи вне транзакции отклоняется (`OutboxRepo.Append`,
тест `TestOutboxRowRequiresTransaction`).

Идемпотентность: клиентские UUID организаций, документов, периодов продления и приглашений;
повтор возвращает текущее состояние (200) либо `CONFLICT_ID_REUSED`, если идентификатор
принадлежит другому пользователю. Оптимистичная блокировка по `expected_version`
для организаций и документов. Сообщение о новом участнике ставится с ключом `mj:<invite_uuid>`.

Конкурентность: два параллельных `PATCH` одного документа дают ровно один `200`
и один `409` — проверено 20 прогонами подряд (`TestConcurrentUpdatesProduceSingleConflict`).
Relay захватывает события `FOR UPDATE SKIP LOCKED` с арендой `CORE_OUTBOX_LEASE`.

## 6. Обнаруженные и исправленные дефекты

| ID | Симптом | Первопричина | Исправление | Проверка |
|---|---|---|---|---|
| D-7 | `goose up` прерывается: `role "core_app" does not exist` | безусловный `REVOKE … FROM core_app` в нормативной DDL; роль создаёт `deploy/postgres/init/01-init.sh`, в чистой тестовой базе её нет | права снимаются внутри `DO $$ … IF EXISTS (SELECT 1 FROM pg_roles …)`, как в `reminders` и `bot` | интеграционные тесты core применяют миграции на чистой базе |
| D-8 | сервис `core` в `compose.yaml` имел адрес `10.77.2.10` и псевдоним `bot-grpc` — те же, что у `bot` | ошибка копирования при подготовке архитектурного каркаса | `core` получил `10.77.2.20` и псевдоним `core-api`; добавлены зависимости от `bot` и `reminders` | `python3 scripts/check_compose.py` — 125 проверок |
| D-9 | `docker compose up --build` не собирает образ `edge`: `COPY frontend/…` при отсутствующем каталоге | Mini App относится к следующему этапу и в репозитории отсутствует | `deploy/edge/Dockerfile` отдаёт страницу-заглушку и проксирует API; сборка интерфейса сохранена в `Dockerfile.frontend` | сборка образа без каталога `frontend/` возможна; состав compose проверен скриптом |
| D-10 | повторная регистрация метрики `vovremya_app_errors_total` приводила к панике при старте | метрика объявлена в общем каркасе и повторно создавалась в сервисе | сервис переиспользует метрику каркаса | автономные тесты core |
| D-11 | 500 при любом обращении с Bearer: `column reference "id" is ambiguous` | список столбцов аккаунта без псевдонима в соединении `sessions ⋈ accounts` | столбцы указаны с псевдонимом `a.` | `TestSessionStartTargetAndMe` и весь набор API-тестов |
| D-12 | строка `init_data` из 8 кириллических символов проходила проверку длины | длина считалась в байтах, а OpenAPI задаёт `minLength` в символах | подсчёт через `utf8.RuneCountInString` | `TestUnauthenticatedAndMalformedRequests` |

Дефекты reminders-service и bot-service на этом этапе не обнаружены; их исходный код не изменялся.

## 7. Тесты

| Группа | Состав | Результат |
|---|---|---|
| T-DOM | границы `StatusOf` (0/30/31/nil), часовые пояса, грамматика `start_param`, роли, применимость справочника для трёх профилей, валидация, выдержка повторов | зелёные |
| T-DOM-AUTH | тест-векторы TV-1…TV-6 из `docs/max/test-vectors.md` | зелёные |
| T-CONF | значения по умолчанию, запрет dev-ключа и `http://` в `APP_ENV=prod`, границы пакета outbox | зелёные |
| T-ARCH | границы слоёв: домен без HTTP/SQL/gRPC, адаптеры не зависят друг от друга, нет общего предметного кода между сервисами | зелёные |
| T-API, T-APP, T-REPO | 18 интеграционных сценариев на реальной PostgreSQL: сессии, справочник с ETag, организации, квоты, документы и реестр, пакет «всё или ничего», роли, приглашения, экспорт ICS, настройки, удаление аккаунта | зелёные |
| T-CONC | 20 прогонов параллельного изменения документа | зелёные |
| T-FAIL | отказ reminders (CRUD работает, `unavailable`, доставка после восстановления), отказ приёма событий (`pending`), отказ bot (`/me` = `unavailable`, приглашение 503), отвергнутое событие, запрет записи события вне транзакции, очистка | зелёные |
| Regression | полные наборы reminders-service и bot-service | зелёные |
| E2E | шесть сценариев на настоящих процессах трёх сервисов | пройдены |

## 8. Состояние автономной работоспособности

core-service собирается (`go build ./services/core/...`), применяет миграции на чистой
PostgreSQL, запускается с собственным admin-портом (`/healthz`, `/readyz`, `/metrics`),
корректно завершает работу по сигналу (`lifecycle.Runner`, `CORE_SHUTDOWN_TIMEOUT`).
Недоступность reminders-service и bot-service не мешает запуску и работе с документами:
план и состояние канала помечаются недоступными, изменения накапливаются в outbox
и доставляются после восстановления.
