package integration_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	botv1 "vovremya/gen/go/vovremya/bot/v1"
	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
	"vovremya/services/reminders/internal/adapters/botgrpc"
	"vovremya/services/reminders/internal/adapters/grpcserver"
	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
)

// startIngestServer поднимает настоящий gRPC-сервер сервиса на bufconn.
func startIngestServer(t *testing.T, e *env) (remindersv1.IngestServiceClient, remindersv1.ReminderQueryServiceClient) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	log := slog.New(slog.DiscardHandler)
	remindersv1.RegisterIngestServiceServer(srv, grpcserver.NewIngestServer(e.ingest, log, 200))
	remindersv1.RegisterReminderQueryServiceServer(srv, grpcserver.NewQueryServer(e.query, log))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return remindersv1.NewIngestServiceClient(conn), remindersv1.NewReminderQueryServiceClient(conn)
}

func protoDocumentEvent(e *env, eventID string, version uint64) *remindersv1.Event {
	return &remindersv1.Event{
		EventId: eventID, Type: remindersv1.EventType_EVENT_TYPE_DOCUMENT_STATE,
		SchemaVersion: 1, SourceService: "core",
		AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_DOCUMENT, AggregateId: docID,
		AggregateVersion: version, OccurredAt: timestamppb.New(e.clock.Now()),
		Payload: &remindersv1.Event_DocumentState{DocumentState: &remindersv1.DocumentState{
			DocumentId: docID, OrganizationId: orgID, Title: "Фискальный накопитель",
			CurrentPeriod:       &remindersv1.DocumentPeriod{PeriodId: periodA, ValidUntil: "2026-12-31"},
			ReminderOffsetsDays: []uint32{30, 14, 3},
		}},
	}
}

