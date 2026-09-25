package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// appendEvent записывает событие в outbox. Вызывается только внутри транзакции
// изменения предметных данных (handoff core-service-v2 §2.1).
func (a *App) appendEvent(ctx context.Context, eventType, aggregateType, aggregateID string,
	version int64, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return a.Outbox.Append(ctx, domain.OutboxEvent{
		EventID:          a.Random.UUID(),
		EventType:        eventType,
		AggregateType:    aggregateType,
		AggregateID:      aggregateID,
		AggregateVersion: version,
		Payload:          raw,
		Status:           domain.OutboxPending,
		NextAttemptAt:    a.Clock.Now(),
		CreatedAt:        a.Clock.Now(),
	})
}

func (a *App) appendOrganizationEvent(ctx context.Context, org domain.Organization) error {
	return a.appendEvent(ctx, domain.EventOrganizationState, domain.AggregateOrganization, org.PublicID,
		int64(org.Version), domain.OrganizationStatePayload{
			OrganizationID: org.PublicID, Name: org.Name, Timezone: org.Timezone,
		})
}

func (a *App) appendOrganizationDeleted(ctx context.Context, org domain.Organization) error {
	return a.appendEvent(ctx, domain.EventOrganizationDeleted, domain.AggregateOrganization, org.PublicID,
		int64(org.Version)+1, domain.OrganizationDeletedPayload{OrganizationID: org.PublicID})
}

func (a *App) appendMembershipEvent(ctx context.Context, org domain.Organization, account domain.Account,
	role domain.Role, notifyEnabled bool, notifyMinutes, version int) error {
	_ = role // роль не влияет на план напоминаний, но фиксируется в аудите
	return a.appendEvent(ctx, domain.EventMembershipState, domain.AggregateMembership,
		domain.MembershipAggregateID(org.PublicID, account.PublicID), int64(version),
		domain.MembershipStatePayload{
			OrganizationID:     org.PublicID,
			AccountID:          account.PublicID,
			AccountKind:        string(account.Kind),
			MaxUserID:          account.MaxUserID,
			NotifyEnabled:      notifyEnabled,
			NotifyLocalMinutes: notifyMinutes,
		})
}

func (a *App) appendMembershipRemoved(ctx context.Context, orgPublicID, accountPublicID string, version int) error {
	return a.appendEvent(ctx, domain.EventMembershipRemoved, domain.AggregateMembership,
		domain.MembershipAggregateID(orgPublicID, accountPublicID), int64(version),
		domain.MembershipRemovedPayload{OrganizationID: orgPublicID, AccountID: accountPublicID})
}

func (a *App) appendDocumentEvent(ctx context.Context, orgPublicID string, doc domain.Document) error {
	period := domain.DocumentPeriodPayload{PeriodID: doc.CurrentPeriod.PublicID}
	if doc.CurrentPeriod.ValidFrom != nil {
		period.ValidFrom = domain.FormatLocalDate(*doc.CurrentPeriod.ValidFrom)
	}
	if doc.CurrentPeriod.ValidUntil != nil {
		period.ValidUntil = domain.FormatLocalDate(*doc.CurrentPeriod.ValidUntil)
	}
	offsets := make([]int, 0, len(doc.ReminderOffsets))
	offsets = append(offsets, doc.ReminderOffsets...)
	return a.appendEvent(ctx, domain.EventDocumentState, domain.AggregateDocument, doc.PublicID,
		int64(doc.Version), domain.DocumentStatePayload{
			DocumentID: doc.PublicID, OrganizationID: orgPublicID, Title: doc.Title,
			CurrentPeriod: period, Offsets: offsets,
		})
}

func (a *App) appendDocumentDeleted(ctx context.Context, orgPublicID string, doc domain.Document) error {
	return a.appendEvent(ctx, domain.EventDocumentDeleted, domain.AggregateDocument, doc.PublicID,
		int64(doc.Version)+1, domain.DocumentDeletedPayload{
			DocumentID: doc.PublicID, OrganizationID: orgPublicID,
		})
}

func (a *App) appendAccountDeleted(ctx context.Context, account domain.Account) error {
	return a.appendEvent(ctx, domain.EventAccountDeleted, domain.AggregateAccount, account.PublicID, 1,
		domain.AccountDeletedPayload{AccountID: account.PublicID})
}

// RunRelay доставляет события в reminders-service: по расписанию и по сигналу
// синхронной попытки после фиксации транзакции.
func (a *App) RunRelay(ctx context.Context) error {
	ticker := time.NewTicker(a.Settings.RelayInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-a.relaySignal:
		}
		if _, err := a.DeliverOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.Log.Warn("outbox delivery failed", slog.Any("error", err))
		}
		if pending, err := a.Outbox.PendingCount(ctx); err == nil {
			a.Metrics.OutboxPending.Set(float64(pending))
		}
	}
}

