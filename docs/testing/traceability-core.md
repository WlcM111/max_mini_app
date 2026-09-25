# Матрица трассируемости core-service (этап 3)

Версия 1.0.0 · 24.09.2026. Источники требований: `docs/requirements/requirements-registry.md`,
`docs/handoffs/core-service.md` (v1.0.0), `docs/handoffs/core-service-v2.md` (v2.0.0),
`openapi.yaml` 1.1.0. Тесты: `services/core/internal/**/*_test.go`, `services/core/test/integration`,
`test/e2e`.

## 1. Требования, полностью реализуемые core-service

| ID | Требование | Реализация | Проверка | Статус |
|---|---|---|---|---|
| FR-01 | Идентификация пользователя по данным запуска MAX | `adapters/maxlaunch/verifier.go`, `app/sessions.go` | `verifier_test.go` (TV-1…TV-6), `TestSessionStartTargetAndMe` | выполнено |
| FR-02 | Серверные непрозрачные сессии, выход | `app/sessions.go`, `adapters/postgres/accounts.go` | `TestSessionStartTargetAndMe`, `TestUnauthenticatedAndMalformedRequests` | выполнено |
| FR-03 | Справочник видов деятельности, регионов, признаков, типов документов | `domain/catalog.go`, `adapters/postgres/catalog.go` | `TestCatalogAndSuggestions` (ETag, 304) | выполнено |
| FR-04 | Организация: создание, профиль, изменение, удаление | `app/organizations.go` | `TestOrganizationLifecycleAndIdempotency` | выполнено |
| FR-05 | Подсказки типовых документов по профилю | `domain/catalog.go` (`Applicable`), `app/organizations.go` | `TestCatalogApplicability`, `TestCatalogAndSuggestions` | выполнено |
| FR-06 | Реестр документов: сортировка по сроку, фильтры, поиск, страницы | `app/documents.go`, `adapters/postgres/documents.go` | `TestDocumentsLifecycleAndRegistry` | выполнено |
| FR-07 | Карточка документа, история периодов, шаги продления | `app/documents.go` (`GetDocument`, `decorate`) | `TestDocumentsLifecycleAndRegistry` | выполнено |
| FR-08 | Добавление документов, в том числе пакетом при онбординге | `app/documents.go` (`CreateDocuments`) | `TestDocumentsBatchIsAllOrNothing` | выполнено |
| FR-09 | Продление документа новым периодом | `app/documents.go` (`RenewDocument`) | `TestDocumentsLifecycleAndRegistry` | выполнено |
| FR-10 | Участники и роли, приглашения по ссылке | `app/members.go`, `app/invites.go` | `TestRolesAndPermissions`, `TestInvitesLifecycle` | выполнено |
| FR-11 | Настройки напоминаний участника | `app/members.go` | `TestNotificationSettingsAndClientEvents` | выполнено |
| FR-14 | Экспорт сроков в календарь (ICS) по одноразовой ссылке | `app/exports.go`, `adapters/ics/calendar.go` | `TestCalendarExport` | выполнено |
| FR-15 | Удаление аккаунта и принадлежащих организаций | `app/accounts.go` (`DeleteMe`) | `TestAccountDeletionRemovesOwnedData` | выполнено |
| FR-17 | Технические события клиента | `app/accounts.go` (`AcceptClientEvents`) | `TestNotificationSettingsAndClientEvents` | выполнено |
| FR-18 | Демонстрационные данные | `app/seed.go`, CLI `seed-demo` | сценарий README §12, e2e-стенд | выполнено |
| FR-19 | Доступ проверяющих без клиента MAX | `app/sessions.go` (`IssueReviewToken`), CLI `review-token` | ручная проверка по README, код возврата 2 без демо-данных | выполнено |
| BR-02 | Квоты продукта (20/500/30/50/5) | `domain/quotas.go`, сценарии app | `TestOrganizationQuota` | выполнено |
| BR-03 | Состояния срока документа | `domain/deadline.go` | `TestStatusOfBoundaries` | выполнено |
| BR-04 | Правила ролей: владелец, редактор, наблюдатель | `domain/role.go`, `app/app.go` (`authorize`) | `TestRoleAllows`, `TestRolesAndPermissions` | выполнено |
| NFR-04 | Идемпотентность изменяющих операций по клиентским UUID | `app/organizations.go`, `documents.go`, `invites.go` | `TestOrganizationLifecycleAndIdempotency`, продление в `TestDocumentsLifecycleAndRegistry` | выполнено |
| NFR-05 | Конкурентные изменения не теряются | `FOR UPDATE`, `expected_version` | `TestConcurrentUpdatesProduceSingleConflict` (20 прогонов) | выполнено |
| NFR-06 | Ошибки в формате RFC 9457 с кодами контракта | `adapters/httpapi/problem.go` | все API-тесты (коды `VALIDATION_FAILED`, `CONFLICT_*`, `QUOTA_EXCEEDED`, `INVITE_*`, `LINK_GONE`) | выполнено |
| NFR-07 | Ограничения частоты и одновременных запросов | `adapters/httpapi/ratelimit.go`, `server.go` | `config_test.go`, ручная проверка пределов | выполнено |
| NFR-10 | Наблюдаемость: логи, метрики, health | `app/metrics.go`, `internal/platform/adminhttp` | `/readyz` и `/metrics` в e2e-стенде | выполнено |
| NFR-11 | Корректное завершение работы | `cmd/core/main.go` (`lifecycle.Runner`) | остановка сервисов в e2e-стенде | выполнено |
| NFR-12 | Хранение секретов только в виде хешей | `sha256` токенов сессий, приглашений, ссылок | схема БД, `TestInvitesLifecycle` | выполнено |
| NFR-13 | Очистка устаревших данных | `app/accounts.go` (`RetentionOnce`) | `TestRetentionRemovesDeliveredEvents` | выполнено |

