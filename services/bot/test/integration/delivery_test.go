package integration_test

import (
	"context"
	"testing"
	"time"

	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/test/testutil"
)

func TestDeliverySuccess(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	s.enqueue(t, "delivery-ok-0001", 1001, "Через 7 дней заканчивается срок: «Договор».",
		docButton("3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"))

	if n, err := s.delivery.ProcessBatch(ctx); err != nil || n != 1 {
		t.Fatalf("обработка пакета: %v %d", err, n)
	}
	sent := s.max.Sent()
	if len(sent) != 1 || sent[0].Message.RecipientMaxUserID != 1001 {
		t.Fatalf("в MAX ушло %d сообщений: %+v", len(sent), sent)
	}
	buttons := sent[0].Message.Buttons
	if len(buttons) != 2 || sent[0].Profile.Username != "vovremya_local_bot" {
		t.Fatalf("кнопки или профиль не переданы: %+v", sent[0])
	}
	if buttons[0].Action != domain.ActionOpenApp ||
		buttons[0].Payload != "doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02" {
		t.Errorf("первая кнопка должна открывать карточку документа: %+v", buttons[0])
	}
	if buttons[1].Action != domain.ActionCallback ||
		buttons[1].Text != domain.SnoozeButtonText ||
		buttons[1].Payload != domain.SnoozePayloadPrefix+"delivery-ok-0001" {
		t.Errorf("вторая кнопка — отложить напоминание на неделю: %+v", buttons[1])
	}
	n := s.status(t, "delivery-ok-0001")
	if n.Status != domain.StatusSent || n.Attempts != 1 || n.SentAt == nil || n.MaxMessageID != "fake-1" {
		t.Errorf("состояние после отправки: %+v", n)
	}
	if n.LastErrorCode != "" {
		t.Errorf("код ошибки должен очищаться: %q", n.LastErrorCode)
	}
	rec, found, err := s.recipients.Get(ctx, 1001)
	if err != nil || !found || rec.LastDeliveryAt == nil {
		t.Errorf("момент доставки не записан: %+v %t %v", rec, found, err)
	}
}

func TestDeliveryFailureClassification(t *testing.T) {
	cases := []struct {
		name        string
		failure     error
		wantStatus  domain.Status
		wantCode    string
		unreachable bool
		penalty     bool
		nextIn      time.Duration
	}{
		{"400 — окончательная ошибка", testFailure(domain.CodeMax4xx, false, false, 0, 400),
			domain.StatusFailed, domain.CodeMax4xx, false, false, 0},
		{"401 — повтор", testFailure(domain.CodeMax401, true, false, 0, 401),
			domain.StatusRetryWait, domain.CodeMax401, false, false, 0},
		{"403 — получатель недоступен", testFailure(domain.CodeRecipientUnreachable, false, true, 0, 403),
			domain.StatusFailed, domain.CodeRecipientUnreachable, true, false, 0},
		{"404 — получатель недоступен", testFailure(domain.CodeRecipientUnreachable, false, true, 0, 404),
			domain.StatusFailed, domain.CodeRecipientUnreachable, true, false, 0},
		{"429 — снижение скорости", testFailure(domain.CodeMax429, true, false, 90*time.Second, 429),
			domain.StatusRetryWait, domain.CodeMax429, false, true, 90 * time.Second},
		{"500 — повтор", testFailure(domain.CodeMax5xx, true, false, 0, 500),
			domain.StatusRetryWait, domain.CodeMax5xx, false, false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newStack(t, stackOptions{})
			ctx := context.Background()
			s.max.Script(c.failure)
			s.enqueue(t, "delivery-fail-0001", 1001, "текст")

			if _, err := s.delivery.ProcessBatch(ctx); err != nil {
				t.Fatalf("обработка: %v", err)
			}
			n := s.status(t, "delivery-fail-0001")
			if n.Status != c.wantStatus || n.LastErrorCode != c.wantCode {
				t.Fatalf("состояние: %s/%s, ожидалось %s/%s", n.Status, n.LastErrorCode, c.wantStatus, c.wantCode)
			}
			if n.Attempts != 1 {
				t.Errorf("попытка должна быть засчитана: %d", n.Attempts)
			}
			if c.nextIn > 0 && n.NextAttemptAt.Before(s.clock.Now().Add(c.nextIn)) {
				t.Errorf("Retry-After не учтён: следующая попытка через %s", n.NextAttemptAt.Sub(s.clock.Now()))
			}
			if c.penalty != (s.limiter.Penalties() > 0) {
				t.Errorf("снижение скорости: получено %d", s.limiter.Penalties())
			}
			rec, found, err := s.recipients.Get(ctx, 1001)
			if err != nil {
				t.Fatalf("чтение получателя: %v", err)
			}
			if c.unreachable && (!found || rec.State != domain.RecipientUnreachable) {
				t.Errorf("получатель должен стать unreachable: %+v %t", rec, found)
			}
			if !c.unreachable && found && rec.State == domain.RecipientUnreachable {
				t.Errorf("состояние получателя изменено без основания: %+v", rec)
			}
		})
	}
}

