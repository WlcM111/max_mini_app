// Команда core — публичный HTTP API мини-приложения «Вовремя» и вся
// предметная логика: сессии, организации, документы, приглашения, экспорт,
// доставка событий в reminders-service.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"vovremya/demo"
	"vovremya/internal/platform/adminhttp"
	"vovremya/internal/platform/lifecycle"
	"vovremya/internal/platform/logging"
	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/core/internal/adapters/botgrpc"
	"vovremya/services/core/internal/adapters/clock"
	"vovremya/services/core/internal/adapters/httpapi"
	"vovremya/services/core/internal/adapters/maxlaunch"
	"vovremya/services/core/internal/adapters/postgres"
	"vovremya/services/core/internal/adapters/remindersgrpc"
	"vovremya/services/core/internal/app"
	"vovremya/services/core/internal/config"
	"vovremya/services/core/internal/domain"
	"vovremya/services/core/migrations"
)

const serviceName = "core"

// demoOrganizationID — организация демонстрационных данных (handoff §8).
const demoOrganizationID = "0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "core:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}
	cfg, err := config.Load(command)
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel, serviceName, cfg.AppVersion)

	switch command {
	case "serve":
		return serve(cfg, log)
	case "migrate":
		direction := "up"
		if len(args) > 0 {
			direction = args[0]
		}
		return migrate(cfg, log, direction)
	case "healthcheck":
		return healthcheck(cfg)
	case "review-token":
		return reviewToken(cfg, log, args)
	case "seed-demo":
		return seedDemo(cfg, log)
	default:
		return fmt.Errorf("неизвестная команда %q: допустимо serve, migrate, healthcheck, review-token, seed-demo", command)
	}
}

// migrate применяет или откатывает миграции схемы core.
func migrate(cfg config.Config, log *slog.Logger, direction string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	switch direction {
	case "up":
		if err := pgkit.MigrateUp(ctx, cfg.MigrateDatabaseURL, migrations.FS, serviceName); err != nil {
			return err
		}
		log.Info("migrations applied")
		return nil
	case "down":
		if err := pgkit.MigrateDownTo(ctx, cfg.MigrateDatabaseURL, migrations.FS, serviceName, 0); err != nil {
			return err
		}
		log.Info("migrations reverted")
		return nil
	default:
		return fmt.Errorf("migrate: допустимо up или down, получено %q", direction)
	}
}

// healthcheck обращается к собственному admin-порту: используется в образе.
func healthcheck(cfg config.Config) error {
	addr := cfg.AdminAddr
	if len(addr) > 0 && addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/readyz")
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: статус %d", resp.StatusCode)
	}
	return nil
}

// dependencies собирает сценарии и ресурсы, общие для serve и команд CLI.
type dependencies struct {
	app       *app.App
	pool      *pgkit.Pool
	metrics   *metrics.Registry
	bot       *botgrpc.Client
	reminders *remindersgrpc.Client
}

func (d dependencies) Close() {
	if d.bot != nil {
		_ = d.bot.Close()
	}
	if d.reminders != nil {
		_ = d.reminders.Close()
	}
	if d.pool != nil {
		d.pool.Close()
	}
}

func build(ctx context.Context, cfg config.Config, log *slog.Logger) (dependencies, error) {
	m := metrics.New()
	appMetrics := app.NewMetrics(m)

	pool, err := pgkit.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns, serviceName)
	if err != nil {
		return dependencies{}, err
	}
	verifier, err := maxlaunch.New(cfg.MaxWebAppSecretHex, cfg.LaunchMaxAge, cfg.LaunchFutureSkew)
	if err != nil {
		pool.Close()
		return dependencies{}, err
	}
	botClient, err := botgrpc.New(cfg.BotGRPCAddr)
	if err != nil {
		pool.Close()
		return dependencies{}, err
	}
	remindersClient, err := remindersgrpc.New(cfg.RemindersGRPCAddr)
	if err != nil {
		_ = botClient.Close()
		pool.Close()
		return dependencies{}, err
	}

	application := app.New(app.Deps{
		Tx:        pool,
		Accounts:  postgres.NewAccountRepo(pool, m),
		Sessions:  postgres.NewSessionRepo(pool, m),
		Catalog:   postgres.NewCatalogRepo(pool, m),
		Orgs:      postgres.NewOrganizationRepo(pool, m),
		Docs:      postgres.NewDocumentRepo(pool, m),
		Invites:   postgres.NewInviteRepo(pool, m),
		Exports:   postgres.NewExportRepo(pool, m),
		Audit:     postgres.NewAuditRepo(pool, m),
		Outbox:    postgres.NewOutboxRepo(pool, m),
		Launch:    verifier,
		Bot:       botClient,
		Reminders: remindersClient,
		Clock:     clock.System{},
		Random:    clock.Random{},
		Log:       log,
		Metrics:   appMetrics,
		Settings: app.Settings{
			SessionTTL:          cfg.SessionTTL,
			InviteTTL:           cfg.InviteTTL,
			ExportTTL:           cfg.ExportTTL,
			PublicBaseURL:       cfg.PublicBaseURL,
			BotProfileCacheTTL:  cfg.BotProfileCacheTTL,
			BotRPCTimeout:       cfg.BotRPCTimeout,
			RemindersRPCTimeout: cfg.RemindersRPCTimeout,
			SyncFlushTimeout:    cfg.SyncFlushTimeout,
			OutboxBatch:         cfg.OutboxBatch,
			OutboxLease:         cfg.OutboxLease,
			RelayInterval:       cfg.RelayInterval,
			RelayConcurrency:    cfg.RelayConcurrency,
			RetentionInterval:   cfg.RetentionInterval,
			SessionRetention:    cfg.SessionRetention,
			AuditRetention:      cfg.AuditRetention,
			OutboxRetention:     cfg.OutboxRetention,
		},
	})
	if err := application.LoadCatalog(ctx); err != nil {
		_ = remindersClient.Close()
		_ = botClient.Close()
		pool.Close()
		return dependencies{}, fmt.Errorf("загрузка справочника: %w", err)
	}
	return dependencies{app: application, pool: pool, metrics: m, bot: botClient, reminders: remindersClient}, nil
}

