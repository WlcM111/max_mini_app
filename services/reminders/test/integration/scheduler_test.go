package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"vovremya/services/reminders/internal/app"
	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
)

// advanceToFirstReminder сдвигает часы к моменту ближайшего напоминания.
func advanceToFirstReminder(t *testing.T, e *env) domain.Reminder {
	t.Helper()
	items := e.planFor(t, docID)
	for _, it := range items {
		if it.Status == domain.StatusPlanned {
			e.clock.Set(it.DueAt)
			return it
		}
	}
	t.Fatal("нет запланированных напоминаний")
	return domain.Reminder{}
}

func TestSchedulerHandsOffDueReminder(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	first := advanceToFirstReminder(t, e)

	handed, err := e.scheduler.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if handed != 1 {
		t.Fatalf("ожидалась передача 1 напоминания, получено %d", handed)
	}
	accepted := e.bot.Accepted()
	if len(accepted) != 1 {
		t.Fatalf("bot должен получить 1 задание, получено %d", len(accepted))
	}
	want := first.Key.IdempotencyKey()
	if accepted[0].IdempotencyKey != want {
		t.Errorf("ключ идемпотентности: ожидалось %q, получено %q", want, accepted[0].IdempotencyKey)
	}
	if accepted[0].ButtonPayload != "doc_"+docID {
		t.Errorf("payload кнопки: получено %q", accepted[0].ButtonPayload)
	}
	if accepted[0].Text == "" || accepted[0].RecipientMaxUserID <= 0 {
		t.Errorf("некорректное задание: %+v", accepted[0])
	}

	items := e.planFor(t, docID)
	var handedOff int
	for _, it := range items {
		if it.Status == domain.StatusHandedOff {
			handedOff++
			if it.HandedOffAt == nil {
				t.Error("handed_off_at должен быть заполнен")
			}
		}
	}
	if handedOff != 1 {
		t.Fatalf("ожидалось 1 переданное напоминание, получено %d", handedOff)
	}

	// Повторный тик не передаёт то же напоминание снова.
	again, err := e.scheduler.Tick(context.Background())
	if err != nil {
		t.Fatalf("повторный Tick: %v", err)
	}
	if again != 0 || e.bot.Calls() != 1 {
		t.Errorf("повторная передача: handed=%d, вызовов bot=%d", again, e.bot.Calls())
	}
}

func TestSchedulerRetriesOnUnavailableBot(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	first := advanceToFirstReminder(t, e)
	e.bot.FailNext(1, &ports.GatewayError{Code: "unavailable", Retryable: true, Err: errors.New("bot down")})

	if _, err := e.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	items := e.planFor(t, docID)
	var target *domain.Reminder
	for i := range items {
		if items[i].ID == first.ID {
			target = &items[i]
		}
	}
	if target == nil {
		t.Fatal("напоминание не найдено")
	}
	if target.Status != domain.StatusPlanned {
		t.Fatalf("после временной ошибки статус должен остаться planned, получено %s", target.Status)
	}
	if target.Attempts != 1 {
		t.Errorf("ожидалась 1 неудачная попытка, получено %d", target.Attempts)
	}
	if target.LastErrorCode != "unavailable" {
		t.Errorf("код ошибки: получено %q", target.LastErrorCode)
	}
	if !target.NextAttemptAt.After(e.clock.Now()) {
		t.Errorf("повтор должен быть отложен: next_attempt_at=%s, now=%s", target.NextAttemptAt, e.clock.Now())
	}

	// После задержки напоминание передаётся успешно.
	e.clock.Advance(domain.DefaultBackoff.MaxDelay() + time.Second)
	handed, err := e.scheduler.Tick(context.Background())
	if err != nil {
		t.Fatalf("повторный Tick: %v", err)
	}
	if handed != 1 {
		t.Fatalf("ожидалась успешная передача после повтора, получено %d", handed)
	}
}

func TestSchedulerSkipsOnPermanentError(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	advanceToFirstReminder(t, e)
	e.bot.FailNext(1, &ports.GatewayError{Code: "invalid_argument", Retryable: false, Err: errors.New("bad request")})

	if _, err := e.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	items := e.planFor(t, docID)
	if got := countByStatus(items, domain.StatusSkipped); got != 1 {
		t.Fatalf("ожидалось 1 пропущенное напоминание, получено %d", got)
	}
	for _, it := range items {
		if it.Status == domain.StatusSkipped && it.LastErrorCode != "invalid_argument" {
			t.Errorf("код ошибки пропуска: %q", it.LastErrorCode)
		}
	}
}

