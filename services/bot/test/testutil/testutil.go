// Package testutil — средства автономных тестов bot-service: изолированная
// PostgreSQL, управляемые часы и контролируемый двойник MAX Bot API.
// Используется только тестами и не входит в production-сборку.
package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"vovremya/internal/platform/pgkit"
	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
	"vovremya/services/bot/migrations"
)

// EnvDSN — переменная окружения с DSN администратора тестовой PostgreSQL.
const EnvDSN = "BOT_TEST_DATABASE_URL"

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
		sharedDSN, sharedErr = CreateDatabase(admin, fmt.Sprintf("bot_test_%d", time.Now().UnixNano()))
	})
	if sharedErr != nil {
		t.Fatalf("подготовка тестовой базы: %v", sharedErr)
	}
	return sharedDSN
}

// NewDatabase создаёт отдельную базу с применёнными миграциями.
func NewDatabase(t *testing.T, suffix string) string {
	t.Helper()
	admin := os.Getenv(EnvDSN)
	if admin == "" {
		t.Skipf("%s не задан: интеграционный тест пропущен", EnvDSN)
	}
	dsn, err := CreateDatabase(admin, fmt.Sprintf("bot_%s_%d", suffix, time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("создание базы: %v", err)
	}
	return dsn
}

// CreateDatabase создаёт базу, схему bot и применяет миграции сервиса.
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
	if _, err := dbConn.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS bot`); err != nil {
		return "", fmt.Errorf("create schema: %w", err)
	}
	if err := pgkit.MigrateUp(ctx, dsn, migrations.FS, "bot"); err != nil {
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

// Pool открывает пул к общей тестовой базе и очищает таблицы перед тестом.
func Pool(t *testing.T) *pgkit.Pool {
	t.Helper()
	pool, err := pgkit.Open(context.Background(), DSN(t), 10, "bot-test")
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
	_, err := pool.DB(ctx).Exec(ctx, `TRUNCATE bot.outbound_buttons, bot.outbound_messages,
		bot.recipients, bot.inbound_updates RESTART IDENTITY CASCADE`)
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

// Random — детерминированный источник «случайных» чисел.
type Random struct{ Value float64 }

// Float64 возвращает заданное значение.
func (r Random) Float64() float64 { return r.Value }

// SentMessage — сообщение, принятое двойником MAX.
type SentMessage struct {
	Message domain.Message
	Profile domain.Profile
	At      time.Time
}

// FakeMax — двойник MAX Bot API для автономных тестов доставки.
// Отвечает по сценарию: успех, ошибка или задержка.
type FakeMax struct {
	mu        sync.Mutex
	sent      []SentMessage
	script    []error
	profile   domain.Profile
	delay     time.Duration
	failGetMe error
}

// NewFakeMax создаёт двойник с заданным профилем.
func NewFakeMax(profile domain.Profile) *FakeMax {
	return &FakeMax{profile: profile}
}

// Script задаёт последовательность результатов отправки (nil — успех).
func (f *FakeMax) Script(results ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.script = append(f.script, results...)
}

// Delay задаёт задержку ответа.
func (f *FakeMax) Delay(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delay = d
}

// FailGetMe заставляет двойник отклонять GET /me.
func (f *FakeMax) FailGetMe(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failGetMe = err
}

// Sent возвращает принятые сообщения.
func (f *FakeMax) Sent() []SentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]SentMessage(nil), f.sent...)
}

// SendMessage реализует порт клиента MAX.
func (f *FakeMax) SendMessage(ctx context.Context, msg domain.Message, profile domain.Profile) (ports.SendResult, error) {
	f.mu.Lock()
	delay := f.delay
	var next error
	if len(f.script) > 0 {
		next = f.script[0]
		f.script = f.script[1:]
	}
	f.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ports.SendResult{}, &ports.SendError{
				Failure: domain.SendFailure{Code: domain.CodeTimeout, Retryable: true}, Err: ctx.Err()}
		}
	}
	if next != nil {
		return ports.SendResult{}, next
	}
	if len(msg.Buttons) > 0 && msg.HasOpenAppButtons() && profile.Username == "" {
		return ports.SendResult{}, fmt.Errorf("кнопка open_app без профиля бота")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, SentMessage{Message: msg, Profile: profile, At: time.Now()})
	return ports.SendResult{MessageID: fmt.Sprintf("fake-%d", len(f.sent))}, nil
}

// GetMe возвращает профиль двойника.
func (f *FakeMax) GetMe(context.Context) (domain.Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failGetMe != nil {
		return domain.Profile{}, f.failGetMe
	}
	return f.profile, nil
}

// ListSubscriptions не используется в этих тестах.
func (f *FakeMax) ListSubscriptions(context.Context) ([]string, error) { return nil, nil }

// Subscribe не используется в этих тестах.
func (f *FakeMax) Subscribe(context.Context, string, []string, string) error { return nil }

// SetCommands не используется в этих тестах.
func (f *FakeMax) SetCommands(context.Context, []ports.BotCommand) error { return nil }

// NoLimit — ограничитель без задержек.
type NoLimit struct {
	mu        sync.Mutex
	penalties int
}

// Wait разрешает отправку немедленно.
func (n *NoLimit) Wait(ctx context.Context, _ int64) error { return ctx.Err() }

// Penalize считает срабатывания ограничения 429.
func (n *NoLimit) Penalize() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.penalties++
}

// Penalties возвращает число снижений скорости.
func (n *NoLimit) Penalties() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.penalties
}

// SendFailure — готовая ошибка отправки для сценариев двойника.
func SendFailure(code string, retryable, unreachable bool, retryAfter time.Duration, status int) error {
	return &ports.SendError{
		Failure: domain.SendFailure{Code: code, Retryable: retryable,
			RecipientUnreachable: unreachable, RetryAfter: retryAfter},
		HTTPStatus: status,
		Err:        fmt.Errorf("MAX ответил %d", status),
	}
}
