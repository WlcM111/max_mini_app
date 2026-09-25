# Дерево репозитория и команда создания структуры

Версия 1.0.0 · 20.09.2026 · владелец: Разработчик B (REPO-01)

## 1. Принципы

Дерево содержит все файлы, которые хранятся в git: проектные документы, контракты, миграции, конфигурацию и пустые файлы будущей реализации из заданий ([core](../handoffs/core-service.md) §6, [bot](../handoffs/bot-service.md) §6, [frontend](../frontend/frontend-architecture.md) §2). Файлы, которые создают инструменты, в дерево и команду не входят и перечислены в разделе 5: пустая заготовка такого файла сломала бы инструмент (например, `npm ci` с пустым `package-lock.json`).

Порядок REPO-01: выполнить команду раздела 3 в пустом каталоге репозитория, затем скопировать поверх содержимое проектного комплекта (непустые файлы), затем выполнить генерацию из раздела 5.

## 2. Дерево

Всего файлов: 327, каталогов: 80.

```text
vovremya/
├── api/
│   └── proto/
│       └── vovremya/
│           └── bot/
│               └── v1/
│                   └── messaging.proto
├── demo/
│   ├── demo-data.json
│   └── embed.go
├── deploy/
│   ├── ca/
│   │   ├── russian_trusted_root_ca.pem
│   │   └── russian_trusted_sub_ca.pem
│   ├── edge/
│   │   ├── Caddyfile
│   │   └── Dockerfile
│   ├── postgres/
│   │   └── init/
│   │       └── 01-init.sh
│   └── prometheus/
│       └── prometheus.yml
├── docs/
│   ├── adr/
│   │   ├── ADR-001-product-scope.md
│   │   ├── ADR-002-service-boundaries.md
│   │   ├── ADR-003-frontend-framework.md
│   │   ├── ADR-004-public-http-api.md
│   │   ├── ADR-005-grpc.md
│   │   ├── ADR-006-data-ownership.md
│   │   ├── ADR-007-max-authorization.md
│   │   ├── ADR-008-max-interaction.md
│   │   ├── ADR-009-session-storage.md
│   │   ├── ADR-010-long-running-operations.md
│   │   ├── ADR-011-frontend-delivery-edge.md
│   │   ├── ADR-012-observability.md
│   │   ├── ADR-013-compose-deployment.md
│   │   ├── ADR-014-postgres-access.md
│   │   ├── ADR-015-configuration-secrets.md
│   │   ├── ADR-016-review-mode.md
│   │   └── README.md
│   ├── architecture/
│   │   ├── backend-services.md
│   │   ├── deployment.md
│   │   ├── load-reliability.md
│   │   ├── long-operations.md
│   │   ├── observability.md
│   │   ├── security.md
│   │   ├── sequences.md
│   │   └── system-overview.md
│   ├── contracts/
│   │   ├── README.md
│   │   ├── data-schemas.md
│   │   ├── examples.md
│   │   ├── grpc-contract.md
│   │   └── http-api.md
│   ├── database/
│   │   └── data-model.md
│   ├── frontend/
│   │   └── frontend-architecture.md
│   ├── handoffs/
│   │   ├── README.md
│   │   ├── bot-service.md
│   │   ├── core-service.md
│   │   ├── frontend-miniapp.md
│   │   ├── max-integration.md
│   │   └── platform-infra.md
│   ├── max/
│   │   ├── max-integration-spec.md
│   │   ├── max-platform-facts.md
│   │   └── test-vectors.md
│   ├── operations/
│   │   ├── compose-and-runbook.md
│   │   ├── configuration.md
│   │   ├── repository-tree.md
│   │   └── submission.md
│   ├── plan/
│   │   └── team-plan.md
│   ├── requirements/
│   │   ├── conflicts-assumptions.md
│   │   ├── glossary.md
│   │   ├── materials-registry.md
│   │   ├── product-brief.md
│   │   ├── requirements-registry.md
│   │   └── traceability-matrix.md
│   ├── testing/
│   │   ├── acceptance.md
│   │   ├── consistency-report.md
│   │   └── test-strategy.md
│   ├── ARCHITECTURE_TZ.md
│   └── README.md
├── frontend/
│   ├── e2e/
│   │   ├── documents.spec.ts
│   │   ├── launch.spec.ts
│   │   ├── members.spec.ts
│   │   └── network.spec.ts
│   ├── public/
│   │   └── favicon.svg
│   ├── src/
│   │   ├── api/
│   │   │   ├── client.ts
│   │   │   ├── errors.ts
│   │   │   └── queryKeys.ts
│   │   ├── app/
│   │   │   ├── App.tsx
│   │   │   ├── ErrorBoundary.tsx
│   │   │   ├── bootstrap.ts
│   │   │   ├── providers.tsx
│   │   │   ├── router.test.tsx
│   │   │   ├── router.tsx
│   │   │   └── routes.ts
│   │   ├── features/
│   │   │   ├── documents/
│   │   │   │   ├── DocumentCardPage.tsx
│   │   │   │   ├── DocumentFormPage.tsx
│   │   │   │   ├── DocumentListItem.tsx
│   │   │   │   ├── DocumentsPage.test.tsx
│   │   │   │   ├── DocumentsPage.tsx
│   │   │   │   ├── RenewPage.tsx
│   │   │   │   ├── api.ts
│   │   │   │   ├── documentForm.test.ts
│   │   │   │   └── documentForm.ts
│   │   │   ├── export/
│   │   │   │   ├── CalendarExportButton.tsx
│   │   │   │   └── api.ts
│   │   │   ├── invites/
│   │   │   │   ├── InviteAcceptPage.tsx
│   │   │   │   └── api.ts
│   │   │   ├── members/
│   │   │   │   ├── InvitePage.tsx
│   │   │   │   ├── MembersPage.tsx
│   │   │   │   └── api.ts
│   │   │   ├── onboarding/
│   │   │   │   ├── DatesPage.tsx
│   │   │   │   ├── FeaturesPage.tsx
│   │   │   │   ├── OrganizationFormPage.tsx
│   │   │   │   ├── SuggestionsPage.tsx
│   │   │   │   ├── WelcomePage.tsx
│   │   │   │   └── onboardingDraft.ts
│   │   │   ├── organizations/
│   │   │   │   ├── DashboardPage.tsx
│   │   │   │   ├── OrganizationSettingsPage.tsx
│   │   │   │   ├── OrganizationSwitcher.tsx
│   │   │   │   └── api.ts
│   │   │   ├── settings/
│   │   │   │   ├── AccountPage.tsx
│   │   │   │   ├── SettingsPage.tsx
│   │   │   │   └── api.ts
│   │   │   └── system/
│   │   │       ├── LaunchErrorPage.tsx
│   │   │       ├── NotFoundPage.tsx
│   │   │       └── NotInMaxPage.tsx
│   │   ├── platform/
│   │   │   └── max/
│   │   │       ├── backButton.ts
│   │   │       ├── bridge.test.ts
│   │   │       ├── bridge.ts
│   │   │       ├── closingConfirmation.ts
│   │   │       ├── codeReader.ts
│   │   │       ├── download.ts
│   │   │       ├── haptics.ts
│   │   │       ├── links.ts
│   │   │       ├── mockBridge.ts
│   │   │       ├── share.ts
│   │   │       └── types.ts
│   │   ├── session/
│   │   │   ├── sessionStore.ts
│   │   │   └── useSession.ts
│   │   ├── shared/
│   │   │   ├── lib/
│   │   │   │   ├── dates.test.ts
│   │   │   │   ├── dates.ts
│   │   │   │   ├── plural.ts
│   │   │   │   ├── storage.ts
│   │   │   │   ├── telemetry.ts
│   │   │   │   ├── uuid.ts
│   │   │   │   ├── validation.test.ts
│   │   │   │   └── validation.ts
│   │   │   ├── styles/
│   │   │   │   └── app.css
│   │   │   └── ui/
│   │   │       ├── AppShell.tsx
│   │   │       ├── ConfirmDialog.tsx
│   │   │       ├── DateField.tsx
│   │   │       ├── ModelDataBadge.tsx
│   │   │       ├── OffsetChips.tsx
│   │   │       ├── StateViews.tsx
│   │   │       └── StatusBadge.tsx
│   │   ├── test/
│   │   │   ├── msw/
│   │   │   │   ├── handlers.ts
│   │   │   │   └── server.ts
│   │   │   ├── fixtures.ts
│   │   │   └── setup.ts
│   │   ├── main.tsx
│   │   └── vite-env.d.ts
│   ├── .env.development
│   ├── eslint.config.js
│   ├── index.html
│   ├── package.json
│   ├── playwright.config.ts
│   ├── tsconfig.json
│   ├── tsconfig.node.json
│   └── vite.config.ts
├── scripts/
│   ├── derive_webapp_secret.py
│   ├── generate_prod_env.py
│   ├── measure_build.sh
│   ├── run_data_api_checks.py
│   └── sign_initdata.py
├── services/
│   ├── bot/
│   │   ├── cmd/
│   │   │   └── bot/
│   │   │       └── main.go
│   │   ├── internal/
│   │   │   ├── adapters/
│   │   │   │   ├── clock/
│   │   │   │   │   └── clock.go
│   │   │   │   ├── grpcserver/
│   │   │   │   │   └── server.go
│   │   │   │   ├── maxapi/
│   │   │   │   │   ├── client.go
│   │   │   │   │   ├── client_test.go
│   │   │   │   │   ├── models.go
│   │   │   │   │   ├── stub.go
│   │   │   │   │   └── stub_test.go
│   │   │   │   ├── postgres/
│   │   │   │   │   ├── db.go
│   │   │   │   │   ├── inbound.go
│   │   │   │   │   ├── messages.go
│   │   │   │   │   └── recipients.go
│   │   │   │   ├── ratelimit/
│   │   │   │   │   ├── limiter.go
│   │   │   │   │   └── limiter_test.go
│   │   │   │   └── webhook/
│   │   │   │       ├── testdata/
│   │   │   │       │   ├── README.md
│   │   │   │       │   ├── bot_started.json
│   │   │   │       │   ├── dialog_muted.json
│   │   │   │       │   └── message_created.json
│   │   │   │       ├── handler.go
│   │   │   │       ├── parse.go
│   │   │   │       └── parse_test.go
│   │   │   ├── app/
│   │   │   │   ├── background.go
│   │   │   │   ├── delivery.go
│   │   │   │   ├── messaging.go
│   │   │   │   ├── metrics.go
│   │   │   │   ├── profile.go
│   │   │   │   └── webhook.go
│   │   │   ├── archtest/
│   │   │   │   └── arch_test.go
│   │   │   ├── config/
│   │   │   │   ├── config.go
│   │   │   │   └── config_test.go
│   │   │   ├── domain/
│   │   │   │   ├── backoff.go
│   │   │   │   ├── delivery.go
│   │   │   │   ├── domain_test.go
│   │   │   │   ├── errors.go
│   │   │   │   ├── notification.go
│   │   │   │   ├── profile.go
│   │   │   │   ├── recipient.go
│   │   │   │   ├── texts.go
│   │   │   │   └── update.go
│   │   │   └── ports/
│   │   │       └── ports.go
│   │   ├── migrations/
│   │   │   ├── 00001_init.sql
│   │   │   ├── 00002_migration_history_privileges.sql
│   │   │   └── embed.go
│   │   ├── test/
│   │   │   ├── integration/
│   │   │   │   ├── delivery_test.go
│   │   │   │   ├── fixtures_test.go
│   │   │   │   ├── grpc_test.go
│   │   │   │   ├── messaging_test.go
│   │   │   │   ├── repos_test.go
│   │   │   │   ├── security_test.go
│   │   │   │   ├── shutdown_test.go
│   │   │   │   └── webhook_test.go
│   │   │   └── testutil/
│   │   │       └── testutil.go
│   │   └── Dockerfile
│   └── core/
│       ├── cmd/
│       │   └── core/
│       │       └── main.go
│       ├── internal/
│       │   ├── adapters/
│       │   │   ├── botgrpc/
│       │   │   │   ├── client.go
│       │   │   │   ├── client_test.go
│       │   │   │   └── fake.go
│       │   │   ├── httpapi/
│       │   │   │   ├── auth.go
│       │   │   │   ├── contract_test.go
│       │   │   │   ├── dto.go
│       │   │   │   ├── handlers_catalog.go
│       │   │   │   ├── handlers_documents.go
│       │   │   │   ├── handlers_exports.go
│       │   │   │   ├── handlers_invites.go
│       │   │   │   ├── handlers_members.go
│       │   │   │   ├── handlers_organizations.go
│       │   │   │   ├── handlers_session.go
│       │   │   │   ├── handlers_telemetry.go
│       │   │   │   ├── handlers_test.go
│       │   │   │   ├── middleware.go
│       │   │   │   ├── problem.go
│       │   │   │   ├── ratelimit.go
│       │   │   │   ├── router.go
│       │   │   │   └── server.go
│       │   │   ├── ics/
│       │   │   │   ├── calendar.go
│       │   │   │   └── calendar_test.go
│       │   │   ├── maxlaunch/
│       │   │   │   ├── verifier.go
│       │   │   │   └── verifier_test.go
│       │   │   └── postgres/
│       │   │       ├── queries/
│       │   │       │   ├── accounts.sql
│       │   │       │   ├── audit.sql
│       │   │       │   ├── catalog.sql
│       │   │       │   ├── documents.sql
│       │   │       │   ├── exports.sql
│       │   │       │   ├── invites.sql
│       │   │       │   ├── memberships.sql
│       │   │       │   ├── organizations.sql
│       │   │       │   ├── periods.sql
│       │   │       │   ├── reminders.sql
│       │   │       │   ├── retention.sql
│       │   │       │   └── sessions.sql
│       │   │       ├── db.go
│       │   │       ├── repositories.go
│       │   │       ├── repositories_integration_test.go
│       │   │       └── tx.go
│       │   ├── application/
│       │   │   ├── accounts.go
│       │   │   ├── auth.go
│       │   │   ├── authz.go
│       │   │   ├── documents.go
│       │   │   ├── documents_test.go
│       │   │   ├── exports.go
│       │   │   ├── invites.go
│       │   │   ├── invites_test.go
│       │   │   ├── members.go
│       │   │   ├── notifications.go
│       │   │   ├── organizations.go
│       │   │   ├── retention.go
│       │   │   ├── review.go
│       │   │   ├── scheduler.go
│       │   │   ├── scheduler_test.go
│       │   │   ├── seed.go
│       │   │   ├── suggestions.go
│       │   │   └── telemetry.go
│       │   ├── archtest/
│       │   │   └── arch_test.go
│       │   ├── domain/
│       │   │   ├── account.go
│       │   │   ├── applicability.go
│       │   │   ├── applicability_test.go
│       │   │   ├── catalog.go
│       │   │   ├── deadline.go
│       │   │   ├── deadline_test.go
│       │   │   ├── document.go
│       │   │   ├── errors.go
│       │   │   ├── invite.go
│       │   │   ├── membership.go
│       │   │   ├── organization.go
│       │   │   ├── period.go
│       │   │   ├── plural.go
│       │   │   ├── reminder_plan.go
│       │   │   ├── reminder_plan_test.go
│       │   │   ├── role.go
│       │   │   ├── role_test.go
│       │   │   ├── start_param.go
│       │   │   ├── start_param_test.go
│       │   │   └── texts.go
│       │   ├── infrastructure/
│       │   │   ├── cli/
│       │   │   │   └── cli.go
│       │   │   ├── config/
│       │   │   │   └── config.go
│       │   │   ├── lifecycle/
│       │   │   │   └── lifecycle.go
│       │   │   ├── logging/
│       │   │   │   └── logging.go
│       │   │   ├── metrics/
│       │   │   │   └── metrics.go
│       │   │   └── tracing/
│       │   │       └── tracing.go
│       │   └── ports/
│       │       ├── clock.go
│       │       ├── launch.go
│       │       ├── messaging.go
│       │       ├── repositories.go
│       │       └── tokens.go
│       ├── migrations/
│       │   ├── 00001_init.sql
│       │   ├── 00002_catalog_seed.sql
│       │   └── embed.go
│       ├── Dockerfile
│       └── sqlc.yaml
├── tests/
│   └── load/
│       └── k6-core-api.js
├── .dockerignore
├── .editorconfig
├── .env.example
├── .gitignore
├── DATA-API.yaml
├── Makefile
├── README.md
├── buf.gen.yaml
├── buf.yaml
├── compose.yaml
├── go.mod
└── openapi.yaml
```

