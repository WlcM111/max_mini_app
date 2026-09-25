# ADR-012. Observability

**Вопрос.** Как диагностировать систему без отдельного стека логирования?

**Требования.** Структурированные логи с полями timestamp, severity, service, version, request_id, trace_id, span_id, operation, duration, result, error_code; метрики; health checks liveness/readiness; без Elasticsearch/ClickHouse.

**Решение.** Логи — `log/slog` JSON в stdout, сбор — `docker compose logs` (драйвер `json-file`, ротация 10 МБ × 5). Метрики — Prometheus client, `/metrics` на admin-порту `:8081`; Prometheus — профиль `monitoring`. Трассировка — OpenTelemetry SDK: идентификаторы trace/span создаются всегда и пишутся в логи, распространяются через gRPC-метаданные (otelgrpc); экспорт OTLP включается переменной `OTEL_EXPORTER_OTLP_ENDPOINT` (по умолчанию выключен). Health: `/healthz` (процесс жив), `/readyz` (ping БД 500 мс; для `bot` в режиме live — ещё загружен профиль бота не требуется). Схема полей и метрики — [observability.md](../architecture/observability.md).

**Основания.** Минимум компонентов; все поля доступны через `docker compose logs | jq`.

**Недостатки.** Нет полнотекстового поиска и долговременного хранения логов.

**Проверка.** T-NFR-OBS: каждая строка лога API содержит обязательные поля; `/metrics` отдаёт перечисленные метрики.

**Условия пересмотра.** Больше одного хоста — централизованный сбор логов.
