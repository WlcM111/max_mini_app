package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
)

// Окно отправки сводки: 3 часа после времени напоминаний участника в понедельник.
const digestWindow = 3 * time.Hour

// DigestRunner раз в неделю (понедельник, время напоминаний участника) отправляет
// сводку: просроченные документы и истекающие в ближайшие 7 дней. Повторы в окне
// гасит ключ идемпотентности bot-service (один ключ на участника и неделю).
type DigestRunner struct {
	repo     ports.DigestRepo
	bot      ports.MessagingGateway
	clock    ports.Clock
	log      *slog.Logger
	interval time.Duration
	sent     map[string]bool
}

func NewDigestRunner(repo ports.DigestRepo, bot ports.MessagingGateway, clock ports.Clock, log *slog.Logger) *DigestRunner {
	return &DigestRunner{repo: repo, bot: bot, clock: clock, log: log, interval: 10 * time.Minute, sent: map[string]bool{}}
}

// Run выполняет проверку каждые 10 минут до отмены контекста.
func (d *DigestRunner) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		d.RunOnce(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// RunOnce отправляет сводки тем, у кого сейчас окно отправки.
func (d *DigestRunner) RunOnce(ctx context.Context) {
	recipients, err := d.repo.ListDigestRecipients(ctx)
	if err != nil {
		d.log.Warn("digest: recipients", slog.Any("error", err))
		return
	}
	now := d.clock.Now()
	for _, r := range recipients {
		loc, err := time.LoadLocation(r.Timezone)
		if err != nil {
			loc = time.UTC
		}
		// Окно считается от понедельника текущей недели, а не от дня недели «сейчас»:
		// при времени напоминаний после 21:00 окно переходит через полночь во вторник (BUG-019).
		start := digestStart(now.In(loc), r.NotifyLocalMinutes, loc)
		if now.Before(start) || now.After(start.Add(digestWindow)) {
			continue
		}
		year, week := start.ISOWeek()
		key := fmt.Sprintf("digest:%s:%s:%d%02d", r.OrganizationID, r.AccountID, year, week)
		if d.sent[key] {
			continue
		}
		today := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		docs, err := d.repo.ListDigestDocuments(ctx, r.OrganizationID, today.AddDate(0, 0, 7))
		if err != nil {
			d.log.Warn("digest: documents", slog.Any("error", err))
			continue
		}
		items := make([]domain.DigestItem, 0, len(docs))
		for _, doc := range docs {
			// Личные документы других ответственных в сводку не попадают.
			if doc.ResponsibleAccountID != "" && doc.ResponsibleAccountID != r.AccountID {
				continue
			}
			until := doc.ValidUntil
			items = append(items, domain.DigestItem{Title: doc.Title,
				ValidUntil: time.Date(until.Year(), until.Month(), until.Day(), 0, 0, 0, 0, time.UTC)})
		}
		if len(items) == 0 {
			d.sent[key] = true
			continue
		}
		_, err = d.bot.Enqueue(ctx, ports.NotificationRequest{
			IdempotencyKey:     key,
			RecipientMaxUserID: r.MaxUserID,
			Text:               domain.DigestText(r.OrganizationName, today, items),
			Buttons:            []ports.NotificationButton{{Text: domain.ButtonOpenApp, Payload: domain.OrganizationLinkPayload(r.OrganizationID)}},
			NotAfter:           now.Add(12 * time.Hour),
		})
		if err != nil {
			d.log.Warn("digest: enqueue", slog.Any("error", err), slog.String("idempotency_key", key))
			continue
		}
		d.sent[key] = true
	}
	if len(d.sent) > 10000 {
		d.sent = map[string]bool{}
	}
}

// digestStart возвращает начало окна сводки для недели, в которую попадает момент local:
// понедельник этой недели в поясе организации во время напоминаний участника.
func digestStart(local time.Time, notifyMinutes int, loc *time.Location) time.Time {
	sinceMonday := (int(local.Weekday()) + 6) % 7
	monday := time.Date(local.Year(), local.Month(), local.Day()-sinceMonday, 0, 0, 0, 0, loc)
	return monday.Add(time.Duration(notifyMinutes) * time.Minute)
}