func TestDeliveryRetriesUntilLimit(t *testing.T) {
	s := newStack(t, stackOptions{maxAttempts: 3})
	ctx := context.Background()
	s.max.Script(
		testFailure(domain.CodeMax5xx, true, false, 0, 500),
		testFailure(domain.CodeMax5xx, true, false, 0, 500),
		testFailure(domain.CodeMax5xx, true, false, 0, 500),
	)
	s.enqueue(t, "delivery-retry-0001", 1001, "текст")

	for i := 1; i <= 3; i++ {
		if _, err := s.delivery.ProcessBatch(ctx); err != nil {
			t.Fatalf("попытка %d: %v", i, err)
		}
		n := s.status(t, "delivery-retry-0001")
		if n.Attempts != i {
			t.Fatalf("попытка %d: счётчик %d", i, n.Attempts)
		}
		if i < 3 {
			if n.Status != domain.StatusRetryWait {
				t.Fatalf("после попытки %d ожидался retry_wait, получено %s", i, n.Status)
			}
			// Следующая попытка не раньше текущего момента и не позже предела выдержки.
			if n.NextAttemptAt.Before(s.clock.Now()) {
				t.Errorf("следующая попытка в прошлом: %s", n.NextAttemptAt)
			}
			s.clock.Advance(domain.DefaultBackoff.Ceiling(i) + time.Second)
		} else if n.Status != domain.StatusFailed {
			t.Errorf("после исчерпания попыток ожидался failed, получено %s", n.Status)
		}
	}
}

func TestDeliveryPreSendChecks(t *testing.T) {
	t.Run("истёк срок not_after", func(t *testing.T) {
		s := newStack(t, stackOptions{})
		ctx := context.Background()
		s.enqueue(t, "delivery-expired-001", 1001, "текст")
		s.clock.Advance(25 * time.Hour)
		if _, err := s.delivery.ProcessBatch(ctx); err != nil {
			t.Fatalf("обработка: %v", err)
		}
		n := s.status(t, "delivery-expired-001")
		if n.Status != domain.StatusExpired || n.LastErrorCode != domain.CodeExpired {
			t.Errorf("состояние: %s/%s", n.Status, n.LastErrorCode)
		}
		if len(s.max.Sent()) != 0 {
			t.Error("просроченное сообщение не должно уходить в MAX")
		}
	})

	t.Run("получатель остановил бота", func(t *testing.T) {
		s := newStack(t, stackOptions{})
		ctx := context.Background()
		err := s.pool.WithinTx(ctx, func(ctx context.Context) error {
			r, err := s.recipients.LockOrCreate(ctx, 1001, s.clock.Now())
			if err != nil {
				return err
			}
			next, _ := r.ApplyEvent(domain.UpdateBotStopped, s.clock.Now())
			return s.recipients.Save(ctx, next, s.clock.Now())
		})
		if err != nil {
			t.Fatalf("подготовка получателя: %v", err)
		}
		s.enqueue(t, "delivery-stopped-001", 1001, "текст")
		if _, err := s.delivery.ProcessBatch(ctx); err != nil {
			t.Fatalf("обработка: %v", err)
		}
		n := s.status(t, "delivery-stopped-001")
		if n.Status != domain.StatusFailed || n.LastErrorCode != domain.CodeRecipientStopped {
			t.Errorf("состояние: %s/%s", n.Status, n.LastErrorCode)
		}
		if len(s.max.Sent()) != 0 {
			t.Error("остановившему бота сообщение не отправляется")
		}
	})

	t.Run("профиль бота не загружен", func(t *testing.T) {
		s := newStack(t, stackOptions{profile: &domain.Profile{}})
		ctx := context.Background()
		s.enqueue(t, "delivery-noprofile-1", 1001, "текст", docButton("3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"))
		if _, err := s.delivery.ProcessBatch(ctx); err != nil {
			t.Fatalf("обработка: %v", err)
		}
		n := s.status(t, "delivery-noprofile-1")
		if n.Status != domain.StatusQueued || n.Attempts != 0 {
			t.Errorf("сообщение должно вернуться в очередь без попытки: %+v", n)
		}
		if !n.NextAttemptAt.Equal(s.clock.Now().Add(30 * time.Second)) {
			t.Errorf("отсрочка: ожидалось 30 с, получено %s", n.NextAttemptAt.Sub(s.clock.Now()))
		}
		if len(s.max.Sent()) != 0 {
			t.Error("без профиля сообщение с диплинком не отправляется")
		}

		// После загрузки профиля сообщение уходит.
		s.profiles.Set(domain.Profile{UserID: 500100, Username: "vovremya_local_bot", DisplayName: "Вовремя"})
		s.clock.Advance(31 * time.Second)
		if _, err := s.delivery.ProcessBatch(ctx); err != nil {
			t.Fatalf("повторная обработка: %v", err)
		}
		if n := s.status(t, "delivery-noprofile-1"); n.Status != domain.StatusSent {
			t.Errorf("после загрузки профиля: %s", n.Status)
		}
	})
}

func TestDeliveryTimeout(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	s.max.Delay(3 * time.Second) // больше RequestTimeout = 2 с
	s.enqueue(t, "delivery-timeout-01", 1001, "текст")
	if _, err := s.delivery.ProcessBatch(ctx); err != nil {
		t.Fatalf("обработка: %v", err)
	}
	n := s.status(t, "delivery-timeout-01")
	if n.Status != domain.StatusRetryWait || n.LastErrorCode != domain.CodeTimeout {
		t.Errorf("таймаут MAX: %s/%s", n.Status, n.LastErrorCode)
	}
}

func testFailure(code string, retryable, unreachable bool, retryAfter time.Duration, status int) error {
	return testutil.SendFailure(code, retryable, unreachable, retryAfter, status)
}
