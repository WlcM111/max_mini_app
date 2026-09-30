package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"vovremya/services/reminders/internal/ports"
)

type digestClock struct{ now time.Time }

func (c *digestClock) Now() time.Time { return c.now }

type digestRepo struct {
	recipients []ports.DigestRecipient
	docs       []ports.DigestDocument
}

func (r *digestRepo) ListDigestRecipients(context.Context) ([]ports.DigestRecipient, error) {
	return r.recipients, nil
}

func (r *digestRepo) ListDigestDocuments(context.Context, string, time.Time) ([]ports.DigestDocument, error) {
	return r.docs, nil
}

type digestBot struct {
	sent []ports.NotificationRequest
	err  error
}

func (b *digestBot) Enqueue(_ context.Context, req ports.NotificationRequest) (ports.NotificationResult, error) {
	if b.err != nil {
		return ports.NotificationResult{}, b.err
	}
	b.sent = append(b.sent, req)
	return ports.NotificationResult{}, nil
}

const (
	digestOrg  = "0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01"
	digestAcc  = "9f3c7b21-4d6e-4a8b-b5c0-1e2f3a4b5c06"
	digestAcc2 = "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c07"
)

func newDigest(now time.Time, minutes int, docs []ports.DigestDocument) (*DigestRunner, *digestBot, *digestClock) {
	repo := &digestRepo{
		recipients: []ports.DigestRecipient{{OrganizationID: digestOrg, AccountID: digestAcc, MaxUserID: 1036,
			NotifyLocalMinutes: minutes, OrganizationName: "Кафе", Timezone: "Europe/Moscow"}},
		docs: docs,
	}
	bot := &digestBot{}
	clk := &digestClock{now: now}
	return NewDigestRunner(repo, bot, clk, slog.New(slog.DiscardHandler)), bot, clk
}

var oneDoc = []ports.DigestDocument{{Title: "Лицензия", ValidUntil: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}}

func weekKey(monday time.Time) string {
	year, week := monday.ISOWeek()
	return fmt.Sprintf("digest:%s:%s:%d%02d", digestOrg, digestAcc, year, week)
}

// TestDigestSentOnMondayWindowOnce: сводка уходит в понедельник в окне после времени
// напоминаний, с ключом недели и кнопкой организации, и только один раз (BUG-007).
func TestDigestSentOnMondayWindowOnce(t *testing.T) {
	// 28.09.2026 — понедельник; 09:30 МСК = 06:30 UTC, время напоминаний 09:00.
	d, bot, _ := newDigest(time.Date(2026, 9, 28, 6, 30, 0, 0, time.UTC), 9*60, oneDoc)
	d.RunOnce(context.Background())
	d.RunOnce(context.Background())
	if len(bot.sent) != 1 {
		t.Fatalf("ожидалась одна сводка, отправлено %d", len(bot.sent))
	}
	req := bot.sent[0]
	if req.IdempotencyKey != weekKey(time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)) || req.RecipientMaxUserID != 1036 {
		t.Fatalf("ключ или получатель: %+v", req)
	}
	if !strings.Contains(req.Text, "Лицензия") || len(req.Buttons) != 1 || req.Buttons[0].Payload != "org_"+digestOrg {
		t.Fatalf("содержимое сводки: %+v", req)
	}
}

// TestDigestOutsideWindow: до времени напоминаний, после окна и не в понедельник сводки нет.
func TestDigestOutsideWindow(t *testing.T) {
	for name, now := range map[string]time.Time{
		"понедельник до времени":  time.Date(2026, 9, 28, 5, 59, 0, 0, time.UTC),
		"понедельник после окна":  time.Date(2026, 9, 28, 9, 1, 0, 0, time.UTC),
		"среда в то же время":     time.Date(2026, 9, 30, 6, 30, 0, 0, time.UTC),
		"вторник днём":            time.Date(2026, 9, 29, 6, 30, 0, 0, time.UTC),
		"воскресенье перед окном": time.Date(2026, 9, 27, 20, 0, 0, 0, time.UTC),
	} {
		d, bot, _ := newDigest(now, 9*60, oneDoc)
		d.RunOnce(context.Background())
		if len(bot.sent) != 0 {
			t.Errorf("%s: сводка не должна уходить, отправлено %d", name, len(bot.sent))
		}
	}
}

// TestDigestWindowCrossesMidnight: время 23:00 — окно продолжается во вторник после полуночи,
// а ключ остаётся ключом недели понедельника (BUG-019).
func TestDigestWindowCrossesMidnight(t *testing.T) {
	// Вторник 29.09, 00:30 МСК = понедельник 28.09, 21:30 UTC.
	d, bot, _ := newDigest(time.Date(2026, 9, 28, 21, 30, 0, 0, time.UTC), 23*60, oneDoc)
	d.RunOnce(context.Background())
	if len(bot.sent) != 1 || bot.sent[0].IdempotencyKey != weekKey(time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("сводка после полуночи в пределах окна: %+v", bot.sent)
	}
}

// TestDigestRespectsResponsible: документ с другим ответственным в сводку не попадает;
// пустая сводка не отправляется.
func TestDigestRespectsResponsible(t *testing.T) {
	docs := []ports.DigestDocument{{Title: "Чужой", ValidUntil: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		ResponsibleAccountID: digestAcc2}}
	d, bot, _ := newDigest(time.Date(2026, 9, 28, 6, 30, 0, 0, time.UTC), 9*60, docs)
	d.RunOnce(context.Background())
	if len(bot.sent) != 0 {
		t.Fatalf("сводка без своих документов не нужна: %+v", bot.sent)
	}
}

// TestDigestRetriesAfterEnqueueError: сбой передачи не помечает сводку отправленной.
func TestDigestRetriesAfterEnqueueError(t *testing.T) {
	d, bot, clk := newDigest(time.Date(2026, 9, 28, 6, 30, 0, 0, time.UTC), 9*60, oneDoc)
	bot.err = errors.New("bot-service недоступен")
	d.RunOnce(context.Background())
	bot.err = nil
	clk.now = clk.now.Add(10 * time.Minute)
	d.RunOnce(context.Background())
	if len(bot.sent) != 1 {
		t.Fatalf("после сбоя сводка должна уйти повторно, отправлено %d", len(bot.sent))
	}
}