## 3. Команда создания структуры

Выполняется из корня пустого репозитория (bash или sh).

```sh
mkdir -p \
  api/proto/vovremya/bot/v1 \
  demo \
  deploy/ca \
  deploy/edge \
  deploy/postgres/init \
  deploy/prometheus \
  docs/adr \
  docs/architecture \
  docs/contracts \
  docs/database \
  docs/frontend \
  docs/handoffs \
  docs/max \
  docs/operations \
  docs/plan \
  docs/requirements \
  docs/testing \
  frontend/e2e \
  frontend/public \
  frontend/src/api \
  frontend/src/app \
  frontend/src/features/documents \
  frontend/src/features/export \
  frontend/src/features/invites \
  frontend/src/features/members \
  frontend/src/features/onboarding \
  frontend/src/features/organizations \
  frontend/src/features/settings \
  frontend/src/features/system \
  frontend/src/platform/max \
  frontend/src/session \
  frontend/src/shared/lib \
  frontend/src/shared/styles \
  frontend/src/shared/ui \
  frontend/src/test/msw \
  scripts \
  services/bot/cmd/bot \
  services/bot/internal/adapters/grpcserver \
  services/bot/internal/adapters/maxapi \
  services/bot/internal/adapters/webhook/testdata \
  services/bot/internal/adapters/clock \
  services/bot/internal/adapters/ratelimit \
  services/bot/internal/config \
  services/bot/test/integration \
  services/bot/test/testutil \
  services/bot/internal/app \
  services/bot/internal/archtest \
  services/bot/internal/domain \
  services/bot/internal/ports \
  services/bot/migrations \
  services/core/cmd/core \
  services/core/internal/adapters/botgrpc \
  services/core/internal/adapters/httpapi \
  services/core/internal/adapters/ics \
  services/core/internal/adapters/maxlaunch \
  services/core/internal/adapters/postgres/queries \
  services/core/internal/application \
  services/core/internal/archtest \
  services/core/internal/domain \
  services/core/internal/infrastructure/cli \
  services/core/internal/infrastructure/config \
  services/core/internal/infrastructure/lifecycle \
  services/core/internal/infrastructure/logging \
  services/core/internal/infrastructure/metrics \
  services/core/internal/infrastructure/tracing \
  services/core/internal/ports \
  services/core/migrations \
  tests/load \
&& touch \
  .dockerignore \
  .editorconfig \
  .env.example \
  .gitignore \
  DATA-API.yaml \
  Makefile \
  README.md \
  api/proto/vovremya/bot/v1/messaging.proto \
  buf.gen.yaml \
  buf.yaml \
  compose.yaml \
  demo/demo-data.json \
  demo/embed.go \
  deploy/ca/russian_trusted_root_ca.pem \
  deploy/ca/russian_trusted_sub_ca.pem \
  deploy/edge/Caddyfile \
  deploy/edge/Dockerfile \
  deploy/postgres/init/01-init.sh \
  deploy/prometheus/prometheus.yml \
  docs/ARCHITECTURE_TZ.md \
  docs/README.md \
  docs/adr/ADR-001-product-scope.md \
  docs/adr/ADR-002-service-boundaries.md \
  docs/adr/ADR-003-frontend-framework.md \
  docs/adr/ADR-004-public-http-api.md \
  docs/adr/ADR-005-grpc.md \
  docs/adr/ADR-006-data-ownership.md \
  docs/adr/ADR-007-max-authorization.md \
  docs/adr/ADR-008-max-interaction.md \
  docs/adr/ADR-009-session-storage.md \
  docs/adr/ADR-010-long-running-operations.md \
  docs/adr/ADR-011-frontend-delivery-edge.md \
  docs/adr/ADR-012-observability.md \
  docs/adr/ADR-013-compose-deployment.md \
  docs/adr/ADR-014-postgres-access.md \
  docs/adr/ADR-015-configuration-secrets.md \
  docs/adr/ADR-016-review-mode.md \
  docs/adr/README.md \
  docs/architecture/backend-services.md \
  docs/architecture/deployment.md \
  docs/architecture/load-reliability.md \
  docs/architecture/long-operations.md \
  docs/architecture/observability.md \
  docs/architecture/security.md \
  docs/architecture/sequences.md \
  docs/architecture/system-overview.md \
  docs/contracts/README.md \
  docs/contracts/data-schemas.md \
  docs/contracts/examples.md \
  docs/contracts/grpc-contract.md \
  docs/contracts/http-api.md \
  docs/database/data-model.md \
  docs/frontend/frontend-architecture.md \
  docs/handoffs/README.md \
  docs/handoffs/bot-service.md \
  docs/handoffs/core-service.md \
  docs/handoffs/frontend-miniapp.md \
  docs/handoffs/max-integration.md \
  docs/handoffs/platform-infra.md \
  docs/max/max-integration-spec.md \
  docs/max/max-platform-facts.md \
  docs/max/test-vectors.md \
  docs/operations/compose-and-runbook.md \
  docs/operations/configuration.md \
  docs/operations/repository-tree.md \
  docs/operations/submission.md \
  docs/plan/team-plan.md \
  docs/requirements/conflicts-assumptions.md \
  docs/requirements/glossary.md \
  docs/requirements/materials-registry.md \
  docs/requirements/product-brief.md \
  docs/requirements/requirements-registry.md \
  docs/requirements/traceability-matrix.md \
  docs/testing/acceptance.md \
  docs/testing/consistency-report.md \
  docs/testing/test-strategy.md \
  frontend/.env.development \
  frontend/e2e/documents.spec.ts \
  frontend/e2e/launch.spec.ts \
  frontend/e2e/members.spec.ts \
  frontend/e2e/network.spec.ts \
  frontend/eslint.config.js \
  frontend/index.html \
  frontend/package.json \
  frontend/playwright.config.ts \
  frontend/public/favicon.svg \
  frontend/src/api/client.ts \
  frontend/src/api/errors.ts \
  frontend/src/api/queryKeys.ts \
  frontend/src/app/App.tsx \
  frontend/src/app/ErrorBoundary.tsx \
  frontend/src/app/bootstrap.ts \
  frontend/src/app/providers.tsx \
  frontend/src/app/router.test.tsx \
  frontend/src/app/router.tsx \
  frontend/src/app/routes.ts \
  frontend/src/features/documents/DocumentCardPage.tsx \
  frontend/src/features/documents/DocumentFormPage.tsx \
  frontend/src/features/documents/DocumentListItem.tsx \
  frontend/src/features/documents/DocumentsPage.test.tsx \
  frontend/src/features/documents/DocumentsPage.tsx \
  frontend/src/features/documents/RenewPage.tsx \
  frontend/src/features/documents/api.ts \
  frontend/src/features/documents/documentForm.test.ts \
  frontend/src/features/documents/documentForm.ts \
  frontend/src/features/export/CalendarExportButton.tsx \
  frontend/src/features/export/api.ts \
  frontend/src/features/invites/InviteAcceptPage.tsx \
  frontend/src/features/invites/api.ts \
  frontend/src/features/members/InvitePage.tsx \
  frontend/src/features/members/MembersPage.tsx \
  frontend/src/features/members/api.ts \
  frontend/src/features/onboarding/DatesPage.tsx \
  frontend/src/features/onboarding/FeaturesPage.tsx \
  frontend/src/features/onboarding/OrganizationFormPage.tsx \
  frontend/src/features/onboarding/SuggestionsPage.tsx \
  frontend/src/features/onboarding/WelcomePage.tsx \
  frontend/src/features/onboarding/onboardingDraft.ts \
  frontend/src/features/organizations/DashboardPage.tsx \
  frontend/src/features/organizations/OrganizationSettingsPage.tsx \
  frontend/src/features/organizations/OrganizationSwitcher.tsx \
  frontend/src/features/organizations/api.ts \
  frontend/src/features/settings/AccountPage.tsx \
  frontend/src/features/settings/SettingsPage.tsx \
  frontend/src/features/settings/api.ts \
  frontend/src/features/system/LaunchErrorPage.tsx \
  frontend/src/features/system/NotFoundPage.tsx \
  frontend/src/features/system/NotInMaxPage.tsx \
  frontend/src/main.tsx \
  frontend/src/platform/max/backButton.ts \
  frontend/src/platform/max/bridge.test.ts \
  frontend/src/platform/max/bridge.ts \
  frontend/src/platform/max/closingConfirmation.ts \
  frontend/src/platform/max/codeReader.ts \
  frontend/src/platform/max/download.ts \
  frontend/src/platform/max/haptics.ts \
  frontend/src/platform/max/links.ts \
  frontend/src/platform/max/mockBridge.ts \
  frontend/src/platform/max/share.ts \
  frontend/src/platform/max/types.ts \
  frontend/src/session/sessionStore.ts \
  frontend/src/session/useSession.ts \
  frontend/src/shared/lib/dates.test.ts \
  frontend/src/shared/lib/dates.ts \
  frontend/src/shared/lib/plural.ts \
  frontend/src/shared/lib/storage.ts \
  frontend/src/shared/lib/telemetry.ts \
  frontend/src/shared/lib/uuid.ts \
  frontend/src/shared/lib/validation.test.ts \
  frontend/src/shared/lib/validation.ts \
  frontend/src/shared/styles/app.css \
  frontend/src/shared/ui/AppShell.tsx \
  frontend/src/shared/ui/ConfirmDialog.tsx \
  frontend/src/shared/ui/DateField.tsx \
  frontend/src/shared/ui/ModelDataBadge.tsx \
  frontend/src/shared/ui/OffsetChips.tsx \
  frontend/src/shared/ui/StateViews.tsx \
  frontend/src/shared/ui/StatusBadge.tsx \
  frontend/src/test/fixtures.ts \
  frontend/src/test/msw/handlers.ts \
  frontend/src/test/msw/server.ts \
  frontend/src/test/setup.ts \
  frontend/src/vite-env.d.ts \
  frontend/tsconfig.json \
  frontend/tsconfig.node.json \
  frontend/vite.config.ts \
  go.mod \
  openapi.yaml \
  scripts/derive_webapp_secret.py \
  scripts/generate_prod_env.py \
  scripts/measure_build.sh \
  scripts/run_data_api_checks.py \
  scripts/sign_initdata.py \
  services/bot/Dockerfile \
  services/bot/cmd/bot/main.go \
  services/bot/internal/adapters/grpcserver/server.go \
  services/bot/internal/adapters/grpcserver/server_test.go \
  services/bot/internal/adapters/maxapi/client.go \
  services/bot/internal/adapters/maxapi/client_test.go \
  services/bot/internal/adapters/maxapi/models.go \
  services/bot/internal/adapters/maxapi/stub.go \
  services/bot/internal/adapters/postgres/db.go \
  services/bot/internal/adapters/postgres/queries/inbound.sql \
  services/bot/internal/adapters/postgres/queries/outbound.sql \
  services/bot/internal/adapters/postgres/queries/recipients.sql \
  services/bot/internal/adapters/postgres/queries/retention.sql \
  services/bot/internal/adapters/postgres/repositories.go \
  services/bot/internal/adapters/postgres/repositories_integration_test.go \
  services/bot/internal/adapters/postgres/tx.go \
  services/bot/internal/adapters/webhook/handler.go \
  services/bot/internal/adapters/webhook/handler_test.go \
  services/bot/internal/adapters/webhook/testdata/bot_started.json \
  services/bot/internal/adapters/webhook/testdata/dialog_muted.json \
  services/bot/internal/adapters/webhook/testdata/message_created.json \
  services/bot/internal/application/delivery.go \
  services/bot/internal/application/delivery_test.go \
  services/bot/internal/application/enqueue.go \
  services/bot/internal/application/enqueue_test.go \
  services/bot/internal/application/lease_reaper.go \
  services/bot/internal/application/profile.go \
  services/bot/internal/application/retention.go \
  services/bot/internal/application/subscription.go \
  services/bot/internal/application/webhook.go \
  services/bot/internal/application/webhook_test.go \
  services/bot/internal/archtest/arch_test.go \
  services/bot/internal/domain/backoff.go \
  services/bot/internal/domain/backoff_test.go \
  services/bot/internal/domain/deeplink.go \
  services/bot/internal/domain/errors.go \
  services/bot/internal/domain/notification.go \
  services/bot/internal/domain/notification_test.go \
  services/bot/internal/domain/recipient.go \
  services/bot/internal/domain/render.go \
  services/bot/internal/domain/render_test.go \
  services/bot/internal/domain/update.go \
  services/bot/internal/domain/update_test.go \
  services/bot/internal/infrastructure/cli/cli.go \
  services/bot/internal/infrastructure/config/config.go \
  services/bot/internal/infrastructure/lifecycle/lifecycle.go \
  services/bot/internal/infrastructure/logging/logging.go \
  services/bot/internal/infrastructure/metrics/metrics.go \
  services/bot/internal/infrastructure/ratelimit/limiter.go \
  services/bot/internal/infrastructure/ratelimit/limiter_test.go \
  services/bot/internal/infrastructure/tracing/tracing.go \
  services/bot/internal/ports/clock.go \
  services/bot/internal/ports/maxclient.go \
  services/bot/internal/ports/repositories.go \
  services/bot/migrations/00001_init.sql \
  services/bot/migrations/embed.go \
  services/bot/sqlc.yaml \
  services/core/Dockerfile \
  services/core/cmd/core/main.go \
  services/core/internal/adapters/botgrpc/client.go \
  services/core/internal/adapters/botgrpc/client_test.go \
  services/core/internal/adapters/botgrpc/fake.go \
  services/core/internal/adapters/httpapi/auth.go \
  services/core/internal/adapters/httpapi/contract_test.go \
  services/core/internal/adapters/httpapi/dto.go \
  services/core/internal/adapters/httpapi/handlers_catalog.go \
  services/core/internal/adapters/httpapi/handlers_documents.go \
  services/core/internal/adapters/httpapi/handlers_exports.go \
  services/core/internal/adapters/httpapi/handlers_invites.go \
  services/core/internal/adapters/httpapi/handlers_members.go \
  services/core/internal/adapters/httpapi/handlers_organizations.go \
  services/core/internal/adapters/httpapi/handlers_session.go \
  services/core/internal/adapters/httpapi/handlers_telemetry.go \
  services/core/internal/adapters/httpapi/handlers_test.go \
  services/core/internal/adapters/httpapi/middleware.go \
  services/core/internal/adapters/httpapi/problem.go \
  services/core/internal/adapters/httpapi/ratelimit.go \
  services/core/internal/adapters/httpapi/router.go \
  services/core/internal/adapters/httpapi/server.go \
  services/core/internal/adapters/ics/calendar.go \
  services/core/internal/adapters/ics/calendar_test.go \
  services/core/internal/adapters/maxlaunch/verifier.go \
  services/core/internal/adapters/maxlaunch/verifier_test.go \
  services/core/internal/adapters/postgres/db.go \
  services/core/internal/adapters/postgres/queries/accounts.sql \
  services/core/internal/adapters/postgres/queries/audit.sql \
  services/core/internal/adapters/postgres/queries/catalog.sql \
  services/core/internal/adapters/postgres/queries/documents.sql \
  services/core/internal/adapters/postgres/queries/exports.sql \
  services/core/internal/adapters/postgres/queries/invites.sql \
  services/core/internal/adapters/postgres/queries/memberships.sql \
  services/core/internal/adapters/postgres/queries/organizations.sql \
  services/core/internal/adapters/postgres/queries/periods.sql \
  services/core/internal/adapters/postgres/queries/reminders.sql \
  services/core/internal/adapters/postgres/queries/retention.sql \
  services/core/internal/adapters/postgres/queries/sessions.sql \
  services/core/internal/adapters/postgres/repositories.go \
  services/core/internal/adapters/postgres/repositories_integration_test.go \
  services/core/internal/adapters/postgres/tx.go \
  services/core/internal/application/accounts.go \
  services/core/internal/application/auth.go \
  services/core/internal/application/authz.go \
  services/core/internal/application/documents.go \
  services/core/internal/application/documents_test.go \
  services/core/internal/application/exports.go \
  services/core/internal/application/invites.go \
  services/core/internal/application/invites_test.go \
  services/core/internal/application/members.go \
  services/core/internal/application/notifications.go \
  services/core/internal/application/organizations.go \
  services/core/internal/application/retention.go \
  services/core/internal/application/review.go \
  services/core/internal/application/scheduler.go \
  services/core/internal/application/scheduler_test.go \
  services/core/internal/application/seed.go \
  services/core/internal/application/suggestions.go \
  services/core/internal/application/telemetry.go \
  services/core/internal/archtest/arch_test.go \
  services/core/internal/domain/account.go \
  services/core/internal/domain/applicability.go \
  services/core/internal/domain/applicability_test.go \
  services/core/internal/domain/catalog.go \
  services/core/internal/domain/deadline.go \
  services/core/internal/domain/deadline_test.go \
  services/core/internal/domain/document.go \
  services/core/internal/domain/errors.go \
  services/core/internal/domain/invite.go \
  services/core/internal/domain/membership.go \
  services/core/internal/domain/organization.go \
  services/core/internal/domain/period.go \
  services/core/internal/domain/plural.go \
  services/core/internal/domain/reminder_plan.go \
  services/core/internal/domain/reminder_plan_test.go \
  services/core/internal/domain/role.go \
  services/core/internal/domain/role_test.go \
  services/core/internal/domain/start_param.go \
  services/core/internal/domain/start_param_test.go \
  services/core/internal/domain/texts.go \
  services/core/internal/infrastructure/cli/cli.go \
  services/core/internal/infrastructure/config/config.go \
  services/core/internal/infrastructure/lifecycle/lifecycle.go \
  services/core/internal/infrastructure/logging/logging.go \
  services/core/internal/infrastructure/metrics/metrics.go \
  services/core/internal/infrastructure/tracing/tracing.go \
  services/core/internal/ports/clock.go \
  services/core/internal/ports/launch.go \
  services/core/internal/ports/messaging.go \
  services/core/internal/ports/repositories.go \
  services/core/internal/ports/tokens.go \
  services/core/migrations/00001_init.sql \
  services/core/migrations/00002_catalog_seed.sql \
  services/core/migrations/embed.go \
  services/core/sqlc.yaml \
  tests/load/k6-core-api.js
```

