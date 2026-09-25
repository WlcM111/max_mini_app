# Задание: core-service в архитектуре 2.0.0 (к реализации)

Версия 2.0.0. Документ описывает изменения относительно задания v1.0.0 (`docs/handoffs/core-service.md`),
которое остаётся действующим во всём, что не перечислено ниже.

## 1. Что перестаёт принадлежать core-service

- таблица напоминаний и её жизненный цикл;
- планировщик напоминаний и передача сообщений в bot-service для напоминаний;
- тексты напоминаний и склонение дней (переносятся в reminders-service).

Из структуры v1.0.0 исключаются: `internal/application/scheduler.go`, `internal/domain/reminder_plan.go`,
`internal/domain/texts.go` (в части напоминаний), `internal/domain/plural.go`,
таблица `core.reminders` и связанные запросы.

## 2. Что добавляется

### 2.1 Модуль outbox

Таблица `core.outbox_events`:

| Поле | Тип | Ограничения |
|---|---|---|
| id | bigint identity | первичный ключ, определяет порядок доставки |
| event_id | uuid | уникально; ключ идемпотентности получателя |
| event_type | text | одно из значений vovremya.reminders.v1.EventType |
| aggregate_type | text | organization, membership, document, account |
| aggregate_id | text | UUID либо `<org_uuid>:<account_uuid>` для участия |
| aggregate_version | bigint | версия агрегата в core, монотонна |
| payload | jsonb | сериализованное сообщение payload контракта; схема — нормативный .proto |
| snapshot | boolean | признак снимка для сверки |
| status | text | pending, sent, failed |
| attempts | smallint | число неудачных попыток |
| next_attempt_at | timestamptz | момент следующей попытки и аренда |
| last_error_code | text | код последней ошибки |
| created_at, sent_at | timestamptz | отметки времени |

Индексы: `(status, next_attempt_at)` для выборки, `(aggregate_type, aggregate_id, aggregate_version)` для сверки,
`(created_at)` для контроля возраста и очистки.

Правило: строка outbox пишется **в той же транзакции**, что и изменение предметных данных.
Отдельная транзакция для события запрещена.

### 2.2 Relay

Фоновый компонент core: выбирает пакет `FOR UPDATE SKIP LOCKED` с арендой, вызывает
`IngestService/ApplyEvents`, фиксирует исход по каждому событию, повторяет с выдержкой
`min(5 мин, 10 с × 2^attempts)` плюс случайная добавка до 5 с, не более 4 одновременных вызовов.
Протокол и отказные сценарии — `docs/architecture/consistency.md`, разделы 4 и 5.

### 2.3 Синхронная попытка доставки

После фиксации транзакции изменения, влияющего на напоминания, core выполняет попытку доставки
с пределом `CORE_REMINDERS_SYNC_FLUSH_TIMEOUT`. Результат определяет поле `reminders_state` в ответе API.

### 2.4 Версии агрегатов

В `core.organizations` и `core.documents` версия уже есть. В `core.memberships` добавляется поле
`version bigint NOT NULL DEFAULT 1`, увеличиваемое при каждом изменении роли или настроек уведомлений.

### 2.5 Чтение плана

Клиент `ReminderQueryService`: `GetNextReminders` при выдаче реестра и карточки документа,
`GetSyncStatus` для определения `reminders_state`. Срок вызова — `CORE_REMINDERS_RPC_TIMEOUT` (2 с).
Недоступность reminders-service не блокирует чтение и изменение документов: поле `next_reminder_at`
не возвращается, `reminders_state=unavailable`.

## 3. Изменения публичного контракта

OpenAPI повышается до версии 1.1.0; изменения только добавляющие:

| Изменение | Где | Причина |
|---|---|---|
| Поле `reminders_state` (actual, pending, unavailable) | Document, DocumentListItem, ответы изменяющих операций над документами | запрет выдавать непересчитанный план за актуальный |
| `next_reminder_at` допускает отсутствие значения | Document, DocumentListItem | план может быть временно недоступен |
| Ответ `default` со схемой Problem у всех операций | все пути | ответы 429, 500, 503 не были объявлены (дефект 1 прежнего анализа) |
| Ответ 409 у `revokeInvite` | /invites/{id} | расхождение с описанием правил (дефект 2) |
| Уточнение правила повтора создания | описание | расхождение двух формулировок идемпотентности (дефект 3) |
| Нижняя граница `init_data` — 16 символов | SessionCreateRequest | расхождение с текстом спецификации (дефект 4) |