func TestGRPCApplyEventsAndQuery(t *testing.T) {
	e := newEnv(t)
	ingest, query := startIngestServer(t, e)
	ctx := context.Background()

	orgEv := &remindersv1.Event{
		EventId: uuidSeq(9001), Type: remindersv1.EventType_EVENT_TYPE_ORGANIZATION_STATE,
		SchemaVersion: 1, SourceService: "core",
		AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_ORGANIZATION, AggregateId: orgID,
		AggregateVersion: 1, OccurredAt: timestamppb.New(e.clock.Now()),
		Payload: &remindersv1.Event_OrganizationState{OrganizationState: &remindersv1.OrganizationState{
			OrganizationId: orgID, Name: "Кафе на Неве", Timezone: "Europe/Moscow"}},
	}
	memberEv := &remindersv1.Event{
		EventId: uuidSeq(9002), Type: remindersv1.EventType_EVENT_TYPE_MEMBERSHIP_STATE,
		SchemaVersion: 1, SourceService: "core",
		AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_MEMBERSHIP,
		AggregateId:   orgID + ":" + accID, AggregateVersion: 1, OccurredAt: timestamppb.New(e.clock.Now()),
		Payload: &remindersv1.Event_MembershipState{MembershipState: &remindersv1.MembershipState{
			OrganizationId: orgID, AccountId: accID, AccountKind: remindersv1.AccountKind_ACCOUNT_KIND_MAX,
			MaxUserId: 1001, NotifyEnabled: true, NotifyLocalMinutes: 540}},
	}
	docEv := protoDocumentEvent(e, uuidSeq(9003), 1)

	resp, err := ingest.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{
		Events: []*remindersv1.Event{orgEv, memberEv, docEv}, BatchId: "batch-1"})
	if err != nil {
		t.Fatalf("ApplyEvents: %v", err)
	}
	if len(resp.GetResults()) != 3 {
		t.Fatalf("ожидалось 3 результата, получено %d", len(resp.GetResults()))
	}
	for _, r := range resp.GetResults() {
		if r.GetOutcome() != remindersv1.EventOutcome_EVENT_OUTCOME_APPLIED {
			t.Errorf("событие %s: исход %s (%s)", r.GetEventId(), r.GetOutcome(), r.GetMessage())
		}
	}

	// Повторная доставка того же пакета — дубликаты, план не меняется.
	repeat, err := ingest.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{
		Events: []*remindersv1.Event{orgEv, memberEv, docEv}, BatchId: "batch-1-retry"})
	if err != nil {
		t.Fatalf("повторный ApplyEvents: %v", err)
	}
	for _, r := range repeat.GetResults() {
		if r.GetOutcome() != remindersv1.EventOutcome_EVENT_OUTCOME_DUPLICATE {
			t.Errorf("повтор события %s: исход %s", r.GetEventId(), r.GetOutcome())
		}
	}

	next, err := query.GetNextReminders(ctx, &remindersv1.GetNextRemindersRequest{
		AccountId: accID, DocumentIds: []string{docID}})
	if err != nil {
		t.Fatalf("GetNextReminders: %v", err)
	}
	if len(next.GetItems()) != 1 || next.GetItems()[0].GetDaysBefore() != 30 {
		t.Fatalf("ближайшее напоминание: %+v", next.GetItems())
	}
	wantDue := time.Date(2026, 12, 1, 6, 0, 0, 0, time.UTC)
	if got := next.GetItems()[0].GetDueAt().AsTime(); !got.Equal(wantDue) {
		t.Errorf("due_at: ожидалось %s, получено %s", wantDue, got)
	}

	plan, err := query.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{DocumentId: docID})
	if err != nil {
		t.Fatalf("GetDocumentPlan: %v", err)
	}
	if plan.GetAppliedVersion() != 1 || len(plan.GetItems()) != 3 {
		t.Errorf("план: версия=%d, элементов=%d", plan.GetAppliedVersion(), len(plan.GetItems()))
	}

	sync, err := query.GetSyncStatus(ctx, &remindersv1.GetSyncStatusRequest{
		AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_DOCUMENT, AggregateId: docID, ExpectedVersion: 1})
	if err != nil {
		t.Fatalf("GetSyncStatus: %v", err)
	}
	if !sync.GetInSync() || sync.GetAppliedVersion() != 1 {
		t.Errorf("состояние синхронизации: in_sync=%t version=%d", sync.GetInSync(), sync.GetAppliedVersion())
	}

	state, err := ingest.GetIngestState(ctx, &remindersv1.GetIngestStateRequest{
		Aggregates: []*remindersv1.AggregateRef{
			{AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_DOCUMENT, AggregateId: docID}}})
	if err != nil {
		t.Fatalf("GetIngestState: %v", err)
	}
	if state.GetAggregates()[0].GetAppliedVersion() != 1 {
		t.Errorf("версия агрегата: %d", state.GetAggregates()[0].GetAppliedVersion())
	}
}

func TestGRPCValidationErrors(t *testing.T) {
	e := newEnv(t)
	ingest, query := startIngestServer(t, e)
	ctx := context.Background()

	if _, err := ingest.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("пустой пакет: ожидался INVALID_ARGUMENT, получено %v", err)
	}
	bad := protoDocumentEvent(e, "not-a-uuid", 1)
	resp, err := ingest.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{Events: []*remindersv1.Event{bad}})
	if err != nil {
		t.Fatalf("ApplyEvents: %v", err)
	}
	if resp.GetResults()[0].GetOutcome() != remindersv1.EventOutcome_EVENT_OUTCOME_REJECTED {
		t.Errorf("некорректный event_id: ожидался REJECTED, получено %s", resp.GetResults()[0].GetOutcome())
	}
	noPayload := &remindersv1.Event{
		EventId: uuidSeq(9100), Type: remindersv1.EventType_EVENT_TYPE_DOCUMENT_STATE, SchemaVersion: 1,
		SourceService: "core", AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_DOCUMENT,
		AggregateId: docID, AggregateVersion: 1, OccurredAt: timestamppb.New(e.clock.Now()),
	}
	resp, err = ingest.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{Events: []*remindersv1.Event{noPayload}})
	if err != nil {
		t.Fatalf("ApplyEvents: %v", err)
	}
	if resp.GetResults()[0].GetOutcome() != remindersv1.EventOutcome_EVENT_OUTCOME_REJECTED {
		t.Errorf("событие без payload должно отвергаться, получено %s", resp.GetResults()[0].GetOutcome())
	}
	if _, err := query.GetNextReminders(ctx, &remindersv1.GetNextRemindersRequest{
		AccountId: "oops", DocumentIds: []string{docID}}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("некорректный account_id: ожидался INVALID_ARGUMENT, получено %v", err)
	}
	if _, err := query.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{
		DocumentId: docID2}); status.Code(err) != codes.NotFound {
		t.Errorf("неизвестный документ: ожидался NOT_FOUND, получено %v", err)
	}
}

