#!/usr/bin/env bash
# Сквозная проверка трёх сервисов (core, reminders, bot) на одной PostgreSQL.
# Запускает настоящие процессы всех сервисов (без Docker), прогоняет сценарии
# и проверяет изоляцию схем. Требует переменную E2E_ADMIN_DSN с правами
# суперпользователя. Пример:
#   E2E_ADMIN_DSN="postgres://postgres:devonly_pg_admin@127.0.0.1:5433/postgres" bash test/e2e/run.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

ADMIN_DSN="${E2E_ADMIN_DSN:?E2E_ADMIN_DSN не задан}"
DB_NAME="${E2E_DB_NAME:-vovremya_e2e}"
WORK_DIR="${E2E_WORK_DIR:-$(mktemp -d)}"
HOST_PORT="${ADMIN_DSN#*@}"; HOST_PORT="${HOST_PORT%%/*}"
PGCONN=(psql "$ADMIN_DSN" -v ON_ERROR_STOP=1 -q -t -A)

BOT_GRPC_PORT="${E2E_BOT_GRPC_PORT:-19090}"
BOT_HTTP_PORT="${E2E_BOT_HTTP_PORT:-18080}"
BOT_ADMIN_PORT="${E2E_BOT_ADMIN_PORT:-18081}"
REM_GRPC_PORT="${E2E_REM_GRPC_PORT:-19091}"
REM_ADMIN_PORT="${E2E_REM_ADMIN_PORT:-18091}"
CORE_HTTP_PORT="${E2E_CORE_HTTP_PORT:-18070}"
CORE_ADMIN_PORT="${E2E_CORE_ADMIN_PORT:-18071}"

BOT_PID=""; REM_PID=""; CORE_PID=""
cleanup() {
  [ -n "$CORE_PID" ] && kill "$CORE_PID" 2>/dev/null || true
  [ -n "$BOT_PID" ] && kill "$BOT_PID" 2>/dev/null || true
  [ -n "$REM_PID" ] && kill "$REM_PID" 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup EXIT

say() { printf '\n\033[1m%s\033[0m\n' "$*"; }

say "[1/10] Подготовка базы $DB_NAME и ролей"
"${PGCONN[@]}" <<SQL
DROP DATABASE IF EXISTS $DB_NAME;
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bot_migrator') THEN CREATE ROLE bot_migrator LOGIN; END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bot_app') THEN CREATE ROLE bot_app LOGIN; END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'reminders_migrator') THEN CREATE ROLE reminders_migrator LOGIN; END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'reminders_app') THEN CREATE ROLE reminders_app LOGIN; END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'core_migrator') THEN CREATE ROLE core_migrator LOGIN; END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'core_app') THEN CREATE ROLE core_app LOGIN; END IF;
END\$\$;
ALTER ROLE bot_migrator WITH LOGIN PASSWORD 'devonly_bot_migrator';
ALTER ROLE bot_app WITH LOGIN PASSWORD 'devonly_bot_app' CONNECTION LIMIT 20;
ALTER ROLE reminders_migrator WITH LOGIN PASSWORD 'devonly_reminders_migrator';
ALTER ROLE reminders_app WITH LOGIN PASSWORD 'devonly_reminders_app' CONNECTION LIMIT 20;
ALTER ROLE core_migrator WITH LOGIN PASSWORD 'devonly_core_migrator';
ALTER ROLE core_app WITH LOGIN PASSWORD 'devonly_core_app' CONNECTION LIMIT 60;
CREATE DATABASE $DB_NAME;
SQL

DB_DSN_ADMIN="${ADMIN_DSN%/*}/$DB_NAME"
psql "$DB_DSN_ADMIN" -v ON_ERROR_STOP=1 -q <<SQL
REVOKE ALL ON DATABASE $DB_NAME FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE $DB_NAME TO bot_migrator, bot_app, reminders_migrator, reminders_app, core_migrator, core_app;
CREATE SCHEMA bot AUTHORIZATION bot_migrator;
CREATE SCHEMA reminders AUTHORIZATION reminders_migrator;
CREATE SCHEMA core AUTHORIZATION core_migrator;
GRANT USAGE ON SCHEMA bot TO bot_app;
GRANT USAGE ON SCHEMA reminders TO reminders_app;
GRANT USAGE ON SCHEMA core TO core_app;
ALTER DEFAULT PRIVILEGES FOR ROLE bot_migrator IN SCHEMA bot GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bot_app;
ALTER DEFAULT PRIVILEGES FOR ROLE bot_migrator IN SCHEMA bot GRANT USAGE, SELECT ON SEQUENCES TO bot_app;
ALTER DEFAULT PRIVILEGES FOR ROLE reminders_migrator IN SCHEMA reminders GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO reminders_app;
ALTER DEFAULT PRIVILEGES FOR ROLE reminders_migrator IN SCHEMA reminders GRANT USAGE, SELECT ON SEQUENCES TO reminders_app;
ALTER DEFAULT PRIVILEGES FOR ROLE core_migrator IN SCHEMA core GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO core_app;
ALTER DEFAULT PRIVILEGES FOR ROLE core_migrator IN SCHEMA core GRANT USAGE, SELECT ON SEQUENCES TO core_app;
ALTER ROLE bot_app SET search_path = bot;
ALTER ROLE reminders_app SET search_path = reminders;
ALTER ROLE core_app SET search_path = core;
SQL

