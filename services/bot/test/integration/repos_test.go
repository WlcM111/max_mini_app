package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"vovremya/services/bot/internal/domain"
)

func TestMessageRepoInsertAndRead(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	n := s.enqueue(t, "rem:doc-1:acc-1:30", 1001, "Через 30 дней заканчивается срок: «Лицензия».", docButton("3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"))
	if n.ID == 0 || n.PublicID == "" || n.Status != domain.StatusQueued {
		t.Fatalf("сохранённое сообщение: %+v", n)
	}

	got, err := s.messages.GetByKey(ctx, "rem:doc-1:acc-1:30")
	if err != nil {
		t.Fatalf("чтение по ключу: %v", err)
	}
	if got.PublicID != n.PublicID || got.Text != n.Text || got.Kind != domain.KindReminder {
		t.Errorf("прочитанное сообщение отличается: %+v", got)
	}
	if len(got.Buttons) != 1 || got.Buttons[0].Action != domain.ActionOpenApp ||
		got.Buttons[0].Payload != "doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02" {
		t.Errorf("кнопки не сохранены: %+v", got.Buttons)
	}
	if _, err := s.messages.GetByKey(ctx, "rem:missing:key:1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("отсутствующий ключ: ожидалась ErrNotFound, получено %v", err)
	}
}

func TestSchemaConstraints(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	cases := []struct {
		name string
		sql  string
		args []any
	}{
		{"ключ не по шаблону", `INSERT INTO bot.outbound_messages (idempotency_key, request_hash, kind,
			recipient_max_user_id, text, status, not_after) VALUES ($1, $2, 'reminder', 1, 'x', 'queued', now())`,
			[]any{"ВЕРХНИЙ РЕГИСТР", make([]byte, 32)}},
		{"неизвестный вид", `INSERT INTO bot.outbound_messages (idempotency_key, request_hash, kind,
			recipient_max_user_id, text, status, not_after) VALUES ('kind-test-1', $1, 'unknown', 1, 'x', 'queued', now())`,
			[]any{make([]byte, 32)}},
		{"хеш не 32 байта", `INSERT INTO bot.outbound_messages (idempotency_key, request_hash, kind,
			recipient_max_user_id, text, status, not_after) VALUES ('hash-test-1', $1, 'reminder', 1, 'x', 'queued', now())`,
			[]any{make([]byte, 16)}},
		{"sending без аренды", `INSERT INTO bot.outbound_messages (idempotency_key, request_hash, kind,
			recipient_max_user_id, text, status, not_after) VALUES ('lease-test-1', $1, 'reminder', 1, 'x', 'sending', now())`,
			[]any{make([]byte, 32)}},
		{"sent без момента отправки", `INSERT INTO bot.outbound_messages (idempotency_key, request_hash, kind,
			recipient_max_user_id, text, status, not_after) VALUES ('sent-test-1', $1, 'reminder', 1, 'x', 'sent', now())`,
			[]any{make([]byte, 32)}},
		{"неизвестное состояние получателя", `INSERT INTO bot.recipients (max_user_id, state, state_changed_at)
			VALUES (9, 'blocked', now())`, nil},
		{"исход события вне словаря", `INSERT INTO bot.inbound_updates (dedupe_key, update_type, event_time, outcome)
			VALUES ($1, 'bot_started', now(), 'applied_twice')`, []any{make([]byte, 32)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := s.pool.DB(ctx).Exec(ctx, c.sql, c.args...); err == nil {
				t.Fatal("ограничение схемы не сработало")
			}
		})
	}

	t.Run("кнопка с двумя действиями", func(t *testing.T) {
		n := s.enqueue(t, "btn-constraint-1", 1, "текст")
		_, err := s.pool.DB(ctx).Exec(ctx, `INSERT INTO bot.outbound_buttons
			(message_id, position, text, action, open_app_payload, url)
			VALUES ($1, 1, 'кнопка', 'open_app', 'p', 'https://example.org')`, n.ID)
		if err == nil {
			t.Fatal("кнопка не может иметь оба действия")
		}
	})

	t.Run("кнопки удаляются каскадом", func(t *testing.T) {
		n := s.enqueue(t, "btn-cascade-1", 1, "текст", docButton("11111111-1111-4111-8111-111111111111"))
		if _, err := s.pool.DB(ctx).Exec(ctx, `DELETE FROM bot.outbound_messages WHERE id = $1`, n.ID); err != nil {
			t.Fatalf("удаление сообщения: %v", err)
		}
		var count int
		if err := s.pool.DB(ctx).QueryRow(ctx,
			`SELECT count(*) FROM bot.outbound_buttons WHERE message_id = $1`, n.ID).Scan(&count); err != nil {
			t.Fatalf("подсчёт кнопок: %v", err)
		}
		if count != 0 {
			t.Errorf("кнопки остались после удаления сообщения: %d", count)
		}
	})
}

