// Package grpcserver — входящий адаптер gRPC: перевод нормативного контракта
// vovremya.reminders.v1 в типы сценариев и обратно. Предметных правил здесь нет.
package grpcserver

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
	"vovremya/services/reminders/internal/app"
	"vovremya/services/reminders/internal/domain"
)

var aggregateToApp = map[remindersv1.AggregateType]string{
	remindersv1.AggregateType_AGGREGATE_TYPE_ORGANIZATION: app.AggregateOrganization,
	remindersv1.AggregateType_AGGREGATE_TYPE_MEMBERSHIP:   app.AggregateMembership,
	remindersv1.AggregateType_AGGREGATE_TYPE_DOCUMENT:     app.AggregateDocument,
	remindersv1.AggregateType_AGGREGATE_TYPE_ACCOUNT:      app.AggregateAccount,
}

var aggregateToProto = map[string]remindersv1.AggregateType{
	app.AggregateOrganization: remindersv1.AggregateType_AGGREGATE_TYPE_ORGANIZATION,
	app.AggregateMembership:   remindersv1.AggregateType_AGGREGATE_TYPE_MEMBERSHIP,
	app.AggregateDocument:     remindersv1.AggregateType_AGGREGATE_TYPE_DOCUMENT,
	app.AggregateAccount:      remindersv1.AggregateType_AGGREGATE_TYPE_ACCOUNT,
}

var eventTypeToApp = map[remindersv1.EventType]app.EventType{
	remindersv1.EventType_EVENT_TYPE_ORGANIZATION_STATE:   app.EventOrganizationState,
	remindersv1.EventType_EVENT_TYPE_ORGANIZATION_DELETED: app.EventOrganizationDeleted,
	remindersv1.EventType_EVENT_TYPE_MEMBERSHIP_STATE:     app.EventMembershipState,
	remindersv1.EventType_EVENT_TYPE_MEMBERSHIP_REMOVED:   app.EventMembershipRemoved,
	remindersv1.EventType_EVENT_TYPE_DOCUMENT_STATE:       app.EventDocumentState,
	remindersv1.EventType_EVENT_TYPE_DOCUMENT_DELETED:     app.EventDocumentDeleted,
	remindersv1.EventType_EVENT_TYPE_ACCOUNT_DELETED:      app.EventAccountDeleted,
}

var outcomeToProto = map[app.Outcome]remindersv1.EventOutcome{
	app.OutcomeApplied:   remindersv1.EventOutcome_EVENT_OUTCOME_APPLIED,
	app.OutcomeDuplicate: remindersv1.EventOutcome_EVENT_OUTCOME_DUPLICATE,
	app.OutcomeStale:     remindersv1.EventOutcome_EVENT_OUTCOME_STALE,
	app.OutcomeRejected:  remindersv1.EventOutcome_EVENT_OUTCOME_REJECTED,
}

var statusToProto = map[domain.Status]remindersv1.ReminderStatus{
	domain.StatusPlanned:   remindersv1.ReminderStatus_REMINDER_STATUS_PLANNED,
	domain.StatusHandedOff: remindersv1.ReminderStatus_REMINDER_STATUS_HANDED_OFF,
	domain.StatusCancelled: remindersv1.ReminderStatus_REMINDER_STATUS_CANCELLED,
	domain.StatusSkipped:   remindersv1.ReminderStatus_REMINDER_STATUS_SKIPPED,
}

