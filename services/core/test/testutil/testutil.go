// Package testutil — средства автономных тестов core-service: изолированная
// PostgreSQL, управляемые часы, контролируемые двойники соседних сервисов
// и подписанные данные запуска MAX. Используется только тестами.
package testutil

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/core/internal/adapters/httpapi"
	"vovremya/services/core/internal/adapters/maxlaunch"
	"vovremya/services/core/internal/adapters/postgres"
	"vovremya/services/core/internal/app"
	"vovremya/services/core/internal/config"
	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
	"vovremya/services/core/migrations"
)

// EnvDSN — переменная окружения с DSN администратора тестовой PostgreSQL.
const EnvDSN = "CORE_TEST_DATABASE_URL"

// SecretHex — ключ мини-приложения из тест-векторов (только для тестов).
const SecretHex = "e45f53166d1da778700f29abbf89f6ac247864cb97356f927707e56f0066d663"

var (
	once      sync.Once
	sharedDSN string
	sharedErr error
)

// DSN возвращает DSN тестовой базы, создавая её при первом обращении.
func DSN(t *testing.T) string {
	t.Helper()
	admin := os.Getenv(EnvDSN)
	if admin == "" {
		t.Skipf("%s не задан: интеграционный тест пропущен", EnvDSN)
	}
	once.Do(func() {
		sharedDSN, sharedErr = CreateDatabase(admin, fmt.Sprintf("core_test_%d", time.Now().UnixNano()))
	})
	if sharedErr != nil {
		t.Fatalf("подготовка тестовой базы: %v", sharedErr)
	}
	return sharedDSN
}

// CreateDatabase создаёт базу, схему core, применяет миграции и справочник.
func CreateDatabase(adminDSN, name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return "", fmt.Errorf("connect admin: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		return "", fmt.Errorf("create database: %w", err)
	}
	dsn, err := ReplaceDatabase(adminDSN, name)
	if err != nil {
		return "", err
	}
	dbConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return "", fmt.Errorf("connect test database: %w", err)
	}
	defer func() { _ = dbConn.Close(ctx) }()
	if _, err := dbConn.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS core`); err != nil {
		return "", fmt.Errorf("create schema: %w", err)
	}
	if err := pgkit.MigrateUp(ctx, dsn, migrations.FS, "core"); err != nil {
		return "", fmt.Errorf("migrate: %w", err)
	}
	return dsn, nil
}

// ReplaceDatabase подменяет имя базы в DSN.
func ReplaceDatabase(dsn, name string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	u.Path = "/" + name
	return u.String(), nil
}

// Pool открывает пул к общей тестовой базе и очищает данные перед тестом.
func Pool(t *testing.T) *pgkit.Pool {
	t.Helper()
	pool, err := pgkit.Open(context.Background(), DSN(t), 10, "core-test")
	if err != nil {
		t.Fatalf("открытие пула: %v", err)
	}
	t.Cleanup(pool.Close)
	Truncate(t, pool)
	return pool
}

// Truncate очищает пользовательские таблицы, сохраняя справочник.
func Truncate(t *testing.T, pool *pgkit.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.DB(ctx).Exec(ctx, `TRUNCATE core.outbox_events, core.audit_events, core.calendar_exports,
		core.document_reminder_offsets, core.document_periods, core.documents, core.invites,
		core.memberships, core.organization_features, core.organizations, core.sessions, core.accounts
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("очистка таблиц: %v", err)
	}
}

// Clock — управляемые часы.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// NewClock создаёт часы с заданным моментом.
func NewClock(now time.Time) *Clock { return &Clock{now: now.UTC()} }

// Now возвращает текущий момент часов.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance сдвигает часы вперёд.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// FakeBot — контролируемый двойник bot-service (нормативный контракт
// MessagingService). Применяется только в автономных тестах core.
type FakeBot struct {
	mu            sync.Mutex
	Notifications []ports.NotificationRequest
	Status        ports.RecipientStatus
	Profile       ports.BotProfile
	Fail          bool
}

// NewFakeBot создаёт двойника с типовым профилем бота.
func NewFakeBot() *FakeBot {
	return &FakeBot{
		Status: ports.RecipientStatus{State: "active", BotChatURL: "https://max.ru/vovremya_local_bot"},
		Profile: ports.BotProfile{
			Username: "vovremya_local_bot", DisplayName: "Вовремя",
			ChatURL:             "https://max.ru/vovremya_local_bot",
			OpenAppLinkTemplate: "https://max.ru/vovremya_local_bot?startapp={payload}",
		},
	}
}

// EnqueueNotification записывает задание на отправку.
func (f *FakeBot) EnqueueNotification(_ context.Context, req ports.NotificationRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail {
		return fmt.Errorf("bot недоступен")
	}
	f.Notifications = append(f.Notifications, req)
	return nil
}