// TestClaimIsExclusive — параллельные воркеры не берут одно сообщение дважды.
func TestClaimIsExclusive(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	const total = 40
	for i := 0; i < total; i++ {
		s.enqueue(t, fmt.Sprintf("claim-test-%03d", i), int64(1000+i), "текст")
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		claims = map[int64]int{}
	)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				batch, err := s.messages.ClaimReady(ctx, s.clock.Now(), time.Minute, 7)
				if err != nil {
					t.Errorf("захват: %v", err)
					return
				}
				mu.Lock()
				for _, n := range batch {
					claims[n.ID]++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	for id, n := range claims {
		if n != 1 {
			t.Errorf("сообщение %d захвачено %d раз", id, n)
		}
	}
	if len(claims) != total {
		t.Errorf("захвачено %d сообщений из %d", len(claims), total)
	}
	var sending int
	if err := s.pool.DB(ctx).QueryRow(ctx,
		`SELECT count(*) FROM bot.outbound_messages WHERE status = 'sending' AND attempts = 1`).Scan(&sending); err != nil {
		t.Fatalf("подсчёт: %v", err)
	}
	if sending != total {
		t.Errorf("в состоянии sending %d сообщений, ожидалось %d", sending, total)
	}
}

// TestLeaseGuardsFinalUpdate — воркер с устаревшей арендой не может записать результат.
func TestLeaseGuardsFinalUpdate(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	s.enqueue(t, "lease-guard-0001", 1001, "текст")

	first, err := s.messages.ClaimReady(ctx, s.clock.Now(), 30*time.Second, 10)
	if err != nil || len(first) != 1 {
		t.Fatalf("первый захват: %v %d", err, len(first))
	}
	staleLease := *first[0].LockedUntil

	// Аренда истекла, сборщик вернул сообщение в очередь, его захватил другой воркер.
	s.clock.Advance(31 * time.Second)
	reaped, err := s.reaper.RunOnce(ctx)
	if err != nil || reaped != 1 {
		t.Fatalf("возврат аренды: %v %d", err, reaped)
	}
	second, err := s.messages.ClaimReady(ctx, s.clock.Now(), 30*time.Second, 10)
	if err != nil || len(second) != 1 {
		t.Fatalf("второй захват: %v %d", err, len(second))
	}

	ok, err := s.messages.MarkSent(ctx, first[0].ID, staleLease, "mid-stale", s.clock.Now())
	if err != nil {
		t.Fatalf("запись результата устаревшим воркером: %v", err)
	}
	if ok {
		t.Fatal("воркер с истёкшей арендой не должен записывать результат")
	}
	ok, err = s.messages.MarkSent(ctx, second[0].ID, *second[0].LockedUntil, "mid-fresh", s.clock.Now())
	if err != nil || !ok {
		t.Fatalf("действующая аренда должна позволять запись: %v %t", err, ok)
	}
	n := s.status(t, "lease-guard-0001")
	if n.Status != domain.StatusSent || n.MaxMessageID != "mid-fresh" || n.Attempts != 2 {
		t.Errorf("итоговое состояние: %+v", n)
	}
}

func TestReleaseReturnsMessageWithoutAttempt(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	s.enqueue(t, "release-test-0001", 1001, "текст")
	batch, err := s.messages.ClaimReady(ctx, s.clock.Now(), time.Minute, 10)
	if err != nil || len(batch) != 1 {
		t.Fatalf("захват: %v", err)
	}
	next := s.clock.Now().Add(30 * time.Second)
	ok, err := s.messages.Release(ctx, batch[0].ID, *batch[0].LockedUntil, next, s.clock.Now())
	if err != nil || !ok {
		t.Fatalf("возврат в очередь: %v %t", err, ok)
	}
	n := s.status(t, "release-test-0001")
	if n.Status != domain.StatusQueued || n.Attempts != 0 || n.LockedUntil != nil {
		t.Errorf("после возврата ожидались queued и 0 попыток: %+v", n)
	}
	if !n.NextAttemptAt.Equal(next) {
		t.Errorf("следующая попытка: ожидалось %s, получено %s", next, n.NextAttemptAt)
	}
}

func TestRecipientStateOrdering(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	now := s.clock.Now()

	err := s.pool.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.recipients.LockOrCreate(ctx, 1001, now)
		if err != nil {
			return err
		}
		if r.State != domain.RecipientUnknown {
			t.Errorf("новый получатель: %+v", r)
		}
		next, ok := r.ApplyEvent(domain.UpdateBotStarted, now)
		if !ok {
			t.Fatal("событие должно применяться")
		}
		return s.recipients.Save(ctx, next, now)
	})
	if err != nil {
		t.Fatalf("создание получателя: %v", err)
	}

	got, found, err := s.recipients.Get(ctx, 1001)
	if err != nil || !found || got.State != domain.RecipientActive {
		t.Fatalf("состояние после bot_started: %+v %t %v", got, found, err)
	}

	// Более старое событие не меняет состояние.
	err = s.pool.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.recipients.LockOrCreate(ctx, 1001, now)
		if err != nil {
			return err
		}
		if next, ok := r.ApplyEvent(domain.UpdateBotStopped, now.Add(-time.Hour)); ok {
			return s.recipients.Save(ctx, next, now)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("устаревшее событие: %v", err)
	}
	got, _, _ = s.recipients.Get(ctx, 1001)
	if got.State != domain.RecipientActive {
		t.Errorf("устаревшее событие изменило состояние: %s", got.State)
	}
}

