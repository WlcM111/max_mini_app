// Package config описывает конфигурацию core-service.
// Полный словарь переменных — docs/operations/configuration.md.
package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"

	platformconfig "vovremya/internal/platform/config"
)

// DevSecretHex — ключ мини-приложения из тест-векторов; в prod запрещён.
const DevSecretHex = "e45f53166d1da778700f29abbf89f6ac247864cb97356f927707e56f0066d663"

// Config — параметры запуска сервиса.
type Config struct {
	AppEnv     string `env:"APP_ENV" envDefault:"local"`
	AppVersion string `env:"APP_VERSION" envDefault:"dev"`
	LogLevel   string `env:"LOG_LEVEL" envDefault:"info"`

	HTTPAddr  string `env:"CORE_HTTP_ADDR" envDefault:":8080"`
	AdminAddr string `env:"CORE_ADMIN_ADDR" envDefault:":8081"`

	DatabaseURL        string `env:"CORE_DATABASE_URL"`
	MigrateDatabaseURL string `env:"CORE_MIGRATE_DATABASE_URL"`
	DBMaxConns         int32  `env:"CORE_DB_MAX_CONNS" envDefault:"20"`

	MaxWebAppSecretHex string        `env:"CORE_MAX_WEBAPP_SECRET_HEX"`
	LaunchMaxAge       time.Duration `env:"CORE_LAUNCH_MAX_AGE" envDefault:"1h"`
	LaunchFutureSkew   time.Duration `env:"CORE_LAUNCH_FUTURE_SKEW" envDefault:"60s"`
	SessionTTL         time.Duration `env:"CORE_SESSION_TTL" envDefault:"12h"`

	BotGRPCAddr         string        `env:"CORE_BOT_GRPC_ADDR" envDefault:"bot:9090"`
	BotRPCTimeout       time.Duration `env:"CORE_BOT_RPC_TIMEOUT" envDefault:"2s"`
	BotProfileCacheTTL  time.Duration `env:"CORE_BOT_PROFILE_CACHE_TTL" envDefault:"10m"`
	RemindersGRPCAddr   string        `env:"CORE_REMINDERS_GRPC_ADDR" envDefault:"reminders:9091"`
	RemindersRPCTimeout time.Duration `env:"CORE_REMINDERS_RPC_TIMEOUT" envDefault:"2s"`
	SyncFlushTimeout    time.Duration `env:"CORE_REMINDERS_SYNC_FLUSH_TIMEOUT" envDefault:"300ms"`

	PublicBaseURL     string   `env:"CORE_PUBLIC_BASE_URL" envDefault:"http://localhost:8080"`
	TrustedProxyCIDRs []string `env:"CORE_TRUSTED_PROXY_CIDRS" envSeparator:"," envDefault:"10.77.1.0/24"`

	HTTPMaxInflight int           `env:"CORE_HTTP_MAX_INFLIGHT" envDefault:"256"`
	HandlerTimeout  time.Duration `env:"CORE_HANDLER_TIMEOUT" envDefault:"5s"`

	RateAccountRPS      float64 `env:"CORE_RATE_ACCOUNT_RPS" envDefault:"10"`
	RateAccountBurst    int     `env:"CORE_RATE_ACCOUNT_BURST" envDefault:"30"`
	RateSessionPerUser  int     `env:"CORE_RATE_SESSION_PER_USER_MIN" envDefault:"10"`
	RateSessionPerIP    int     `env:"CORE_RATE_SESSION_PER_IP_MIN" envDefault:"300"`
	RateInvitePerMinute int     `env:"CORE_RATE_INVITE_PER_MIN" envDefault:"10"`
	RateEventsPerMinute int     `env:"CORE_RATE_CLIENT_EVENTS_PER_MIN" envDefault:"60"`

	InviteTTL time.Duration `env:"CORE_INVITE_TTL" envDefault:"72h"`
	ExportTTL time.Duration `env:"CORE_EXPORT_TTL" envDefault:"10m"`

	OutboxBatch       int           `env:"CORE_OUTBOX_BATCH" envDefault:"100"`
	OutboxLease       time.Duration `env:"CORE_OUTBOX_LEASE" envDefault:"60s"`
	RelayInterval     time.Duration `env:"CORE_RELAY_INTERVAL" envDefault:"2s"`
	RelayConcurrency  int           `env:"CORE_RELAY_CONCURRENCY" envDefault:"4"`
	RetentionInterval time.Duration `env:"CORE_RETENTION_INTERVAL" envDefault:"1h"`
	SessionRetention  time.Duration `env:"CORE_SESSION_RETENTION" envDefault:"720h"`
	AuditRetention    time.Duration `env:"CORE_AUDIT_RETENTION" envDefault:"4320h"`
	OutboxRetention   time.Duration `env:"CORE_OUTBOX_RETENTION" envDefault:"168h"`

	ShutdownTimeout time.Duration `env:"CORE_SHUTDOWN_TIMEOUT" envDefault:"25s"`
}

// Load читает конфигурацию окружения для указанной команды.
func Load(command string) (Config, error) {
	if err := platformconfig.ExpandFileRefs("CORE_DATABASE_URL", "CORE_MIGRATE_DATABASE_URL",
		"CORE_MAX_WEBAPP_SECRET_HEX"); err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}
	if err := cfg.validate(command); err != nil {
		return Config{}, err
	}
	if err := platformconfig.VerifySecrets(cfg.AppEnv, map[string]string{
		"CORE_DATABASE_URL":         cfg.DatabaseURL,
		"CORE_MIGRATE_DATABASE_URL": cfg.MigrateDatabaseURL,
	}); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate(command string) error {
	if c.AppEnv != "local" && c.AppEnv != "prod" {
		return fmt.Errorf("APP_ENV: допустимо local или prod, получено %q", c.AppEnv)
	}
	switch command {
	case "migrate":
		if c.MigrateDatabaseURL == "" {
			return fmt.Errorf("CORE_MIGRATE_DATABASE_URL: обязательна для команды migrate")
		}
		return nil
	case "healthcheck":
		return nil
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("CORE_DATABASE_URL: обязательна")
	}
	secret := strings.TrimSpace(c.MaxWebAppSecretHex)
	if len(secret) != 64 {
		return fmt.Errorf("CORE_MAX_WEBAPP_SECRET_HEX: ожидается 64 шестнадцатеричных символа")
	}
	if c.AppEnv == "prod" && strings.EqualFold(secret, DevSecretHex) {
		return fmt.Errorf("insecure default secret: CORE_MAX_WEBAPP_SECRET_HEX содержит ключ тест-векторов")
	}
	if u, err := url.Parse(c.PublicBaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("CORE_PUBLIC_BASE_URL: ожидается абсолютный адрес")
	}
	if c.AppEnv == "prod" && !strings.HasPrefix(c.PublicBaseURL, "https://") {
		return fmt.Errorf("CORE_PUBLIC_BASE_URL: в prod требуется https")
	}
	if c.HTTPMaxInflight <= 0 {
		return fmt.Errorf("CORE_HTTP_MAX_INFLIGHT: ожидается положительное число")
	}
	if c.OutboxBatch <= 0 || c.OutboxBatch > 200 {
		return fmt.Errorf("CORE_OUTBOX_BATCH: ожидается 1…200 (предел пакета ApplyEvents)")
	}
	if c.RelayConcurrency <= 0 || c.RelayConcurrency > 4 {
		return fmt.Errorf("CORE_RELAY_CONCURRENCY: ожидается 1…4")
	}
	return nil
}
