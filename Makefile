.PHONY: up down logs gen gen-proto gen-api lint test test-integration e2e e2e-services load measure-build verify-contracts seed-demo review-token
up:
	docker compose up -d --build
down:
	docker compose down
logs:
	docker compose logs -f --tail=200
gen: gen-proto gen-api
gen-proto:
	buf lint && buf generate
# Запросы всех трёх сервисов написаны вручную поверх pgx (решение этапа 2,
# см. docs/implementation/THIRD_SERVICE_REPORT.md §3): цель gen-sql не нужна.
gen-api:
	cd frontend && npm run gen-api
lint:
	buf lint
	go vet ./...
	cd frontend && npm run lint && npm run typecheck
test:
	go test ./...
	cd frontend && npm test -- --run
# Тесты Go, которым нужна настоящая PostgreSQL. Сервисы reminders и bot
# создают собственные временные базы из указанного административного DSN;
# без переменных такие тесты пропускаются.
test-integration: PG_TEST_DSN ?= postgres://vovremya_admin:devonly_pg_admin@127.0.0.1:55432/vovremya?sslmode=disable
test-integration:
	docker compose --profile test up -d --wait postgres-test
	CORE_TEST_DATABASE_URL="$(PG_TEST_DSN)" REMINDERS_TEST_DATABASE_URL="$(PG_TEST_DSN)" \
	BOT_TEST_DATABASE_URL="$(PG_TEST_DSN)" go test -p 1 -count=1 ./...

# Сквозная проверка трёх сервисов на настоящих процессах: мини-приложение →
# core → reminders → bot, отказы reminders и bot, восстановление.
e2e-services: PG_TEST_DSN ?= postgres://vovremya_admin:devonly_pg_admin@127.0.0.1:55432/vovremya?sslmode=disable
e2e-services:
	docker compose --profile test up -d --wait postgres-test
	E2E_ADMIN_DSN="$(PG_TEST_DSN)" bash test/e2e/run.sh
e2e:
	cd frontend && npx playwright test
load:
	docker compose --profile loadtest run --rm k6 run /scripts/k6-core-api.js
measure-build:
	./scripts/measure_build.sh
# Базовая линия контрактов — первый релиз 4.0.0 (c98282a): gRPC-контракты должны оставаться
# обратно совместимыми с ним (BUG-012).
CONTRACTS_BASELINE ?= c98282a82c560eb58f7ef034a888e008a238f5d9
verify-contracts:
	buf lint && buf breaking --against ".git#ref=$(CONTRACTS_BASELINE)"
	git diff --exit-code gen/ frontend/src/api/schema.d.ts

# Демонстрационные данные и токен проверяющего внутри запущенного контейнера.
seed-demo:
	docker compose exec core /app/core seed-demo
review-token:
	docker compose exec core /app/core review-token issue --login demo_reviewer --role editor --ttl 168h

# --- Mini App (frontend) ---
.PHONY: fe-install fe-test fe-build fe-check stack integration
fe-install:
	cd frontend && npm ci
fe-test:
	cd frontend && npm test -- --run
fe-build:
	cd frontend && npm run build
fe-check:
	cd frontend && npm run lint && npm run typecheck && npm test -- --run && npm run build

# Локальный стенд из трёх настоящих сервисов без Docker.
stack: PG_TEST_DSN ?= postgres://vovremya_admin:devonly_pg_admin@127.0.0.1:55432/postgres
stack:
	STACK_ADMIN_DSN="$(PG_TEST_DSN)" bash scripts/run_local_stack.sh

# Интеграционные проверки мини-приложения против настоящих трёх сервисов.
integration: PG_TEST_DSN ?= postgres://vovremya_admin:devonly_pg_admin@127.0.0.1:55432/postgres
integration:
	STACK_ADMIN_DSN="$(PG_TEST_DSN)" bash scripts/run_local_stack.sh -- npm --prefix frontend run test:integration