func TestInboundDedupe(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()
	rec := domain.InboundRecord{
		Key:        domain.NewDedupeKey([]byte(`{"update_type":"bot_started","timestamp":1}`)),
		UpdateType: "bot_started", EventTime: s.clock.Now(), MaxUserID: 1001,
		Outcome: domain.OutcomeApplied, ReceivedAt: s.clock.Now(),
	}
	inserted, err := s.inbound.Insert(ctx, rec)
	if err != nil || !inserted {
		t.Fatalf("первая запись: %v %t", err, inserted)
	}
	inserted, err = s.inbound.Insert(ctx, rec)
	if err != nil || inserted {
		t.Fatalf("повтор должен распознаваться: %v %t", err, inserted)
	}
}

func TestRetention(t *testing.T) {
	s := newStack(t, stackOptions{})
	ctx := context.Background()

	// Доставленное сообщение, событие webhook и остановленный диалог.
	n := s.enqueue(t, "retention-test-001", 1001, "текст")
	batch, _ := s.messages.ClaimReady(ctx, s.clock.Now(), time.Minute, 10)
	if _, err := s.messages.MarkSent(ctx, n.ID, *batch[0].LockedUntil, "mid", s.clock.Now()); err != nil {
		t.Fatalf("отметка отправки: %v", err)
	}
	if _, err := s.inbound.Insert(ctx, domain.InboundRecord{
		Key: domain.NewDedupeKey([]byte("retention")), UpdateType: "bot_started",
		EventTime: s.clock.Now(), Outcome: domain.OutcomeIgnored, ReceivedAt: s.clock.Now(),
	}); err != nil {
		t.Fatalf("запись события: %v", err)
	}
	err := s.pool.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.recipients.LockOrCreate(ctx, 2002, s.clock.Now())
		if err != nil {
			return err
		}
		next, _ := r.ApplyEvent(domain.UpdateBotStopped, s.clock.Now())
		return s.recipients.Save(ctx, next, s.clock.Now())
	})
	if err != nil {
		t.Fatalf("остановленный диалог: %v", err)
	}

	// Сроки не истекли: ничего не удаляется.
	s.clock.Advance(6 * 24 * time.Hour)
	if err := s.retention.RunOnce(ctx); err != nil {
		t.Fatalf("очистка: %v", err)
	}
	if _, err := s.messages.GetByKey(ctx, "retention-test-001"); err != nil {
		t.Errorf("сообщение удалено раньше срока: %v", err)
	}

	// Событие старше 7 суток и сообщение старше 30 суток удаляются.
	s.clock.Advance(25 * 24 * time.Hour)
	if err := s.retention.RunOnce(ctx); err != nil {
		t.Fatalf("очистка: %v", err)
	}
	if _, err := s.messages.GetByKey(ctx, "retention-test-001"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("доставленное сообщение старше 30 суток должно удаляться: %v", err)
	}
	var inboundLeft, recipientsLeft int
	if err := s.pool.DB(ctx).QueryRow(ctx, `SELECT count(*) FROM bot.inbound_updates`).Scan(&inboundLeft); err != nil {
		t.Fatalf("подсчёт событий: %v", err)
	}
	if err := s.pool.DB(ctx).QueryRow(ctx,
		`SELECT count(*) FROM bot.recipients WHERE max_user_id = 2002`).Scan(&recipientsLeft); err != nil {
		t.Fatalf("подсчёт получателей: %v", err)
	}
	if inboundLeft != 0 {
		t.Errorf("события webhook старше 7 суток остались: %d", inboundLeft)
	}
	if recipientsLeft != 0 {
		t.Errorf("остановленный диалог старше 30 суток остался: %d", recipientsLeft)
	}
}

