package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
)

// Replanner перестраивает план напоминаний по локальным проекциям.
// Вызывается внутри транзакции приёма события: проекция и план меняются вместе.
type Replanner struct {
	proj      ports.ProjectionRepo
	reminders ports.ReminderRepo
	clock     ports.Clock
	grace     time.Duration
}

// NewReplanner создаёт планировщик перестроения.
func NewReplanner(proj ports.ProjectionRepo, reminders ports.ReminderRepo, clock ports.Clock, grace time.Duration) *Replanner {
	return &Replanner{proj: proj, reminders: reminders, clock: clock, grace: grace}
}

// ReplanDocument перестраивает план одного документа:
// недостающие напоминания добавляются, лишние planned — отменяются,
// уже переданные в bot-service (handed_off) не изменяются.
func (r *Replanner) ReplanDocument(ctx context.Context, documentID string) error {
	doc, err := r.proj.GetDocument(ctx, documentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get document: %w", err)
	}
	if doc.Deleted {
		_, err := r.reminders.CancelByDocument(ctx, documentID)
		return err
	}
	org, err := r.proj.GetOrganization(ctx, doc.OrganizationID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Организация ещё не спроецирована: план построить нельзя,
			// событие организации перестроит его при поступлении.
			_, cancelErr := r.reminders.CancelByDocument(ctx, documentID)
			return cancelErr
		}
		return fmt.Errorf("get organization: %w", err)
	}
	members, err := r.proj.ListMembers(ctx, doc.OrganizationID)
	if err != nil {
		return fmt.Errorf("list members: %w", err)
	}

	items, err := domain.BuildPlan(domain.PlanInput{
		Organization: org,
		Document:     doc,
		Members:      members,
		Now:          r.clock.Now(),
		Grace:        r.grace,
	})
	if err != nil {
		return fmt.Errorf("build plan: %w", err)
	}

	keep := make([]domain.PlanKey, 0, len(items))
	for _, item := range items {
		keep = append(keep, item.Key)
		if err := r.reminders.Upsert(ctx, domain.Reminder{
			DocumentID:     doc.ID,
			OrganizationID: doc.OrganizationID,
			Key:            item.Key,
			DueAt:          item.DueAt,
			Status:         domain.StatusPlanned,
			NextAttemptAt:  item.DueAt,
		}); err != nil {
			return fmt.Errorf("upsert reminder: %w", err)
		}
	}
	if _, err := r.reminders.CancelOutsideKeys(ctx, doc.ID, keep); err != nil {
		return fmt.Errorf("cancel obsolete reminders: %w", err)
	}
	return nil
}

// ReplanOrganization перестраивает план всех документов организации.
// Используется при смене часового пояса и изменении состава или настроек участников.
func (r *Replanner) ReplanOrganization(ctx context.Context, organizationID string) error {
	ids, err := r.proj.ListDocumentIDs(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("list documents: %w", err)
	}
	for _, id := range ids {
		if err := r.ReplanDocument(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
