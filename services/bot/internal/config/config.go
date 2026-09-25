// Package config описывает конфигурацию bot-service.
// Полный словарь переменных — docs/operations/configuration.md.
package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"

	platformconfig "vovremya/internal/platform/config"
	"vovremya/services/bot/internal/domain"
)

// Config — параметры запуска сервиса.
type Config struct {
	AppEnv     string `env:"APP_ENV" envDefault:"local"`
	AppVersion string `env:"APP_VERSION" envDefault:"dev"`
	LogLevel   string `env:"LOG_LEVEL" envDefault:"info"`

	Mode string `env:"BOT_MODE" envDefault:"stub"`

	HTTPAddr  string `env:"BOT_HTTP_ADDR" envDefault:":8080"`
	AdminAddr string `env:"BOT_ADMIN_ADDR" envDefault:":8081"`
	GRPCAddr  string `env:"BOT_GRPC_ADDR" envDefault:":9090"`

	DatabaseURL        string `env:"BOT_DATABASE_URL"`
	MigrateDatabaseURL string `env:"BOT_MIGRATE_DATABASE_URL"`
	DBMaxConns         int32  `env:"BOT_DB_MAX_CONNS" envDefault:"8"`

	MaxAPIBaseURL     string        `env:"BOT_MAX_API_BASE_URL" envDefault:"https://platform-api2.max.ru"`
	MaxToken          string        `env:"BOT_MAX_TOKEN"`
	MaxExtraCAFile    string        `env:"BOT_MAX_EXTRA_CA_FILE" envDefault:"/etc/vovremya/ca/russian_trusted_ca_bundle.pem"`
	MaxRequestTimeout time.Duration `env:"BOT_MAX_REQUEST_TIMEOUT" envDefault:"10s"`

	WebhookPublicURL        string        `env:"BOT_WEBHOOK_PUBLIC_URL"`
	WebhookSecret           string        `env:"BOT_WEBHOOK_SECRET"`
	WebhookUpdateTypes      []string      `env:"BOT_WEBHOOK_UPDATE_TYPES" envSeparator:"," envDefault:"bot_started,bot_stopped,dialog_removed,dialog_muted,dialog_unmuted,message_created"`
	SubscriptionCheckPeriod time.Duration `env:"BOT_SUBSCRIPTION_CHECK_INTERVAL" envDefault:"10m"`
	WebhookMaxBodyBytes     int64         `env:"BOT_WEBHOOK_MAX_BODY_BYTES" envDefault:"262144"`
	WebhookHandlerTimeout   time.Duration `env:"BOT_WEBHOOK_TIMEOUT" envDefault:"10s"`
	GlobalRPS               float64       `env:"BOT_GLOBAL_RPS" envDefault:"20"`
	PerRecipientInterval    time.Duration `env:"BOT_PER_RECIPIENT_INTERVAL" envDefault:"600ms"`
	Workers                 int           `env:"BOT_WORKERS" envDefault:"4"`
	WorkerBatch             int           `env:"BOT_WORKER_BATCH" envDefault:"50"`
	WorkerPollInterval      time.Duration `env:"BOT_WORKER_POLL_INTERVAL" envDefault:"500ms"`
	Lease                   time.Duration `env:"BOT_LEASE" envDefault:"60s"`
	MaxAttempts             int           `env:"BOT_MAX_ATTEMPTS" envDefault:"8"`
	RetryBase               time.Duration `env:"BOT_RETRY_BASE" envDefault:"5s"`
	RetryMax                time.Duration `env:"BOT_RETRY_MAX" envDefault:"10m"`
	QueueLimit              int64         `env:"BOT_QUEUE_LIMIT" envDefault:"50000"`
	StubUsername            string        `env:"BOT_STUB_USERNAME" envDefault:"vovremya_local_bot"`
	OpenAppButtonKind       string        `env:"BOT_OPEN_APP_BUTTON_KIND" envDefault:"link"`
	ProfileRetryInterval    time.Duration `env:"BOT_PROFILE_RETRY_INTERVAL" envDefault:"30s"`
	LeaseReapInterval       time.Duration `env:"BOT_LEASE_REAP_INTERVAL" envDefault:"30s"`
	QueueDepthRefreshPeriod time.Duration `env:"BOT_QUEUE_DEPTH_INTERVAL" envDefault:"5s"`
	RetentionInterval       time.Duration `env:"BOT_RETENTION_INTERVAL" envDefault:"1h"`
	InboundTTL              time.Duration `env:"BOT_INBOUND_TTL" envDefault:"168h"`
	FinalizedTTL            time.Duration `env:"BOT_FINALIZED_TTL" envDefault:"720h"`
	StoppedRecipientTTL     time.Duration `env:"BOT_STOPPED_RECIPIENT_TTL" envDefault:"720h"`
	HandlerTimeout          time.Duration `env:"BOT_HANDLER_TIMEOUT" envDefault:"10s"`
	ShutdownTimeout         time.Duration `env:"BOT_SHUTDOWN_TIMEOUT" envDefault:"25s"`
	GRPCGracefulStopTimeout time.Duration `env:"BOT_GRPC_STOP_TIMEOUT" envDefault:"5s"`
	HTTPShutdownStopTimeout time.Duration `env:"BOT_HTTP_STOP_TIMEOUT" envDefault:"20s"`
}

var webhookSecretRe = regexp.MustCompile(`^[A-Za-z0-9_-]{5,256}$`)

