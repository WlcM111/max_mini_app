// Package remindersgrpc — клиент reminders-service: доставка событий core
// (IngestService) и чтение плана напоминаний (ReminderQueryService).
package remindersgrpc

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// Client — клиент reminders-service.
type Client struct {
	conn   *grpc.ClientConn
	ingest remindersv1.IngestServiceClient
	query  remindersv1.ReminderQueryServiceClient
}

var _ ports.RemindersGateway = (*Client)(nil)

// New открывает соединение с reminders-service.
func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("reminders grpc client: %w", err)
	}
	return &Client{
		conn:   conn,
		ingest: remindersv1.NewIngestServiceClient(conn),
		query:  remindersv1.NewReminderQueryServiceClient(conn),
	}, nil
}

// Close закрывает соединение.
func (c *Client) Close() error { return c.conn.Close() }

// ApplyEvents передаёт пакет событий core в reminders-service.
func (c *Client) ApplyEvents(ctx context.Context, batchID string, events []ports.IngestEvent) ([]ports.IngestResult, error) {
	pb := make([]*remindersv1.Event, 0, len(events))
	for _, e := range events {
		event, err := toProtoEvent(e)
		if err != nil {
			return nil, err
		}
		pb = append(pb, event)
	}
	resp, err := c.ingest.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{Events: pb, BatchId: batchID})
	if err != nil {
		return nil, fmt.Errorf("ApplyEvents: %w", err)
	}
	out := make([]ports.IngestResult, 0, len(resp.GetResults()))
	for _, r := range resp.GetResults() {
		res := ports.IngestResult{EventID: r.GetEventId(), Message: r.GetMessage()}
		switch r.GetOutcome() {
		case remindersv1.EventOutcome_EVENT_OUTCOME_APPLIED:
			res.Outcome = "applied"
		case remindersv1.EventOutcome_EVENT_OUTCOME_DUPLICATE:
			res.Outcome = "duplicate"
		case remindersv1.EventOutcome_EVENT_OUTCOME_STALE:
			res.Outcome = "stale"
		case remindersv1.EventOutcome_EVENT_OUTCOME_REJECTED:
			res.Outcome = "rejected"
		default:
			res.Outcome = "unspecified"
			res.Retryable = true
		}
		out = append(out, res)
	}
	return out, nil
}

