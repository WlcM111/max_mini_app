package integration_test

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
	"vovremya/services/reminders/internal/adapters/grpcserver"
	"vovremya/services/reminders/internal/app"
	"vovremya/services/reminders/internal/domain"
)

// accMaxUser — идентификатор MAX участника accID в фикстурах (1000 + длина UUID).
const accMaxUser = int64(1036)

func (e *env) snoozer() *app.SnoozeService {
	return app.NewSnoozeService(e.pool, e.proj, e.reminders, e.clock, slog.New(slog.DiscardHandler))
}

func snoozeRows(items []domain.Reminder) []domain.Reminder {
	var out []domain.Reminder
	for _, r := range items {
		if r.Key.SnoozeDay > 0 {
			out = append(out, r)
		}
	}
	return out
}

func offsetKey(days int) string {
	return domain.PlanKey{PeriodID: periodA, AccountID: accID, DaysBefore: days}.IdempotencyKey()
}

func (e *env) mustSnooze(t *testing.T, key string) app.SnoozeResult {
	t.Helper()
	res, err := e.snoozer().Snooze(context.Background(), key, accMaxUser)
	if err != nil {
		t.Fatalf("отложить: %v", err)
	}
	return res
}

// TestSnoozePlansReminderAWeekLater: повтор становится строкой плана через неделю во время
// напоминаний участника; повторное нажатие в тот же день второй строки не создаёт.
func TestSnoozePlansReminderAWeekLater(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)

	res := e.mustSnooze(t, offsetKey(30))
	// 20.09 + 7 дней = 27.09, 09:00 Europe/Moscow = 06:00 UTC.
	want := time.Date(2026, 9, 27, 6, 0, 0, 0, time.UTC)
	if res.Outcome != domain.SnoozeSnoozed || !res.DueAt.Equal(want) {
		t.Fatalf("исход %v, момент %s; ожидалось snoozed на %s", res.Outcome, res.DueAt, want)
	}
	rows := snoozeRows(e.planFor(t, docID))
	if len(rows) != 1 || rows[0].Status != domain.StatusPlanned || !rows[0].DueAt.Equal(want) {
		t.Fatalf("строка повтора: %+v", rows)
	}
	// С 27.09 до 31.12 — 95 дней: текст повтора сообщит точный остаток.
	day := domain.SnoozeDay(e.clock.Now())
	wantKey := "rem:" + periodA + ":" + accID + ":95:snz:" + strconv.Itoa(day)
	if rows[0].Key.DaysBefore != 95 || rows[0].Key.IdempotencyKey() != wantKey {
		t.Fatalf("ключ повтора %q, ожидался %q", rows[0].Key.IdempotencyKey(), wantKey)
	}

	if again := e.mustSnooze(t, offsetKey(14)); again.Outcome != domain.SnoozeAlreadySnoozed {
		t.Fatalf("повторное нажатие в тот же день: %v", again.Outcome)
	}
	if n := len(snoozeRows(e.planFor(t, docID))); n != 1 {
		t.Fatalf("повторное нажатие создало %d строк", n)
	}
}

// TestSnoozeRejectsForeignAndUnknown: отложить можно только своё существующее напоминание.
func TestSnoozeRejectsForeignAndUnknown(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	s := e.snoozer()
	ctx := context.Background()
	cases := []struct {
		name string
		key  string
		user int64
		want domain.SnoozeOutcome
	}{
		{"чужой пользователь", offsetKey(30), 555, domain.SnoozeForbidden},
		{"без отправителя", offsetKey(30), 0, domain.SnoozeForbidden},
		{"неверный ключ", "rem:не-ключ", accMaxUser, domain.SnoozeNotFound},
		{"несуществующее напоминание", offsetKey(60), accMaxUser, domain.SnoozeNotFound},
		{"ключ сводки", "digest:" + orgID + ":" + accID + ":202640", accMaxUser, domain.SnoozeNotFound},
	}
	for _, tc := range cases {
		res, err := s.Snooze(ctx, tc.key, tc.user)
		if err != nil || res.Outcome != tc.want {
			t.Errorf("%s: исход %v, ошибка %v; ожидалось %v", tc.name, res.Outcome, err, tc.want)
		}
	}
	if n := len(snoozeRows(e.planFor(t, docID))); n != 0 {
		t.Fatalf("отклонённые нажатия создали %d строк", n)
	}
}

// TestSnoozeTooLate: повтор не планируется, если пришёл бы в день окончания срока или позже.
func TestSnoozeTooLate(t *testing.T) {
	e := newEnv(t)
	e.apply(t, e.organizationEvent(1, "Europe/Moscow"))
	e.apply(t, e.memberEvent(1, accID, true, 9*60))
	e.apply(t, e.documentEvent(t, 1, periodA, "2026-09-26", []int{3}))
	if res := e.mustSnooze(t, offsetKey(3)); res.Outcome != domain.SnoozeTooLate {
		t.Fatalf("исход %v, ожидался too_late", res.Outcome)
	}
}

