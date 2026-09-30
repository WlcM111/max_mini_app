# Наблюдаемость

Версия 1.0.0 · 20.09.2026. Решение — ADR-012.

## 1. Логи

Формат — JSON в stdout, одна строка на событие, `log/slog`. Обязательные поля каждой строки о запросе или операции:

| Поле | Тип | Пример | Источник |
|---|---|---|---|
| `time` | RFC 3339 с мс, UTC | `2026-09-24T09:00:03.120Z` | slog |
| `level` | DEBUG/INFO/WARN/ERROR | `INFO` | slog |
| `service` | `core` \| `bot` | `core` | конфигурация |
| `version` | строка | `1.0.0` | `-ldflags` при сборке |
| `request_id` | строка | из `X-Request-Id` edge или новый UUID | middleware |
| `trace_id`, `span_id` | hex | `4bf92f35…`, `00f067aa…` | OpenTelemetry |
| `operation` | operationId, имя RPC или задачи | `createDocument`, `EnqueueNotification`, `scheduler.batch` | обработчик |
| `duration_ms` | число | `12.4` | middleware |
| `result` | `ok` \| `client_error` \| `server_error` | `ok` | middleware |
| `error_code` | `ErrorCode` или код gRPC/MAX | `CONFLICT_VERSION` | обработчик |
| `http_status` | число | `409` | HTTP |
| `account_id` | public UUID аккаунта | — | после аутентификации |

Запрещено логировать: `init_data`, `hash`, токены сессий, приглашений и экспорта, токен бота, секрет webhook, тексты сообщений пользователей боту, номера и заметки документов.

## 2. Метрики

| Метрика | Тип | Метки | Сервис |
|---|---|---|---|
| `vovremya_http_requests_total` | counter | `operation`, `method`, `code` | core, bot |
| `vovremya_http_request_duration_seconds` | histogram (0,005…10) | `operation`, `method` | core, bot |
| `vovremya_http_inflight_requests` | gauge | — | core |
| `vovremya_session_create_total` | counter | `result` | core |
| `vovremya_client_events_total` | counter | `name`, `platform`, `code` | core |
| `vovremya_grpc_client_requests_total` | counter | `method`, `code` | core |
| `vovremya_grpc_server_handled_total` | counter | `method`, `code` | bot |
| `vovremya_grpc_duration_seconds` | histogram | `method`, `side` | core, bot |
| `vovremya_db_pool_acquired_conns` | gauge | — | core, bot |
| `vovremya_db_query_duration_seconds` | histogram | `query` (имя sqlc) | core, bot |
| `vovremya_reminders_due_backlog` | gauge | — | core (строк `planned` с `next_attempt_at ≤ now`) |
| `vovremya_reminders_handoff_total` | counter | `result` (`handed_off`, `retry`, `skipped`) | core |
| `vovremya_outbound_queue_depth` | gauge | `status` | bot |
| `vovremya_outbound_send_total` | counter | `result` (`sent`, `retry`, `failed`, `expired`, `timeout`) | bot |
| `vovremya_max_api_requests_total` | counter | `endpoint`, `status_class` | bot |
| `vovremya_max_api_request_duration_seconds` | histogram | `endpoint` | bot |
| `vovremya_max_webhook_updates_total` | counter | `update_type`, `result` | bot |
| `vovremya_max_subscription_ok` | gauge 0/1 | — | bot |
| `vovremya_app_errors_total` | counter | `code` | core, bot |

Метки с неограниченным числом значений (ID пользователей, документов) запрещены; `code` клиентских событий берётся из белого списка, прочие значения → `other`.

## 3. Health checks

| Endpoint | Порт | Смысл | Логика |
|---|---|---|---|
| `/healthz` | 8081 | liveness: процесс отвечает | всегда 200, пока не началась остановка |
| `/readyz` | 8081 | readiness: можно принимать трафик | 200, если ping БД ≤ 500 мс и не идёт остановка; иначе 503. Для core доступность bot не входит в readiness (ADR-002) |
| `grpc.health.v1.Health/Check` | 9090 | готовность gRPC bot | `SERVING` при готовности БД |

## 4. Сигналы для проверки (PromQL, профиль monitoring)

| Сигнал | Запрос | Порог |
|---|---|---|
| Ошибки API | `sum(rate(vovremya_http_requests_total{code=~"5.."}[5m])) / sum(rate(vovremya_http_requests_total[5m]))` | > 1 % |
| Задержка API | `histogram_quantile(0.95, sum by (le) (rate(vovremya_http_request_duration_seconds_bucket[5m])))` | > 0,3 с |
| Отставание напоминаний | `vovremya_reminders_due_backlog` | > 1 000 за 10 мин |
| Очередь бота | `vovremya_outbound_queue_depth{status="retry_wait"}` | > 500 |
| Подписка webhook | `vovremya_max_subscription_ok` | 0 дольше 15 мин |
| Неудачные запуски | `rate(vovremya_session_create_total{result!="ok"}[15m])` | рост в 3 раза к прошлой неделе |

## 5. Трассировка и телеметрия клиента

Edge ставит `X-Request-Id`; core формирует идентификаторы W3C Trace Context (`traceparent`, пакет `internal/platform/tracectx`) и передаёт их в bot и reminders через gRPC-метаданные — trace_id и span_id попадают в логи всех сервисов. Экспорт спанов не выполняется (ADR-030). Мини-приложение отправляет `POST /client-events`: `bootstrap_completed` (длительность), `bootstrap_failed` (код), `bridge_error` (метод и код), `api_error_shown` (код ошибки) — это единственный способ узнать о сбоях запуска на устройствах пользователей.