// toAppEvent переводит событие контракта в тип сценария.
// Ошибка означает нарушение контракта на уровне конверта: ответ INVALID_ARGUMENT.
func toAppEvent(in *remindersv1.Event) (app.Event, error) {
	if in == nil {
		return app.Event{}, status.Error(codes.InvalidArgument, "event: обязательное поле")
	}
	evType, ok := eventTypeToApp[in.GetType()]
	if !ok {
		return app.Event{}, status.Error(codes.InvalidArgument, "type: неизвестный тип события")
	}
	aggType, ok := aggregateToApp[in.GetAggregateType()]
	if !ok {
		return app.Event{}, status.Error(codes.InvalidArgument, "aggregate_type: неизвестный тип агрегата")
	}
	if in.GetOccurredAt() == nil {
		return app.Event{}, status.Error(codes.InvalidArgument, "occurred_at: обязательное поле")
	}
	ev := app.Event{
		ID:               in.GetEventId(),
		Type:             evType,
		SchemaVersion:    in.GetSchemaVersion(),
		SourceService:    in.GetSourceService(),
		AggregateType:    aggType,
		AggregateID:      in.GetAggregateId(),
		AggregateVersion: in.GetAggregateVersion(),
		OccurredAt:       in.GetOccurredAt().AsTime(),
		Snapshot:         in.GetSnapshot(),
	}

	switch payload := in.GetPayload().(type) {
	case *remindersv1.Event_OrganizationState:
		org := payload.OrganizationState
		ev.Organization = &domain.Organization{
			ID:       org.GetOrganizationId(),
			Name:     org.GetName(),
			Timezone: org.GetTimezone(),
		}
	case *remindersv1.Event_OrganizationDeleted:
		ev.AggregateID = payload.OrganizationDeleted.GetOrganizationId()
	case *remindersv1.Event_MembershipState:
		m := payload.MembershipState
		kind := domain.AccountKindReview
		if m.GetAccountKind() == remindersv1.AccountKind_ACCOUNT_KIND_MAX {
			kind = domain.AccountKindMax
		}
		ev.Member = &domain.Member{
			OrganizationID:     m.GetOrganizationId(),
			AccountID:          m.GetAccountId(),
			Kind:               kind,
			MaxUserID:          m.GetMaxUserId(),
			NotifyEnabled:      m.GetNotifyEnabled(),
			NotifyLocalMinutes: int(m.GetNotifyLocalMinutes()),
		}
	case *remindersv1.Event_MembershipRemoved:
		r := payload.MembershipRemoved
		ev.AggregateID = app.MembershipAggregateID(r.GetOrganizationId(), r.GetAccountId())
	case *remindersv1.Event_DocumentState:
		d := payload.DocumentState
		doc := &domain.Document{
			ID:                   d.GetDocumentId(),
			OrganizationID:       d.GetOrganizationId(),
			Title:                d.GetTitle(),
			ResponsibleAccountID: d.GetResponsibleAccountId(),
		}
		if p := d.GetCurrentPeriod(); p != nil {
			doc.Period.ID = p.GetPeriodId()
			if v := p.GetValidFrom(); v != "" {
				parsed, err := domain.ParseDate(v)
				if err != nil {
					return app.Event{}, status.Error(codes.InvalidArgument, "valid_from: ожидается YYYY-MM-DD")
				}
				doc.Period.ValidFrom = &parsed
			}
			if v := p.GetValidUntil(); v != "" {
				parsed, err := domain.ParseDate(v)
				if err != nil {
					return app.Event{}, status.Error(codes.InvalidArgument, "valid_until: ожидается YYYY-MM-DD")
				}
				doc.Period.ValidUntil = &parsed
			}
		}
		for _, days := range d.GetReminderOffsetsDays() {
			if days > domain.MaxOffsetDays {
				return app.Event{}, status.Error(codes.InvalidArgument, "reminder_offsets_days: допустимо 0..365")
			}
			doc.OffsetsDays = append(doc.OffsetsDays, int(days))
		}
		ev.Document = doc
	case *remindersv1.Event_DocumentDeleted:
		ev.AggregateID = payload.DocumentDeleted.GetDocumentId()
	case *remindersv1.Event_AccountDeleted:
		ev.AggregateID = payload.AccountDeleted.GetAccountId()
	case nil:
		return app.Event{}, status.Error(codes.InvalidArgument, "payload: обязательное поле")
	}
	return ev, nil
}

func toProtoResult(r app.Result) *remindersv1.EventResult {
	return &remindersv1.EventResult{
		EventId:        r.EventID,
		Outcome:        outcomeToProto[r.Outcome],
		AppliedVersion: r.AppliedVersion,
		Message:        r.Message,
	}
}

// mapError переводит ошибку сценария в код gRPC.
func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrValidation):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "не найдено")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "запрос отменён")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "истёк срок обработки")
	default:
		// Внутренние ошибки (в том числе SQL) наружу не раскрываются.
		return status.Error(codes.Internal, "внутренняя ошибка сервиса")
	}
}
