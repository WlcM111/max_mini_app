# Матрица трассируемости

Версия 1.1.0 · 29.09.2026. Задачи — [team-plan.md](../plan/team-plan.md); проверки — [test-strategy.md](../testing/test-strategy.md) и [acceptance.md](../testing/acceptance.md).

| Требование | Проектное решение | Артефакт | Задача | Проверка |
|---|---|---|---|---|
| BR-01 | ADR-001 | product-brief.md | PLN-01, DEMO-01 | слайд 2 |
| BR-02 | ADR-001, онбординг frontend | features/onboarding, `createDocumentsBatch` | FE-02, CORE-03, CORE-04 | AC-02 |
| BR-03 | правила применимости | core.applicability_rules, 00002_catalog_seed.sql | CORE-03 | T-DOM-APPL |
| BR-05 | ADR-001, ADR-002 | frontend/, ограниченный набор ответов бота | FE-01…FE-05, BOT-03 | AC-01…AC-10, T-BOT (бот не меняет данные) |
| BR-04 | `data_status`, пометка демо | Catalog.document_types[].data_status, seed-demo | CORE-07, FE-03 | AC-02, AC-11 |
| FR-01 | ADR-007, spec §3–4 | `POST /sessions`, maxlaunch/verifier.go | CORE-02, FE-01, MAX-03 | AC-01, AC-MAX-01…03, T-DOM-AUTH |
| FR-02 | ADR-009 | core.sessions, `DELETE /sessions/current` | CORE-02, FE-01 | T-API, T-INT-05 |
| FR-03 | data-model | `POST /organizations` | CORE-03, FE-02 | AC-02 |
| FR-04 | ADR-001 | `GET /organizations/{id}/suggestions` | CORE-03, FE-02 | T-DOM-APPL |
| FR-05 | ADR-004 | `/documents`, `/documents/batch` | CORE-04, FE-03 | T-APP-DOC, AC-03 |
| FR-06 | data-model §статусы | `listDocuments`, `DocumentStats` | CORE-04, FE-02, FE-03 | T-DOM-STATUS, AC-03 |
| FR-07 | ADR-010 | `renewDocument`, core.document_periods | CORE-04, FE-03 | AC-05 |
| FR-08 | ADR-010, spec §8 | core.reminders, `EnqueueNotification`, bot.outbound_messages, services/bot/internal/app/{messaging,delivery}.go | CORE-05, BOT-02, INT-02 | T-DOM-REM, T-APP-REM, AC-04, test/e2e (сценарий main) |
| FR-09 | ADR-010 | `notification-settings`, `GetRecipientStatus`, services/bot/internal/domain/recipient.go | CORE-06, BOT-03, FE-04 | AC-06, AC-MAX-12, TestGRPCRecipientStatus |
| FR-10 | ADR-004, spec §6 | `/invites*`, `/members*`, `GetBotProfile` | CORE-06, FE-04 | T-APP-INV, AC-07, AC-MAX-10 |
| FR-11 | spec §6 | `createCalendarExport`, `downloadCalendar` | CORE-07, FE-04 | AC-08, AC-MAX-11 |
| FR-12 | spec §7, §15 | services/bot/internal/adapters/webhook/, internal/app/webhook.go, internal/domain/texts.go | BOT-03 | T-BOT, AC-MAX-06, TestWebhook* |
| FR-13 | spec §6 | `openCodeReader` в форме документа | FE-03 | AC-09 |
| FR-14 | security §ПДн | `DELETE /me`, `DELETE /organizations/{id}` | CORE-06, FE-04 | T-APP-ACC, AC-10 |
| FR-15 | frontend-architecture | DashboardPage | FE-02 | AC-03 |
| FR-16 | spec §6 | platform/max/backButton.ts, closingConfirmation.ts | FE-01, FE-03 | AC-MAX-09, T-FE-MAX |
| FR-17 | ADR-016 | CLI `review-token`, `seed-demo`, DATA-API.yaml | CORE-07, DEMO-01 | AC-11 |
| FR-18 | ADR-012 | `POST /client-events` | CORE-07, FE-05 | T-API |
| FR-19 | ADR-010 | `NOTIFICATION_KIND_MEMBER_JOINED` | CORE-06 | T-APP-INV |
| FR-20 | ADR-001 | `renewalChecklist.ts`, DocumentCardPage | FE-03 | unit-тесты `renewalChecklist`, T-FE-PAGE |
| FR-21 | ADR-032, ADR-033 | `POST /organizations/{id}/documents/draft`, adapters/gigachat, domain/assistant_dates.go | доработка 26–29.09 | `TestAssistantDraftDocument`, `TestAssistantDraftMergesTextDates`, `TestExtractTextDates`, T-FE-LLM |
| FR-22 | ADR-032 | `POST /profile-match` | доработка 26–29.09 | `TestAssistantMatchProfile`, T-FE-LLM |
| FR-23 | ADR-033 | `POST /organizations/{id}/documents/draft-image`, gigachat/vision.go, httpapi/budget.go, Caddy `@draftImage` | доработка 28–29.09 | `TestAssistantDraftImage`, `TestDraftDocumentFromImage*`, `TestRequestBudget`, T-FE-LLM (фото) |
| FR-24 | ADR-034 | ImportPage, importRows.ts, shared/lib/spreadsheet.ts, `POST …/documents/batch` | доработка 28–29.09 | T-FE-IMPORT, DATA-API `documents-batch` |
| FR-25 | ADR-035 | adapters/xlsx/registry.go, `GET /downloads/{token}?format=xlsx`, RegistryExportButton | доработка 28–29.09 | `TestRegistry`, `TestDocumentsLifecycleAndRegistry`, DATA-API `registry-xlsx` |
| FR-26 | ADR-036 | services/bot/internal/app/snooze.go, domain/snooze.go, подписка `message_callback`, `POST /answers` | доработка 28–29.09 | `TestSnooze*`, README §12.2 шаг 10 |
| FR-27 | ADR-001 | SuggestionsPage, DatesPage, documentPick.ts, маршрут `/o/:orgId/documents/typical` | доработка 29.09 | T-FE-TYPICAL, T-FE-PAGE |
| NFR-01 | ADR-003, spec §6 | platform/max adapter | FE-01, MAX-03, MAX-04 | AC-MAX-01 |
| NFR-02 | load-reliability | индексы, пулы, лимиты | CORE-04, TST-03 | T-NFR-LOAD |
| NFR-03 | frontend-architecture §производительность | Vite code splitting | FE-05 | T-NFR-FE-SIZE |
| NFR-04 | ADR-010 | outbox, lease, idempotency; аренда и условные обновления в services/bot/internal/adapters/postgres | CORE-05, BOT-02 | T-NFR-RESTART, T-IDEM, TestLeaseGuardsFinalUpdate, test/e2e (bot-down) |
| NFR-05 | security | CSP, verifier, rate limits, роли БД | CORE-02, INF-01, TST-05 | T-SEC-01…08 |
| NFR-06 | security §ПДн | retention jobs, удаление | CORE-06, CORE-07 | T-SEC-07, AC-10 |
| NFR-07 | ADR-012, observability | slog, /metrics, /healthz, /readyz; services/bot/internal/app/metrics.go | CORE-01, BOT-01 | T-NFR-OBS, TestRegistryHasOnlyTechnicalMetrics |
| NFR-08 | ADR-013 | Dockerfile, cache mounts | INF-01, REL-01 | T-NFR-BUILD, AC-12 |
| NFR-09 | frontend-architecture §доступность | MAX UI, стили | FE-05 | T-FE-A11Y |
| NFR-10 | load-reliability §shutdown | lifecycle, порядок остановки в services/bot/cmd/bot/main.go | CORE-01, BOT-01 | T-NFR-SHUT, TestShutdownReleasesClaimedMessages |
| NFR-11 | data-model §квоты | блокировка строки организации | CORE-04, CORE-06 | T-APP-QUOTA |
| NFR-12 | ADR-002, матрица отказов | таймауты, backoff; классификация ответов MAX в services/bot/internal/domain/delivery.go | CORE-05, TST-04 | T-FAIL, TestDeliveryFailureClassification |
| NFR-13 | ADR-010 | `due_at` в часовом поясе | CORE-05 | T-DOM-REM |
| CON-01 | ADR-002 | services/core, services/bot, go.mod | CORE-01, BOT-01 | T-ARCH, проверка 22 отчёта |
| CON-02 | ADR-006, ADR-014 | postgres в compose.yaml, миграции | INF-01, CORE-01 | T-INT-01 |
| CON-03 | ADR-002 | два сервиса: core и bot | PLN-01 | проверка 8 отчёта |
| CON-04 | backend-services | слои domain, application, ports, adapters, infrastructure | CORE-01, BOT-01 | T-ARCH |
| CON-05 | ADR-005 | messaging.proto | CORE-05, BOT-01 | T-GRPC, проверка 10 отчёта |
| CON-06 | ADR-004, ADR-011 | openapi.yaml, Caddyfile | INF-01, FE-01 | T-INT-EDGE, проверка 11 отчёта |
| CON-07 | ADR-013 | compose.yaml | INF-01 | AC-12 |
| CON-08 | load-reliability | модель P1–P3 | TST-03 | T-NFR-LOAD |
| CON-09 | frontend-architecture, handoff frontend | frontend/ | FE-01…FE-05 | T-FE-*, T-E2E-* |
| CON-10 | load-reliability §отвергнутая инфраструктура | — | PLN-01 | проверка отчёта 22 |
| CON-11 | max-platform-facts | — | MAX-01 | проверки 12–13 отчёта |
| CON-12 | conflicts-assumptions | реестр C-01…C-11 с решениями | PLN-01 | проверка 1 отчёта, раздел 5 основного документа |
| CON-13 | состав комплекта | — | — | проверка 28 отчёта |
| CON-14 | team-plan | задачи A и B | PLN-01 | проверка 23 отчёта |
| CON-15 | team-plan | календарь до 29.09 | RES-01 | проверки 26, 27 отчёта |
| SUB-01 | runbook, spec §13 | стенд | REL-01, MAX-04 | AC-MAX-01 |
| SUB-02 | submission | тег v1.0.0 | REL-01 | чек-лист сдачи |
| SUB-03 | submission | README.md | DOC-01 | чек-лист README |
| SUB-04 | ADR-013 | go.mod, go.sum, package-lock.json | CORE-01, FE-01 | сборка |
| SUB-05 | ADR-015 | Dockerfile, compose.yaml, .dockerignore, .env.example | INF-01 | T-SEC-05 |
| SUB-06 | ADR-013 | scripts/measure_build.sh | REL-01 | T-NFR-BUILD |
| SUB-07 | ADR-013 | compose.yaml | REL-02 | AC-12 |
| SUB-08 | submission | presentation.pdf | DEMO-01 | репетиция REL-02 |
| SUB-09 | ADR-011 | Caddy ACME | INF-01 | AC-MAX-01 |
| SUB-10 | ADR-004 | openapi.yaml | PLN-01 | T-CON |
| SUB-11 | ADR-016 | review-token | CORE-07, DEMO-01 | AC-11 |
| SUB-12 | ADR-016 | DATA-API.yaml | DOC-01 | AC-11 |
| SUB-13 | ADR-016 | seed-demo | CORE-07 | AC-11 |
| SUB-14 | security §лицензии | лицензии зависимостей | TST-05 | T-SEC-08 |
| SUB-15 | ADR-015 | .gitignore, devonly-проверка | INF-01 | T-SEC-05 |
| SUB-16 | ADR-008 | ник бота из `GET /me`, не зашит в код | MAX-01, BOT-01 | AC-MAX-12 |
| SUB-17 | ADR-003 | frontend на TypeScript и React | FE-01 | проверка 17 отчёта |
| PLAT-01 | max-integration-spec §13 | URL `https://<домен>/` | INF-01, MAX-03 | AC-MAX-01 |
| PLAT-02 | max-integration-spec §7, §9 | webhook через edge на 443 | BOT-03, INF-01 | AC-MAX-07 |
| PLAT-03 | max-integration-spec §8, ADR-008 | services/bot/internal/adapters/ratelimit/limiter.go | BOT-02 | T-BOT, TestGlobalRate, TestPerRecipientInterval |
| PLAT-04 | max-integration-spec §5, §11 | грамматика `start_param` | CORE-02, MAX-02 | T-DOM-START |
| PLAT-05 | max-integration-spec §8, runbook §2 | deploy/ca, `BOT_MAX_EXTRA_CA_FILE`, пул доверия в maxapi/client.go | INF-02, BOT-02 | AC-MAX-05, TestSendMessageTimeoutAndTLS |
| PLAT-06 | max-integration-spec §8, security §6 | services/bot/internal/adapters/maxapi/ (токен только в заголовке, без редиректов) | BOT-02 | T-BOT, T-SEC-05, TestSendMessageRequestFormat |
| PLAT-07 | max-integration-spec §6 | вызовы Bridge из обработчиков клика | FE-04 | T-FE-MAX |
| PLAT-08 | max-integration-spec §6 | запасные пути для web и desktop | FE-03, FE-04, MAX-03 | T-FE-MAX, AC-MAX-09 |
| DOC-01 | ARCHITECTURE_TZ.md | 26 обязательных разделов и приложения | PLN-01 | проверка 2 отчёта |
| DOC-02 | adr/ | ADR-001…ADR-036 | PLN-01 | проверка 2 отчёта |
| DOC-03 | system-overview, backend-services, sequences, long-operations, deployment, frontend-architecture | диаграммы D1–D9 | PLN-01 | рендеринг Mermaid (отчёт §2) |
| DOC-04 | ADR-003 | сравнение вариантов frontend | PLN-01 | проверка 17 отчёта |
| DOC-05 | data-model §5 | нормализация | PLN-01 | проверка 16 отчёта |
| DOC-06 | load-reliability §1, §5 | профили и матрица отказов | PLN-01 | проверка 15 отчёта |
| DOC-07 | security, observability | — | PLN-01 | проверки 19–21 отчёта |
| DOC-08 | repository-tree §2–§4 | дерево и команда | REPO-01 | проверки 29–31 отчёта |
| DOC-09 | team-plan §3–§5, §8 | план и этапы | PLN-01 | проверки 23, 26, 27 отчёта |
| DOC-10 | handoffs/ | 5 заданий | PLN-01 | проверки 24, 25 отчёта |
| DOC-11 | consistency-report | 31 проверка | PLN-01 | отчёт |
| DOC-12 | test-strategy | группы тестов | PLN-01 | проверка 3 отчёта |
| DOC-13 | submission | чек-лист сдачи | REL-01 | чек-лист |
| DOC-14 | весь комплект | — | PLN-01 | проверка 28 отчёта |
| DOC-15 | configuration | словарь конфигурации | PLN-01 | сверка с compose.yaml (отчёт §2) |
| DOC-16 | ARCHITECTURE_TZ §26 | блокеры и итоговый отчёт | PLN-01 | отчёт |