// DeliverOnce отправляет один пакет готовых событий и фиксирует исходы.
func (a *App) DeliverOnce(ctx context.Context) (int, error) {
	now := a.Clock.Now()
	events, err := a.Outbox.ClaimReady(ctx, now, a.Settings.OutboxLease, a.Settings.OutboxBatch)
	if err != nil || len(events) == 0 {
		return 0, err
	}
	batch := make([]ports.IngestEvent, 0, len(events))
	for _, e := range events {
		batch = append(batch, ports.IngestEvent{
			EventID: e.EventID, EventType: e.EventType, AggregateType: e.AggregateType,
			AggregateID: e.AggregateID, AggregateVersion: e.AggregateVersion,
			Payload: e.Payload, Snapshot: e.Snapshot, OccurredAt: e.CreatedAt,
		})
	}
	callCtx, cancel := context.WithTimeout(ctx, a.Settings.RemindersRPCTimeout)
	results, err := a.Reminders.ApplyEvents(callCtx, "core-relay", batch)
	cancel()
	if err != nil {
		for _, e := range events {
			next := domain.OutboxBackoff(now, e.Attempts+1, a.Random.Float64())
			if markErr := a.Outbox.MarkFailed(ctx, e.ID, next, "unavailable"); markErr != nil {
				a.Log.Warn("outbox mark failed", slog.Any("error", markErr))
			}
		}
		a.Metrics.OutboxDelivery.WithLabelValues("unavailable").Add(float64(len(events)))
		return 0, nil
	}

	outcome := make(map[string]ports.IngestResult, len(results))
	for _, r := range results {
		outcome[r.EventID] = r
	}
	delivered := 0
	for _, e := range events {
		r, ok := outcome[e.EventID]
		switch {
		case !ok:
			next := domain.OutboxBackoff(now, e.Attempts+1, a.Random.Float64())
			_ = a.Outbox.MarkFailed(ctx, e.ID, next, "no_result")
			a.Metrics.OutboxDelivery.WithLabelValues("no_result").Inc()
		case r.Outcome == "applied" || r.Outcome == "duplicate" || r.Outcome == "stale":
			if err := a.Outbox.MarkSent(ctx, e.ID, a.Clock.Now()); err != nil {
				return delivered, err
			}
			delivered++
			a.Metrics.OutboxDelivery.WithLabelValues(r.Outcome).Inc()
		case r.Retryable:
			next := domain.OutboxBackoff(now, e.Attempts+1, a.Random.Float64())
			_ = a.Outbox.MarkFailed(ctx, e.ID, next, "retryable")
			a.Metrics.OutboxDelivery.WithLabelValues("retryable").Inc()
		default:
			// Событие отвергнуто получателем: повтор без изменения данных не поможет,
			// поэтому попытки разрежаются до суток и фиксируются в журнале.
			_ = a.Outbox.MarkFailed(ctx, e.ID, now.Add(24*time.Hour), "rejected")
			a.Metrics.OutboxDelivery.WithLabelValues("rejected").Inc()
			a.Log.Error("reminders rejected event",
				slog.String("event_id", e.EventID),
				slog.String("event_type", e.EventType),
				slog.String("message", r.Message))
		}
	}
	return delivered, nil
}

// flushAfterCommit — синхронная попытка доставки после фиксации транзакции
// (handoff core-service-v2 §2.3). Возвращает состояние плана для ответа API.
func (a *App) flushAfterCommit(ctx context.Context) {
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.Settings.SyncFlushTimeout)
	defer cancel()
	if _, err := a.DeliverOnce(flushCtx); err != nil {
		a.Log.Debug("sync flush failed", slog.Any("error", err))
	}
	select {
	case a.relaySignal <- struct{}{}:
	default:
	}
}

// remindersStateFor определяет состояние плана по одному агрегату.
func (a *App) remindersStateFor(ctx context.Context, aggregateType, aggregateID string, planAvailable bool) domain.RemindersState {
	if !planAvailable {
		return domain.RemindersUnavailable
	}
	pending, err := a.Outbox.HasPending(ctx, aggregateType, aggregateID)
	if err != nil {
		a.Log.Warn("outbox pending check failed", slog.Any("error", err))
		return domain.RemindersUnavailable
	}
	if pending {
		return domain.RemindersPending
	}
	return domain.RemindersActual
}