## 4. Сверка дерева и команды

Проверка выполнена скриптом 20.09.2026 (дерево и команда сгенерированы из одного списка путей, затем проверены независимо).

| Проверка | Способ | Результат |
|---|---|---|
| Дерево → множество путей | разбор текста дерева по отступам | совпадает с эталонным списком (327 файлов) |
| Команда → множество путей | разбор аргументов `touch` | совпадает: лишних путей нет, отсутствующих нет |
| Выполнение команды | запуск во временном каталоге и обход созданных файлов | созданы ровно 327 файлов дерева |
| Покрытие комплекта | все файлы проектного комплекта присутствуют в дереве | да |
| Списки в заданиях | каждое имя файла из структур core, bot, frontend найдено в дереве | да; `schema.d.ts` — генерируемый файл (раздел 5) |

Повторная проверка (выполняется в REPO-01): выполнить команду раздела 3 в пустом каталоге, затем `find . -type f | sed 's|^./||' | sort` и сравнить со списком файлов дерева раздела 2.

## 5. Файлы, создаваемые инструментами

| Файл | Команда | Хранится в git |
|---|---|---|
| `go.sum` | `go mod tidy` | да |
| `gen/go/vovremya/bot/v1/messaging.pb.go`, `messaging_grpc.pb.go` | `make gen-proto` | да |
| `services/core/internal/adapters/postgres/sqlcgen/*.go` | `make gen-sql` | да |
| `services/bot/internal/adapters/postgres/sqlcgen/*.go` | `make gen-sql` | да |
| `frontend/package-lock.json` | `npm install` в `frontend/` | да |
| `frontend/src/api/schema.d.ts` | `npm run gen-api` | да |
| `frontend/node_modules/`, `frontend/dist/` | `npm ci`, `npm run build` | нет (`.gitignore`) |
| `.env` | `scripts/generate_prod_env.py` (стенд) | нет (`.gitignore`) |