say "[2/10] Сборка сервисов"
go build -o "$WORK_DIR/bot" ./services/bot/cmd/bot
go build -o "$WORK_DIR/reminders" ./services/reminders/cmd/reminders
go build -o "$WORK_DIR/core" ./services/core/cmd/core
go build -o "$WORK_DIR/e2e" ./test/e2e

export APP_ENV=local LOG_LEVEL=info APP_VERSION=e2e
export BOT_MIGRATE_DATABASE_URL="postgres://bot_migrator:devonly_bot_migrator@$HOST_PORT/$DB_NAME?sslmode=disable"
export BOT_DATABASE_URL="postgres://bot_app:devonly_bot_app@$HOST_PORT/$DB_NAME?sslmode=disable"
export REMINDERS_MIGRATE_DATABASE_URL="postgres://reminders_migrator:devonly_reminders_migrator@$HOST_PORT/$DB_NAME?sslmode=disable"
export REMINDERS_DATABASE_URL="postgres://reminders_app:devonly_reminders_app@$HOST_PORT/$DB_NAME?sslmode=disable"
export CORE_MIGRATE_DATABASE_URL="postgres://core_migrator:devonly_core_migrator@$HOST_PORT/$DB_NAME?sslmode=disable"
export CORE_DATABASE_URL="postgres://core_app:devonly_core_app@$HOST_PORT/$DB_NAME?sslmode=disable"

say "[3/10] Применение миграций всех сервисов"
"$WORK_DIR/bot" migrate up
"$WORK_DIR/reminders" migrate up
"$WORK_DIR/core" migrate up

export BOT_MODE=stub
export BOT_GRPC_ADDR="127.0.0.1:$BOT_GRPC_PORT"
export BOT_REMINDERS_GRPC_ADDR="127.0.0.1:$REM_GRPC_PORT"
export BOT_HTTP_ADDR="127.0.0.1:$BOT_HTTP_PORT"
export BOT_ADMIN_ADDR="127.0.0.1:$BOT_ADMIN_PORT"
export BOT_WEBHOOK_SECRET="${BOT_WEBHOOK_SECRET:-devonly-webhook-secret}"
export BOT_WORKER_POLL_INTERVAL=200ms
export BOT_STUB_USERNAME=vovremya_local_bot
export REMINDERS_GRPC_ADDR="127.0.0.1:$REM_GRPC_PORT"
export REMINDERS_ADMIN_ADDR="127.0.0.1:$REM_ADMIN_PORT"
export REMINDERS_BOT_GRPC_ADDR="127.0.0.1:$BOT_GRPC_PORT"
export REMINDERS_SCHEDULER_INTERVAL=3s
export E2E_BOT_GRPC="127.0.0.1:$BOT_GRPC_PORT"
export E2E_BOT_ADMIN="http://127.0.0.1:$BOT_ADMIN_PORT"
export E2E_BOT_HTTP="http://127.0.0.1:$BOT_HTTP_PORT"
export E2E_REMINDERS_GRPC="127.0.0.1:$REM_GRPC_PORT"
export CORE_HTTP_ADDR="127.0.0.1:$CORE_HTTP_PORT"
export CORE_ADMIN_ADDR="127.0.0.1:$CORE_ADMIN_PORT"
export CORE_BOT_GRPC_ADDR="127.0.0.1:$BOT_GRPC_PORT"
export CORE_REMINDERS_GRPC_ADDR="127.0.0.1:$REM_GRPC_PORT"
export CORE_MAX_WEBAPP_SECRET_HEX="${CORE_MAX_WEBAPP_SECRET_HEX:-e45f53166d1da778700f29abbf89f6ac247864cb97356f927707e56f0066d663}"
export CORE_PUBLIC_BASE_URL="http://127.0.0.1:$CORE_HTTP_PORT"
export CORE_RELAY_INTERVAL=1s
export E2E_CORE_HTTP="http://127.0.0.1:$CORE_HTTP_PORT"
export E2E_CORE_ADMIN="http://127.0.0.1:$CORE_ADMIN_PORT"

wait_ready() { # адрес, имя
  for _ in $(seq 1 40); do
    if curl -fsS "http://$1/readyz" >/dev/null 2>&1; then echo "   $2 готов"; return 0; fi
    sleep 0.5
  done
  echo "   $2 не поднялся" >&2; return 1
}

