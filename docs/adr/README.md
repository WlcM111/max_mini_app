# Architecture Decision Records

Версия 1.0.0 · 20.09.2026. Статус всех решений — «принято». Изменение решения — новый ADR со ссылкой «заменяет ADR-NNN» и согласие владельца затронутого компонента.

Формат каждого ADR: вопрос → требования → варианты → решение → основания → недостатки → последствия → проверка → условия пересмотра.

| ID | Решение | Владелец |
|---|---|---|
| [ADR-001](ADR-001-product-scope.md) | Продукт «Вовремя»: сроки документов малого бизнеса, границы MVP | B |
| [ADR-002](ADR-002-service-boundaries.md) | Два Go-сервиса: `core` и `bot` | B |
| [ADR-003](ADR-003-frontend-framework.md) | React 18 + TypeScript + Vite + MAX UI | A |
| [ADR-004](ADR-004-public-http-api.md) | Публичный REST/JSON API в `core`, OpenAPI 3.1, problem+json | B |
| [ADR-005](ADR-005-grpc.md) | gRPC core → bot, buf, сгенерированный код в репозитории | A |
| [ADR-006](ADR-006-data-ownership.md) | Один PostgreSQL, схемы `core` и `bot`, раздельные роли | B |
| [ADR-007](ADR-007-max-authorization.md) | Авторизация по подписи initData в `core` производным ключом | A |
| [ADR-008](ADR-008-max-interaction.md) | Взаимодействие с MAX только через `bot`: webhook, собственный клиент Bot API | A |
| [ADR-009](ADR-009-session-storage.md) | Непрозрачный серверный токен сессии, хеш в PostgreSQL | B |
| [ADR-010](ADR-010-long-running-operations.md) | Напоминания: план в БД, планировщик с lease, очередь доставки в bot | B |
| [ADR-011](ADR-011-frontend-delivery-edge.md) | Caddy: TLS, статика, прокси, один origin | B |
| [ADR-012](ADR-012-observability.md) | slog JSON, Prometheus, OpenTelemetry-идентификаторы | B |
| [ADR-013](ADR-013-compose-deployment.md) | Docker Compose на одном хосте, профили, одноразовые миграции | B |
| [ADR-014](ADR-014-postgres-access.md) | pgx + sqlc + goose | B |
| [ADR-015](ADR-015-configuration-secrets.md) | Переменные окружения, dev-значения `devonly_`, защита prod | B |
| [ADR-016](ADR-016-review-mode.md) | Токены проверяющих через CLI и демо-данные | B |

## Архитектура 2.0.0 (три сервиса)

| ADR | Решение | Статус |
|---|---|---|
| ADR-017 | Переход на три микросервиса (вариант C) | принято, заменяет ADR-002 |
| ADR-018 | Границы reminders-service | принято |
| ADR-019 | Владение данными в трёх сервисах | принято, уточняет ADR-006 |
| ADR-020 | Модель согласованности между core и reminders | принято, заменяет часть ADR-010 |
| ADR-021 | Транзакционный outbox в core и inbox в reminders | принято |
| ADR-022 | Локальные проекции core-данных в reminders | принято |
| ADR-023 | Чтение плана для публичного API | принято |
| ADR-024 | Идемпотентность межсервисных операций | принято |
| ADR-025 | Чистая слоистая архитектура внутри сервисов | принято |
| ADR-026 | Общий технический каркас internal/platform | принято |
| ADR-027 | Стратегия проверки reminders-service | принято |
| ADR-028 | Фоновые операции reminders-service | принято |
| ADR-029 | Сетевая изоляция межсервисного gRPC | принято, исправляет ADR-005 |
| ADR-030 | Идентификаторы трассировки без OTel SDK | принято, уточняет ADR-012 |
| ADR-031 | Запрет production-заглушек | принято |
| ADR-032 | Языковой ассистент ввода данных (GigaChat) | принято |

### Изменённый статус прежних решений

| ADR | Новый статус |
|---|---|
| ADR-002 «Два сервиса» | заменён ADR-017 |
| ADR-005 «gRPC core → bot» | действует; сетевая часть уточнена ADR-029; добавлен вызов reminders → bot |
| ADR-006 «Схемы и роли PostgreSQL» | действует; расширен схемой reminders (ADR-019) |
| ADR-010 «Планировщик в core» | в части размещения плана заменён ADR-017 и ADR-020; алгоритм аренды сохранён (ADR-028) |
| ADR-012 «Наблюдаемость» | действует; для reminders-service уточнён ADR-030 |
| ADR-014 «Доступ к PostgreSQL» | действует; для reminders-service SQL написан вручную поверх pgx (см. docs/handoffs/reminders-service.md §5) |

Одновременно действующих противоречащих решений нет: каждое прежнее решение,
затронутое переходом на вариант C, помечено выше как заменённое или уточнённое.
