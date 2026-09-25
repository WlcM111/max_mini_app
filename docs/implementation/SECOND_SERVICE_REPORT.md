# Отчёт реализации второго микросервиса

Дата: 23.09.2026. Версия архитектуры: 2.0.0. Предыдущий этап: [FIRST_SERVICE_REPORT.md](FIRST_SERVICE_REPORT.md).

## 1. Выбранный сервис и обоснование

Выбран **bot-service**.

| Основание | Источник |
|---|---|
| Прямое указание предыдущего этапа: «Реализовать bot-service, дорожка A» | FIRST_SERVICE_REPORT.md §9, п. 1 |
| Задача R-1 (21–23.09, владелец A) и R-5 плана второй версии | docs/plan/team-plan-v2.md |
| Задачи BOT-01…BOT-03 исходного плана | docs/plan/team-plan.md |
| Контракт `MessagingService` зафиксирован в версии 1.0.0 и уже вызывается реализованным reminders-service: реализация проверяет контракт с обеих сторон | api/proto/vovremya/bot/v1/messaging.proto |
| bot-service не зависит от core-service: его можно завершить целиком, а не частично | docs/handoffs/bot-service-v2.md §2 |
| Без bot-service интеграция двух сервисов проверяется только двойником; с ним — настоящими процессами | требование этапа |

Отвергнут core-service: он зависит от контрактов reminders и bot, его объём (HTTP API, каталог,
сессии, outbox) не укладывается в один этап целиком, а отложенная проверка связки оставляет
контракт bot непроверенным до последней недели перед сдачей.

## 2. Реализованная функциональность

| Область | Состав |
|---|---|
| gRPC MessagingService | `EnqueueNotification`, `GetNotificationStatus`, `GetRecipientStatus`, `GetBotProfile` — 4 из 4 RPC контракта, плюс стандартный Health |
| Идемпотентность | ключ `idempotency_key`, SHA-256 детерминированной сериализации запроса с очищенным ключом; повтор — `duplicate=true`, смена содержимого — `ALREADY_EXISTS` |
| Очередь доставки | состояния `queued → sending → sent/retry_wait/failed/expired`, захват `FOR UPDATE SKIP LOCKED` с арендой, условные финальные обновления по `(id, status, locked_until)` |
| Доставка в MAX | `POST /messages` с текстом без разметки, кнопки `link` или `open_app`, идентификатор `message.body.mid` |
| Классификация ответов | `400 → max_4xx` (окончательно), `401 → max_401`, `403/404 → recipient_unreachable`, `429 → max_429` с учётом `Retry-After`, `5xx → max_5xx`, таймаут и сетевая ошибка — повтор |
| Повторы | выдержка U(0; min(10 мин, 5 с·2ⁿ⁻¹)), предел попыток, снижение скорости вдвое на минуту после 429 |
| Ограничение скорости | глобальный токен-бакет 20 rps (официальный предел 30, F-43) и не чаще одного сообщения в 600 мс одному получателю (F-50) |
| Проверки перед отправкой | истёкший `not_after` → `expired`, остановивший бота получатель → `failed/recipient_stopped`, отсутствие профиля для диплинка → возврат в очередь без расхода попытки |
| Webhook | `POST /max/webhook`: секрет сравнивается за постоянное время, предел тела, толерантный разбор, дедупликация по SHA-256 тела, одна транзакция на событие, ответ `200 {}` или `503` |
| Состояния получателей | `unknown/active/muted/stopped/unreachable` по событиям MAX с проверкой порядка по времени события (D8-4) |
| Ответы пользователю | приветствие на `bot_started`, справка на `/help` и `/start`, подсказка на произвольный текст не чаще раза в 10 минут; ответы ставятся в ту же очередь с ключом `reply:<hex>` и сроком 1 час |
| Профиль бота | загрузка `GET /me` с повтором, установка команд меню, шаблон диплинка `https://max.ru/<ник>?startapp={payload}` |
| Подписка webhook | проверка и восстановление подписки в режиме `live` по расписанию (spec §9) |
| Режим stub | локальный канал вместо MAX: кольцевой буфер на 500 сообщений и `GET /debug/stub/messages` на admin-порту; запрещён при `APP_ENV=prod` |
| Фоновые задания | сборщик истёкших аренд, монитор глубины очереди (backpressure → `RESOURCE_EXHAUSTED`), очистка по срокам хранения |
| Эксплуатация | конфигурация с проверкой пределов, логи JSON с корреляцией, метрики Prometheus, `/healthz`, `/readyz`, `/metrics`, graceful shutdown, миграции up/down |