say "[4/10] Запуск трёх сервисов"
"$WORK_DIR/bot" serve >"$WORK_DIR/bot.log" 2>&1 & BOT_PID=$!
"$WORK_DIR/reminders" serve >"$WORK_DIR/reminders.log" 2>&1 & REM_PID=$!
wait_ready "127.0.0.1:$BOT_ADMIN_PORT" "bot-service"
wait_ready "127.0.0.1:$REM_ADMIN_PORT" "reminders-service"
"$WORK_DIR/core" serve >"$WORK_DIR/core.log" 2>&1 & CORE_PID=$!
wait_ready "127.0.0.1:$CORE_ADMIN_PORT" "core-service"

say "[5/10] Сценарий: события по контракту → напоминание → сообщение в MAX"
"$WORK_DIR/e2e" -case=main

say "[6/10] Сценарий трёх сервисов: мини-приложение → core → reminders → bot"
"$WORK_DIR/e2e" -case=core

say "[7/10] Сценарий: отказ reminders-service и восстановление (T-CONS-01)"
kill "$REM_PID"; wait "$REM_PID" 2>/dev/null || true; REM_PID=""
echo "   reminders-service остановлен"
"$WORK_DIR/e2e" -case=core-pending
"$WORK_DIR/reminders" serve >>"$WORK_DIR/reminders.log" 2>&1 & REM_PID=$!
wait_ready "127.0.0.1:$REM_ADMIN_PORT" "reminders-service (перезапуск)"
"$WORK_DIR/e2e" -case=core-recovered

say "[8/10] Сценарий: отказ bot-service и восстановление"
kill "$BOT_PID"; wait "$BOT_PID" 2>/dev/null || true; BOT_PID=""
echo "   bot-service остановлен"
"$WORK_DIR/e2e" -case=bot-down
"$WORK_DIR/e2e" -case=core-bot-down
"$WORK_DIR/bot" serve >>"$WORK_DIR/bot.log" 2>&1 & BOT_PID=$!
wait_ready "127.0.0.1:$BOT_ADMIN_PORT" "bot-service (перезапуск)"
"$WORK_DIR/e2e" -case=bot-recovered

say "[9/10] Изоляция схем и прав"
check_denied() { # dsn, запрос, описание
  if psql "$1" -v ON_ERROR_STOP=1 -q -t -A -c "$2" >/dev/null 2>&1; then
    echo "   НАРУШЕНИЕ: $3" >&2; return 1
  fi
  echo "   запрещено, как и ожидалось: $3"
}
check_denied "$BOT_DATABASE_URL" "SELECT count(*) FROM reminders.reminders" "bot_app читает схему reminders"
check_denied "$REMINDERS_DATABASE_URL" "SELECT count(*) FROM bot.outbound_messages" "reminders_app читает схему bot"
check_denied "$BOT_DATABASE_URL" "DELETE FROM bot.goose_db_version" "bot_app правит историю миграций"
check_denied "$CORE_DATABASE_URL" "SELECT count(*) FROM reminders.reminders" "core_app читает схему reminders"
check_denied "$CORE_DATABASE_URL" "SELECT count(*) FROM bot.outbound_messages" "core_app читает схему bot"
check_denied "$CORE_DATABASE_URL" "DELETE FROM core.goose_db_version" "core_app правит историю миграций"
check_denied "$REMINDERS_DATABASE_URL" "SELECT count(*) FROM core.documents" "reminders_app читает схему core"
CROSS_FK=$(psql "$DB_DSN_ADMIN" -t -A -c "SELECT count(*) FROM pg_constraint c
  JOIN pg_class ch ON ch.oid = c.conrelid JOIN pg_namespace cn ON cn.oid = ch.relnamespace
  JOIN pg_class p ON p.oid = c.confrelid JOIN pg_namespace pn ON pn.oid = p.relnamespace
  WHERE c.contype = 'f' AND cn.nspname <> pn.nspname AND cn.nspname IN ('bot','reminders','core')")
echo "   межсервисных внешних ключей: $CROSS_FK"
[ "$CROSS_FK" = "0" ] || { echo "   НАРУШЕНИЕ: найдены межсервисные внешние ключи" >&2; exit 1; }

say "[10/10] Итоговое состояние хранилища"
psql "$DB_DSN_ADMIN" -c "SELECT status, count(*) FROM bot.outbound_messages GROUP BY status ORDER BY status;"
psql "$DB_DSN_ADMIN" -c "SELECT status, count(*) FROM reminders.reminders GROUP BY status ORDER BY status;"
psql "$DB_DSN_ADMIN" -c "SELECT state, count(*) FROM bot.recipients GROUP BY state;"
psql "$DB_DSN_ADMIN" -c "SELECT status, count(*) FROM core.outbox_events GROUP BY status ORDER BY status;"
psql "$DB_DSN_ADMIN" -c "SELECT count(*) AS documents FROM core.documents;"
echo
echo "Журналы сервисов: $WORK_DIR/bot.log, $WORK_DIR/reminders.log, $WORK_DIR/core.log"
echo "Сквозная проверка пройдена."
