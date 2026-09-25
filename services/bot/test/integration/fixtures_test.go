// Пакет integration проверяет bot-service на реальной PostgreSQL:
// репозитории, сценарии, gRPC, webhook, доставку, конкурентность и отказы.
package integration_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/bot/internal/adapters/postgres"
	"vovremya/services/bot/internal/app"
	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
	"vovremya/services/bot/test/testutil"
)

var testStart = time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)

type stack struct {
	pool       *pgkit.Pool
	clock      *testutil.Clock
	messages   *postgres.MessageRepo
	recipients *postgres.RecipientRepo
	inbound    *postgres.InboundRepo
	profiles   *app.ProfileStore
	depth      *app.QueueDepth
	metrics    *app.Metrics
	messaging  *app.MessagingService
	webhook    *app.WebhookService
	delivery   *app.Delivery
	reaper     *app.LeaseReaper
	monitor    *app.QueueMonitor
	retention  *app.RetentionJob
	max        *testutil.FakeMax
	limiter    *testutil.NoLimit
	log        *slog.Logger
}

type stackOptions struct {
	queueLimit  int64
	maxAttempts int
	lease       time.Duration
	batch       int
	profile     *domain.Profile
}

func newStack(t *testing.T, opt stackOptions) *stack {
	t.Helper()
	if opt.queueLimit == 0 {
		opt.queueLimit = 50000
	}
	if opt.maxAttempts == 0 {
		opt.maxAttempts = 8
	}
	if opt.lease == 0 {
		opt.lease = time.Minute
	}
	if opt.batch == 0 {
		opt.batch = 50
	}
	pool := testutil.Pool(t)
	clock := testutil.NewClock(testStart)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := app.NewMetrics(metrics.New())

	s := &stack{
		pool:       pool,
		clock:      clock,
		messages:   postgres.NewMessageRepo(pool, nil),
		recipients: postgres.NewRecipientRepo(pool, nil),
		inbound:    postgres.NewInboundRepo(pool, nil),
		profiles:   app.NewProfileStore(domain.ModeStub),
		depth:      &app.QueueDepth{},
		metrics:    m,
		max:        testutil.NewFakeMax(domain.Profile{UserID: 500100, Username: "vovremya_local_bot", DisplayName: "Вовремя"}),
		limiter:    &testutil.NoLimit{},
		log:        log,
	}
	profile := domain.Profile{UserID: 500100, Username: "vovremya_local_bot", DisplayName: "Вовремя"}
	if opt.profile != nil {
		profile = *opt.profile
	}
	if opt.profile == nil || opt.profile.Username != "" {
		s.profiles.Set(profile)
	}
	s.messaging = app.NewMessagingService(pool, s.messages, s.recipients, s.profiles, s.depth, clock, opt.queueLimit)
	s.webhook = app.NewWebhookService(pool, s.inbound, s.recipients, s.messages, s.profiles, clock, log, m)
	s.delivery = app.NewDelivery(pool, s.messages, s.recipients, s.max, s.limiter, s.profiles, clock,
		testutil.Random{Value: 0.5}, app.DeliveryConfig{
			Workers: 1, Batch: opt.batch, PollInterval: 10 * time.Millisecond, Lease: opt.lease,
			RequestTimeout: 2 * time.Second, ProfileWait: 30 * time.Second,
			Policy: domain.RetryPolicy{MaxAttempts: opt.maxAttempts, Backoff: domain.DefaultBackoff},
		}, log, m)
	s.reaper = app.NewLeaseReaper(s.messages, clock, log, m)
	s.monitor = app.NewQueueMonitor(s.messages, s.depth, m, log)
	s.retention = app.NewRetentionJob(s.inbound, s.messages, s.recipients, clock, app.RetentionConfig{
		Interval: time.Hour, InboundTTL: 168 * time.Hour, FinalTTL: 720 * time.Hour, StoppedTTL: 720 * time.Hour,
	}, log, m)
	return s
}

// enqueue ставит в очередь напоминание в формате reminders-service.
func (s *stack) enqueue(t *testing.T, key string, recipient int64, text string, buttons ...domain.Button) domain.Notification {
	t.Helper()
	req := domain.EnqueueRequest{
		IdempotencyKey: key,
		Message: domain.Message{
			Kind: domain.KindReminder, RecipientMaxUserID: recipient, Text: text, Buttons: buttons,
		},
		NotAfter: s.clock.Now().Add(24 * time.Hour),
	}
	res, err := s.messaging.Enqueue(context.Background(), req, domain.ContentHash(req))
	if err != nil {
		t.Fatalf("постановка в очередь %s: %v", key, err)
	}
	return res.Notification
}

func (s *stack) status(t *testing.T, key string) domain.Notification {
	t.Helper()
	n, err := s.messaging.NotificationStatus(context.Background(), key)
	if err != nil {
		t.Fatalf("чтение состояния %s: %v", key, err)
	}
	return n
}

// newDeliveryWithClient собирает воркер доставки с другим клиентом MAX.
func newDeliveryWithClient(s *stack, client ports.MaxClient) *app.Delivery {
	return app.NewDelivery(s.pool, s.messages, s.recipients, client, s.limiter, s.profiles, s.clock,
		testutil.Random{Value: 0.5}, app.DeliveryConfig{
			Workers: 1, Batch: 3, PollInterval: 10 * time.Millisecond, Lease: time.Minute,
			RequestTimeout: 2 * time.Second, ProfileWait: 30 * time.Second,
			Policy: domain.RetryPolicy{MaxAttempts: 8, Backoff: domain.DefaultBackoff},
		}, s.log, s.metrics)
}

func docButton(documentID string) domain.Button {
	return domain.Button{Text: "Открыть документ", Action: domain.ActionOpenApp, Payload: "doc_" + documentID}
}
