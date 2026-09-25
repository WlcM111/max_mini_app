# Журнал интеграционной проверки мини-приложения с тремя сервисами

Команда: `STACK_ADMIN_DSN="postgres://USER:PASSWORD@127.0.0.1:5432/postgres" bash scripts/run_local_stack.sh -- npm --prefix frontend run test:integration`

Дата: 24.09.2026. Код возврата: 0.

```text

[1/5] База vovremya_stack, роли и схемы

[2/5] Сборка сервисов

[3/5] Миграции трёх схем
2026/09/24 11:21:07 OK   00001_init.sql (18.87ms)
2026/09/24 11:21:07 OK   00002_migration_history_privileges.sql (2.99ms)
2026/09/24 11:21:07 goose: successfully migrated database to version: 2
{"time":"2026-09-24T11:21:07.254613587Z","level":"INFO","msg":"migrations applied","service":"bot","version":"stack","schema":"bot"}
2026/09/24 11:21:07 OK   00001_init.sql (20.89ms)
2026/09/24 11:21:07 goose: successfully migrated database to version: 1
{"time":"2026-09-24T11:21:07.296099136Z","level":"INFO","msg":"migrations applied","service":"reminders","version":"stack","schema":"reminders"}
2026/09/24 11:21:07 OK   00001_init.sql (56.5ms)
2026/09/24 11:21:07 OK   00002_catalog_seed.sql (6.19ms)
2026/09/24 11:21:07 OK   00003_migration_history_privileges.sql (910.65µs)
2026/09/24 11:21:07 goose: successfully migrated database to version: 3
{"time":"2026-09-24T11:21:07.375758369Z","level":"INFO","msg":"migrations applied","service":"core","version":"stack"}

[4/5] Запуск bot, reminders, core
   bot-service готов
   reminders-service готов
   core-service готов

[5/5] Стенд готов
   публичный API:      http://127.0.0.1:18170/api/v1
   admin core:         http://127.0.0.1:18171/readyz
   сообщения бота:     http://127.0.0.1:18181/debug/stub/messages
   журналы:            /tmp/tmp.GfGF3ul1u5

> vovremya-miniapp@1.0.0 test:integration
> vitest run --config vitest.integration.config.ts


 RUN  v4.0.9 /home/claude/stage4/vovremya/frontend

 ✓ test/integration/api.int.test.ts (20 tests) 912ms
     ✓ T-INT-15: владелец организации получает сообщение о новом участнике  504ms

 Test Files  1 passed (1)
      Tests  20 passed (20)
   Start at  11:21:08
   Duration  1.42s (transform 281ms, setup 0ms, collect 356ms, tests 912ms, environment 0ms, prepare 10ms)

```
