#!/bin/sh
# Выполняется образом postgres один раз при инициализации пустого тома.
# Пароли: только [A-Za-z0-9_-] (проверяется ниже).
set -eu
for v in "$PG_CORE_MIGRATOR_PASSWORD" "$PG_CORE_APP_PASSWORD" "$PG_BOT_MIGRATOR_PASSWORD" "$PG_BOT_APP_PASSWORD" "$PG_REMINDERS_MIGRATOR_PASSWORD" "$PG_REMINDERS_APP_PASSWORD"; do
  case "$v" in *[!A-Za-z0-9_-]*|"") echo "invalid role password format" >&2; exit 1;; esac
done
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<SQL
REVOKE ALL ON DATABASE ${POSTGRES_DB} FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
CREATE ROLE core_migrator LOGIN PASSWORD '${PG_CORE_MIGRATOR_PASSWORD}';
CREATE ROLE core_app LOGIN PASSWORD '${PG_CORE_APP_PASSWORD}' CONNECTION LIMIT 60;
CREATE ROLE bot_migrator LOGIN PASSWORD '${PG_BOT_MIGRATOR_PASSWORD}';
CREATE ROLE bot_app LOGIN PASSWORD '${PG_BOT_APP_PASSWORD}' CONNECTION LIMIT 20;
CREATE ROLE reminders_migrator LOGIN PASSWORD '${PG_REMINDERS_MIGRATOR_PASSWORD}';
CREATE ROLE reminders_app LOGIN PASSWORD '${PG_REMINDERS_APP_PASSWORD}' CONNECTION LIMIT 20;
GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO core_migrator, core_app, bot_migrator, bot_app, reminders_migrator, reminders_app;
CREATE SCHEMA core AUTHORIZATION core_migrator;
CREATE SCHEMA bot AUTHORIZATION bot_migrator;
CREATE SCHEMA reminders AUTHORIZATION reminders_migrator;
GRANT USAGE ON SCHEMA core TO core_app;
GRANT USAGE ON SCHEMA bot TO bot_app;
GRANT USAGE ON SCHEMA reminders TO reminders_app;
ALTER DEFAULT PRIVILEGES FOR ROLE core_migrator IN SCHEMA core GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO core_app;
ALTER DEFAULT PRIVILEGES FOR ROLE core_migrator IN SCHEMA core GRANT USAGE, SELECT ON SEQUENCES TO core_app;
ALTER DEFAULT PRIVILEGES FOR ROLE bot_migrator IN SCHEMA bot GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bot_app;
ALTER DEFAULT PRIVILEGES FOR ROLE bot_migrator IN SCHEMA bot GRANT USAGE, SELECT ON SEQUENCES TO bot_app;
ALTER DEFAULT PRIVILEGES FOR ROLE reminders_migrator IN SCHEMA reminders GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO reminders_app;
ALTER DEFAULT PRIVILEGES FOR ROLE reminders_migrator IN SCHEMA reminders GRANT USAGE, SELECT ON SEQUENCES TO reminders_app;
ALTER ROLE core_migrator SET search_path = core;
ALTER ROLE core_app SET search_path = core;
ALTER ROLE bot_migrator SET search_path = bot;
ALTER ROLE bot_app SET search_path = bot;
ALTER ROLE reminders_migrator SET search_path = reminders;
ALTER ROLE reminders_app SET search_path = reminders;
SQL
