package integration_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"vovremya/internal/platform/pgkit"
	"vovremya/services/reminders/migrations"
	"vovremya/services/reminders/test/testutil"
)

// TestMigrationsRolePrivileges проверяет разделение прав: роль приложения
// работает с данными, но не может изменять историю миграций
// (исправление находки 7 прежнего архитектурного анализа).
func TestMigrationsRolePrivileges(t *testing.T) {
	admin := os.Getenv(testutil.EnvDSN)
	if admin == "" {
		t.Skipf("%s не задан", testutil.EnvDSN)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	// Роли глобальны для экземпляра PostgreSQL и могли остаться от другого прогона
	// (например, от стенда test/e2e/run.sh) с другим паролем, поэтому пароль
	// задаётся явно и после создания роли (дефект D-13 этапа 3).
	for _, stmt := range []string{
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='reminders_migrator')
		   THEN CREATE ROLE reminders_migrator LOGIN PASSWORD 'devonly_migrator'; END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='reminders_app')
		   THEN CREATE ROLE reminders_app LOGIN PASSWORD 'devonly_app'; END IF; END $$`,
		`ALTER ROLE reminders_migrator WITH LOGIN PASSWORD 'devonly_migrator'`,
		`ALTER ROLE reminders_app WITH LOGIN PASSWORD 'devonly_app'`,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("создание роли: %v", err)
		}
	}

	dbName := fmt.Sprintf("reminders_roles_%d", time.Now().UnixNano())
	if _, err := conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %q`, dbName)); err != nil {
		t.Fatalf("create database: %v", err)
	}

	u, err := url.Parse(admin)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	adminDB := *u
	adminDB.Path = "/" + dbName
	dbConn, err := pgx.Connect(ctx, adminDB.String())
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	defer func() { _ = dbConn.Close(ctx) }()
	// Повторяет deploy/postgres/init/01-init.sh для схемы reminders.
	for _, stmt := range []string{
		`CREATE SCHEMA reminders AUTHORIZATION reminders_migrator`,
		`GRANT USAGE ON SCHEMA reminders TO reminders_app`,
		`ALTER DEFAULT PRIVILEGES FOR ROLE reminders_migrator IN SCHEMA reminders
		   GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO reminders_app`,
		`ALTER DEFAULT PRIVILEGES FOR ROLE reminders_migrator IN SCHEMA reminders
		   GRANT USAGE, SELECT ON SEQUENCES TO reminders_app`,
	} {
		if _, err := dbConn.Exec(ctx, stmt); err != nil {
			t.Fatalf("подготовка схемы: %v", err)
		}
	}

	migratorDSN := *u
	migratorDSN.Path = "/" + dbName
	migratorDSN.User = url.UserPassword("reminders_migrator", "devonly_migrator")
	if err := pgkit.MigrateUp(ctx, migratorDSN.String(), migrations.FS, "reminders"); err != nil {
		t.Fatalf("migrate up ролью-владельцем: %v", err)
	}

	checks := []struct {
		query string
		want  bool
		what  string
	}{
		{`SELECT has_table_privilege('reminders_app','reminders.reminders','INSERT')`, true, "запись в план напоминаний"},
		{`SELECT has_table_privilege('reminders_app','reminders.reminders','DELETE')`, true, "очистка плана"},
		{`SELECT has_table_privilege('reminders_app','reminders.goose_db_version','SELECT')`, true, "чтение версии миграций"},
		{`SELECT has_table_privilege('reminders_app','reminders.goose_db_version','INSERT')`, false, "запись истории миграций"},
		{`SELECT has_table_privilege('reminders_app','reminders.goose_db_version','UPDATE')`, false, "изменение истории миграций"},
		{`SELECT has_table_privilege('reminders_app','reminders.goose_db_version','DELETE')`, false, "удаление истории миграций"},
	}
	for _, c := range checks {
		var got bool
		if err := dbConn.QueryRow(ctx, c.query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		if got != c.want {
			t.Errorf("право %q: ожидалось %t, получено %t", c.what, c.want, got)
		}
	}

	// Откат миграции на чистой базе должен проходить без ошибок.
	if err := pgkit.MigrateDownTo(ctx, migratorDSN.String(), migrations.FS, "reminders", 0); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	var tables int
	if err := dbConn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_schema='reminders' AND table_name <> 'goose_db_version'`).
		Scan(&tables); err != nil {
		t.Fatalf("проверка таблиц: %v", err)
	}
	if tables != 0 {
		t.Errorf("после отката должно остаться 0 таблиц, найдено %d", tables)
	}
}

// TestSchemaOwnershipHasNoForeignReferences проверяет отсутствие внешних ключей
// за пределы схемы reminders (запрет межсервисных FK, ADR-020).
func TestSchemaOwnershipHasNoForeignReferences(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	rows, err := e.pool.DB(ctx).Query(ctx, `
SELECT con.conname, nsp_f.nspname
FROM pg_constraint con
JOIN pg_class rel ON rel.oid = con.conrelid
JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace
JOIN pg_class frel ON frel.oid = con.confrelid
JOIN pg_namespace nsp_f ON nsp_f.oid = frel.relnamespace
WHERE con.contype = 'f' AND nsp.nspname = 'reminders'`)
	if err != nil {
		t.Fatalf("запрос внешних ключей: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, schema string
		if err := rows.Scan(&name, &schema); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if schema != "reminders" {
			t.Errorf("внешний ключ %s ссылается на чужую схему %s", name, schema)
		}
	}
}

// TestUniqueReminderKey проверяет, что план не допускает дублей ключа.
func TestUniqueReminderKey(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	ctx := context.Background()
	_, err := e.pool.DB(ctx).Exec(ctx, `
INSERT INTO reminders.reminders (document_id, organization_id, period_id, account_id, days_before,
                                 due_at, status, next_attempt_at)
SELECT document_id, organization_id, period_id, account_id, days_before, due_at, status, next_attempt_at
FROM reminders.reminders LIMIT 1`)
	if err == nil {
		t.Fatal("ожидалось нарушение уникальности (period_id, account_id, days_before)")
	}
}

// TestSQLInjectionAttemptIsTreatedAsData проверяет, что параметризованные
// запросы не исполняют содержимое пользовательских строк.
func TestSQLInjectionAttemptIsTreatedAsData(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	ctx := context.Background()
	evil := e.documentEvent(t, 2, periodA, "2026-12-31", []int{30})
	evil.Document.Title = "'; DROP TABLE reminders.reminders; --"
	if res := e.apply(t, evil); res.Outcome != "applied" {
		t.Fatalf("исход применения: %s", res.Outcome)
	}
	doc, err := e.proj.GetDocument(ctx, docID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc.Title != evil.Document.Title {
		t.Errorf("название должно сохраняться как данные: %q", doc.Title)
	}
	if _, err := e.reminders.CountPlanned(ctx); err != nil {
		t.Fatalf("таблица плана должна существовать: %v", err)
	}
}
