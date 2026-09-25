package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"vovremya/internal/platform/logging"
	"vovremya/services/reminders/internal/ports"
)

// RetentionConfig — сроки хранения служебных данных сервиса.
type RetentionConfig struct {
	Interval     time.Duration // период запуска очистки
	InboxTTL     time.Duration // срок хранения журнала входящих событий
	FinalizedTTL time.Duration // срок хранения неактивных напоминаний
}

// RetentionJob удаляет данные с истёкшим сроком хранения
// (docs/database/data-model.md, раздел сроков хранения).
type RetentionJob struct {
	inbox     ports.InboxRepo
	reminders ports.ReminderRepo
	clock     ports.Clock
	cfg       RetentionConfig
	log       *slog.Logger
	metrics   *Metrics
}

// NewRetentionJob создаёт фоновую задачу очистки.
func NewRetentionJob(inbox ports.InboxRepo, reminders ports.ReminderRepo, clock ports.Clock,
	cfg RetentionConfig, log *slog.Logger, m *Metrics) *RetentionJob {
	return &RetentionJob{inbox: inbox, reminders: reminders, clock: clock, cfg: cfg, log: log, metrics: m}
}

// Run выполняет очистку по расписанию до отмены контекста.
func (j *RetentionJob) Run(ctx context.Context) error {
	ticker := time.NewTicker(j.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := j.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logging.From(logging.WithOperation(ctx, "retention"), j.log).
					Error("retention failed", slog.Any("error", err))
				j.metrics.AppErrors.WithLabelValues("retention").Inc()
			}
		}
	}
}

// RunOnce выполняет один проход очистки.
func (j *RetentionJob) RunOnce(ctx context.Context) error {
	now := j.clock.Now()
	events, err := j.inbox.DeleteOlderThan(ctx, now.Add(-j.cfg.InboxTTL))
	if err != nil {
		return err
	}
	rems, err := j.reminders.DeleteFinalizedBefore(ctx, now.Add(-j.cfg.FinalizedTTL))
	if err != nil {
		return err
	}
	if events > 0 || rems > 0 {
		logging.From(logging.WithOperation(ctx, "retention"), j.log).Info("retention done",
			slog.Int64("inbox_events_deleted", events), slog.Int64("reminders_deleted", rems))
	}
	return nil
}
