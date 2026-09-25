// Package testutil — вспомогательные средства автономных тестов reminders-service:
// изолированная PostgreSQL, управляемые часы и контролируемый двойник bot-service.
// Используется только тестами и не входит в production-сборку.
package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"vovremya/internal/platform/pgkit"
	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
	"vovremya/services/reminders/migrations"
)

// EnvDSN — переменная окружения с DSN администратора тестовой PostgreSQL.
const EnvDSN = "REMINDERS_TEST_DATABASE_URL"

var (
	once      sync.Once
	sharedDSN string
	sharedErr error
)

// DSN возвращает DSN тестовой базы, создавая её при первом обращении.
// Если переменная окружения не задана, тест пропускается: интеграционные
// проверки требуют реальной PostgreSQL (ADR-027).
func DSN(t *testing.T) string {
	t.Helper()
	admin := os.Getenv(EnvDSN)
	if admin == "" {
		t.Skipf("%s не задан: интеграционный тест пропущен", EnvDSN)
	}
	once.Do(func() {
		sharedDSN, sharedErr = createDatabase(admin, fmt.Sprintf("reminders_test_%d", time.Now().UnixNano()))
	})
	if sharedErr != nil {
		t.Fatalf("подготовка тестовой базы: %v", sharedErr)
	}
	return sharedDSN
}

// NewDatabase создаёт отдельную пустую базу с применёнными миграциями.
// Возвращает DSN; используется тестами прав доступа и миграций.
func NewDatabase(t *testing.T, suffix string) string {
	t.Helper()
	admin := os.Getenv(EnvDSN)
	if admin == "" {
		t.Skipf("%s не задан: интеграционный тест пропущен", EnvDSN)
	}
	dsn, err := createDatabase(admin, fmt.Sprintf("reminders_%s_%d", suffix, time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("создание базы: %v", err)
	}
	return dsn
}

func createDatabase(adminDSN, name string) (string, error) {
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
	dsn, err := replaceDatabase(adminDSN, name)
	if err != nil {
		return "", err
	}
	dbConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return "", fmt.Errorf("connect test database: %w", err)
	}
	defer func() { _ = dbConn.Close(ctx) }()
	if _, err := dbConn.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS reminders`); err != nil {
		return "", fmt.Errorf("create schema: %w", err)
	}
	if err := pgkit.MigrateUp(ctx, dsn, migrations.FS, "reminders"); err != nil {
		return "", fmt.Errorf("migrate: %w", err)
	}
	return dsn, nil
}

func replaceDatabase(dsn, name string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	u.Path = "/" + name
	return u.String(), nil
}

// Pool открывает пул к общей тестовой базе и очищает таблицы перед тестом.
func Pool(t *testing.T) *pgkit.Pool {
	t.Helper()
	dsn := DSN(t)
	pool, err := pgkit.Open(context.Background(), dsn, 10, "reminders-test")
	if err != nil {
		t.Fatalf("открытие пула: %v", err)
	}
	t.Cleanup(pool.Close)
	Truncate(t, pool)
	return pool
}

// Truncate очищает все таблицы сервиса.
func Truncate(t *testing.T, pool *pgkit.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.DB(ctx).Exec(ctx, `TRUNCATE reminders.reminders, reminders.document_offsets,
		reminders.documents, reminders.members, reminders.organizations, reminders.inbox_events
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("очистка таблиц: %v", err)
	}
}

// Clock — управляемые часы для детерминированных тестов.
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

// Set устанавливает момент часов.
func (c *Clock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t.UTC()
}

// FakeBot — тестовая реализация исходящего порта к bot-service.
// Проверяет содержимое задания и ведёт себя по заданному сценарию:
// подтверждение любого запроса без проверки запрещено правилами тестирования.
type FakeBot struct {
	mu        sync.Mutex
	accepted  map[string]ports.NotificationRequest
	order     []string
	failNext  int
	failErr   error
	callCount int
}

// NewFakeBot создаёт двойник bot-service.
func NewFakeBot() *FakeBot {
	return &FakeBot{accepted: make(map[string]ports.NotificationRequest)}
}

// FailNext заставляет двойник отклонить следующие n вызовов указанной ошибкой.
func (f *FakeBot) FailNext(n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNext, f.failErr = n, err
}

// Enqueue принимает задание, проверяя обязательные поля контракта.
func (f *FakeBot) Enqueue(_ context.Context, req ports.NotificationRequest) (ports.NotificationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	if f.failNext > 0 {
		f.failNext--
		return ports.NotificationResult{}, f.failErr
	}
	if !strings.HasPrefix(req.IdempotencyKey, "rem:") {
		return ports.NotificationResult{}, fmt.Errorf("некорректный ключ идемпотентности %q", req.IdempotencyKey)
	}
	if req.RecipientMaxUserID <= 0 {
		return ports.NotificationResult{}, fmt.Errorf("некорректный получатель %d", req.RecipientMaxUserID)
	}
	runes := []rune(req.Text)
	if len(runes) == 0 || len(runes) > domain.MaxMessageLen {
		return ports.NotificationResult{}, fmt.Errorf("некорректная длина текста %d", len(runes))
	}
	if req.NotAfter.IsZero() {
		return ports.NotificationResult{}, fmt.Errorf("не задан предельный срок отправки")
	}
	if prev, ok := f.accepted[req.IdempotencyKey]; ok {
		_ = prev
		return ports.NotificationResult{NotificationID: notificationID(len(f.order)), Duplicate: true}, nil
	}
	f.accepted[req.IdempotencyKey] = req
	f.order = append(f.order, req.IdempotencyKey)
	return ports.NotificationResult{NotificationID: notificationID(len(f.order))}, nil
}

// Accepted возвращает принятые задания в порядке поступления.
func (f *FakeBot) Accepted() []ports.NotificationRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ports.NotificationRequest, 0, len(f.order))
	for _, k := range f.order {
		out = append(out, f.accepted[k])
	}
	return out
}

// Calls возвращает общее число обращений.
func (f *FakeBot) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callCount
}

func notificationID(n int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
}

var _ ports.MessagingGateway = (*FakeBot)(nil)
