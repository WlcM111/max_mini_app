package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"vovremya/internal/platform/logging"
	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
)

// IngestService принимает изменения из core-service, обновляет локальные проекции
// и перестраивает план напоминаний. Каждое событие обрабатывается в собственной
// транзакции: проекция и план всегда согласованы между собой.
type IngestService struct {
	tx      ports.TxManager
	inbox   ports.InboxRepo
	proj    ports.ProjectionRepo
	planner *Replanner
	clock   ports.Clock
	log     *slog.Logger
	metrics *Metrics
}

// NewIngestService собирает сценарий приёма событий.
func NewIngestService(tx ports.TxManager, inbox ports.InboxRepo, proj ports.ProjectionRepo,
	planner *Replanner, clock ports.Clock, log *slog.Logger, m *Metrics) *IngestService {
	return &IngestService{tx: tx, inbox: inbox, proj: proj, planner: planner, clock: clock, log: log, metrics: m}
}

// ApplyBatch обрабатывает пакет событий и возвращает результат по каждому.
// Пакет не атомарен: отказ одного события не отменяет уже применённые.
func (s *IngestService) ApplyBatch(ctx context.Context, events []Event) ([]Result, error) {
	results := make([]Result, 0, len(events))
	for _, ev := range events {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		res, err := s.Apply(ctx, ev)
		if err != nil {
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

// Apply обрабатывает одно событие идемпотентно по event_id.
func (s *IngestService) Apply(ctx context.Context, ev Event) (Result, error) {
	if err := ev.Validate(); err != nil {
		s.metrics.IngestEvents.WithLabelValues(string(ev.Type), string(OutcomeRejected)).Inc()
		logging.From(ctx, s.log).Warn("event rejected",
			slog.String("event_id", ev.ID), slog.String("event_type", string(ev.Type)),
			slog.String("reason", err.Error()))
		return Result{EventID: ev.ID, Outcome: OutcomeRejected, Message: err.Error()}, nil
	}

	var result Result
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		state, err := s.proj.AggregateVersion(ctx, ev.AggregateType, ev.AggregateID)
		if err != nil {
			return fmt.Errorf("aggregate version: %w", err)
		}
		outcome := OutcomeApplied
		if state.Exists && ev.AggregateVersion <= state.Version {
			outcome = OutcomeStale
		}
		inserted, err := s.inbox.Insert(ctx, ports.InboxRecord{
			EventID:          ev.ID,
			EventType:        string(ev.Type),
			AggregateType:    ev.AggregateType,
			AggregateID:      ev.AggregateID,
			AggregateVersion: ev.AggregateVersion,
			SchemaVersion:    ev.SchemaVersion,
			SourceService:    ev.SourceService,
			Snapshot:         ev.Snapshot,
			Outcome:          string(outcome),
			OccurredAt:       ev.OccurredAt,
		})
		if err != nil {
			return fmt.Errorf("inbox insert: %w", err)
		}
		if !inserted {
			result = Result{EventID: ev.ID, Outcome: OutcomeDuplicate, AppliedVersion: state.Version}
			return nil
		}
		if outcome == OutcomeStale {
			result = Result{EventID: ev.ID, Outcome: OutcomeStale, AppliedVersion: state.Version}
			return nil
		}
		if err := s.applyPayload(ctx, ev); err != nil {
			return err
		}
		result = Result{EventID: ev.ID, Outcome: OutcomeApplied, AppliedVersion: ev.AggregateVersion}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	s.metrics.IngestEvents.WithLabelValues(string(ev.Type), string(result.Outcome)).Inc()
	if result.Outcome == OutcomeApplied {
		s.metrics.ProjectionLagSecond.Set(s.clock.Now().Sub(ev.OccurredAt).Seconds())
		logging.From(ctx, s.log).Debug("event applied",
			slog.String("event_id", ev.ID), slog.String("event_type", string(ev.Type)),
			slog.String("aggregate_id", ev.AggregateID), slog.Uint64("aggregate_version", ev.AggregateVersion))
	}
	return result, nil
}

// applyPayload обновляет проекцию и перестраивает затронутую часть плана.
// Вызывается внутри транзакции.
func (s *IngestService) applyPayload(ctx context.Context, ev Event) error {
	switch ev.Type {
	case EventOrganizationState:
		org := *ev.Organization
		org.Version = ev.AggregateVersion
		prev, err := s.proj.GetOrganization(ctx, org.ID)
		timezoneChanged := true
		if err == nil {
			timezoneChanged = prev.Timezone != org.Timezone
		} else if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("get organization: %w", err)
		}
		if err := s.proj.UpsertOrganization(ctx, org); err != nil {
			return fmt.Errorf("upsert organization: %w", err)
		}
		// Часовой пояс входит в вычисление due_at, название — в текст сообщения.
		// Пересчёт нужен только при смене пояса; название читается при отправке.
		if timezoneChanged {
			return s.planner.ReplanOrganization(ctx, org.ID)
		}
		return nil

	case EventOrganizationDeleted:
		if err := s.proj.MarkOrganizationDeleted(ctx, ev.AggregateID, ev.AggregateVersion); err != nil {
			return fmt.Errorf("delete organization: %w", err)
		}
		if _, err := s.planner.reminders.CancelByOrganization(ctx, ev.AggregateID); err != nil {
			return fmt.Errorf("cancel organization reminders: %w", err)
		}
		return nil

	case EventMembershipState:
		m := *ev.Member
		m.Version = ev.AggregateVersion
		if err := s.proj.UpsertMember(ctx, m); err != nil {
			return fmt.Errorf("upsert member: %w", err)
		}
		return s.planner.ReplanOrganization(ctx, m.OrganizationID)

	case EventMembershipRemoved:
		orgID, accountID, _ := SplitMembershipAggregateID(ev.AggregateID)
		if err := s.proj.MarkMemberRemoved(ctx, orgID, accountID, ev.AggregateVersion); err != nil {
			return fmt.Errorf("remove member: %w", err)
		}
		if _, err := s.planner.reminders.CancelByMember(ctx, orgID, accountID); err != nil {
			return fmt.Errorf("cancel member reminders: %w", err)
		}
		// Если ушёл ответственный, напоминания по его документам снова получают все участники.
		if err := s.planner.ReplanOrganization(ctx, orgID); err != nil {
			return fmt.Errorf("replan organization: %w", err)
		}
		return nil

	case EventDocumentState:
		d := *ev.Document
		d.Version = ev.AggregateVersion
		if err := s.proj.UpsertDocument(ctx, d); err != nil {
			return fmt.Errorf("upsert document: %w", err)
		}
		return s.planner.ReplanDocument(ctx, d.ID)

	case EventDocumentDeleted:
		if err := s.proj.MarkDocumentDeleted(ctx, ev.AggregateID, ev.AggregateVersion); err != nil {
			return fmt.Errorf("delete document: %w", err)
		}
		if _, err := s.planner.reminders.CancelByDocument(ctx, ev.AggregateID); err != nil {
			return fmt.Errorf("cancel document reminders: %w", err)
		}
		return nil

	case EventAccountDeleted:
		if _, err := s.planner.reminders.CancelByAccount(ctx, ev.AggregateID); err != nil {
			return fmt.Errorf("cancel account reminders: %w", err)
		}
		if err := s.proj.RemoveMemberships(ctx, ev.AggregateID); err != nil {
			return fmt.Errorf("remove memberships: %w", err)
		}
		return nil
	}
	return fmt.Errorf("%w: неизвестный тип события %s", domain.ErrValidation, ev.Type)
}

// IngestState — применённое состояние агрегата для сверки с core.
type IngestState struct {
	AggregateType string
	AggregateID   string
	Version       uint64
	Deleted       bool
	AppliedAt     time.Time
	Exists        bool
}

// GetIngestState возвращает применённые версии агрегатов.
// Используется relay сервиса core для обнаружения рассогласования.
func (s *IngestService) GetIngestState(ctx context.Context, refs []IngestState) ([]IngestState, error) {
	out := make([]IngestState, 0, len(refs))
	for _, ref := range refs {
		st, err := s.proj.AggregateVersion(ctx, ref.AggregateType, ref.AggregateID)
		if err != nil {
			return nil, fmt.Errorf("aggregate version: %w", err)
		}
		out = append(out, IngestState{
			AggregateType: ref.AggregateType,
			AggregateID:   ref.AggregateID,
			Version:       st.Version,
			Deleted:       st.Deleted,
			AppliedAt:     st.AppliedAt,
			Exists:        st.Exists,
		})
	}
	return out, nil
}
