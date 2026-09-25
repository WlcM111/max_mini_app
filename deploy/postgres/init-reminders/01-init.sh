#!/bin/sh
# Инициализация PostgreSQL для автономного запуска reminders-service.
# Создаёт роли и схему только этого сервиса (compose.reminders.yaml).
set -eu
for v in "$PG_REMINDERS_MIGRATOR_PASSWORD" "$PG_REMINDERS_APP_PASSWORD"; do
  case "$v" in *[!A-Za-z0-9_-]*|"") echo "invalid role password format" >&2; exit 1;; esac
done
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<SQL
REVOKE ALL ON DATABASE ${POSTGRES_DB} FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
CREATE ROLE reminders_migrator LOGIN PASSWORD '${PG_REMINDERS_MIGRATOR_PASSWORD}';
CREATE ROLE reminders_app LOGIN PASSWORD '${PG_REMINDERS_APP_PASSWORD}' CONNECTION LIMIT 20;
GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO reminders_migrator, reminders_app;
CREATE SCHEMA reminders AUTHORIZATION reminders_migrator;
GRANT USAGE ON SCHEMA reminders TO reminders_app;
ALTER DEFAULT PRIVILEGES FOR ROLE reminders_migrator IN SCHEMA reminders GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO reminders_app;
ALTER DEFAULT PRIVILEGES FOR ROLE reminders_migrator IN SCHEMA reminders GRANT USAGE, SELECT ON SEQUENCES TO reminders_app;
ALTER ROLE reminders_migrator SET search_path = reminders;
ALTER ROLE reminders_app SET search_path = reminders;
SQL
