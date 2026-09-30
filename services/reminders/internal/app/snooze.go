package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
)

// SnoozeResult — исход запроса отложить напоминание и момент повтора.
type SnoozeResult struct {
	Outcome domain.SnoozeOutcome
	DueAt   time.Time
}

// SnoozeService откладывает напоминание по кнопке «Напомнить через неделю» (ADR-036).
// Повтор становится строкой плана: его отменяют продление и удаление документа,
// исключение участника, удаление аккаунта или организации и отключение уведомлений,
// а планировщик перед отправкой повторно проверяет актуальность состояния.
type SnoozeService struct {
	tx        ports.TxManager
	proj      ports.ProjectionRepo
	reminders ports.ReminderRepo
	clock     ports.Clock
	log       *slog.Logger
}

// NewSnoozeService создаёт сценарий отложенного повтора.
func NewSnoozeService(tx ports.TxManager, proj ports.ProjectionRepo, reminders ports.ReminderRepo,
	clock ports.Clock, log *slog.Logger) *SnoozeService {
	return &SnoozeService{tx: tx, proj: proj, reminders: reminders, clock: clock, log: log}
}

// Snooze откладывает напоминание с ключом key по нажатию пользователя maxUserID.
// Исходы, не требующие повтора запроса (не найдено, чужое, устарело, поздно), возвращаются
// без ошибки; ошибка означает сбой хранилища — нажатие можно повторить.
func (s *SnoozeService) Snooze(ctx context.Context, key string, maxUserID int64) (SnoozeResult, error) {
	if maxUserID <= 0 {
		return SnoozeResult{Outcome: domain.SnoozeForbidden}, nil
	}
	planKey, err := domain.ParseIdempotencyKey(key)
	if err != nil {
		return SnoozeResult{Outcome: domain.SnoozeNotFound}, nil
	}
	var res SnoozeResult
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var txErr error
		res, txErr = s.snooze(ctx, planKey, maxUserID)
		return txErr
	})
	if err != nil {
		return SnoozeResult{}, err
	}
	return res, nil
}

func (s *SnoozeService) snooze(ctx context.Context, key domain.PlanKey, maxUserID int64) (SnoozeResult, error) {
	rem, err := s.reminders.FindByKey(ctx, key)
	if errors.Is(err, domain.ErrNotFound) {
		return SnoozeResult{Outcome: domain.SnoozeNotFound}, nil
	}
	if err != nil {
		return SnoozeResult{}, err
	}
	member, err := s.proj.GetMember(ctx, rem.OrganizationID, rem.Key.AccountID)
	if errors.Is(err, domain.ErrNotFound) {
		return SnoozeResult{Outcome: domain.SnoozeStale}, nil
	}
	if err != nil {
		return SnoozeResult{}, err
	}
	// Кнопку нажимает только адресат: чужое напоминание отложить нельзя.
	if member.MaxUserID != maxUserID {
		return SnoozeResult{Outcome: domain.SnoozeForbidden}, nil
	}
	doc, err := s.proj.GetDocument(ctx, rem.DocumentID)
	if errors.Is(err, domain.ErrNotFound) {
		return SnoozeResult{Outcome: domain.SnoozeStale}, nil
	}
	if err != nil {
		return SnoozeResult{}, err
	}
	org, err := s.proj.GetOrganization(ctx, rem.OrganizationID)
	if errors.Is(err, domain.ErrNotFound) {
		return SnoozeResult{Outcome: domain.SnoozeStale}, nil
	}
	if err != nil {
		return SnoozeResult{}, err
	}
	stillRecipient, err := s.recipientOf(ctx, doc, member)
	if err != nil {
		return SnoozeResult{}, err
	}
	if doc.Deleted || org.Deleted || doc.Period.ID != rem.Key.PeriodID || doc.Period.ValidUntil == nil || !stillRecipient {
		return SnoozeResult{Outcome: domain.SnoozeStale}, nil
	}
	loc, err := time.LoadLocation(org.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := s.clock.Now()
	due, daysLeft, ok := domain.SnoozeDue(now, *doc.Period.ValidUntil, member.NotifyLocalMinutes, loc)
	if !ok {
		return SnoozeResult{Outcome: domain.SnoozeTooLate}, nil
	}
	inserted, err := s.reminders.InsertSnooze(ctx, domain.Reminder{
		DocumentID:     rem.DocumentID,
		OrganizationID: rem.OrganizationID,
		Key: domain.PlanKey{PeriodID: rem.Key.PeriodID, AccountID: rem.Key.AccountID,
			DaysBefore: daysLeft, SnoozeDay: domain.SnoozeDay(now)},
		DueAt:         due,
		Status:        domain.StatusPlanned,
		NextAttemptAt: due,
	})
	if err != nil {
		return SnoozeResult{}, err
	}
	if !inserted {
		return SnoozeResult{Outcome: domain.SnoozeAlreadySnoozed, DueAt: due}, nil
	}
	s.log.Info("reminder snoozed", slog.String("document_id", rem.DocumentID), slog.Time("due_at", due))
	return SnoozeResult{Outcome: domain.SnoozeSnoozed, DueAt: due}, nil
}

// recipientOf повторяет правило domain.RecipientsFor: при назначенном ответственном,
// который получает напоминания, остальные участники их не получают.
func (s *SnoozeService) recipientOf(ctx context.Context, doc domain.Document, member domain.Member) (bool, error) {
	if !member.Receives() {
		return false, nil
	}
	if doc.ResponsibleAccountID == "" || doc.ResponsibleAccountID == member.AccountID {
		return true, nil
	}
	responsible, err := s.proj.GetMember(ctx, doc.OrganizationID, doc.ResponsibleAccountID)
	if errors.Is(err, domain.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !responsible.Receives(), nil
}
