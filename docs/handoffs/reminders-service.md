# Задание: reminders-service (реализовано)

Версия 2.0.0. Документ самодостаточен: для работы с сервисом не требуется переписка или иные источники.
Нормативные приложения внутри архива: `api/proto/vovremya/reminders/v1/*.proto`,
`api/proto/vovremya/bot/v1/messaging.proto`, `services/reminders/migrations/00001_init.sql`,
`docs/architecture/consistency.md`, ADR-017…ADR-031.

## 1. Контекст продукта

«Вовремя» — мини-приложение MAX для контроля сроков документов малого бизнеса.
Пользователь ведёт реестр документов организации; система заранее напоминает в чате бота
о приближении даты окончания срока. reminders-service отвечает за «заранее»: что, кому и когда напомнить.

## 2. Назначение сервиса

Принимает изменения предметных данных из core-service, хранит их локальные проекции,
строит и перестраивает план напоминаний, по расписанию передаёт наступившие напоминания
в bot-service и отвечает на запросы плана для публичного API.

## 3. Границы ответственности

Принадлежит сервису:
- план напоминаний: ключ (период документа, получатель, отступ в днях), момент отправки, состояние;
- расписание и захват напоминаний с арендой, повторы, отмена, пропуск;
- текст напоминания и payload кнопки диплинка;
- локальные проекции организаций, участий, документов и отступов;
- журнал входящих событий и его очистка.

Не принадлежит:
- источник истины по документам, организациям, участникам (core-service);
- доставка в MAX, состояние получателей, токен бота (bot-service);
- публичный HTTP API (core-service);
- решение о видимости `next_reminder_at` в интерфейсе (core-service, ADR-023).

## 4. Стек и зависимости

Go 1.27, PostgreSQL 18 (проверено на 16), gRPC, goose (миграции), pgx v5, Prometheus client, caarlos0/env.
Внешние сервисы: bot-service (исходящий gRPC). Брокеры сообщений и кэши не используются.

## 5. Структура каталогов

```
services/reminders/
  cmd/reminders/main.go              composition root: serve | migrate up|down | healthcheck
  migrations/00001_init.sql          схема reminders
  migrations/embed.go                встраивание миграций в бинарник
  internal/domain/                   предметная модель (только stdlib)
    projection.go  организация, участие, документ, инварианты
    plan.go        BuildPlan — чистая функция построения плана
    reminder.go    состояния, переходы, ключ идемпотентности, просрочка
    backoff.go     выдержка повторов
    text.go        текст напоминания, склонение дней, payload диплинка
    date.go        календарная дата и перевод в момент времени пояса
    uuid.go        проверка формата идентификаторов
    errors.go      таксономия ошибок домена
  internal/ports/                    интерфейсы, нужные сценариям
  internal/app/                      сценарии
    events.go      модель события и его проверка
    ingest.go      приём событий, идемпотентность, версии
    replan.go      перестроение плана документа и организации
    scheduler.go   захват, проверка состояния, передача в bot, повторы
    query.go       чтение плана и состояния синхронизации
    retention.go   очистка по срокам хранения
  internal/adapters/postgres/        репозитории на параметризованном SQL
  internal/adapters/grpcserver/      входящий gRPC: перевод контракта в сценарии
  internal/adapters/botgrpc/         исходящий gRPC: классификация ошибок
  internal/adapters/clock/           системные часы
  internal/config/                   конфигурация и её проверка
  internal/archtest/                 тест направления зависимостей
  test/testutil/                     изолированная PostgreSQL, часы, двойник bot
  test/integration/                  интеграционные, контрактные и отказные тесты
  test/botdouble/                    управляемый двойник bot-service (dev/test)
  Dockerfile                         образ сервиса
  Dockerfile.botdouble               образ двойника (только dev/test)
```

Правила слоёв (проверяются `internal/archtest`): домен не импортирует адаптеры, сценарии,
порты, генерируемый код, pgx, grpc, net/http, database/sql, encoding/json; сценарии не импортируют адаптеры;
адаптеры не импортируют друг друга; общий каркас не импортирует сервисы.

## 6. Доменная модель и инварианты

| Сущность | Идентификатор | Инварианты |
|---|---|---|
| Organization (проекция) | UUID v4 | название 1..100 после обрезки пробелов; пояс IANA, известен системе; версия монотонна |
| Member (проекция) | (organization_id, account_id) | kind=max требует max_user_id > 0; kind=review требует max_user_id = 0; notify_local_minutes 0..1439 |
| Document (проекция) | UUID v4 | название 1..200; отступов не более 5, значения 0..365 без повторов; valid_until ≥ valid_from |
| Reminder | (period_id, account_id, days_before) | уникальность ключа; handed_off всегда имеет handed_off_at; attempts ≥ 0 |

Переходы состояний напоминания: `planned → handed_off | cancelled | skipped`, `cancelled → planned`.
Состояния `handed_off` и `skipped` терминальны.

