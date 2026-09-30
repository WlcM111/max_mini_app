// Команда bot — точка входа bot-service.
//
//	bot serve        запуск webhook, gRPC-сервера, воркеров доставки и служебного HTTP
//	bot migrate up   применение миграций схемы bot
//	bot migrate down откат миграций схемы bot
//	bot healthcheck  проверка готовности (используется в HEALTHCHECK образа)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"vovremya/internal/platform/adminhttp"
	"vovremya/internal/platform/grpckit"
	"vovremya/internal/platform/lifecycle"
	"vovremya/internal/platform/logging"
	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/bot/internal/adapters/clock"
	"vovremya/services/bot/internal/adapters/grpcserver"
	"vovremya/services/bot/internal/adapters/maxapi"
	"vovremya/services/bot/internal/adapters/postgres"
	"vovremya/services/bot/internal/adapters/ratelimit"
	"vovremya/services/bot/internal/adapters/remindersgrpc"
	"vovremya/services/bot/internal/adapters/webhook"
	"vovremya/services/bot/internal/app"
	"vovremya/services/bot/internal/config"
	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
	"vovremya/services/bot/migrations"
)

const serviceName = "bot"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "bot: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
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
		if len(args) > 1 {
			direction = args[1]
		}
		return migrate(cfg, log, direction)
	case "healthcheck":
		return healthcheck(cfg)
	default:
		return fmt.Errorf("неизвестная команда %q: допустимо serve, migrate, healthcheck", command)
	}
}

