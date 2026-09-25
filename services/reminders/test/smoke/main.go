// Команда smoke — сквозная проверка reminders-service в автономной среде:
// отправляет события core через нормативный контракт, дожидается передачи
// напоминания в двойник bot-service и печатает результат.
// Предназначена для dev/test окружения.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
)

const (
	orgID  = "0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01"
	docID  = "3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"
	perID  = "6d1f0a73-2b8c-4e5f-9a10-3c7b5e2d8f04"
	accID  = "9f3c7b21-4d6e-4a8b-b5c0-1e2f3a4b5c06"
	maxUID = 1001
)

func main() {
	addr := envOr("REMINDERS_GRPC_ADDR", "127.0.0.1:9091")
	doubleAdmin := envOr("BOTDOUBLE_ADMIN_URL", "http://127.0.0.1:8090")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("smoke: соединение с %s: %v", addr, err)
	}
	defer func() { _ = conn.Close() }()
	ingest := remindersv1.NewIngestServiceClient(conn)
	query := remindersv1.NewReminderQueryServiceClient(conn)

	now := time.Now().UTC()
	// Момент отправки подбирается на 10 минут в прошлом по времени организации:
	// напоминание уже наступило, но не просрочено (grace по умолчанию 24 ч),
	// поэтому планировщик обработает его на ближайшем тике.
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Fatalf("smoke: часовой пояс: %v", err)
	}
	target := now.In(loc).Add(-10 * time.Minute)
	notifyMinutes := uint32(target.Hour()*60 + target.Minute())
	validUntil := target.AddDate(0, 0, 30).Format("2006-01-02")

	events := []*remindersv1.Event{
		{
			EventId: "5a000000-0000-4000-8000-000000000001",
			Type:    remindersv1.EventType_EVENT_TYPE_ORGANIZATION_STATE, SchemaVersion: 1,
			SourceService: "core", AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_ORGANIZATION,
			AggregateId: orgID, AggregateVersion: 1, OccurredAt: timestamppb.New(now),
			Payload: &remindersv1.Event_OrganizationState{OrganizationState: &remindersv1.OrganizationState{
				OrganizationId: orgID, Name: "Кафе на Неве (тестовые данные)", Timezone: "Europe/Moscow"}},
		},
		{
			EventId: "5a000000-0000-4000-8000-000000000002",
			Type:    remindersv1.EventType_EVENT_TYPE_MEMBERSHIP_STATE, SchemaVersion: 1,
			SourceService: "core", AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_MEMBERSHIP,
			AggregateId: orgID + ":" + accID, AggregateVersion: 1, OccurredAt: timestamppb.New(now),
			Payload: &remindersv1.Event_MembershipState{MembershipState: &remindersv1.MembershipState{
				OrganizationId: orgID, AccountId: accID,
				AccountKind: remindersv1.AccountKind_ACCOUNT_KIND_MAX, MaxUserId: maxUID,
				NotifyEnabled: true, NotifyLocalMinutes: notifyMinutes}},
		},
		{
			EventId: "5a000000-0000-4000-8000-000000000003",
			Type:    remindersv1.EventType_EVENT_TYPE_DOCUMENT_STATE, SchemaVersion: 1,
			SourceService: "core", AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_DOCUMENT,
			AggregateId: docID, AggregateVersion: 1, OccurredAt: timestamppb.New(now),
			Payload: &remindersv1.Event_DocumentState{DocumentState: &remindersv1.DocumentState{
				DocumentId: docID, OrganizationId: orgID, Title: "Фискальный накопитель (тестовые данные)",
				CurrentPeriod:       &remindersv1.DocumentPeriod{PeriodId: perID, ValidUntil: validUntil},
				ReminderOffsetsDays: []uint32{30}}},
		},
	}

	resp, err := ingest.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{Events: events, BatchId: "smoke"})
	if err != nil {
		log.Fatalf("smoke: ApplyEvents: %v", err)
	}
	for _, r := range resp.GetResults() {
		fmt.Printf("событие %s: %s (версия %d) %s\n", r.GetEventId(), r.GetOutcome(), r.GetAppliedVersion(), r.GetMessage())
	}

	plan, err := query.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{DocumentId: docID})
	if err != nil {
		log.Fatalf("smoke: GetDocumentPlan: %v", err)
	}
	fmt.Printf("план документа: элементов %d, применённая версия %d\n", len(plan.GetItems()), plan.GetAppliedVersion())
	for _, it := range plan.GetItems() {
		fmt.Printf("  за %d дн., статус %s, отправка %s\n", it.GetDaysBefore(), it.GetStatus(), it.GetDueAt().AsTime().Format(time.RFC3339))
	}

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		plan, err = query.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{DocumentId: docID})
		if err == nil && len(plan.GetItems()) > 0 &&
			plan.GetItems()[0].GetStatus() == remindersv1.ReminderStatus_REMINDER_STATUS_HANDED_OFF {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if len(plan.GetItems()) == 0 || plan.GetItems()[0].GetStatus() != remindersv1.ReminderStatus_REMINDER_STATUS_HANDED_OFF {
		log.Fatalf("smoke: напоминание не передано в bot-service за отведённое время")
	}
	fmt.Println("напоминание передано в bot-service")

	res, err := http.Get(doubleAdmin + "/messages")
	if err != nil {
		log.Fatalf("smoke: запрос к двойнику bot: %v", err)
	}
	defer res.Body.Close()
	var msgs []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&msgs); err != nil {
		log.Fatalf("smoke: разбор ответа двойника: %v", err)
	}
	if len(msgs) == 0 {
		log.Fatal("smoke: двойник bot-service не получил сообщений")
	}
	out, _ := json.MarshalIndent(msgs, "", "  ")
	fmt.Printf("сообщения двойника bot-service:\n%s\n", out)
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
