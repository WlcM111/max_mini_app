package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

type fakeSnoozer struct {
	outcome ports.SnoozeOutcome
	err     error
	calls   int
	key     string
	user    int64
}

func (f *fakeSnoozer) SnoozeReminder(_ context.Context, key string, user int64) (ports.SnoozeOutcome, time.Time, error) {
	f.calls++
	f.key, f.user = key, user
	return f.outcome, time.Time{}, f.err
}

type fakeAnswerer struct {
	callbackID, text string
	err              error
}

func (f *fakeAnswerer) AnswerCallback(_ context.Context, callbackID, text string) error {
	f.callbackID, f.text = callbackID, text
	return f.err
}

func snoozeUpdate(user int64) InboundUpdate {
	return InboundUpdate{
		Parsed: true, CallbackID: "cb-1", CallbackPayload: domain.SnoozePayloadPrefix + "rem:p:a:30",
		Update: domain.Update{MaxUserID: user},
	}
}

// TestSnoozeForwardsToReminders: нажатие передаётся в reminders-service с ключом без
// префикса и отправителем, а исход превращается в понятный ответ (BUG-002, BUG-006).
func TestSnoozeForwardsToReminders(t *testing.T) {
	cases := []struct {
		outcome ports.SnoozeOutcome
		err     error
		want    string
	}{
		{ports.SnoozeSnoozed, nil, snoozeAnswerDone},
		{ports.SnoozeAlreadySnoozed, nil, snoozeAnswerAlready},
		{ports.SnoozeTooLate, nil, snoozeAnswerTooLate},
		{ports.SnoozeStale, nil, snoozeAnswerStale},
		{ports.SnoozeForbidden, nil, snoozeAnswerForeign},
		{ports.SnoozeNotFound, nil, snoozeAnswerNotFound},
		{ports.SnoozeUnknown, nil, snoozeAnswerRetry},
		{ports.SnoozeUnknown, errors.New("reminders-service недоступен"), snoozeAnswerRetry},
	}
	for _, tc := range cases {
		snoozer := &fakeSnoozer{outcome: tc.outcome, err: tc.err}
		answerer := &fakeAnswerer{}
		s := &WebhookService{log: slog.New(slog.DiscardHandler)}
		s.UseSnoozer(snoozer)
		s.UseCallbackAnswers(answerer)

		s.snooze(context.Background(), snoozeUpdate(1036))

		if snoozer.calls != 1 || snoozer.key != "rem:p:a:30" || snoozer.user != 1036 {
			t.Fatalf("вызов reminders-service: %+v", snoozer)
		}
		if answerer.callbackID != "cb-1" || answerer.text != tc.want {
			t.Errorf("исход %v: ответ %q, ожидался %q", tc.outcome, answerer.text, tc.want)
		}
	}
}

// TestSnoozeRequiresSender: без идентификатора отправителя нельзя проверить адресата,
// поэтому запрос в reminders-service не отправляется (BUG-017).
func TestSnoozeRequiresSender(t *testing.T) {
	snoozer := &fakeSnoozer{outcome: ports.SnoozeSnoozed}
	answerer := &fakeAnswerer{}
	s := &WebhookService{log: slog.New(slog.DiscardHandler)}
	s.UseSnoozer(snoozer)
	s.UseCallbackAnswers(answerer)

	s.snooze(context.Background(), snoozeUpdate(0))

	if snoozer.calls != 0 {
		t.Fatal("при нулевом отправителе reminders-service вызываться не должен")
	}
	if answerer.text != snoozeAnswerNotFound {
		t.Fatalf("ответ %q", answerer.text)
	}
}

// TestSnoozeWithoutAnswererOrClient: без клиента MAX или reminders-service обработка не падает.
func TestSnoozeWithoutAnswererOrClient(t *testing.T) {
	s := &WebhookService{log: slog.New(slog.DiscardHandler)}
	s.snooze(context.Background(), snoozeUpdate(1036)) // ни клиента, ни ответчика

	answerer := &fakeAnswerer{err: errors.New("MAX недоступен")}
	s.UseCallbackAnswers(answerer)
	s.snooze(context.Background(), snoozeUpdate(1036))
	if answerer.text != snoozeAnswerNotFound {
		t.Fatalf("без reminders-service ожидался ответ %q, получен %q", snoozeAnswerNotFound, answerer.text)
	}
}
