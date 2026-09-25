# ADR-005. gRPC между сервисами

**Вопрос.** Как устроен контракт core → bot?

**Требования.** Межсервисно только gRPC; единый источник истины; генерация кода; обратная совместимость; deadlines.

**Варианты.** A. protoc вручную. B. buf (lint + breaking + generate) с удалёнными плагинами. C. buf с локальными плагинами `protoc-gen-go`, `protoc-gen-go-grpc`.

**Решение.** B с запасным C. Файл `api/proto/vovremya/bot/v1/messaging.proto`, пакет `vovremya.bot.v1`, сервис `MessagingService` с RPC `EnqueueNotification`, `GetNotificationStatus`, `GetRecipientStatus`, `GetBotProfile`; стандартный `grpc.health.v1.Health`. Сгенерированный код `gen/go/vovremya/bot/v1/*.pb.go` коммитится, чтобы Docker-сборка не требовала buf. Deadline клиента — 2 с (`CORE_BOT_RPC_TIMEOUT`). Транспорт — h2c внутри сети compose без TLS (сеть `data` изолирована, см. ADR-013). Правила изменения — [contracts/README.md](../contracts/README.md).

**Основания.** buf ловит несовместимые изменения (`buf breaking` против тега `contracts-v1.0.0`); коммит сгенерированного кода ускоряет сборку (требование 5 минут).

**Недостатки.** Удалённые плагины требуют сети при генерации; при недоступности buf.build — локальные плагины (вариант C) с версиями из [handoff bot](../handoffs/bot-service.md).

**Последствия.** Владелец `.proto` — Разработчик A; изменения через pull request с review Разработчика B.

**Проверка.** `make verify-contracts`; gRPC-тесты T-GRPC-*.

**Условия пересмотра.** Выход сервисов за пределы одного хоста — включение mTLS.
