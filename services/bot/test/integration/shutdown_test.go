package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// blockingMax задерживает первую отправку до сигнала: так проверяется поведение
// воркера при остановке сервиса во время работы с пакетом сообщений.
type blockingMax struct {
	mu      sync.Mutex
	started chan struct{}
	release chan struct{}
	sent    int
	once    sync.Once
}

func (b *blockingMax) SendMessage(ctx context.Context, msg domain.Message, _ domain.Profile) (ports.SendResult, error) {
	b.once.Do(func() {
		close(b.started)
		<-b.release
	})
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sent++
	return ports.SendResult{MessageID: "blocked"}, nil
}

func (b *blockingMax) GetMe(context.Context) (domain.Profile, error) {
	return domain.Profile{Username: "vovremya_local_bot"}, nil
}
func (b *blockingMax) ListSubscriptions(context.Context) ([]string, error)       { return nil, nil }
func (b *blockingMax) Subscribe(context.Context, string, []string, string) error { return nil }
func (b *blockingMax) SetCommands(context.Context, []ports.BotCommand) error     { return nil }
func (b *blockingMax) Count() int                                                { b.mu.Lock(); defer b.mu.Unlock(); return b.sent }

// TestShutdownReleasesClaimedMessages — при остановке воркер завершает текущее
// сообщение, а остальные захваченные возвращает в очередь без потери (NFR-10).
func TestShutdownReleasesClaimedMessages(t *testing.T) {
	s := newStack(t, stackOptions{batch: 3})
	ctx, cancel := context.WithCancel(context.Background())
	blocker := &blockingMax{started: make(chan struct{}), release: make(chan struct{})}
	s.delivery = newDeliveryWithClient(s, blocker)

	for i := 1; i <= 3; i++ {
		s.enqueue(t, "shutdown-test-000"+string(rune('0'+i)), int64(1000+i), "текст")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.delivery.ProcessBatch(ctx)
	}()

	<-blocker.started // первое сообщение отправляется
	cancel()          // сигнал остановки во время отправки
	close(blocker.release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("обработка пакета не завершилась")
	}

	if blocker.Count() != 1 {
		t.Errorf("после остановки отправлено %d сообщений, ожидалось одно (текущее)", blocker.Count())
	}
	sent := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages WHERE status = 'sent'`)
	queued := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages WHERE status = 'queued' AND attempts = 0`)
	sending := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages WHERE status = 'sending'`)
	if sent != 1 || queued != 2 || sending != 0 {
		t.Errorf("после остановки: sent=%d queued=%d sending=%d, ожидалось 1/2/0", sent, queued, sending)
	}
}