// GetRecipientStatus возвращает состояние канала.
func (f *FakeBot) GetRecipientStatus(_ context.Context, _ int64) (ports.RecipientStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail {
		return ports.RecipientStatus{}, fmt.Errorf("bot недоступен")
	}
	return f.Status, nil
}

// GetBotProfile возвращает профиль бота.
func (f *FakeBot) GetBotProfile(context.Context) (ports.BotProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail {
		return ports.BotProfile{}, domain.ErrDependencyUnavailable
	}
	return f.Profile, nil
}

// Queued возвращает копию списка заданий.
func (f *FakeBot) Queued() []ports.NotificationRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ports.NotificationRequest, len(f.Notifications))
	copy(out, f.Notifications)
	return out
}

// FakeReminders — контролируемый двойник reminders-service.
type FakeReminders struct {
	mu        sync.Mutex
	Applied   []ports.IngestEvent
	Next      map[string]time.Time
	Fail      bool
	FailApply bool
	Rejected  map[string]bool
}

// NewFakeReminders создаёт двойника без плана напоминаний.
func NewFakeReminders() *FakeReminders {
	return &FakeReminders{Next: map[string]time.Time{}, Rejected: map[string]bool{}}
}

// ApplyEvents принимает события и возвращает исходы по контракту.
func (f *FakeReminders) ApplyEvents(_ context.Context, _ string, events []ports.IngestEvent) ([]ports.IngestResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail || f.FailApply {
		return nil, fmt.Errorf("reminders недоступен")
	}
	results := make([]ports.IngestResult, 0, len(events))
	for _, e := range events {
		f.Applied = append(f.Applied, e)
		outcome := "applied"
		if f.Rejected[e.EventType] {
			outcome = "rejected"
		}
		results = append(results, ports.IngestResult{EventID: e.EventID, Outcome: outcome})
	}
	return results, nil
}

// GetNextReminders возвращает заданный план напоминаний.
func (f *FakeReminders) GetNextReminders(_ context.Context, _ string, documentIDs []string) ([]ports.NextReminder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail {
		return nil, fmt.Errorf("reminders недоступен")
	}
	out := make([]ports.NextReminder, 0, len(documentIDs))
	for _, id := range documentIDs {
		if due, ok := f.Next[id]; ok {
			out = append(out, ports.NextReminder{DocumentID: id, DueAt: due, DaysBefore: 7})
		}
	}
	return out, nil
}

// GetSyncStatus сообщает о применении версии агрегата.
func (f *FakeReminders) GetSyncStatus(context.Context, string, string, int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail {
		return false, fmt.Errorf("reminders недоступен")
	}
	return true, nil
}

// AppliedEvents возвращает копию принятых событий.
func (f *FakeReminders) AppliedEvents() []ports.IngestEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ports.IngestEvent, len(f.Applied))
	copy(out, f.Applied)
	return out
}

// SetFail включает или выключает отказ двойника целиком.
func (f *FakeReminders) SetFail(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Fail = v
}

// SetFailApply отключает только приём событий, оставляя чтение плана рабочим.
func (f *FakeReminders) SetFailApply(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.FailApply = v
}

// SetNext задаёт ближайшее напоминание по документу.
func (f *FakeReminders) SetNext(documentID string, due time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Next[documentID] = due
}

// Harness — собранный сервис для тестов: сценарии, HTTP API и двойники.
type Harness struct {
	App       *app.App
	Pool      *pgkit.Pool
	Bot       *FakeBot
	Reminders *FakeReminders
	Clock     *Clock
	Server    *httptest.Server
	Config    config.Config
}