func migrate(cfg config.Config, log *slog.Logger, direction string) error {
	dsn := cfg.MigrateDatabaseURL
	if dsn == "" {
		dsn = cfg.DatabaseURL
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	switch direction {
	case "up":
		if err := pgkit.MigrateUp(ctx, dsn, migrations.FS, serviceName); err != nil {
			return err
		}
		log.Info("migrations applied", slog.String("schema", serviceName))
		return nil
	case "down":
		if err := pgkit.MigrateDownTo(ctx, dsn, migrations.FS, serviceName, 0); err != nil {
			return err
		}
		log.Info("migrations reverted", slog.String("schema", serviceName))
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

// serve — composition root: сборка зависимостей и запуск компонентов.
func serve(cfg config.Config, log *slog.Logger) error {
	ctx := context.Background()
	m := metrics.New()
	appMetrics := app.NewMetrics(m)

	pool, err := pgkit.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns, serviceName)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()

	messageRepo := postgres.NewMessageRepo(pool, m)
	recipientRepo := postgres.NewRecipientRepo(pool, m)
	inboundRepo := postgres.NewInboundRepo(pool, m)

	sysClock := clock.System{}
	profiles := app.NewProfileStore(cfg.BotMode())

	var (
		maxClient ports.MaxClient
		stub      *maxapi.Stub
	)
	buttonKind := maxapi.ButtonKind(cfg.OpenAppButtonKind)
	if cfg.LiveMode() {
		maxClient, err = maxapi.New(maxapi.Config{
			BaseURL:        cfg.MaxAPIBaseURL,
			Token:          cfg.MaxToken,
			Timeout:        cfg.MaxRequestTimeout,
			ButtonKind:     buttonKind,
			ExtraCAFile:    cfg.MaxExtraCAFile,
			RequireExtraCA: true,
		}, log, m)
		if err != nil {
			return fmt.Errorf("max api client: %w", err)
		}
	} else {
		stub = maxapi.NewStub(cfg.StubUsername, buttonKind, log, func() time.Time { return sysClock.Now() })
		maxClient = stub
	}

	limiter := ratelimit.New(cfg.GlobalRPS, cfg.PerRecipientInterval)
	depth := &app.QueueDepth{}
	messaging := app.NewMessagingService(pool, messageRepo, recipientRepo, profiles, depth, sysClock, cfg.QueueLimit)
	delivery := app.NewDelivery(pool, messageRepo, recipientRepo, maxClient, limiter, profiles, sysClock,
		clock.Random{}, app.DeliveryConfig{
			Workers:        cfg.Workers,
			Batch:          cfg.WorkerBatch,
			PollInterval:   cfg.WorkerPollInterval,
			Lease:          cfg.Lease,
			RequestTimeout: cfg.MaxRequestTimeout,
			ProfileWait:    cfg.ProfileRetryInterval,
			Policy: domain.RetryPolicy{
				MaxAttempts: cfg.MaxAttempts,
				Backoff:     domain.Backoff{Base: cfg.RetryBase, Max: cfg.RetryMax},
			},
		}, log, appMetrics)
	reaper := app.NewLeaseReaper(messageRepo, sysClock, log, appMetrics)
	reaper.Interval = cfg.LeaseReapInterval
	monitor := app.NewQueueMonitor(messageRepo, depth, appMetrics, log)
	monitor.Interval = cfg.QueueDepthRefreshPeriod
	webhookService := app.NewWebhookService(pool, inboundRepo, recipientRepo, messageRepo, profiles, sysClock, log, appMetrics)
	webhookService.UseCallbackAnswers(maxClient) // «Напомнить через неделю»
	// Отложенный повтор ведёт reminders-service (ADR-036); соединение устанавливается лениво.
	remindersConn, err := grpc.NewClient(cfg.RemindersGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpckit.UnaryClientInterceptor(m)))
	if err != nil {
		return fmt.Errorf("reminders grpc client: %w", err)
	}
	defer func() { _ = remindersConn.Close() }()
	webhookService.UseSnoozer(remindersgrpc.NewClient(remindersConn, cfg.RemindersRPCTimeout))
	profileLoader := app.NewProfileLoader(maxClient, profiles, log)
	profileLoader.RetryInterval = cfg.ProfileRetryInterval
	retention := app.NewRetentionJob(inboundRepo, messageRepo, recipientRepo, sysClock, app.RetentionConfig{
		Interval:   cfg.RetentionInterval,
		InboundTTL: cfg.InboundTTL,
		FinalTTL:   cfg.FinalizedTTL,
		StoppedTTL: cfg.StoppedRecipientTTL,
	}, log, appMetrics)

	grpcSrv := grpc.NewServer(
		grpc.UnaryInterceptor(grpckit.UnaryServerInterceptor(log, m, cfg.HandlerTimeout)),
		grpc.KeepaliveParams(keepalive.ServerParameters{Time: 30 * time.Second, Timeout: 10 * time.Second}),
		grpc.MaxRecvMsgSize(1<<20),
	)
	grpcserver.New(messaging, log).Register(grpcSrv)
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcSrv, healthSrv)
	if cfg.AppEnv == "local" {
		reflection.Register(grpcSrv)
	}
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	webhookHandler := webhook.NewHandler(webhookService, cfg.WebhookSecret, cfg.WebhookMaxBodyBytes,
		cfg.WebhookHandlerTimeout, log, m)
	webhookSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           webhookHandler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	admin := adminhttp.New(cfg.AdminAddr, m.Gatherer(), func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return err
		}
		m.DBPoolAcquired.Set(float64(pool.AcquiredConns()))
		return nil
	}, 2*time.Second)
	if stub != nil {
		// Буфер локального режима: сообщения не уходят в MAX (spec §12).
		admin.Handle("GET /debug/stub/messages", stub.Handler())
	}

	runner := lifecycle.New(log, cfg.ShutdownTimeout)
	// Порядок остановки: сначала перестаём принимать webhook и gRPC,
	// затем останавливаем фоновые задания и служебный HTTP.
	runner.Add(lifecycle.Component{
		Name: "webhook-http",
		Start: func(context.Context) error {
			log.Info("webhook server started", slog.String("addr", cfg.HTTPAddr))
			if err := webhookSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
		Stop: func(ctx context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(ctx, cfg.HTTPShutdownStopTimeout)
			defer cancel()
			return webhookSrv.Shutdown(shutdownCtx)
		},
	})
	runner.Add(lifecycle.Component{
		Name: "grpc",
		Start: func(context.Context) error {
			lis, err := net.Listen("tcp", cfg.GRPCAddr)
			if err != nil {
				return fmt.Errorf("listen %s: %w", cfg.GRPCAddr, err)
			}
			log.Info("grpc server started", slog.String("addr", cfg.GRPCAddr))
			if err := grpcSrv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				return err
			}
			return nil
		},
		Stop: func(context.Context) error {
			healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
			done := make(chan struct{})
			go func() {
				grpcSrv.GracefulStop()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(cfg.GRPCGracefulStopTimeout):
				grpcSrv.Stop()
			}
			return nil
		},
	})
	runner.Add(lifecycle.Component{Name: "delivery", Start: delivery.Run})
	runner.Add(lifecycle.Component{Name: "lease-reaper", Start: reaper.Run})
	runner.Add(lifecycle.Component{Name: "queue-monitor", Start: monitor.Run})
	runner.Add(lifecycle.Component{Name: "profile-loader", Start: profileLoader.Run})
	if cfg.LiveMode() {
		keeper := app.NewSubscriptionKeeper(maxClient, cfg.WebhookPublicURL, cfg.WebhookUpdateTypes,
			cfg.WebhookSecret, cfg.SubscriptionCheckPeriod, log, appMetrics)
		runner.Add(lifecycle.Component{Name: "subscription", Start: keeper.Run})
	}
	runner.Add(lifecycle.Component{Name: "retention", Start: retention.Run})
	runner.Add(lifecycle.Component{
		Name:  "admin",
		Start: func(context.Context) error { return admin.Start() },
		Stop:  admin.Shutdown,
	})

	log.Info("bot-service started",
		slog.String("env", cfg.AppEnv),
		slog.String("mode", cfg.Mode),
		slog.String("http_addr", cfg.HTTPAddr),
		slog.String("grpc_addr", cfg.GRPCAddr),
		slog.String("admin_addr", cfg.AdminAddr),
		slog.String("max_api", cfg.MaxAPIBaseURL))
	return runner.Run(ctx)
}