# Версия 2.0.0

Дерево отражает фактическое состояние архива на 20.09.2026.
Каталогов: 73. Файлов: 181. Заготовок-пустышек нет: перечислены только существующие файлы.

## Новые и изменённые каталоги

| Путь | Назначение | Состояние |
|---|---|---|
| internal/platform/ | общий технический каркас трёх сервисов (ADR-026) | реализован |
| api/proto/vovremya/reminders/v1/ | нормативные контракты ingest и query | реализованы |
| gen/go/vovremya/reminders/v1/ | код, сгенерированный из контрактов | сгенерирован |
| services/reminders/ | исходный код, миграции и тесты reminders-service | реализован |
| services/reminders/test/botdouble/ | двойник bot-service для автономной проверки | реализован |
| services/reminders/test/smoke/ | сквозной сценарий проверки | реализован |
| deploy/postgres/init-reminders/ | инициализация ролей для автономного запуска | реализован |
| compose.reminders.yaml, .env.reminders.example | автономный запуск сервиса | реализованы |
| docs/architecture/consistency.md | межсервисная согласованность | реализован |
| docs/handoffs/reminders-service.md, core-service-v2.md, bot-service-v2.md | самостоятельные задания | реализованы |
| docs/adr/ADR-017…ADR-031 | решения версии 2.0.0 | реализованы |
| docs/implementation/ | аудит входных материалов, манифест, отчёт реализации | реализован |
| docs/testing/consistency-report-v2.md, docs/plan/team-plan-v2.md | проверка согласованности и пересчитанный план | реализованы |