// TestNoCrossSchemaAccess — bot не обращается к таблицам других сервисов.
func TestNoCrossSchemaAccess(t *testing.T) {
	ctx := context.Background()
	s := newStack(t, stackOptions{})
	var found []string
	rows, err := s.pool.DB(ctx).Query(ctx, `SELECT table_schema || '.' || table_name
		FROM information_schema.tables WHERE table_schema IN ('core', 'reminders')`)
	if err != nil {
		t.Fatalf("чтение словаря: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		found = append(found, name)
	}
	if len(found) > 0 {
		t.Logf("в тестовой базе присутствуют чужие схемы: %s", strings.Join(found, ", "))
	}
	// Межсервисных внешних ключей нет.
	var crossFK int
	err = s.pool.DB(ctx).QueryRow(ctx, `
		SELECT count(*) FROM pg_constraint c
		JOIN pg_class child ON child.oid = c.conrelid
		JOIN pg_namespace cn ON cn.oid = child.relnamespace
		JOIN pg_class parent ON parent.oid = c.confrelid
		JOIN pg_namespace pn ON pn.oid = parent.relnamespace
		WHERE c.contype = 'f' AND cn.nspname = 'bot' AND pn.nspname <> 'bot'`).Scan(&crossFK)
	if err != nil {
		t.Fatalf("чтение ограничений: %v", err)
	}
	if crossFK != 0 {
		t.Errorf("найдены межсервисные внешние ключи: %d", crossFK)
	}
}
