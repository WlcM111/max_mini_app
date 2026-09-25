package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"vovremya/services/bot/internal/domain"
)

func TestEnqueueIdempotency(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	req := domain.EnqueueRequest{
		IdempotencyKey: "rem:6d1f0a73-2b8c-4e5f-9a10-3c7b5e2d8f04:9f3c7b21-4d6e-4a8b-b5c0-1e2f3a4b5c06:30",
		Message: domain.Message{
			Kind: domain.KindReminder, RecipientMaxUserID: 1001,
			Text:    "Через 30 дней заканчивается срок: «Лицензия».",
			Buttons: []domain.Button{docButton("3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02")},
		},
		NotAfter: s.clock.Now().Add(24 * time.Hour),
	}
	hash := domain.ContentHash(req)

	first, err := s.messaging.Enqueue(ctx, req, hash)
	if err != nil || first.Duplicate {
		t.Fatalf("первая постановка: %v %+v", err, first)
	}
	second, err := s.messaging.Enqueue(ctx, req, hash)
	if err != nil {
		t.Fatalf("повтор с тем же содержимым: %v", err)
	}
	if !second.Duplicate || second.Notification.PublicID != first.Notification.PublicID {
		t.Errorf("повтор должен возвращать прежнее сообщение: %+v", second)
	}

	var count int
	if err := s.pool.DB(ctx).QueryRow(ctx,
		`SELECT count(*) FROM bot.outbound_messages WHERE idempotency_key = $1`, req.IdempotencyKey).Scan(&count); err != nil {
		t.Fatalf("подсчёт: %v", err)
	}
	if count != 1 {
		t.Errorf("в очереди %d строк, ожидалась одна", count)
	}

	// Тот же ключ с другим содержимым — конфликт (grpc-contract §1).
	other := req
	other.Text = "Другой текст"
	if _, err := s.messaging.Enqueue(ctx, other, domain.ContentHash(other)); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Errorf("ожидался конфликт идемпотентности, получено %v", err)
	}
}

func TestEnqueueValidationAndBackpressure(t *testing.T) {
	s := newStack(t, stackOptions{queueLimit: 1})
	ctx := context.Background()

	bad := domain.EnqueueRequest{
		IdempotencyKey: "ВЕРХНИЙ",
		Message:        domain.Message{Kind: domain.KindReminder, RecipientMaxUserID: 1, Text: "x"},
		NotAfter:       s.clock.Now().Add(time.Hour),
	}
	if _, err := s.messaging.Enqueue(ctx, bad, domain.ContentHash(bad)); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("ожидалась ошибка валидации, получено %v", err)
	}

	s.enqueue(t, "queue-limit-0001", 1001, "текст")
	s.enqueue(t, "queue-limit-0002", 1002, "текст")
	if err := s.monitor.Refresh(ctx); err != nil {
		t.Fatalf("обновление глубины очереди: %v", err)
	}
	if s.depth.Value() != 2 {
		t.Fatalf("глубина очереди: %d", s.depth.Value())
	}
	req := domain.EnqueueRequest{
		IdempotencyKey: "queue-limit-0003",
		Message:        domain.Message{Kind: domain.KindReminder, RecipientMaxUserID: 1003, Text: "текст"},
		NotAfter:       s.clock.Now().Add(time.Hour),
	}
	if _, err := s.messaging.Enqueue(ctx, req, domain.ContentHash(req)); !errors.Is(err, domain.ErrQueueFull) {
		t.Errorf("при превышении предела очереди ожидалась ErrQueueFull, получено %v", err)
	}
}

func TestStatusQueries(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()

	if _, err := s.messaging.NotificationStatus(ctx, "absent-key-0001"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("отсутствующее сообщение: %v", err)
	}
	if _, err := s.messaging.NotificationStatus(ctx, "BAD KEY"); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("некорректный ключ: %v", err)
	}

	st, err := s.messaging.RecipientStatus(ctx, 4242)
	if err != nil {
		t.Fatalf("состояние получателя: %v", err)
	}
	if st.Known || st.Recipient.State != domain.RecipientUnknown {
		t.Errorf("незнакомый получатель должен быть UNKNOWN: %+v", st)
	}
	if st.ChatURL != "https://max.ru/vovremya_local_bot" {
		t.Errorf("ссылка на чат с ботом: %q", st.ChatURL)
	}
	if _, err := s.messaging.RecipientStatus(ctx, 0); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("нулевой идентификатор: %v", err)
	}

	p, mode, err := s.messaging.BotProfile()
	if err != nil || p.Username != "vovremya_local_bot" || mode != domain.ModeStub {
		t.Errorf("профиль бота: %+v %s %v", p, mode, err)
	}

	empty := newStack(t, stackOptions{profile: &domain.Profile{}})
	if _, _, err := empty.messaging.BotProfile(); !errors.Is(err, domain.ErrProfileUnavailable) {
		t.Errorf("до загрузки профиля ожидалась ErrProfileUnavailable, получено %v", err)
	}
}
