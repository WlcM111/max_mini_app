# Модель данных reminders-service

Версия 2.0.0. Нормативный DDL — `services/reminders/migrations/00001_init.sql`.

## 1. Матрица переноса данных

| Исходная таблица (v1.0.0) | Исходный владелец | Новый владелец | Изменение | Связи | Миграция | Критерий проверки |
|---|---|---|---|---|---|---|
| core.reminders | core | reminders (reminders.reminders) | перенос в отдельную схему, добавлены organization_id, attempts, next_attempt_at, last_error_code, bot_notification_id | FK на reminders.documents вместо core.document_periods | новая схема создаётся с нуля (пользовательских данных нет) | TestIngestBuildsPlanFromCoreEvents |
| core.documents | core | core (источник истины) + reminders.documents (проекция) | проекция хранит название, текущий период, версию | FK внутри схемы | заполняется событиями | TestRenewalCancelsOldPeriodAndPlansNew |
| core.document_periods | core | core + поля period_id, valid_from, valid_until в проекции | текущий период хранится в строке документа | — | события | TestRenewalCancelsOldPeriodAndPlansNew |
| core.document_reminder_offsets | core | core + reminders.document_offsets | проекция множества отступов | FK на reminders.documents | события | TestIngestBuildsPlanFromCoreEvents |
| core.organizations | core | core + reminders.organizations | проекция названия и пояса | — | события | TestTimezoneChangeReplansOrganization |
| core.memberships | core | core + reminders.members | проекция настроек уведомлений и max_user_id | — | события | TestNotificationSettingsChangeCancelsPlan |
| core.accounts | core | core | в reminders проекции нет; используется account_id и max_user_id из событий участия | — | — | TestMemberRemovalAndAccountDeletionCancelReminders |
| — | — | reminders.inbox_events | новая таблица дедупликации | — | новая | TestIngestIsIdempotentByEventID |
| — | — | core.outbox_events | новая таблица (реализуется вместе с core-service) | — | новая | задание core-service |

## 2. Таблицы reminders

| Таблица | Назначение | Ключ | Основные ограничения |
|---|---|---|---|
| inbox_events | журнал принятых событий | event_id | outcome ∈ (applied, stale, rejected); aggregate_type ∈ (organization, membership, document, account) |
| organizations | проекция организации | organization_id | название 1..100, пояс 1..64, version ≥ 0 |
| members | проекция участия | (organization_id, account_id) | account_kind ∈ (max, review); согласованность kind и max_user_id; notify_local_minutes 0..1439 |
| documents | проекция документа | document_id | название 1..200; valid_until ≥ valid_from |
| document_offsets | отступы документа | (document_id, days_before) | days_before 0..365; каскад при удалении документа |
| reminders | план напоминаний | id; уникальный (period_id, account_id, days_before) | status ∈ (planned, handed_off, cancelled, skipped); handed_off ⇔ handed_off_at IS NOT NULL |

## 3. Индексы и основные запросы

| Индекс | Запрос |
|---|---|
| reminders_due_idx (next_attempt_at) WHERE status='planned' | захват наступивших напоминаний |
| reminders_account_planned_idx (account_id, document_id) WHERE status='planned' | ближайшие напоминания для реестра |
| reminders_doc_idx (document_id) | план документа, перепланирование |
| reminders_org_idx (organization_id) WHERE status='planned' | отмена при изменениях организации |
| reminders_updated_idx (updated_at) WHERE status<>'planned' | очистка по сроку хранения |
| inbox_events_received_idx (received_at) | очистка журнала |
| inbox_events_aggregate_idx (aggregate_type, aggregate_id, aggregate_version DESC) | диагностика доставки |
| members_account_idx (account_id) | удаление аккаунта |
| documents_org_idx (organization_id) | перепланирование организации |

## 4. Сроки хранения

| Данные | Срок | Параметр |
|---|---|---|
| Журнал входящих событий | 7 суток | REMINDERS_INBOX_TTL |
| Напоминания в терминальных состояниях | 30 суток | REMINDERS_FINALIZED_TTL |
| Проекции | пока существует источник в core | — |

## 5. Права доступа

| Роль | Права |
|---|---|
| reminders_migrator | владелец схемы, DDL, запись истории миграций |
| reminders_app | SELECT, INSERT, UPDATE, DELETE на таблицах данных; только SELECT на goose_db_version |
| core_app, bot_app | прав на схему reminders не имеют |