## Полное дерево

```text
vovremya/
.dockerignore
.editorconfig
.env.example
.env.reminders.example
.gitignore
DATA-API.yaml
Makefile
README.md
buf.gen.yaml
buf.yaml
compose.reminders.yaml
compose.yaml
go.mod
go.sum
openapi.yaml
api/
  proto/
    vovremya/
      bot/
        v1/
          messaging.proto
      reminders/
        v1/
          ingest.proto
          query.proto
demo/
  demo-data.json
  embed.go
deploy/
  ca/
    russian_trusted_root_ca.pem
    russian_trusted_sub_ca.pem
  edge/
    Caddyfile
    Dockerfile
  postgres/
    init/
      01-init.sh
    init-reminders/
      01-init.sh
  prometheus/
    prometheus.yml
docs/
  ARCHITECTURE_TZ.md
  README.md
  adr/
    ADR-001-product-scope.md
    ADR-002-service-boundaries.md
    ADR-003-frontend-framework.md
    ADR-004-public-http-api.md
    ADR-005-grpc.md
    ADR-006-data-ownership.md
    ADR-007-max-authorization.md
    ADR-008-max-interaction.md
    ADR-009-session-storage.md
    ADR-010-long-running-operations.md
    ADR-011-frontend-delivery-edge.md
    ADR-012-observability.md
    ADR-013-compose-deployment.md
    ADR-014-postgres-access.md
    ADR-015-configuration-secrets.md
    ADR-016-review-mode.md
    ADR-017-three-services.md
    ADR-018-reminders-boundaries.md
    ADR-019-data-ownership.md
    ADR-020-consistency-model.md
    ADR-021-outbox-inbox.md
    ADR-022-projections.md
    ADR-023-next-reminder-read.md
    ADR-024-idempotency.md
    ADR-025-clean-layers.md
    ADR-026-platform-chassis.md
    ADR-027-testing-strategy.md
    ADR-028-scheduler-runtime.md
    ADR-029-grpc-network-isolation.md
    ADR-030-tracing-ids.md
    ADR-031-no-production-stubs.md
    README.md
  architecture/
    backend-services.md
    consistency.md
    deployment.md
    load-reliability.md
    long-operations.md
    observability.md
    security.md
    sequences.md
    system-overview.md
  contracts/
    README.md
    data-schemas.md
    examples.md
    grpc-contract.md
    http-api.md
  database/
    data-model.md
    reminders-data-model.md
  frontend/
    frontend-architecture.md
  handoffs/
    README.md
    bot-service-v2.md
    bot-service.md
    core-service-v2.md
    core-service.md
    frontend-miniapp.md
    max-integration.md
    platform-infra.md
    reminders-service.md
  implementation/
    FIRST_SERVICE_REPORT.md
    input-audit.md
    input-manifest.json
  max/
    max-integration-spec.md
    max-platform-facts.md
    test-vectors.md
  operations/
    compose-and-runbook.md
    configuration.md
    repository-tree.md
    submission.md
  plan/
    team-plan-v2.md
    team-plan.md
  requirements/
    conflicts-assumptions.md
    glossary.md
    materials-registry.md
    product-brief.md
    requirements-registry.md
    traceability-matrix.md
  testing/
    acceptance.md
    consistency-report-v2.md
    consistency-report.md
    test-strategy.md
gen/
  go/
    vovremya/
      bot/
        v1/
          messaging.pb.go
          messaging_grpc.pb.go
      reminders/
        v1/
          ingest.pb.go
          ingest_grpc.pb.go
          query.pb.go
          query_grpc.pb.go
internal/
  platform/
    adminhttp/
      adminhttp.go
    config/
      config.go
    grpckit/
      grpckit.go
    lifecycle/
      lifecycle.go
    logging/
      logging.go
    metrics/
      metrics.go
    pgkit/
      dbtx.go
      migrate.go
      pgkit.go
    tracectx/
      tracectx.go
scripts/
  derive_webapp_secret.py
  generate_prod_env.py
  measure_build.sh
  run_data_api_checks.py
  sign_initdata.py
services/
  bot/
    Dockerfile
    sqlc.yaml
    migrations/
      00001_init.sql
  core/
    Dockerfile
    sqlc.yaml
    migrations/
      00001_init.sql
      00002_catalog_seed.sql
  reminders/
    Dockerfile
    Dockerfile.botdouble
    cmd/
      reminders/
        main.go
    internal/
      adapters/
        botgrpc/
          client.go
        clock/
          clock.go
        grpcserver/
          mapping.go
          server.go
        postgres/
          db.go
          inbox.go
          projections.go
          reminders.go
      app/
        events.go
        ingest.go
        query.go
        replan.go
        retention.go
        scheduler.go
      archtest/
        arch_test.go
      config/
        config.go
      domain/
        backoff.go
        date.go
        domain_test.go
        errors.go
        plan.go
        plan_test.go
        projection.go
        reminder.go
        text.go
        uuid.go
      ports/
        gateway_error.go
        ports.go
    migrations/
      00001_init.sql
      embed.go
    test/
      botdouble/
        main.go
      integration/
        fixtures_test.go
        grpc_test.go
        ingest_test.go
        scheduler_test.go
        security_test.go
      smoke/
        main.go
      testutil/
        testutil.go
tests/
  load/
    k6-core-api.js
```

## Сверка

Числа выше получены тем же обходом каталога, что и дерево, поэтому расхождение
заявленного и фактического состава исключено (дефект версии 1.0.0: 80 против 99 каталогов).