// TestSnoozeSurvivesEditButNotRenewal: правка документа повтор сохраняет, продление — отменяет,
// а нажатие под напоминанием прежнего периода сообщает об устаревании (BUG-002).
func TestSnoozeSurvivesEditButNotRenewal(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	e.mustSnooze(t, offsetKey(30))

	e.apply(t, e.documentEvent(t, 2, periodA, "2026-12-31", []int{30, 14, 3}))
	if rows := snoozeRows(e.planFor(t, docID)); len(rows) != 1 || rows[0].Status != domain.StatusPlanned {
		t.Fatalf("после правки повтор должен остаться: %+v", rows)
	}

	e.apply(t, e.documentEvent(t, 3, periodB, "2027-12-31", []int{30}))
	if rows := snoozeRows(e.planFor(t, docID)); len(rows) != 1 || rows[0].Status != domain.StatusCancelled {
		t.Fatalf("после продления повтор должен быть отменён: %+v", rows)
	}
	e.clock.Advance(24 * time.Hour)
	if res := e.mustSnooze(t, offsetKey(14)); res.Outcome != domain.SnoozeStale {
		t.Fatalf("нажатие под напоминанием прежнего периода: %v", res.Outcome)
	}
}

// TestSnoozeCancelledWithRecipientOrDocument: повтор отменяется всеми путями отмены плана (BUG-002).
func TestSnoozeCancelledWithRecipientOrDocument(t *testing.T) {
	cases := []struct {
		name  string
		event func(e *env) app.Event
	}{
		{"отключение уведомлений", func(e *env) app.Event { return e.memberEvent(2, accID, false, 9*60) }},
		{"исключение участника", func(e *env) app.Event {
			return app.Event{ID: e.eventID(), Type: app.EventMembershipRemoved, SchemaVersion: app.SchemaVersion,
				SourceService: "core", AggregateType: app.AggregateMembership,
				AggregateID: app.MembershipAggregateID(orgID, accID), AggregateVersion: 2, OccurredAt: e.clock.Now()}
		}},
		{"удаление аккаунта", func(e *env) app.Event {
			return app.Event{ID: e.eventID(), Type: app.EventAccountDeleted, SchemaVersion: app.SchemaVersion,
				SourceService: "core", AggregateType: app.AggregateAccount, AggregateID: accID,
				AggregateVersion: 1, OccurredAt: e.clock.Now()}
		}},
		{"удаление документа", func(e *env) app.Event {
			return app.Event{ID: e.eventID(), Type: app.EventDocumentDeleted, SchemaVersion: app.SchemaVersion,
				SourceService: "core", AggregateType: app.AggregateDocument, AggregateID: docID,
				AggregateVersion: 2, OccurredAt: e.clock.Now()}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.seedPlan(t)
			e.mustSnooze(t, offsetKey(30))
			e.apply(t, tc.event(e))
			for _, r := range snoozeRows(e.planFor(t, docID)) {
				if r.Status == domain.StatusPlanned {
					t.Fatalf("повтор остался запланированным: %+v", r)
				}
			}
		})
	}
}

// TestSnoozeIsSentBySchedulerWithPrefix: в срок планировщик отправляет повтор с пометкой
// и ключом :snz:, по которому bot-service снова добавит кнопку.
func TestSnoozeIsSentBySchedulerWithPrefix(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	res := e.mustSnooze(t, offsetKey(30))
	e.clock.Set(res.DueAt.Add(time.Minute))
	if _, err := e.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("тик планировщика: %v", err)
	}
	var sent bool
	for _, req := range e.bot.Accepted() {
		if strings.Contains(req.IdempotencyKey, ":snz:") {
			sent = true
			if !strings.HasPrefix(req.Text, domain.SnoozeTextPrefix) || req.RecipientMaxUserID != accMaxUser {
				t.Fatalf("сообщение повтора: %+v", req)
			}
		}
	}
	if !sent {
		t.Fatalf("повтор не передан в bot-service: %+v", e.bot.Accepted())
	}
}

// TestSnoozeOverGRPC: контракт ReminderCommandService — исход, момент и проверка ключа.
func TestSnoozeOverGRPC(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	remindersv1.RegisterReminderCommandServiceServer(srv, grpcserver.NewCommandServer(e.snoozer(), slog.New(slog.DiscardHandler)))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("клиент gRPC: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := remindersv1.NewReminderCommandServiceClient(conn)

	resp, err := client.SnoozeReminder(context.Background(), &remindersv1.SnoozeReminderRequest{
		IdempotencyKey: offsetKey(30), RecipientMaxUserId: accMaxUser})
	if err != nil || resp.GetOutcome() != remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_SNOOZED || resp.GetDueAt() == nil {
		t.Fatalf("ответ %+v, ошибка %v", resp, err)
	}
	_, err = client.SnoozeReminder(context.Background(), &remindersv1.SnoozeReminderRequest{RecipientMaxUserId: accMaxUser})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("пустой ключ: ожидался InvalidArgument, получено %v", err)
	}
}