// serve — composition root: сборка зависимостей и запуск компонентов.
func serve(cfg config.Config, log *slog.Logger) error {
	ctx := context.Background()
	deps, err := build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer deps.Close()

	api := httpapi.New(deps.app, cfg, log)
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	admin := adminhttp.New(cfg.AdminAddr, deps.metrics.Gatherer(), func(ctx context.Context) error {
		return deps.pool.Ping(ctx)
	}, 500*time.Millisecond)

	runner := lifecycle.New(log, cfg.ShutdownTimeout)
	// Порядок остановки: сначала перестаём принимать запросы API,
	// затем останавливаем фоновые задания и служебный HTTP.
	runner.Add(lifecycle.Component{
		Name: "public-http",
		Start: func(context.Context) error {
			log.Info("public api started", slog.String("addr", cfg.HTTPAddr))
			if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
		Stop: func(ctx context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			return httpSrv.Shutdown(shutdownCtx)
		},
	})
	runner.Add(lifecycle.Component{Name: "outbox-relay", Start: deps.app.RunRelay})
	runner.Add(lifecycle.Component{Name: "retention", Start: deps.app.RunRetention})
	runner.Add(lifecycle.Component{
		Name:  "admin",
		Start: func(context.Context) error { return admin.Start() },
		Stop:  admin.Shutdown,
	})

	log.Info("core-service started",
		slog.String("env", cfg.AppEnv),
		slog.String("http_addr", cfg.HTTPAddr),
		slog.String("admin_addr", cfg.AdminAddr),
		slog.String("bot_addr", cfg.BotGRPCAddr),
		slog.String("reminders_addr", cfg.RemindersGRPCAddr))
	return runner.Run(ctx)
}

// reviewToken выдаёт или отзывает сессию учётной записи проверяющего.
func reviewToken(cfg config.Config, log *slog.Logger, args []string) error {
	if len(args) == 0 {
		return errors.New("review-token: укажите issue или revoke")
	}
	action := args[0]
	fs := flag.NewFlagSet("review-token", flag.ContinueOnError)
	login := fs.String("login", "", "логин учётной записи проверяющего")
	role := fs.String("role", "viewer", "роль: editor или viewer")
	ttl := fs.Duration("ttl", 24*time.Hour, "срок действия токена, не более 720h")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *login == "" {
		return errors.New("review-token: обязателен --login")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deps, err := build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer deps.Close()

	switch action {
	case "issue":
		token, expiresAt, err := deps.app.IssueReviewToken(ctx, *login, domain.Role(*role), *ttl, demoOrganizationID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "демонстрационная организация не найдена: сначала выполните seed-demo")
				os.Exit(2)
			}
			return err
		}
		fmt.Println(token)
		fmt.Fprintln(os.Stderr, "срок действия:", expiresAt.Format(time.RFC3339))
		return nil
	case "revoke":
		if err := deps.app.RevokeReviewTokens(ctx, *login); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "сессии отозваны")
		return nil
	default:
		return fmt.Errorf("review-token: допустимо issue или revoke, получено %q", action)
	}
}

// seedDemo идемпотентно загружает демонстрационные данные.
func seedDemo(cfg config.Config, log *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	deps, err := build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer deps.Close()
	orgID, err := deps.app.SeedDemo(ctx, demo.Data)
	if err != nil {
		return err
	}
	log.Info("demo data loaded", slog.String("organization_id", orgID))
	return nil
}
