package integration_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"vovremya/internal/platform/pgkit"
	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/migrations"
	"vovremya/services/bot/test/testutil"
)

// setupRoles создаёт базу с ролями и правами по deploy/postgres/init/01-init.sh
// и применяет миграции ролью bot_migrator.
func setupRoles(t *testing.T) (appDSN, adminDSN string) {
	t.Helper()
	admin := os.Getenv(testutil.EnvDSN)
	if admin == "" {
		t.Skipf("%s не задан: интеграционный тест пропущен", testutil.EnvDSN)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	name := fmt.Sprintf("bot_roles_%d", time.Now().UnixNano())
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("подключение администратора: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	roles := `DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bot_migrator') THEN
        CREATE ROLE bot_migrator LOGIN PASSWORD 'devonly_bot_migrator';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'bot_app') THEN
        CREATE ROLE bot_app LOGIN PASSWORD 'devonly_bot_app' CONNECTION LIMIT 20;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'reminders_app') THEN
        CREATE ROLE reminders_app LOGIN PASSWORD 'devonly_reminders_app' CONNECTION LIMIT 20;
    END IF;
END$$;
ALTER ROLE bot_migrator WITH LOGIN PASSWORD 'devonly_bot_migrator';
ALTER ROLE bot_app WITH LOGIN PASSWORD 'devonly_bot_app';
ALTER ROLE reminders_app WITH LOGIN PASSWORD 'devonly_reminders_app';`
	if _, err := conn.Exec(ctx, roles); err != nil {
		t.Fatalf("создание ролей: %v", err)
	}
	if _, err := conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		t.Fatalf("создание базы: %v", err)
	}
	adminDSN, err = testutil.ReplaceDatabase(admin, name)
	if err != nil {
		t.Fatalf("DSN: %v", err)
	}
	dbConn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatalf("подключение к базе: %v", err)
	}
	defer func() { _ = dbConn.Close(ctx) }()
	setup := []string{
		`GRANT CONNECT ON DATABASE ` + pgx.Identifier{name}.Sanitize() + ` TO bot_migrator, bot_app, reminders_app`,
		`CREATE SCHEMA bot AUTHORIZATION bot_migrator`,
		`CREATE SCHEMA reminders`,
		`CREATE TABLE reminders.reminders (id bigint)`,
		`GRANT USAGE ON SCHEMA bot TO bot_app`,
		`GRANT USAGE ON SCHEMA reminders TO reminders_app`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON reminders.reminders TO reminders_app`,
		`ALTER DEFAULT PRIVILEGES FOR ROLE bot_migrator IN SCHEMA bot GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bot_app`,
		`ALTER DEFAULT PRIVILEGES FOR ROLE bot_migrator IN SCHEMA bot GRANT USAGE, SELECT ON SEQUENCES TO bot_app`,
	}
	for _, q := range setup {
		if _, err := dbConn.Exec(ctx, q); err != nil {
			t.Fatalf("подготовка прав (%s): %v", q, err)
		}
	}
	migratorDSN := withCredentials(t, adminDSN, "bot_migrator", "devonly_bot_migrator")
	if err := pgkit.MigrateUp(ctx, migratorDSN, migrations.FS, "bot"); err != nil {
		t.Fatalf("миграции ролью bot_migrator: %v", err)
	}
	return withCredentials(t, adminDSN, "bot_app", "devonly_bot_app"), adminDSN
}

func withCredentials(t *testing.T, dsn, user, password string) string {
	t.Helper()
	parts := strings.SplitN(dsn, "@", 2)
	if len(parts) != 2 {
		t.Fatalf("некорректный DSN: %s", dsn)
	}
	return "postgres://" + user + ":" + password + "@" + parts[1]
}