func TestSchedulerSkipsExpiredReminder(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	first := advanceToFirstReminder(t, e)
	e.clock.Set(first.DueAt.Add(25 * time.Hour)) // за пределами grace = 24 ч

	if _, err := e.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if e.bot.Calls() != 0 {
		t.Errorf("просроченное напоминание не должно передаваться, вызовов bot: %d", e.bot.Calls())
	}
	items := e.planFor(t, docID)
	var skipped int
	for _, it := range items {
		if it.Status == domain.StatusSkipped {
			skipped++
			if it.LastErrorCode != "expired" {
				t.Errorf("код пропуска: %q", it.LastErrorCode)
			}
		}
	}
	if skipped == 0 {
		t.Fatal("ожидалось пропущенное напоминание")
	}
}

func TestSchedulerDoesNotSendForDeletedDocument(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	first := advanceToFirstReminder(t, e)

	// Имитация гонки: строка плана осталась planned, но документ уже удалён
	// в проекции (перепланирование не успело её отменить).
	ctx := context.Background()
	if err := e.proj.MarkDocumentDeleted(ctx, docID, 99); err != nil {
		t.Fatalf("MarkDocumentDeleted: %v", err)
	}
	if _, err := e.scheduler.Tick(ctx); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if e.bot.Calls() != 0 {
		t.Errorf("для удалённого документа сообщение не должно отправляться, вызовов: %d", e.bot.Calls())
	}
	for _, it := range e.planFor(t, docID) {
		if it.ID == first.ID && it.Status != domain.StatusSkipped {
			t.Errorf("ожидался статус skipped, получено %s", it.Status)
		}
	}
}

func TestSchedulerConcurrentTicksSendOnce(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	advanceToFirstReminder(t, e)

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.scheduler.Tick(context.Background())
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("параллельный Tick: %v", err)
		}
	}
	if calls := e.bot.Calls(); calls != 1 {
		t.Fatalf("при параллельных тиках напоминание должно передаваться один раз, вызовов: %d", calls)
	}
	if got := countByStatus(e.planFor(t, docID), domain.StatusHandedOff); got != 1 {
		t.Fatalf("ожидалось 1 переданное напоминание, получено %d", got)
	}
}

func TestLeaseReturnsReminderAfterCrash(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	first := advanceToFirstReminder(t, e)
	ctx := context.Background()

	// Захват без последующего обновления = аварийное завершение обработчика.
	claimed, err := e.reminders.ClaimDue(ctx, e.clock.Now(), 2*time.Minute, 10)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != first.ID {
		t.Fatalf("ожидался захват напоминания %d, получено %+v", first.ID, claimed)
	}
	// До истечения аренды строка не выдаётся повторно.
	again, err := e.reminders.ClaimDue(ctx, e.clock.Now(), 2*time.Minute, 10)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("до истечения аренды строка не должна выдаваться, получено %d", len(again))
	}
	// После истечения аренды напоминание снова доступно и передаётся.
	e.clock.Advance(3 * time.Minute)
	handed, err := e.scheduler.Tick(ctx)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if handed != 1 {
		t.Fatalf("после истечения аренды ожидалась передача, получено %d", handed)
	}
}

func TestRetentionRemovesFinalizedData(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	advanceToFirstReminder(t, e)
	if _, err := e.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	e.clock.Advance(800 * time.Hour) // больше FinalizedTTL и InboxTTL
	if err := e.retention.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	for _, it := range e.planFor(t, docID) {
		if it.Status != domain.StatusPlanned {
			t.Errorf("неактивное напоминание должно быть удалено: %+v", it)
		}
	}
	if _, ok, err := e.inbox.LastAppliedAt(context.Background()); err != nil || ok {
		t.Errorf("журнал событий должен быть очищен (ok=%t, err=%v)", ok, err)
	}
}

func TestQueryServiceReturnsPlan(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	ctx := context.Background()

	next, err := e.query.NextReminders(ctx, accID, []string{docID, docID2})
	if err != nil {
		t.Fatalf("NextReminders: %v", err)
	}
	if len(next) != 1 {
		t.Fatalf("ожидался 1 документ с напоминанием, получено %d", len(next))
	}
	if got := next[docID].Key.DaysBefore; got != 30 {
		t.Errorf("ближайшее напоминание должно быть за 30 дней, получено %d", got)
	}

	plan, err := e.query.DocumentPlan(ctx, docID)
	if err != nil {
		t.Fatalf("DocumentPlan: %v", err)
	}
	if plan.AppliedVersion != 1 || len(plan.Items) != 3 {
		t.Errorf("план документа: версия=%d, элементов=%d", plan.AppliedVersion, len(plan.Items))
	}

	state, inSync, err := e.query.SyncStatus(ctx, app.AggregateDocument, docID, 1)
	if err != nil {
		t.Fatalf("SyncStatus: %v", err)
	}
	if !inSync || state.Version != 1 {
		t.Errorf("ожидалась синхронность версии 1, получено in_sync=%t version=%d", inSync, state.Version)
	}
	if _, inSync, _ := e.query.SyncStatus(ctx, app.AggregateDocument, docID, 2); inSync {
		t.Error("версия 2 ещё не применена, in_sync должен быть false")
	}
}
