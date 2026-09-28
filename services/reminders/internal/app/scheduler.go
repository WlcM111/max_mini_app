package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"vovremya/internal/platform/logging"
	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
)

// SchedulerConfig — параметры цикла отправки напоминаний.
type SchedulerConfig struct {
	Interval    time.Duration // период опроса плана
	Batch       int           // размер пакета захвата
	Lease       time.Duration // аренда захваченной строки
	Grace       time.Duration // допустимое опоздание напоминания
	Concurrency int           // предел одновременных вызовов bot-service
	Backoff     domain.BackoffParams
}

// Scheduler передаёт наступившие напоминания в bot-service.
// Захват строк выполняется с арендой и FOR UPDATE SKIP LOCKED, поэтому
// несколько экземпляров сервиса не обрабатывают одно напоминание дважды.
type Scheduler struct {
	reminders ports.ReminderRepo
	proj      ports.ProjectionRepo
	bot       ports.MessagingGateway
	clock     ports.Clock
	cfg       SchedulerConfig
	log       *slog.Logger
	metrics   *Metrics
}

// NewScheduler создаёт планировщик отправки.
func NewScheduler(reminders ports.ReminderRepo, proj ports.ProjectionRepo, bot ports.MessagingGateway,
	clock ports.Clock, cfg SchedulerConfig, log *slog.Logger, m *Metrics) *Scheduler {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}
	if cfg.Batch <= 0 {
		cfg.Batch = 100
	}
	return &Scheduler{reminders: reminders, proj: proj, bot: bot, clock: clock, cfg: cfg, log: log, metrics: m}
}

// Run выполняет цикл планировщика до отмены контекста.
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := s.Tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logging.From(logging.WithOperation(ctx, "scheduler.tick"), s.log).
					Error("scheduler tick failed", slog.Any("error", err))
				s.metrics.AppErrors.WithLabelValues("scheduler_tick").Inc()
			}
			s.observe(ctx)
		}
	}
}

