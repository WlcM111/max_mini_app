// Package config описывает конфигурацию reminders-service.
// Полный словарь переменных — docs/operations/configuration.md.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"

	platformconfig "vovremya/internal/platform/config"
)

// Config — параметры запуска сервиса.
type Config struct {
	AppEnv     string `env:"APP_ENV" envDefault:"local"`
	AppVersion string `env:"APP_VERSION" envDefault:"dev"`
	LogLevel   string `env:"LOG_LEVEL" envDefault:"info"`

	GRPCAddr  string `env:"REMINDERS_GRPC_ADDR" envDefault:":9091"`
	AdminAddr string `env:"REMINDERS_ADMIN_ADDR" envDefault:":8081"`

	DatabaseURL        string `env:"REMINDERS_DATABASE_URL"`
	MigrateDatabaseURL string `env:"REMINDERS_MIGRATE_DATABASE_URL"`
	DBMaxConns         int32  `env:"REMINDERS_DB_MAX_CONNS" envDefault:"10"`

	BotGRPCAddr   string        `env:"REMINDERS_BOT_GRPC_ADDR" envDefault:"bot:9090"`
	BotRPCTimeout time.Duration `env:"REMINDERS_BOT_RPC_TIMEOUT" envDefault:"2s"`

	SchedulerInterval    time.Duration `env:"REMINDERS_SCHEDULER_INTERVAL" envDefault:"15s"`
	SchedulerBatch       int           `env:"REMINDERS_SCHEDULER_BATCH" envDefault:"200"`
	SchedulerLease       time.Duration `env:"REMINDERS_SCHEDULER_LEASE" envDefault:"2m"`
	SchedulerConcurrency int           `env:"REMINDERS_SCHEDULER_CONCURRENCY" envDefault:"4"`
	ReminderGrace        time.Duration `env:"REMINDERS_GRACE" envDefault:"24h"`

	RetryBase   time.Duration `env:"REMINDERS_RETRY_BASE" envDefault:"10s"`
	RetryMax    time.Duration `env:"REMINDERS_RETRY_MAX" envDefault:"5m"`
	RetryJitter time.Duration `env:"REMINDERS_RETRY_JITTER" envDefault:"5s"`

	RetentionInterval time.Duration `env:"REMINDERS_RETENTION_INTERVAL" envDefault:"1h"`
	InboxTTL          time.Duration `env:"REMINDERS_INBOX_TTL" envDefault:"168h"`
	FinalizedTTL      time.Duration `env:"REMINDERS_FINALIZED_TTL" envDefault:"720h"`

	HandlerTimeout  time.Duration `env:"REMINDERS_HANDLER_TIMEOUT" envDefault:"10s"`
	ShutdownTimeout time.Duration `env:"REMINDERS_SHUTDOWN_TIMEOUT" envDefault:"25s"`
	MaxBatchEvents  int           `env:"REMINDERS_MAX_BATCH_EVENTS" envDefault:"200"`
}

// Load читает конфигурацию из окружения, раскрывает файловые ссылки секретов
// и проверяет запрет dev-значений в prod.
func Load() (Config, error) {
	if err := platformconfig.ExpandFileRefs(
		"REMINDERS_DATABASE_URL", "REMINDERS_MIGRATE_DATABASE_URL"); err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	if err := platformconfig.VerifySecrets(cfg.AppEnv, map[string]string{
		"REMINDERS_DATABASE_URL":         cfg.DatabaseURL,
		"REMINDERS_MIGRATE_DATABASE_URL": cfg.MigrateDatabaseURL,
	}); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.AppEnv != "local" && c.AppEnv != "prod" {
		return fmt.Errorf("APP_ENV: допустимо local или prod, получено %q", c.AppEnv)
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("REMINDERS_DATABASE_URL: обязательная переменная")
	}
	if c.SchedulerBatch <= 0 || c.SchedulerBatch > 1000 {
		return fmt.Errorf("REMINDERS_SCHEDULER_BATCH: допустимо 1..1000")
	}
	if c.SchedulerConcurrency <= 0 || c.SchedulerConcurrency > 64 {
		return fmt.Errorf("REMINDERS_SCHEDULER_CONCURRENCY: допустимо 1..64")
	}
	if c.SchedulerLease < c.BotRPCTimeout {
		return fmt.Errorf("REMINDERS_SCHEDULER_LEASE должен быть не меньше REMINDERS_BOT_RPC_TIMEOUT")
	}
	if c.MaxBatchEvents <= 0 || c.MaxBatchEvents > 1000 {
		return fmt.Errorf("REMINDERS_MAX_BATCH_EVENTS: допустимо 1..1000")
	}
	return nil
}