## 3. Созданные и изменённые файлы

| Группа | Путь | Действие |
|---|---|---|
| Домен | services/bot/internal/domain/{errors,notification,delivery,backoff,recipient,update,texts,profile}.go | создано |
| Порты | services/bot/internal/ports/ports.go | создано |
| Сценарии | services/bot/internal/app/{messaging,delivery,webhook,profile,background,metrics}.go | создано |
| Адаптеры | services/bot/internal/adapters/{postgres,maxapi,webhook,grpcserver,ratelimit,clock}/ | создано |
| Конфигурация | services/bot/internal/config/config.go | создано |
| Точка входа | services/bot/cmd/bot/main.go | создано |
| Миграции | services/bot/migrations/{00002_migration_history_privileges.sql,embed.go} | создано |
| Тесты | services/bot/internal/**/*_test.go, services/bot/test/{testutil,integration} | создано |
| Сквозная проверка | test/e2e/{main.go,run.sh} | создано |
| Проверка развёртывания | scripts/check_compose.py | создано |
| Нормативная DDL | services/bot/migrations/00001_init.sql | исправлено (дефект D-3) |
| Каркас | internal/platform/metrics/metrics.go, internal/platform/adminhttp/adminhttp.go | изменено |
| reminders | services/reminders/internal/config/config.go, internal/app/{metrics.go,ingest.go,scheduler.go,retention.go}, cmd/reminders/main.go | изменено (дефекты A-1, B-1) |
| Развёртывание | compose.yaml, Makefile | изменено (дефект D-6, снятие sqlc для bot) |
| Документация | docs/operations/{configuration.md,repository-tree.md}, docs/testing/test-strategy.md, docs/requirements/traceability-matrix.md, docs/plan/team-plan-v2.md, README.md | изменено |

Объём: 4 238 строк production-кода Go и 2 914 строк тестов в services/bot; 62 тестовые функции и 24 подтеста.

## 4. Исправленные дефекты

| Код | Где | Суть | Как обнаружен | Исправление |
|---|---|---|---|---|
| A-1 | services/reminders/internal/config/config.go | `REMINDERS_BOT_RPC_TIMEOUT` по умолчанию 5 с против 2 с в нормативном контракте: планировщик держал аренду дольше, чем допускает контракт | сверка кода с docs/contracts/grpc-contract.md §1 | значение 2 с, закреплено тестом `TestBotRPCTimeoutMatchesContract` |
| B-1 | internal/platform/metrics/metrics.go | предметные метрики reminders объявлялись в общем каркасе: bot-service экспортировал бы чужие метрики с нулями | обзор каркаса перед его повторным использованием | метрики сервиса вынесены в `services/<svc>/internal/app/metrics.go`, каркас получил `MustRegister`; закреплено `TestRegistryHasOnlyTechnicalMetrics` |
| D-3 | services/bot/migrations/00001_init.sql | CHECK `open_app_payload ~ '^[A-Za-z0-9_-]{0,512}$'` — PostgreSQL ограничивает счётчик повторов значением 255, поэтому **любая** вставка кнопки завершалась ошибкой 2201B | интеграционный тест репозитория | эквивалентное условие `~ '^[A-Za-z0-9_-]*$' AND char_length(...) <= 512`; проверено в PostgreSQL 16 |
| D-4 | services/bot/internal/app/delivery.go | срок ожидания лимитера вычислялся как абсолютное время аренды: смешение доменных и системных часов приводило к возврату сообщений в очередь | интеграционный тест доставки | относительный бюджет `LockedUntil − now − RequestTimeout`, при неположительном бюджете — возврат с меткой `released_lease` |
| D-5 | services/bot/internal/adapters/webhook/parse.go | неверный тип поля `message.body.text` ломал разбор всего сообщения и терял идентификатор отправителя | модульный тест толерантности разбора | пополевой разбор через `json.RawMessage` |
| D-6 | compose.yaml (профиль `test`) | `postgres-test` не задаёт пароли ролей reminders, а init-скрипт выполняется с `set -eu`: инициализация тестовой базы прерывалась | статическая проверка compose и воспроизведение скрипта | переменные добавлены; проверка внесена в scripts/check_compose.py |