Правило построения плана (`BuildPlan`): для документа с текущим периодом и датой окончания
для каждого получателя (участие не прекращено, уведомления включены, аккаунт MAX)
и каждого отступа момент отправки = (дата окончания − отступ) в локальном времени получателя
в поясе организации, приведённое к UTC. Моменты старше `now − grace` не планируются.

## 7. Сценарии

| Сценарий | Вход | Эффект | Ошибки |
|---|---|---|---|
| Apply | событие core | запись в журнал, обновление проекции, перестроение плана | rejected при нарушении инвариантов; stale при устаревшей версии; duplicate при повторе |
| ApplyBatch | до 200 событий | последовательная обработка, исход по каждому | пакет не атомарен |
| GetIngestState | до 500 агрегатов | применённые версии | INVALID_ARGUMENT при неизвестном типе |
| ReplanDocument | идентификатор документа | добавление, обновление и отмена напоминаний | пропуск, если документ или организация отсутствуют в проекции |
| ReplanOrganization | идентификатор организации | перестроение всех документов | — |
| Scheduler.Tick | — | захват пакета, проверка состояния, передача в bot, обновление плана | повтор при временной ошибке, пропуск при отказе контракта |
| NextReminders | получатель, до 200 документов | ближайшее напоминание по документу | INVALID_ARGUMENT при неверном идентификаторе |
| DocumentPlan | документ | план и применённая версия | NOT_FOUND при отсутствии проекции |
| SyncStatus | агрегат, ожидаемая версия | применённая версия и признак синхронности | INVALID_ARGUMENT при неизвестном типе |
| Retention.RunOnce | — | очистка журнала и неактивных напоминаний | — |

## 8. Контракты

Входящие (реализованы): `vovremya.reminders.v1.IngestService` — ApplyEvents, GetIngestState;
`vovremya.reminders.v1.ReminderQueryService` — GetNextReminders, GetDocumentPlan, GetSyncStatus;
`grpc.health.v1.Health` — Check.

Исходящий (используется): `vovremya.bot.v1.MessagingService/EnqueueNotification`
с `kind=NOTIFICATION_KIND_REMINDER`, ключом `rem:<period_id>:<account_id>:<days_before>`,
одной кнопкой с `open_app_payload=doc_<document_id>` и полем `not_after = due_at + grace`.

Соответствие ошибок:

| Код gRPC от bot | Классификация | Действие |
|---|---|---|
| UNAVAILABLE, DEADLINE_EXCEEDED, RESOURCE_EXHAUSTED, ABORTED, INTERNAL, UNKNOWN | повторяемая | attempts+1, повтор с выдержкой |
| INVALID_ARGUMENT, FAILED_PRECONDITION, ALREADY_EXISTS, PERMISSION_DENIED, UNAUTHENTICATED, UNIMPLEMENTED, NOT_FOUND | окончательная | статус skipped с кодом ошибки |

Ошибки сервиса наружу: INVALID_ARGUMENT (нарушение контракта), NOT_FOUND (нет проекции),
CANCELED и DEADLINE_EXCEEDED (отмена вызова), INTERNAL (внутренняя ошибка без раскрытия деталей).

## 9. Схема PostgreSQL

Схема `reminders`, владелец `reminders_migrator`, приложение `reminders_app`.
Таблицы: `inbox_events`, `organizations`, `members`, `documents`, `document_offsets`, `reminders`.
Полный DDL — `services/reminders/migrations/00001_init.sql`. Внешние ключи только внутри схемы.
Роль приложения не имеет прав на изменение `reminders.goose_db_version`.

## 10. Транзакционные границы

- Одно событие — одна транзакция: журнал, проекция и перестроение плана фиксируются вместе.
- Транзакция не удерживается во время исходящих gRPC-вызовов.
- Планировщик работает без длинной транзакции: захват с арендой, затем отдельные обновления.
- Обновления статуса выполняются с условием на текущий статус (защита от устаревшего обработчика).

## 11. Конкурентность

`FOR UPDATE SKIP LOCKED` при захвате, аренда `REMINDERS_SCHEDULER_LEASE`,
ограничение одновременных вызовов bot-service (`REMINDERS_SCHEDULER_CONCURRENCY`),
завершение всех горутин по контексту, отсутствие неограниченных пулов.

## 12. Конфигурация

Полный словарь — `docs/operations/configuration.md`, раздел reminders-service.
Обязательная переменная: `REMINDERS_DATABASE_URL`. Секреты можно передавать файлом через `<ИМЯ>_FILE`.
При `APP_ENV=prod` запуск с dev-значениями запрещён.

## 13. Наблюдаемость

Логи slog JSON с полями service, version, request_id, trace_id, span_id, operation, duration_ms, result.
Секреты и тексты сообщений в логи не попадают.
Метрики: `vovremya_ingest_events_total`, `vovremya_reminders_handoff_total`, `vovremya_reminders_due_backlog`,
`vovremya_reminders_planned`, `vovremya_projection_lag_seconds`, `vovremya_grpc_*`, `vovremya_db_*`.
Эндпоинты: `/healthz`, `/readyz` (пинг БД), `/metrics` на admin-порту.