// NewHarness собирает core-service поверх изолированной PostgreSQL.
func NewHarness(t *testing.T) *Harness {
	t.Helper()
	pool := Pool(t)
	clock := NewClock(time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC))
	bot := NewFakeBot()
	reminders := NewFakeReminders()

	cfg := config.Config{
		AppEnv: "local", LogLevel: "error", HTTPAddr: ":0", AdminAddr: ":0",
		MaxWebAppSecretHex: SecretHex, LaunchMaxAge: time.Hour, LaunchFutureSkew: time.Minute,
		SessionTTL: 12 * time.Hour, PublicBaseURL: "http://localhost:8080",
		HTTPMaxInflight: 64, HandlerTimeout: 5 * time.Second,
		RateAccountRPS: 1000, RateAccountBurst: 1000, RateSessionPerUser: 1000, RateSessionPerIP: 1000,
		RateInvitePerMinute: 1000, RateEventsPerMinute: 1000,
		InviteTTL: 72 * time.Hour, ExportTTL: 10 * time.Minute,
		OutboxBatch: 50, OutboxLease: time.Minute, RelayInterval: time.Second, RelayConcurrency: 1,
		RetentionInterval: time.Hour, SessionRetention: 720 * time.Hour,
		AuditRetention: 4320 * time.Hour, OutboxRetention: 168 * time.Hour,
		BotRPCTimeout: 2 * time.Second, RemindersRPCTimeout: 2 * time.Second,
		SyncFlushTimeout: 300 * time.Millisecond, BotProfileCacheTTL: 10 * time.Minute,
	}
	verifier, err := maxlaunch.New(cfg.MaxWebAppSecretHex, cfg.LaunchMaxAge, cfg.LaunchFutureSkew)
	if err != nil {
		t.Fatalf("проверяющий данных запуска: %v", err)
	}
	m := metrics.New()
	application := app.New(app.Deps{
		Tx:       pool,
		Accounts: postgres.NewAccountRepo(pool, m),
		Sessions: postgres.NewSessionRepo(pool, m),
		Catalog:  postgres.NewCatalogRepo(pool, m),
		Orgs:     postgres.NewOrganizationRepo(pool, m),
		Docs:     postgres.NewDocumentRepo(pool, m),
		Invites:  postgres.NewInviteRepo(pool, m),
		Exports:  postgres.NewExportRepo(pool, m),
		Audit:    postgres.NewAuditRepo(pool, m),
		Outbox:   postgres.NewOutboxRepo(pool, m),
		Launch:   verifier, Bot: bot, Reminders: reminders,
		Clock: clock, Random: randomSource{}, Log: slog.New(slog.NewTextHandler(os.Stderr,
			&slog.HandlerOptions{Level: slog.LevelError})),
		Metrics: app.NewMetrics(m),
		Settings: app.Settings{
			SessionTTL: cfg.SessionTTL, InviteTTL: cfg.InviteTTL, ExportTTL: cfg.ExportTTL,
			PublicBaseURL: cfg.PublicBaseURL, BotProfileCacheTTL: cfg.BotProfileCacheTTL,
			BotRPCTimeout: cfg.BotRPCTimeout, RemindersRPCTimeout: cfg.RemindersRPCTimeout,
			SyncFlushTimeout: cfg.SyncFlushTimeout, OutboxBatch: cfg.OutboxBatch,
			OutboxLease: cfg.OutboxLease, RelayInterval: cfg.RelayInterval,
			RelayConcurrency: cfg.RelayConcurrency, RetentionInterval: cfg.RetentionInterval,
			SessionRetention: cfg.SessionRetention, AuditRetention: cfg.AuditRetention,
			OutboxRetention: cfg.OutboxRetention,
		},
	})
	if err := application.LoadCatalog(context.Background()); err != nil {
		t.Fatalf("загрузка справочника: %v", err)
	}
	server := httptest.NewServer(httpapi.New(application, cfg, slog.New(slog.NewTextHandler(os.Stderr,
		&slog.HandlerOptions{Level: slog.LevelError}))).Handler())
	t.Cleanup(server.Close)
	return &Harness{App: application, Pool: pool, Bot: bot, Reminders: reminders,
		Clock: clock, Server: server, Config: cfg}
}

type randomSource struct{}

func (randomSource) Float64() float64 { return 0.5 }

// Token возвращает непрозрачный токен из 43 символов base64url.
func (randomSource) Token() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// UUID возвращает UUID версии 4.
func (randomSource) UUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	h := hex.EncodeToString(buf)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// SignInitData собирает подписанную строку данных запуска MAX.
func SignInitData(t *testing.T, params map[string]string) string {
	t.Helper()
	secret, err := hex.DecodeString(SecretHex)
	if err != nil {
		t.Fatalf("ключ: %v", err)
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+params[k])
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.Join(lines, "\n")))
	parts := make([]string, 0, len(keys)+1)
	for _, k := range keys {
		parts = append(parts, k+"="+url.PathEscape(params[k]))
	}
	parts = append(parts, "hash="+hex.EncodeToString(mac.Sum(nil)))
	return strings.Join(parts, "&")
}

// LaunchData собирает данные запуска для пользователя MAX.
func LaunchData(t *testing.T, userID int64, firstName, startParam string, now time.Time) string {
	t.Helper()
	params := map[string]string{
		"auth_date": fmt.Sprintf("%d", now.Unix()),
		"query_id":  fmt.Sprintf("q-%d", userID),
		"user":      fmt.Sprintf(`{"id":%d,"first_name":%q,"last_name":null,"username":null,"language_code":"ru"}`, userID, firstName),
	}
	if startParam != "" {
		params["start_param"] = startParam
	}
	return SignInitData(t, params)
}

// NoRedirectClient — HTTP-клиент тестов.
func NoRedirectClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}