## 2. Требования, затрагивающие два и более сервиса

| ID | Требование | Обязанность core | Обязанность соседа | Точка взаимодействия | Проверка |
|---|---|---|---|---|---|
| FR-12 | Напоминания о приближающемся сроке | сформировать событие изменения в одной транзакции и доставить | reminders строит план, bot доставляет сообщение | `ApplyEvents`, `EnqueueNotification` | e2e `core` (шаги 4–7) |
| FR-13 | Показ ближайшего напоминания пользователю | запросить план, показать `next_reminder_at` | reminders отвечает `GetNextReminders` | `ReminderQueryService` | e2e `core` (шаг 6) |
| FR-16 | Сообщение владельцу о новом участнике | сформировать текст и поставить задание с ключом `mj:` | bot доставляет сообщение | `EnqueueNotification` | `TestInvitesLifecycle` |
| AC-05 | Пользователь не видит непересчитанный план как актуальный | поле `reminders_state`, синхронная попытка доставки | reminders подтверждает приём | outbox + `ApplyEvents` | `TestPendingStateWhenDeliveryLags`, e2e `core-pending`, `core-recovered` |
| NFR-02 | Отказ соседнего сервиса не ломает основной сценарий | деградация чтения и предупреждение в ответе | — | тайм-ауты 2 с | `TestRemindersUnavailableDoesNotBlockDocuments`, `TestBotUnavailableDegradesGracefully`, e2e `core-bot-down` |

## 3. Требования, зависящие от следующего этапа

| ID | Требование | Что готово в core | Чего не хватает |
|---|---|---|---|
| FR-20…FR-24 (интерфейс) | экраны онбординга, реестра, карточки, участников, настроек | все операции API, тексты ошибок, состояния срока и плана | интерфейс Mini App (`docs/handoffs/frontend-miniapp.md`) |
| MAX-01, MAX-02 | реальные тела webhook и кнопка `open_app` | шаблон диплинка берётся из профиля бота | токен MAX и стенд, дорожка A |
