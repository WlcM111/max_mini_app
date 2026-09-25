package app

import (
	"context"
	"fmt"

	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
)

// QueryService отдаёт план напоминаний потребителю публичного API (core-service).
type QueryService struct {
	reminders ports.ReminderRepo
	proj      ports.ProjectionRepo
}

// NewQueryService создаёт сценарии чтения плана.
func NewQueryService(reminders ports.ReminderRepo, proj ports.ProjectionRepo) *QueryService {
	return &QueryService{reminders: reminders, proj: proj}
}

// MaxDocumentsPerQuery — предел размера запроса (защита от неограниченных выборок).
const MaxDocumentsPerQuery = 200

// NextReminders возвращает ближайшее запланированное напоминание получателю
// по каждому из запрошенных документов.
func (q *QueryService) NextReminders(ctx context.Context, accountID string, documentIDs []string) (map[string]domain.Reminder, error) {
	if !domain.IsUUID(accountID) {
		return nil, domain.ValidationError{Field: "account_id", Reason: "ожидается UUID v4"}
	}
	if len(documentIDs) == 0 {
		return map[string]domain.Reminder{}, nil
	}
	if len(documentIDs) > MaxDocumentsPerQuery {
		return nil, domain.ValidationError{Field: "document_ids", Reason: "не более 200 документов в запросе"}
	}
	for _, id := range documentIDs {
		if !domain.IsUUID(id) {
			return nil, domain.ValidationError{Field: "document_ids", Reason: "ожидается UUID v4"}
		}
	}
	res, err := q.reminders.NextForAccount(ctx, accountID, documentIDs)
	if err != nil {
		return nil, fmt.Errorf("next reminders: %w", err)
	}
	return res, nil
}

// DocumentPlan — план документа и версия применённой проекции.
type DocumentPlan struct {
	DocumentID     string
	AppliedVersion uint64
	Items          []domain.Reminder
}

// DocumentPlan возвращает план напоминаний документа для всех получателей.
func (q *QueryService) DocumentPlan(ctx context.Context, documentID string) (DocumentPlan, error) {
	if !domain.IsUUID(documentID) {
		return DocumentPlan{}, domain.ValidationError{Field: "document_id", Reason: "ожидается UUID v4"}
	}
	state, err := q.proj.AggregateVersion(ctx, AggregateDocument, documentID)
	if err != nil {
		return DocumentPlan{}, fmt.Errorf("aggregate version: %w", err)
	}
	if !state.Exists {
		return DocumentPlan{}, domain.ErrNotFound
	}
	items, err := q.reminders.ListByDocument(ctx, documentID)
	if err != nil {
		return DocumentPlan{}, fmt.Errorf("list by document: %w", err)
	}
	return DocumentPlan{DocumentID: documentID, AppliedVersion: state.Version, Items: items}, nil
}

// SyncStatus — состояние синхронизации агрегата с core-service.
type SyncStatus struct {
	AppliedVersion uint64
	InSync         bool
	AppliedAt      string
}

// SyncStatus сообщает, учтена ли в плане версия агрегата из core.
// Позволяет core отличить актуальное значение от ещё не пересчитанного (AC-05).
func (q *QueryService) SyncStatus(ctx context.Context, aggregateType, aggregateID string, expected uint64) (ports.AggregateState, bool, error) {
	switch aggregateType {
	case AggregateOrganization, AggregateMembership, AggregateDocument, AggregateAccount:
	default:
		return ports.AggregateState{}, false, domain.ValidationError{Field: "aggregate_type", Reason: "неизвестный тип агрегата"}
	}
	state, err := q.proj.AggregateVersion(ctx, aggregateType, aggregateID)
	if err != nil {
		return ports.AggregateState{}, false, fmt.Errorf("aggregate version: %w", err)
	}
	return state, state.Exists && state.Version >= expected, nil
}