// Tick обрабатывает один пакет готовых напоминаний и возвращает число переданных.
func (s *Scheduler) Tick(ctx context.Context) (int, error) {
	ctx = logging.WithOperation(ctx, "scheduler.tick")
	now := s.clock.Now()
	batch, err := s.reminders.ClaimDue(ctx, now, s.cfg.Lease, s.cfg.Batch)
	if err != nil {
		return 0, fmt.Errorf("claim due: %w", err)
	}
	if len(batch) == 0 {
		return 0, nil
	}

	var handed int
	results := make([]bool, len(batch))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.cfg.Concurrency)
	for i, r := range batch {
		g.Go(func() error {
			ok, err := s.process(gctx, r)
			if err != nil {
				return err
			}
			results[i] = ok
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return 0, err
	}
	for _, ok := range results {
		if ok {
			handed++
		}
	}
	return handed, nil
}

// process передаёт одно напоминание в bot-service.
// Возвращает true, если напоминание принято получателем.
func (s *Scheduler) process(ctx context.Context, r domain.Reminder) (bool, error) {
	now := s.clock.Now()
	if r.Expired(now, s.cfg.Grace) {
		s.metrics.RemindersHandoff.WithLabelValues("expired").Inc()
		_, err := s.reminders.MarkSkipped(ctx, r.ID, "expired")
		return false, err
	}

	doc, err := s.proj.GetDocument(ctx, r.DocumentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			s.metrics.RemindersHandoff.WithLabelValues("stale_state").Inc()
			_, mErr := s.reminders.MarkSkipped(ctx, r.ID, "document_missing")
			return false, mErr
		}
		return false, fmt.Errorf("get document: %w", err)
	}
	org, err := s.proj.GetOrganization(ctx, r.OrganizationID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			s.metrics.RemindersHandoff.WithLabelValues("stale_state").Inc()
			_, mErr := s.reminders.MarkSkipped(ctx, r.ID, "organization_missing")
			return false, mErr
		}
		return false, fmt.Errorf("get organization: %w", err)
	}
	member, err := s.proj.GetMember(ctx, r.OrganizationID, r.Key.AccountID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return false, fmt.Errorf("get member: %w", err)
	}

	// Защита от отправки по устаревшему состоянию: документ удалён, период
	// заменён продлением, организация удалена или получатель больше не получает
	// напоминания. Обычно такие строки уже отменены перепланированием, эта
	// проверка закрывает гонку между перепланированием и захватом строки.
	if doc.Deleted || org.Deleted || doc.Period.ID != r.Key.PeriodID || !member.Receives() {
		s.metrics.RemindersHandoff.WithLabelValues("stale_state").Inc()
		_, mErr := s.reminders.MarkSkipped(ctx, r.ID, "stale_state")
		return false, mErr
	}
	if doc.Period.ValidUntil == nil {
		s.metrics.RemindersHandoff.WithLabelValues("stale_state").Inc()
		_, mErr := s.reminders.MarkSkipped(ctx, r.ID, "no_expiry")
		return false, mErr
	}

	req := ports.NotificationRequest{
		IdempotencyKey:     r.Key.IdempotencyKey(),
		RecipientMaxUserID: member.MaxUserID,
		Text:               domain.ReminderText(doc.Title, org.Name, r.Key.DaysBefore, *doc.Period.ValidUntil),
		ButtonText:         domain.ButtonOpenDocument,
		ButtonPayload:      domain.DeepLinkPayload(doc.ID),
		// «Напомнить через неделю» добавляет bot-service (callback-кнопка).
		Buttons: []ports.NotificationButton{
			{Text: domain.ButtonOpenDocument, Payload: domain.DeepLinkPayload(doc.ID)},
			{Text: domain.ButtonRenewed, Payload: domain.RenewLinkPayload(doc.ID)},
		},
		NotAfter: r.NotAfter(s.cfg.Grace),
	}
	res, err := s.bot.Enqueue(ctx, req)
	if err != nil {
		var gwErr *ports.GatewayError
		if errors.As(err, &gwErr) && !gwErr.Retryable {
			s.metrics.RemindersHandoff.WithLabelValues("skipped").Inc()
			logging.From(ctx, s.log).Error("notification rejected by bot-service",
				slog.String("idempotency_key", req.IdempotencyKey),
				slog.String("error_code", gwErr.Code), slog.Any("error", err))
			_, mErr := s.reminders.MarkSkipped(ctx, r.ID, gwErr.Code)
			return false, mErr
		}
		code := "unavailable"
		if gwErr != nil {
			code = gwErr.Code
		}
		delay := s.cfg.Backoff.Backoff(r.Attempts)
		s.metrics.RemindersHandoff.WithLabelValues("retry").Inc()
		logging.From(ctx, s.log).Warn("notification handoff failed, retry scheduled",
			slog.String("idempotency_key", req.IdempotencyKey),
			slog.String("error_code", code), slog.Duration("delay", delay))
		_, mErr := s.reminders.Reschedule(ctx, r.ID, now.Add(delay), code)
		return false, mErr
	}

	updated, err := s.reminders.MarkHandedOff(ctx, r.ID, res.NotificationID, now)
	if err != nil {
		return false, fmt.Errorf("mark handed off: %w", err)
	}
	if !updated {
		// Напоминание было отменено или обработано параллельно после отправки.
		// Сообщение уже принято bot-service: повторная постановка невозможна
		// благодаря ключу идемпотентности (доставка «хотя бы один раз», L-04).
		logging.From(ctx, s.log).Warn("reminder changed during handoff",
			slog.String("idempotency_key", req.IdempotencyKey))
		s.metrics.RemindersHandoff.WithLabelValues("raced").Inc()
		return false, nil
	}
	s.metrics.RemindersHandoff.WithLabelValues("handed_off").Inc()
	return true, nil
}

// observe обновляет метрики состояния плана.
func (s *Scheduler) observe(ctx context.Context) {
	if due, err := s.reminders.CountDue(ctx, s.clock.Now()); err == nil {
		s.metrics.RemindersBacklog.Set(float64(due))
	}
	if planned, err := s.reminders.CountPlanned(ctx); err == nil {
		s.metrics.RemindersPlanned.Set(float64(planned))
	}
}
