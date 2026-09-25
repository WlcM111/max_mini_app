package grpcserver

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
	"vovremya/services/reminders/internal/app"
)

// IngestServer реализует vovremya.reminders.v1.IngestService.
type IngestServer struct {
	remindersv1.UnimplementedIngestServiceServer

	ingest   *app.IngestService
	log      *slog.Logger
	maxBatch int
}

// NewIngestServer создаёт сервер приёма событий.
func NewIngestServer(ingest *app.IngestService, log *slog.Logger, maxBatch int) *IngestServer {
	return &IngestServer{ingest: ingest, log: log, maxBatch: maxBatch}
}

// ApplyEvents принимает пакет событий core-service.
func (s *IngestServer) ApplyEvents(ctx context.Context, req *remindersv1.ApplyEventsRequest) (*remindersv1.ApplyEventsResponse, error) {
	events := req.GetEvents()
	if len(events) == 0 {
		return nil, status.Error(codes.InvalidArgument, "events: требуется хотя бы одно событие")
	}
	if len(events) > s.maxBatch {
		return nil, status.Errorf(codes.InvalidArgument, "events: не более %d событий в пакете", s.maxBatch)
	}

	results := make([]*remindersv1.EventResult, 0, len(events))
	for _, in := range events {
		ev, err := toAppEvent(in)
		if err != nil {
			// Нарушение конверта контракта: событие не принимается,
			// повтор такого же запроса даст тот же результат.
			results = append(results, &remindersv1.EventResult{
				EventId: in.GetEventId(),
				Outcome: remindersv1.EventOutcome_EVENT_OUTCOME_REJECTED,
				Message: status.Convert(err).Message(),
			})
			continue
		}
		res, err := s.ingest.Apply(ctx, ev)
		if err != nil {
			return nil, mapError(err)
		}
		results = append(results, toProtoResult(res))
	}
	return &remindersv1.ApplyEventsResponse{Results: results}, nil
}

// GetIngestState возвращает применённые версии агрегатов.
func (s *IngestServer) GetIngestState(ctx context.Context, req *remindersv1.GetIngestStateRequest) (*remindersv1.GetIngestStateResponse, error) {
	refs := req.GetAggregates()
	if len(refs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "aggregates: требуется хотя бы один агрегат")
	}
	if len(refs) > 500 {
		return nil, status.Error(codes.InvalidArgument, "aggregates: не более 500 агрегатов в запросе")
	}
	in := make([]app.IngestState, 0, len(refs))
	for _, ref := range refs {
		aggType, ok := aggregateToApp[ref.GetAggregateType()]
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "aggregate_type: неизвестный тип агрегата")
		}
		in = append(in, app.IngestState{AggregateType: aggType, AggregateID: ref.GetAggregateId()})
	}
	states, err := s.ingest.GetIngestState(ctx, in)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*remindersv1.AggregateState, 0, len(states))
	for _, st := range states {
		item := &remindersv1.AggregateState{
			AggregateType:  aggregateToProto[st.AggregateType],
			AggregateId:    st.AggregateID,
			AppliedVersion: st.Version,
			Deleted:        st.Deleted,
		}
		if !st.AppliedAt.IsZero() {
			item.AppliedAt = timestamppb.New(st.AppliedAt.UTC())
		}
		out = append(out, item)
	}
	return &remindersv1.GetIngestStateResponse{Aggregates: out}, nil
}

// QueryServer реализует vovremya.reminders.v1.ReminderQueryService.
type QueryServer struct {
	remindersv1.UnimplementedReminderQueryServiceServer

	query *app.QueryService
	log   *slog.Logger
}

// NewQueryServer создаёт сервер чтения плана.
func NewQueryServer(query *app.QueryService, log *slog.Logger) *QueryServer {
	return &QueryServer{query: query, log: log}
}

// GetNextReminders возвращает ближайшие напоминания получателя по документам.
func (s *QueryServer) GetNextReminders(ctx context.Context, req *remindersv1.GetNextRemindersRequest) (*remindersv1.GetNextRemindersResponse, error) {
	res, err := s.query.NextReminders(ctx, req.GetAccountId(), req.GetDocumentIds())
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]*remindersv1.NextReminder, 0, len(res))
	for documentID, rem := range res {
		items = append(items, &remindersv1.NextReminder{
			DocumentId: documentID,
			DueAt:      timestamppb.New(rem.DueAt.UTC()),
			DaysBefore: uint32(rem.Key.DaysBefore),
		})
	}
	return &remindersv1.GetNextRemindersResponse{Items: items}, nil
}

// GetDocumentPlan возвращает план документа для всех получателей.
func (s *QueryServer) GetDocumentPlan(ctx context.Context, req *remindersv1.GetDocumentPlanRequest) (*remindersv1.GetDocumentPlanResponse, error) {
	plan, err := s.query.DocumentPlan(ctx, req.GetDocumentId())
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]*remindersv1.PlannedReminder, 0, len(plan.Items))
	for _, rem := range plan.Items {
		item := &remindersv1.PlannedReminder{
			AccountId:  rem.Key.AccountID,
			DaysBefore: uint32(rem.Key.DaysBefore),
			DueAt:      timestamppb.New(rem.DueAt.UTC()),
			Status:     statusToProto[rem.Status],
		}
		if rem.HandedOffAt != nil {
			item.HandedOffAt = timestamppb.New(rem.HandedOffAt.UTC())
		}
		items = append(items, item)
	}
	return &remindersv1.GetDocumentPlanResponse{
		DocumentId:     plan.DocumentID,
		AppliedVersion: plan.AppliedVersion,
		Items:          items,
	}, nil
}

// GetSyncStatus сообщает, учтена ли в плане указанная версия агрегата core.
func (s *QueryServer) GetSyncStatus(ctx context.Context, req *remindersv1.GetSyncStatusRequest) (*remindersv1.GetSyncStatusResponse, error) {
	aggType, ok := aggregateToApp[req.GetAggregateType()]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "aggregate_type: неизвестный тип агрегата")
	}
	if req.GetAggregateId() == "" {
		return nil, status.Error(codes.InvalidArgument, "aggregate_id: обязательное поле")
	}
	state, inSync, err := s.query.SyncStatus(ctx, aggType, req.GetAggregateId(), req.GetExpectedVersion())
	if err != nil {
		return nil, mapError(err)
	}
	resp := &remindersv1.GetSyncStatusResponse{AppliedVersion: state.Version, InSync: inSync}
	if !state.AppliedAt.IsZero() {
		resp.AppliedAt = timestamppb.New(state.AppliedAt.UTC())
	}
	return resp, nil
}

var (
	_ remindersv1.IngestServiceServer        = (*IngestServer)(nil)
	_ remindersv1.ReminderQueryServiceServer = (*QueryServer)(nil)
)
