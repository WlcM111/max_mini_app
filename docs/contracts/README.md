# Контракты: источники истины и правила изменения

Версия 1.0.0 · 20.09.2026

| Контракт | Файл (единственный источник) | Версия | Владелец | Reviewer | Потребители |
|---|---|---|---|---|---|
| Публичный HTTP API | [openapi.yaml](../../openapi.yaml) | 1.0.0 | B | A | frontend (типы `openapi-typescript`), core (обработчики, контрактный тест), проверяющие (DATA-API.yaml) |
| gRPC core → bot | [messaging.proto](../../api/proto/vovremya/bot/v1/messaging.proto) | 1.0.0 | A | B | core (клиент), bot (сервер) |
| Webhook MAX → bot | [max-integration-spec.md, раздел 7](../max/max-integration-spec.md#7-webhook-бота-входящий-контракт-max--bot) | 1.0.0 | A | B | bot |
| Bot API bot → MAX | [max-integration-spec.md, раздел 8](../max/max-integration-spec.md#8-исходящие-сообщения-bot--max) | 1.0.0 | A | B | bot |
| Схема БД core | `services/core/migrations/*.sql` | 00002 | B | A | core |
| Схема БД bot | `services/bot/migrations/*.sql` | 00001 | A | B | bot |
| Проверки API | [DATA-API.yaml](../../DATA-API.yaml) | 1.0 | B | A | проверяющие |

Детали: [http-api.md](http-api.md), [grpc-contract.md](grpc-contract.md), [data-schemas.md](data-schemas.md), [examples.md](examples.md).

## Порядок изменения

1. Автор изменения открывает pull request с меткой `contracts`, меняя только файл-источник и документацию.
2. Reviewer — второй разработчик; без его одобрения слияние запрещено.
3. После слияния автор выполняет `make gen` и коммитит сгенерированный код (`gen/go`, `sqlcgen`, `frontend/src/api/schema.d.ts`).
4. Версия контракта повышается: добавление — минорная (1.1.0), исправление описаний — патч (1.0.1). Мажорная версия в рамках хакатона не выпускается.
5. `make verify-contracts` сравнивает `.proto` с тегом `contracts-v1.0.0` (`buf breaking`) и проверяет, что сгенерированный код совпадает с источниками.

## Правила совместимости

Разрешено: новые необязательные поля ответа, новые пути, новые RPC, новые значения перечислений при условии, что потребители обрабатывают неизвестные значения (frontend — ветка по умолчанию, proto — `*_UNSPECIFIED`). Запрещено без нового ADR: удалять или переименовывать поля, пути, RPC; менять тип или смысл поля; делать необязательное поле обязательным в запросе; переиспользовать номер поля protobuf.

## Генерация кода

| Команда | Результат | Инструмент |
|---|---|---|
| `make gen-proto` | `gen/go/vovremya/bot/v1/messaging.pb.go`, `messaging_grpc.pb.go` | buf v2 (плагины `protocolbuffers/go`, `grpc/go`) |
| `make gen-sql` | `services/<svc>/internal/adapters/postgres/sqlcgen/*.go` | sqlc 1.29+ |
| `make gen-api` | `frontend/src/api/schema.d.ts` | openapi-typescript 7 |

## Заглушки до готовности соседнего компонента

| Потребитель | Заглушка | Где |
|---|---|---|
| frontend без core | MSW-обработчики по схемам OpenAPI с фикстурами | `frontend/src/test/msw/handlers.ts`; в dev-режиме подключаются флагом `VITE_MSW=true` |
| core без bot | `FakeMessagingGateway` (реализация порта в памяти) | `services/core/internal/adapters/botgrpc/fake.go` |
| bot без MAX | `BOT_MODE=stub`, кольцевой буфер сообщений | `services/bot/internal/adapters/maxapi/stub.go` |
| frontend без MAX | имитация Bridge | `frontend/src/platform/max/mockBridge.ts` |

## Общие фикстуры

| Фикстура | Файл | Использование |
|---|---|---|
| Демо-организация и 12 документов | `demo/demo-data.json` | `core seed-demo`, e2e, DATA-API |
| Тест-векторы initData | [test-vectors.md](../max/test-vectors.md) | тесты core, e2e |
| Образцы событий webhook | `services/bot/internal/adapters/webhook/testdata/*.json` | тесты bot (записываются в MAX-01) |

## Интеграционное ревью

Ежедневно в 21:00 владелец интеграции (B) разворачивает `main` на стенде (`INT-03`), оба разработчика проходят сквозной сценарий дня и записывают расхождения контрактов в issue с меткой `contracts`.