Пути, имена операций, коды ошибок и существующие поля не изменяются.

## 4. Модульная структура

Модули: identity, catalog, workspace, documents, exports, audit, telemetry, outbox.
Модули не импортируют друг друга; связи — через порты, объявленные потребителем и связанные в composition root.
Общий технический код — `internal/platform` (ADR-026).

## 5. Порядок реализации

1. Модульный каркас и перенос существующих решений v1.0.0.
2. Схема core без таблицы напоминаний, добавление outbox и версии участий.
3. Модули identity, catalog, workspace, documents.
4. Outbox и relay, синхронная попытка доставки.
5. Клиент reminders для чтения плана, поле `reminders_state`.
6. Exports, audit, telemetry.
7. Тесты: контрактные по OpenAPI, интеграционные с PostgreSQL, тесты relay с двойником reminders.

## 6. Definition of Done

Публичный API соответствует OpenAPI 1.1.0; событие пишется в одной транзакции с изменением;
relay доставляет и подтверждает события; отказ reminders-service не ломает работу с документами;
поле `reminders_state` отражает фактическое состояние; тесты проходят на реальной PostgreSQL.

## 7. Состояние на 24.09.2026 (после реализации core-service)

Задание выполнено: core-service реализован, проверен автономно и в связке с reminders-service
и bot-service. Отчёт — `docs/implementation/THIRD_SERVICE_REPORT.md`, интеграция —
`docs/implementation/integration-report.md` (часть 2). Definition of Done раздела 6 закрыт:
публичный API соответствует OpenAPI 1.1.0, событие пишется в одной транзакции с изменением,
relay доставляет и подтверждает события, отказ reminders-service не ломает работу с документами,
`reminders_state` отражает фактическое состояние, тесты проходят на реальной PostgreSQL.

Следующий этап — интерфейс Mini App (`docs/handoffs/frontend-miniapp.md`).

## 8. Состояние на 23.09.2026 (после реализации bot-service)

Реализованы и проверены вместе **reminders-service** и **bot-service**: отчёты —
`docs/implementation/SECOND_SERVICE_REPORT.md` и `docs/implementation/integration-report.md`.
Разработчику core-service важно следующее.

| Что | Где | Почему важно для core |
|---|---|---|
| Приём событий работает и проверен | `IngestService.ApplyEvents`, `GetIngestState` | relay core отправляет события ровно в этом формате; исходы `APPLIED`, `DUPLICATE`, `STALE`, `REJECTED` уже различаются |
| Сквозной пример подачи событий | `test/e2e/main.go`, функция `baseEvents` | готовый образец пакета из трёх событий (организация, участие, документ) с версиями агрегатов |
| Общий каркас без предметных метрик | `internal/platform/metrics` + `services/<svc>/internal/app/metrics.go` | метрики core объявлять в самом сервисе и регистрировать через `Registry.MustRegister` |
| Раскладка слоёв | `services/bot`, `services/reminders`: `internal/{domain,ports,app,adapters,config}` | та же раскладка ожидается в core; правила закреплены тестами `internal/archtest` |
| Роли и схемы PostgreSQL | `deploy/postgres/init/01-init.sh`, миграция прав на `goose_db_version` | для core нужна такая же вторая миграция, снимающая права на историю миграций с `core_app` |
| Предел регулярных выражений PostgreSQL | дефект D-3 в отчёте | в CHECK нельзя использовать `{n,m}` с `m > 255`; длину проверять через `char_length` |
| Детерминированный `not_after` | замечание §4.2 отчёта об интеграции | bot сравнивает содержимое запроса по хешу: при одинаковом ключе идемпотентности и разном `not_after` возвращается `ALREADY_EXISTS` |
| Профиль бота и диплинки | `GetBotProfile` | core получает `open_app_link_template` и подставляет `payload`, а не собирает ссылку сам |
| Запуск связки для разработки | `bash test/e2e/run.sh`, `make e2e-services` | поднимает reminders и bot на одной PostgreSQL; core подключается к ним теми же адресами |
| Проверка развёртывания без Docker | `python3 scripts/check_compose.py` | добавляя сервис core в compose, прогнать проверку: она контролирует роли, зависимости и сетевые псевдонимы |

Незакрытые задачи дорожки A, которые не блокируют core: MAX-01 (реальные тела событий webhook),
MAX-02 (проверка кнопки `open_app`), R-5 в части проверки режима `live` на стенде MAX.
