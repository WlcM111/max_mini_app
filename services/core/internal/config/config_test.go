package config_test

import (
	"strings"
	"testing"

	"vovremya/services/core/internal/config"
)

func setEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for k, v := range values {
		t.Setenv(k, v)
	}
}

func baseEnv() map[string]string {
	return map[string]string{
		"CORE_DATABASE_URL":          "postgres://core_app:devonly_core_app@postgres:5432/vovremya",
		"CORE_MAX_WEBAPP_SECRET_HEX": strings.Repeat("ab", 32),
		"CORE_PUBLIC_BASE_URL":       "https://example.test",
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, baseEnv())
	cfg, err := config.Load("serve")
	if err != nil {
		t.Fatalf("загрузка конфигурации: %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.AdminAddr != ":8081" {
		t.Fatalf("адреса по умолчанию: %s, %s", cfg.HTTPAddr, cfg.AdminAddr)
	}
	if cfg.SyncFlushTimeout.String() != "300ms" {
		t.Fatalf("CORE_REMINDERS_SYNC_FLUSH_TIMEOUT по умолчанию 300ms, получено %s", cfg.SyncFlushTimeout)
	}
	if cfg.RemindersGRPCAddr != "reminders:9091" || cfg.BotGRPCAddr != "bot:9090" {
		t.Fatalf("адреса соседних сервисов: %s, %s", cfg.RemindersGRPCAddr, cfg.BotGRPCAddr)
	}
}

func TestProdRejectsTestVectorSecret(t *testing.T) {
	env := baseEnv()
	env["APP_ENV"] = "prod"
	env["CORE_MAX_WEBAPP_SECRET_HEX"] = config.DevSecretHex
	setEnv(t, env)
	_, err := config.Load("serve")
	if err == nil || !strings.Contains(err.Error(), "insecure default secret") {
		t.Fatalf("ожидался отказ из-за dev-ключа, получено %v", err)
	}
}

func TestProdRequiresHTTPSPublicURL(t *testing.T) {
	env := baseEnv()
	env["APP_ENV"] = "prod"
	env["CORE_PUBLIC_BASE_URL"] = "http://example.test"
	setEnv(t, env)
	if _, err := config.Load("serve"); err == nil {
		t.Fatal("в prod требуется https")
	}
}

func TestMigrateRequiresMigrationDSN(t *testing.T) {
	setEnv(t, map[string]string{"CORE_MIGRATE_DATABASE_URL": ""})
	if _, err := config.Load("migrate"); err == nil {
		t.Fatal("ожидалось требование CORE_MIGRATE_DATABASE_URL")
	}
}

func TestSecretLengthValidated(t *testing.T) {
	env := baseEnv()
	env["CORE_MAX_WEBAPP_SECRET_HEX"] = "abcd"
	setEnv(t, env)
	if _, err := config.Load("serve"); err == nil {
		t.Fatal("ожидалась проверка длины ключа")
	}
}

func TestOutboxBatchBounded(t *testing.T) {
	env := baseEnv()
	env["CORE_OUTBOX_BATCH"] = "500"
	setEnv(t, env)
	if _, err := config.Load("serve"); err == nil {
		t.Fatal("пакет ApplyEvents ограничен 200 событиями")
	}
}