// Load читает конфигурацию из окружения с учётом выполняемой команды:
// migrate не требует параметров работы сервиса.
func Load(command string) (Config, error) {
	if err := platformconfig.ExpandFileRefs("BOT_DATABASE_URL", "BOT_MIGRATE_DATABASE_URL",
		"BOT_MAX_TOKEN", "BOT_WEBHOOK_SECRET"); err != nil {
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
		"BOT_DATABASE_URL":         cfg.DatabaseURL,
		"BOT_MIGRATE_DATABASE_URL": cfg.MigrateDatabaseURL,
		"BOT_MAX_TOKEN":            cfg.MaxToken,
		"BOT_WEBHOOK_SECRET":       cfg.WebhookSecret,
	}); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// LiveMode сообщает, работает ли сервис с настоящим MAX Bot API.
func (c Config) LiveMode() bool { return domain.Mode(c.Mode) == domain.ModeLive }

// BotMode возвращает режим канала.
func (c Config) BotMode() domain.Mode { return domain.Mode(c.Mode) }

func (c Config) validate(command string) error {
	if c.AppEnv != "local" && c.AppEnv != "prod" {
		return fmt.Errorf("APP_ENV: допустимо local или prod, получено %q", c.AppEnv)
	}
	if c.Mode != string(domain.ModeStub) && c.Mode != string(domain.ModeLive) {
		return fmt.Errorf("BOT_MODE: допустимо stub или live, получено %q", c.Mode)
	}
	if c.AppEnv == "prod" && !c.LiveMode() {
		// Запуск с режимом-заглушкой в prod запрещён (ADR-031).
		return fmt.Errorf("BOT_MODE: в APP_ENV=prod допустим только live")
	}
	switch command {
	case "migrate":
		if c.MigrateDatabaseURL == "" && c.DatabaseURL == "" {
			return fmt.Errorf("BOT_MIGRATE_DATABASE_URL: обязательная переменная")
		}
		return nil
	case "healthcheck":
		return nil
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("BOT_DATABASE_URL: обязательная переменная")
	}
	if !webhookSecretRe.MatchString(c.WebhookSecret) {
		return fmt.Errorf("BOT_WEBHOOK_SECRET: ожидается ^[A-Za-z0-9_-]{5,256}$")
	}
	if c.AppEnv == "prod" && len(c.WebhookSecret) < 64 {
		return fmt.Errorf("BOT_WEBHOOK_SECRET: в prod требуется не менее 64 символов")
	}
	if len(c.WebhookUpdateTypes) == 0 {
		return fmt.Errorf("BOT_WEBHOOK_UPDATE_TYPES: список не может быть пустым")
	}
	if c.OpenAppButtonKind != "link" && c.OpenAppButtonKind != "open_app" {
		return fmt.Errorf("BOT_OPEN_APP_BUTTON_KIND: допустимо link или open_app, получено %q", c.OpenAppButtonKind)
	}
	if c.LiveMode() {
		if c.MaxToken == "" {
			return fmt.Errorf("BOT_MAX_TOKEN: обязательна в режиме live")
		}
		if !strings.HasPrefix(c.WebhookPublicURL, "https://") {
			return fmt.Errorf("BOT_WEBHOOK_PUBLIC_URL: обязателен адрес https:// (F-44)")
		}
		if c.MaxExtraCAFile == "" {
			return fmt.Errorf("BOT_MAX_EXTRA_CA_FILE: обязателен в режиме live (F-40)")
		}
	}
	if c.AppEnv == "prod" && !strings.HasPrefix(c.MaxAPIBaseURL, "https://") {
		return fmt.Errorf("BOT_MAX_API_BASE_URL: в prod допустим только https://")
	}
	if c.GlobalRPS <= 0 || c.GlobalRPS > 30 {
		return fmt.Errorf("BOT_GLOBAL_RPS: ожидается значение в (0; 30] (F-43), получено %v", c.GlobalRPS)
	}
	if c.PerRecipientInterval < 500*time.Millisecond {
		return fmt.Errorf("BOT_PER_RECIPIENT_INTERVAL: не меньше 500ms (F-50)")
	}
	if c.Workers < 1 || c.Workers > 64 {
		return fmt.Errorf("BOT_WORKERS: ожидается 1..64, получено %d", c.Workers)
	}
	if c.WorkerBatch < 1 || c.WorkerBatch > 1000 {
		return fmt.Errorf("BOT_WORKER_BATCH: ожидается 1..1000, получено %d", c.WorkerBatch)
	}
	if c.Lease <= c.MaxRequestTimeout {
		return fmt.Errorf("BOT_LEASE должен быть больше BOT_MAX_REQUEST_TIMEOUT")
	}
	if c.MaxAttempts < 1 || c.MaxAttempts > 20 {
		return fmt.Errorf("BOT_MAX_ATTEMPTS: ожидается 1..20, получено %d", c.MaxAttempts)
	}
	if c.RetryBase <= 0 || c.RetryMax < c.RetryBase {
		return fmt.Errorf("BOT_RETRY_BASE и BOT_RETRY_MAX: ожидается 0 < base ≤ max")
	}
	if c.QueueLimit < 1 {
		return fmt.Errorf("BOT_QUEUE_LIMIT: ожидается положительное число")
	}
	if c.WebhookMaxBodyBytes < 1024 {
		return fmt.Errorf("BOT_WEBHOOK_MAX_BODY_BYTES: ожидается не менее 1024")
	}
	if c.ShutdownTimeout <= 0 || c.ShutdownTimeout > 30*time.Second {
		return fmt.Errorf("BOT_SHUTDOWN_TIMEOUT: ожидается (0; 30s] (NFR-10)")
	}
	return nil
}