## 14. Критерии приёмки

| ID | Требование | Проверка |
|---|---|---|
| RS-01 | План строится из событий core | TestIngestBuildsPlanFromCoreEvents |
| RS-02 | Повторная доставка не меняет план | TestIngestIsIdempotentByEventID |
| RS-03 | Устаревшее событие отбрасывается | TestIngestRejectsStaleVersion |
| RS-04 | Некорректное событие отвергается | TestIngestRejectsInvalidEvent, TestGRPCValidationErrors |
| RS-05 | Продление отменяет прежний период и планирует новый | TestRenewalCancelsOldPeriodAndPlansNew |
| RS-06 | Смена пояса пересчитывает план | TestTimezoneChangeReplansOrganization |
| RS-07 | Настройки уведомлений меняют план | TestNotificationSettingsChangeCancelsPlan |
| RS-08 | Исключение участника и удаление аккаунта отменяют напоминания | TestMemberRemovalAndAccountDeletionCancelReminders |
| RS-09 | Удаление документа и организации отменяют напоминания | TestDocumentAndOrganizationDeletionCancelReminders |
| RS-10 | Наступившее напоминание передаётся в bot | TestSchedulerHandsOffDueReminder |
| RS-11 | Временная ошибка вызывает повтор с выдержкой | TestSchedulerRetriesOnUnavailableBot |
| RS-12 | Окончательная ошибка помечает пропуск | TestSchedulerSkipsOnPermanentError |
| RS-13 | Просроченное напоминание не отправляется | TestSchedulerSkipsExpiredReminder |
| RS-14 | Устаревшее состояние не приводит к отправке | TestSchedulerDoesNotSendForDeletedDocument |
| RS-15 | Параллельные обработчики не дублируют отправку | TestSchedulerConcurrentTicksSendOnce |
| RS-16 | Аренда возвращает напоминание после сбоя | TestLeaseReturnsReminderAfterCrash |
| RS-17 | Очистка удаляет неактивные данные | TestRetentionRemovesFinalizedData |
| RS-18 | Запросы плана отвечают контракту | TestGRPCApplyEventsAndQuery, TestQueryServiceReturnsPlan |
| RS-19 | Отмена вызова распространяется | TestGRPCCancellationIsPropagated |
| RS-20 | Исходящий контракт соблюдается, ошибки классифицируются | TestBotGatewayContractOverRealGRPC |
| RS-21 | Роль приложения не меняет историю миграций | TestMigrationsRolePrivileges |
| RS-22 | Нет внешних ключей за пределы схемы | TestSchemaOwnershipHasNoForeignReferences |
| RS-23 | Ключ плана уникален | TestUniqueReminderKey |
| RS-24 | Пользовательские строки не исполняются как SQL | TestSQLInjectionAttemptIsTreatedAsData |
| RS-25 | Направление зависимостей соблюдено | TestLayerDependencies, TestPlatformDoesNotDependOnServices |

## 15. Команды

```sh
# сборка и статические проверки
go build ./...
go vet ./...
gofmt -l services/reminders internal

# тесты (интеграционные требуют доступной PostgreSQL)
export REMINDERS_TEST_DATABASE_URL="postgres://postgres:PASSWORD@127.0.0.1:5432/postgres"
go test ./services/reminders/... ./internal/...

# миграции и запуск без Docker
export REMINDERS_DATABASE_URL="postgres://reminders_app:PASSWORD@127.0.0.1:5432/vovremya?sslmode=disable"
export REMINDERS_MIGRATE_DATABASE_URL="postgres://reminders_migrator:PASSWORD@127.0.0.1:5432/vovremya?sslmode=disable"
go run ./services/reminders/cmd/reminders migrate up
go run ./services/reminders/cmd/reminders serve

# автономный запуск в Docker
cp .env.reminders.example .env.reminders
docker compose -f compose.reminders.yaml --env-file .env.reminders up -d --build
docker compose -f compose.reminders.yaml --env-file .env.reminders ps
curl -fsS http://127.0.0.1:8081/readyz
curl -fsS http://127.0.0.1:8090/messages
docker compose -f compose.reminders.yaml --env-file .env.reminders down -v
```

## 16. Решения, которые исполнитель не меняет самостоятельно

Нормативные `.proto`, формат ключа идемпотентности, схема `reminders` и порядок миграций,
границы сервисов и владение данными, запрет production-заглушек, правила слоёв.
Изменение любого из них оформляется новым ADR.

## 17. Definition of Done

Сборка без ошибок; `go vet` и `gofmt` чисты; все тесты раздела 14 проходят на реальной PostgreSQL;
обязательная функциональность не содержит TODO и заглушек; документация соответствует коду;
сервис поднимается автономно и отвечает на `/readyz`.