func TestDatabaseRolePrivileges(t *testing.T) {
	appDSN, adminDSN := setupRoles(t)
	ctx := context.Background()
	app, err := pgx.Connect(ctx, appDSN)
	if err != nil {
		t.Fatalf("подключение bot_app: %v", err)
	}
	defer func() { _ = app.Close(ctx) }()

	t.Run("работа с собственными таблицами разрешена", func(t *testing.T) {
		_, err := app.Exec(ctx, `INSERT INTO bot.outbound_messages
			(idempotency_key, request_hash, kind, recipient_max_user_id, text, status, not_after)
			VALUES ('role-test-0001', $1, 'reminder', 1001, 'текст', 'queued', now() + interval '1 hour')`,
			make([]byte, 32))
		if err != nil {
			t.Fatalf("вставка в собственную таблицу: %v", err)
		}
	})

	t.Run("история миграций только для чтения", func(t *testing.T) {
		var version int64
		if err := app.QueryRow(ctx, `SELECT max(version_id) FROM bot.goose_db_version`).Scan(&version); err != nil {
			t.Fatalf("чтение истории миграций: %v", err)
		}
		if version < 2 {
			t.Errorf("применено миграций: %d", version)
		}
		if _, err := app.Exec(ctx, `DELETE FROM bot.goose_db_version`); err == nil {
			t.Error("роль приложения не должна изменять историю миграций")
		}
		if _, err := app.Exec(ctx,
			`INSERT INTO bot.goose_db_version (version_id, is_applied) VALUES (99, true)`); err == nil {
			t.Error("роль приложения не должна добавлять записи в историю миграций")
		}
	})

	t.Run("создание объектов запрещено", func(t *testing.T) {
		if _, err := app.Exec(ctx, `CREATE TABLE bot.injected (id int)`); err == nil {
			t.Error("роль приложения не должна создавать таблицы")
		}
	})

	t.Run("чужая схема недоступна", func(t *testing.T) {
		if _, err := app.Exec(ctx, `SELECT count(*) FROM reminders.reminders`); err == nil {
			t.Error("bot_app не должен читать схему reminders (ADR-019)")
		}
		if _, err := app.Exec(ctx, `INSERT INTO reminders.reminders (id) VALUES (1)`); err == nil {
			t.Error("bot_app не должен писать в схему reminders")
		}
	})

	t.Run("роль соседнего сервиса не видит схему bot", func(t *testing.T) {
		remindersDSN := withCredentials(t, adminDSN, "reminders_app", "devonly_reminders_app")
		conn, err := pgx.Connect(ctx, remindersDSN)
		if err != nil {
			t.Fatalf("подключение reminders_app: %v", err)
		}
		defer func() { _ = conn.Close(ctx) }()
		if _, err := conn.Exec(ctx, `SELECT count(*) FROM bot.outbound_messages`); err == nil {
			t.Error("reminders_app не должен читать схему bot")
		}
	})
}

// TestInjectionAttempts — параметризованный SQL и валидация не допускают инъекций.
func TestInjectionAttempts(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()

	if _, err := s.messaging.NotificationStatus(ctx, "x'; DROP TABLE bot.outbound_messages; --"); err == nil {
		t.Error("ключ с кавычками должен отвергаться валидацией")
	}
	text := "текст'); DROP TABLE bot.outbound_messages; --"
	s.enqueue(t, "injection-test-0001", 1001, text)
	stored := s.status(t, "injection-test-0001")
	if stored.Text != text {
		t.Errorf("текст изменён при сохранении: %q", stored.Text)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages`); n != 1 {
		t.Errorf("таблица повреждена: %d строк", n)
	}

	// Текст пользователя не сохраняется, но событие webhook фиксируется.
	rec := domain.InboundRecord{
		Key:        domain.NewDedupeKey([]byte(`{"t":"injection"}`)),
		UpdateType: "bot_started'; DELETE FROM bot.recipients; --",
		EventTime:  s.clock.Now(), Outcome: domain.OutcomeIgnored, ReceivedAt: s.clock.Now(),
	}
	if _, err := s.inbound.Insert(ctx, rec); err != nil {
		t.Fatalf("запись события: %v", err)
	}
	var storedType string
	if err := s.pool.DB(ctx).QueryRow(ctx,
		`SELECT update_type FROM bot.inbound_updates WHERE dedupe_key = $1`, rec.Key[:]).Scan(&storedType); err != nil {
		t.Fatalf("чтение события: %v", err)
	}
	if storedType != rec.UpdateType {
		t.Errorf("тип события изменён: %q", storedType)
	}
}