// GetNextReminders возвращает ближайшие напоминания получателя по документам.
func (c *Client) GetNextReminders(ctx context.Context, accountPublicID string, documentIDs []string) ([]ports.NextReminder, error) {
	if len(documentIDs) == 0 {
		return nil, nil
	}
	resp, err := c.query.GetNextReminders(ctx, &remindersv1.GetNextRemindersRequest{
		AccountId: accountPublicID, DocumentIds: documentIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("GetNextReminders: %w", err)
	}
	out := make([]ports.NextReminder, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		out = append(out, ports.NextReminder{
			DocumentID: item.GetDocumentId(),
			DueAt:      item.GetDueAt().AsTime(),
			DaysBefore: int(item.GetDaysBefore()),
		})
	}
	return out, nil
}

// GetSyncStatus сообщает, учтена ли в плане последняя версия агрегата.
func (c *Client) GetSyncStatus(ctx context.Context, aggregateType, aggregateID string, expectedVersion int64) (bool, error) {
	resp, err := c.query.GetSyncStatus(ctx, &remindersv1.GetSyncStatusRequest{
		AggregateType:   aggregateTypeToProto(aggregateType),
		AggregateId:     aggregateID,
		ExpectedVersion: uint64(expectedVersion),
	})
	if err != nil {
		return false, fmt.Errorf("GetSyncStatus: %w", err)
	}
	return resp.GetInSync(), nil
}

func aggregateTypeToProto(t string) remindersv1.AggregateType {
	switch t {
	case domain.AggregateOrganization:
		return remindersv1.AggregateType_AGGREGATE_TYPE_ORGANIZATION
	case domain.AggregateMembership:
		return remindersv1.AggregateType_AGGREGATE_TYPE_MEMBERSHIP
	case domain.AggregateDocument:
		return remindersv1.AggregateType_AGGREGATE_TYPE_DOCUMENT
	case domain.AggregateAccount:
		return remindersv1.AggregateType_AGGREGATE_TYPE_ACCOUNT
	default:
		return remindersv1.AggregateType_AGGREGATE_TYPE_UNSPECIFIED
	}
}

// toProtoEvent собирает сообщение контракта из строки outbox.
func toProtoEvent(e ports.IngestEvent) (*remindersv1.Event, error) {
	event := &remindersv1.Event{
		EventId:          e.EventID,
		SchemaVersion:    1,
		SourceService:    "core",
		AggregateType:    aggregateTypeToProto(e.AggregateType),
		AggregateId:      e.AggregateID,
		AggregateVersion: uint64(e.AggregateVersion),
		OccurredAt:       timestamppb.New(e.OccurredAt),
		Snapshot:         e.Snapshot,
	}
	switch e.EventType {
	case domain.EventOrganizationState:
		var p domain.OrganizationStatePayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, err
		}
		event.Type = remindersv1.EventType_EVENT_TYPE_ORGANIZATION_STATE
		event.Payload = &remindersv1.Event_OrganizationState{OrganizationState: &remindersv1.OrganizationState{
			OrganizationId: p.OrganizationID, Name: p.Name, Timezone: p.Timezone,
		}}
	case domain.EventOrganizationDeleted:
		var p domain.OrganizationDeletedPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, err
		}
		event.Type = remindersv1.EventType_EVENT_TYPE_ORGANIZATION_DELETED
		event.Payload = &remindersv1.Event_OrganizationDeleted{
			OrganizationDeleted: &remindersv1.OrganizationDeleted{OrganizationId: p.OrganizationID},
		}
	case domain.EventMembershipState:
		var p domain.MembershipStatePayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, err
		}
		kind := remindersv1.AccountKind_ACCOUNT_KIND_REVIEW
		if p.AccountKind == string(domain.AccountMax) {
			kind = remindersv1.AccountKind_ACCOUNT_KIND_MAX
		}
		event.Type = remindersv1.EventType_EVENT_TYPE_MEMBERSHIP_STATE
		event.Payload = &remindersv1.Event_MembershipState{MembershipState: &remindersv1.MembershipState{
			OrganizationId: p.OrganizationID, AccountId: p.AccountID, AccountKind: kind,
			MaxUserId: p.MaxUserID, NotifyEnabled: p.NotifyEnabled,
			NotifyLocalMinutes: uint32(p.NotifyLocalMinutes),
		}}
	case domain.EventMembershipRemoved:
		var p domain.MembershipRemovedPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, err
		}
		event.Type = remindersv1.EventType_EVENT_TYPE_MEMBERSHIP_REMOVED
		event.Payload = &remindersv1.Event_MembershipRemoved{MembershipRemoved: &remindersv1.MembershipRemoved{
			OrganizationId: p.OrganizationID, AccountId: p.AccountID,
		}}
	case domain.EventDocumentState:
		var p domain.DocumentStatePayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, err
		}
		offsets := make([]uint32, 0, len(p.Offsets))
		for _, d := range p.Offsets {
			offsets = append(offsets, uint32(d))
		}
		event.Type = remindersv1.EventType_EVENT_TYPE_DOCUMENT_STATE
		event.Payload = &remindersv1.Event_DocumentState{DocumentState: &remindersv1.DocumentState{
			DocumentId: p.DocumentID, OrganizationId: p.OrganizationID, Title: p.Title,
			CurrentPeriod: &remindersv1.DocumentPeriod{
				PeriodId: p.CurrentPeriod.PeriodID, ValidFrom: p.CurrentPeriod.ValidFrom,
				ValidUntil: p.CurrentPeriod.ValidUntil,
			},
			ReminderOffsetsDays:  offsets,
			ResponsibleAccountId: p.ResponsibleAccountID,
		}}
	case domain.EventDocumentDeleted:
		var p domain.DocumentDeletedPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, err
		}
		event.Type = remindersv1.EventType_EVENT_TYPE_DOCUMENT_DELETED
		event.Payload = &remindersv1.Event_DocumentDeleted{DocumentDeleted: &remindersv1.DocumentDeleted{
			DocumentId: p.DocumentID, OrganizationId: p.OrganizationID,
		}}
	case domain.EventAccountDeleted:
		var p domain.AccountDeletedPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, err
		}
		event.Type = remindersv1.EventType_EVENT_TYPE_ACCOUNT_DELETED
		event.Payload = &remindersv1.Event_AccountDeleted{
			AccountDeleted: &remindersv1.AccountDeleted{AccountId: p.AccountID},
		}
	default:
		return nil, fmt.Errorf("неизвестный тип события outbox: %q", e.EventType)
	}
	return event, nil
}