## 5. Решения реализации

| Решение | Обоснование |
|---|---|
| SQL пишется вручную поверх pgx, конфигурация sqlc для bot удалена | запросы очереди используют `UPDATE … FROM (SELECT … FOR UPDATE SKIP LOCKED) RETURNING` и условные обновления по аренде; так же реализован reminders-service на первом этапе — единый подход в двух сервисах |
| Ответы бота ставятся в общую очередь, а не отправляются из обработчика webhook | MAX ждёт ответ на webhook за ограниченное время (F-45); постановка в очередь в той же транзакции даёт идемпотентность и переживает перезапуск |
| Ключ ответа `reply:<sha256 тела>` | дедупликация ответов наследует дедупликацию событий: повтор события не даёт второго приветствия |
| Профиль бота хранится в памяти и обновляется фоном | `GET /me` недоступен в режиме stub и может отказать в live; сообщения с диплинком откладываются, а не расходуют попытки |
| Аренда вместо блокировки на время отправки | отправка идёт вне транзакции (сетевой вызов), поэтому результат записывается только при действующей аренде; истёкшие аренды возвращает отдельное задание |
| `context.WithoutCancel` на время вызова MAX | при остановке сервиса уже начатая отправка доводится до конца, чтобы не потерять `mid` и не отправить сообщение дважды |
| Отдельная миграция прав на `goose_db_version` | таблицу создаёт goose после применения 00001, поэтому права правятся следующей миграцией (как в reminders) |

## 6. Результаты проверок

| Проверка | Команда | Результат |
|---|---|---|
| Сборка | `go build ./...` | успешно |
| Форматирование и vet | `gofmt -l`, `go vet ./...` | без замечаний |
| Модульные и интеграционные тесты | `go test -count=1 ./...` | 12 пакетов, все пройдены |
| Гонки | `go test -race -count=1 ./services/... ./internal/...` | гонок нет |
| Покрытие services/bot/internal | `go test -coverpkg=./services/bot/internal/...` | 78,9 % операторов |
| Регрессия reminders | те же команды после правок A-1 и B-1 | все тесты reminders пройдены |
| Сквозная связка двух сервисов | `bash test/e2e/run.sh` | 3 сценария пройдены, см. [integration-report.md](integration-report.md) |
| Конфигурация развёртывания | `python3 scripts/check_compose.py` | 117 проверок, расхождений нет |
| Сборка образов и запуск compose | `docker compose build/up` | **не выполнено: в среде нет Docker** (см. §7) |

## 7. Ограничения

1. **Docker в среде разработки отсутствует**, поэтому `docker compose build`, `up` и проверка HEALTHCHECK образов не выполнялись. Вместо них: статическая проверка compose (117 инвариантов) и запуск обоих сервисов как обычных процессов на одной PostgreSQL с раздельными ролями.
2. **Режим `live` не проверен на настоящем MAX**: токен бота организаторы не выдали. Боевой клиент проверен против двойника платформы по HTTPS с собственным пулом доверия — формат запроса, заголовок токена, разбор ответа, классификация кодов и таймаут. Остаются непроверенными фактические коды ошибок MAX и поведение кнопки `open_app` (X-01, X-02).
3. **Образцы событий webhook синтетические**: составлены по документированной структуре объектов (F-48, F-55). Замена реальными телами со стенда — задача MAX-01.
4. **core-service не реализован**: события в reminders-service подаются сквозной проверкой напрямую по нормативному контракту `IngestService`, как это будет делать core.
5. Готовность (`/readyz`) bot-service зависит только от PostgreSQL: недоступность MAX не выводит сервис из ротации, поскольку очередь сохраняет сообщения и повторяет отправку.
