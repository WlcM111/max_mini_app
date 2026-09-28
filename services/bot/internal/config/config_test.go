package config_test

import (
	"slices"
	"strings"
	"testing"

	"vovremya/services/bot/internal/config"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	base := map[string]string{
		"APP_ENV":            "local",
		"BOT_MODE":           "stub",
		"BOT_DATABASE_URL":   "postgres://bot_app:devonly_bot_app@127.0.0.1:5432/vovremya?sslmode=disable",
		"BOT_WEBHOOK_SECRET": "devonly-webhook-secret",
	}
	for k, v := range kv {
		base[k] = v
	}
	for k, v := range base {
		if v == "" {
			t.Setenv(k, "")
			continue
		}
		t.Setenv(k, v)
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, nil)
	cfg, err := config.Load("serve")
	if err != nil {
		t.Fatalf("конфигурация по умолчанию отвергнута: %v", err)
	}
	if cfg.GlobalRPS != 20 || cfg.PerRecipientInterval.String() != "600ms" {
		t.Errorf("лимиты по умолчанию: %v %v (ожидались 20 rps и 600ms, F-43/F-50)", cfg.GlobalRPS, cfg.PerRecipientInterval)
	}
	if cfg.Workers != 4 || cfg.WorkerBatch != 50 || cfg.Lease.String() != "1m0s" || cfg.MaxAttempts != 8 {
		t.Errorf("параметры доставки по умолчанию: %+v", cfg)
	}
	if cfg.QueueLimit != 50000 || cfg.OpenAppButtonKind != "link" {
		t.Errorf("backpressure или вид кнопки: %d %s", cfg.QueueLimit, cfg.OpenAppButtonKind)
	}
	if len(cfg.WebhookUpdateTypes) != 7 || !slices.Contains(cfg.WebhookUpdateTypes, "message_callback") {
		t.Errorf("типы подписки: %v (ожидались 7 типов, включая message_callback для кнопки отложенного напоминания)",
			cfg.WebhookUpdateTypes)
	}
	if cfg.LiveMode() {
		t.Error("по умолчанию ожидается режим stub")
	}
}

func TestLoadRejects(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"stub в prod запрещён", map[string]string{"APP_ENV": "prod", "BOT_MODE": "stub"}, "BOT_MODE"},
		{"live без токена", map[string]string{"BOT_MODE": "live"}, "BOT_MAX_TOKEN"},
		{"live без публичного URL", map[string]string{"BOT_MODE": "live", "BOT_MAX_TOKEN": "t"}, "BOT_WEBHOOK_PUBLIC_URL"},
		{"live с http webhook", map[string]string{"BOT_MODE": "live", "BOT_MAX_TOKEN": "t",
			"BOT_WEBHOOK_PUBLIC_URL": "http://example.org/max/webhook"}, "BOT_WEBHOOK_PUBLIC_URL"},
		{"секрет короче 5", map[string]string{"BOT_WEBHOOK_SECRET": "abc"}, "BOT_WEBHOOK_SECRET"},
		{"секрет с недопустимым символом", map[string]string{"BOT_WEBHOOK_SECRET": "abc def!"}, "BOT_WEBHOOK_SECRET"},
		{"нет адреса базы", map[string]string{"BOT_DATABASE_URL": ""}, "BOT_DATABASE_URL"},
		{"скорость выше предела MAX", map[string]string{"BOT_GLOBAL_RPS": "31"}, "BOT_GLOBAL_RPS"},
		{"интервал получателя меньше 500ms", map[string]string{"BOT_PER_RECIPIENT_INTERVAL": "100ms"}, "BOT_PER_RECIPIENT_INTERVAL"},
		{"аренда меньше таймаута MAX", map[string]string{"BOT_LEASE": "5s", "BOT_MAX_REQUEST_TIMEOUT": "10s"}, "BOT_LEASE"},
		{"неизвестный вид кнопки", map[string]string{"BOT_OPEN_APP_BUTTON_KIND": "inline"}, "BOT_OPEN_APP_BUTTON_KIND"},
		{"срок остановки больше 30 с", map[string]string{"BOT_SHUTDOWN_TIMEOUT": "45s"}, "BOT_SHUTDOWN_TIMEOUT"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setEnv(t, c.env)
			_, err := config.Load("serve")
			if err == nil {
				t.Fatalf("ожидалась ошибка про %s", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("ошибка не называет %s: %v", c.want, err)
			}
		})
	}
}

func TestProdRejectsDevSecrets(t *testing.T) {
	setEnv(t, map[string]string{
		"APP_ENV": "prod", "BOT_MODE": "live", "BOT_MAX_TOKEN": "devonly-local-bot-token",
		"BOT_WEBHOOK_PUBLIC_URL": "https://example.org/max/webhook",
		"BOT_WEBHOOK_SECRET":     strings.Repeat("s", 64),
	})
	if _, err := config.Load("serve"); err == nil {
		t.Fatal("dev-токен в prod должен отвергаться (ADR-015)")
	}
	setEnv(t, map[string]string{
		"APP_ENV": "prod", "BOT_MODE": "live", "BOT_MAX_TOKEN": "real-token",
		"BOT_WEBHOOK_PUBLIC_URL": "https://example.org/max/webhook",
		"BOT_WEBHOOK_SECRET":     "short-but-valid",
	})
	if _, err := config.Load("serve"); err == nil {
		t.Fatal("короткий секрет webhook в prod должен отвергаться")
	}
}

func TestMigrateCommandNeedsOnlyMigrationDSN(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("BOT_MODE", "stub")
	t.Setenv("BOT_DATABASE_URL", "")
	t.Setenv("BOT_WEBHOOK_SECRET", "")
	t.Setenv("BOT_MIGRATE_DATABASE_URL", "postgres://bot_migrator:devonly_bot_migrator@127.0.0.1:5432/vovremya")
	if _, err := config.Load("migrate"); err != nil {
		t.Fatalf("команда migrate не должна требовать параметров сервиса: %v", err)
	}
}