func TestGRPCCancellationIsPropagated(t *testing.T) {
	e := newEnv(t)
	ingest, _ := startIngestServer(t, e)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ingest.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{
		Events: []*remindersv1.Event{protoDocumentEvent(e, uuidSeq(9200), 1)}})
	if status.Code(err) != codes.Canceled {
		t.Errorf("ожидался CANCELED, получено %v", err)
	}
}

// contractBot — тестовый bot-service, проверяющий нормативный контракт
// vovremya.bot.v1.MessagingService; первые failFirst вызовов отклоняются.
type contractBot struct {
	botv1.UnimplementedMessagingServiceServer
	mu        sync.Mutex
	failFirst int
	received  []*botv1.EnqueueNotificationRequest
}

func (b *contractBot) EnqueueNotification(_ context.Context, req *botv1.EnqueueNotificationRequest) (*botv1.EnqueueNotificationResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failFirst > 0 {
		b.failFirst--
		return nil, status.Error(codes.Unavailable, "bot недоступен")
	}
	if req.GetIdempotencyKey() == "" || req.GetRecipientMaxUserId() <= 0 || req.GetText() == "" {
		return nil, status.Error(codes.InvalidArgument, "нарушение контракта")
	}
	if req.GetKind() != botv1.NotificationKind_NOTIFICATION_KIND_REMINDER {
		return nil, status.Error(codes.InvalidArgument, "ожидается kind=REMINDER")
	}
	b.received = append(b.received, req)
	return &botv1.EnqueueNotificationResponse{
		NotificationId: "00000000-0000-4000-8000-000000000001"}, nil
}

func TestBotGatewayContractOverRealGRPC(t *testing.T) {
	bot := &contractBot{failFirst: 1}
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	botv1.RegisterMessagingServiceServer(srv, bot)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc client: %v", err)
	}
	defer func() { _ = conn.Close() }()

	client := botgrpc.NewClient(conn, 5*time.Second)
	until, _ := domain.ParseDate("2026-12-31")
	req := ports.NotificationRequest{
		IdempotencyKey:     "rem:" + periodA + ":" + accID + ":30",
		RecipientMaxUserID: 1001,
		Text:               domain.ReminderText("Фискальный накопитель", "Кафе на Неве", 30, until),
		ButtonText:         domain.ButtonOpenDocument,
		ButtonPayload:      domain.DeepLinkPayload(docID),
		NotAfter:           time.Date(2026, 12, 2, 6, 0, 0, 0, time.UTC),
	}

	// Первый вызов: сервис недоступен — ошибка должна быть помечена повторяемой.
	if _, err := client.Enqueue(context.Background(), req); err == nil {
		t.Fatal("ожидалась ошибка недоступности")
	} else {
		var gwErr *ports.GatewayError
		if !errors.As(err, &gwErr) || !gwErr.Retryable {
			t.Fatalf("ошибка должна быть повторяемой: %v", err)
		}
	}
	// Второй вызов проходит и передаёт корректное задание.
	res, err := client.Enqueue(context.Background(), req)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if res.NotificationID == "" {
		t.Error("ожидался идентификатор сообщения")
	}
	bot.mu.Lock()
	defer bot.mu.Unlock()
	if len(bot.received) != 1 {
		t.Fatalf("bot должен получить 1 задание, получено %d", len(bot.received))
	}
	got := bot.received[0]
	if got.GetKind() != botv1.NotificationKind_NOTIFICATION_KIND_REMINDER {
		t.Errorf("kind: %s", got.GetKind())
	}
	if len(got.GetButtons()) != 1 || got.GetButtons()[0].GetOpenAppPayload() != "doc_"+docID {
		t.Errorf("кнопка задания некорректна: %+v", got.GetButtons())
	}
	if got.GetNotAfter() == nil {
		t.Error("не передан предельный срок отправки")
	}
}
