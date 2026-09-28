// Команда reminders — точка входа reminders-service.
//
//	reminders serve        запуск gRPC-сервера, планировщика и служебного HTTP
//	reminders migrate up   применение миграций схемы reminders
//	reminders migrate down откат последней миграции
//	reminders healthcheck  проверка готовности (используется в HEALTHCHECK образа)
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

	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
	"vovremya/internal/platform/adminhttp"
	"vovremya/internal/platform/grpckit"
	"vovremya/internal/platform/lifecycle"
	"vovremya/internal/platform/logging"
	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/reminders/internal/adapters/botgrpc"
	"vovremya/services/reminders/internal/adapters/clock"
	"vovremya/services/reminders/internal/adapters/grpcserver"
	"vovremya/services/reminders/internal/adapters/postgres"
	"vovremya/services/reminders/internal/app"
	"vovremya/services/reminders/internal/config"
	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/migrations"
)

const serviceName = "reminders"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "reminders: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
	}
	cfg, err := config.Load()
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

// migrate применяет или откатывает миграции схемы reminders.
// Используется отдельная роль-владелец схемы (REMINDERS_MIGRATE_DATABASE_URL).
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: статус %d", resp.StatusCode)
	}
	return nil
}

// serve — composition root: сборка зависимостей и запуск компонентов.
func serve(cfg config.Config, log *slog.Logger) error {
	ctx := context.Background()
	m := metrics.New()

	pool, err := pgkit.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns, serviceName)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()

	inboxRepo := postgres.NewInboxRepo(pool, m)
	projRepo := postgres.NewProjectionRepo(pool, m)
	reminderRepo := postgres.NewReminderRepo(pool, m)
	sysClock := clock.System{}

	botConn, err := grpc.NewClient(cfg.BotGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpckit.UnaryClientInterceptor(m)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time: 30 * time.Second, Timeout: 10 * time.Second, PermitWithoutStream: false,
		}))
	if err != nil {
		return fmt.Errorf("bot grpc client: %w", err)
	}
	defer func() { _ = botConn.Close() }()
	botClient := botgrpc.NewClient(botConn, cfg.BotRPCTimeout)

	appMetrics := app.NewMetrics(m)
	replanner := app.NewReplanner(projRepo, reminderRepo, sysClock, cfg.ReminderGrace)
	ingestService := app.NewIngestService(pool, inboxRepo, projRepo, replanner, sysClock, log, appMetrics)
	queryService := app.NewQueryService(reminderRepo, projRepo)
	scheduler := app.NewScheduler(reminderRepo, projRepo, botClient, sysClock, app.SchedulerConfig{
		Interval:    cfg.SchedulerInterval,
		Batch:       cfg.SchedulerBatch,
		Lease:       cfg.SchedulerLease,
		Grace:       cfg.ReminderGrace,
		Concurrency: cfg.SchedulerConcurrency,
		Backoff: domain.BackoffParams{
			Base: cfg.RetryBase, Max: cfg.RetryMax, Jitter: cfg.RetryJitter,
		},
	}, log, appMetrics)
	retention := app.NewRetentionJob(inboxRepo, reminderRepo, sysClock, app.RetentionConfig{
		Interval:     cfg.RetentionInterval,
		InboxTTL:     cfg.InboxTTL,
		FinalizedTTL: cfg.FinalizedTTL,
	}, log, appMetrics)

	grpcSrv := grpc.NewServer(
		grpc.UnaryInterceptor(grpckit.UnaryServerInterceptor(log, m, cfg.HandlerTimeout)),
		grpc.MaxRecvMsgSize(4*1024*1024),
		grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: 5 * time.Minute}),
	)
	remindersv1.RegisterIngestServiceServer(grpcSrv, grpcserver.NewIngestServer(ingestService, log, cfg.MaxBatchEvents))
	remindersv1.RegisterReminderQueryServiceServer(grpcSrv, grpcserver.NewQueryServer(queryService, log))
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcSrv, healthSrv)
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	admin := adminhttp.New(cfg.AdminAddr, m.Gatherer(), func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return err
		}
		m.DBPoolAcquired.Set(float64(pool.AcquiredConns()))
		return nil
	}, 2*time.Second)

	runner := lifecycle.New(log, cfg.ShutdownTimeout)
	// Порядок остановки: сначала перестаём принимать запросы и фоновые задания,
	// затем закрываем соединения.
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
			grpcSrv.GracefulStop()
			return nil
		},
	})
	runner.Add(lifecycle.Component{
		Name:  "scheduler",
		Start: scheduler.Run,
	})
	// Недельная сводка: по понедельникам во время напоминаний участника.
	digest := app.NewDigestRunner(projRepo, botClient, sysClock, log)
	runner.Add(lifecycle.Component{
		Name:  "digest",
		Start: digest.Run,
	})
	runner.Add(lifecycle.Component{
		Name:  "retention",
		Start: retention.Run,
	})
	runner.Add(lifecycle.Component{
		Name:  "admin",
		Start: func(context.Context) error { return admin.Start() },
		Stop:  admin.Shutdown,
	})

	log.Info("reminders-service started",
		slog.String("env", cfg.AppEnv),
		slog.String("grpc_addr", cfg.GRPCAddr),
		slog.String("admin_addr", cfg.AdminAddr),
		slog.String("bot_addr", cfg.BotGRPCAddr))
	return runner.Run(ctx)
}
