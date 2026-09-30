// Команда e2e — сквозная проверка связки reminders-service и bot-service.
// Работает с настоящими процессами обоих сервисов: события core подаются
// по нормативному контракту IngestService, результат проверяется в bot-service
// через MessagingService и буфер режима stub. Предназначена для dev/test.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	botv1 "vovremya/gen/go/vovremya/bot/v1"
	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
)

const (
	orgID     = "0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01"
	accID     = "9f3c7b21-4d6e-4a8b-b5c0-1e2f3a4b5c06"
	docFirst  = "3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"
	perFirst  = "6d1f0a73-2b8c-4e5f-9a10-3c7b5e2d8f04"
	docSecond = "8b2d4f60-7c31-4a9e-b8d5-2f6a1c3e4d07"
	perSecond = "1e7a9c48-3b5d-4f2a-9c60-8d4b2e5f7a09"
	maxUID    = 1001
)

type env struct {
	reminders remindersv1.IngestServiceClient
	query     remindersv1.ReminderQueryServiceClient
	bot       botv1.MessagingServiceClient
	botAdmin  string
	botHTTP   string
	secret    string
}

func main() {
	caseName := flag.String("case", "main",
		"сценарий: main, bot-down, bot-recovered, core, core-pending, core-recovered, core-bot-down")
	flag.Parse()

	remindersAddr := envOr("E2E_REMINDERS_GRPC", "127.0.0.1:19091")
	botAddr := envOr("E2E_BOT_GRPC", "127.0.0.1:19090")
	e := &env{
		botAdmin: envOr("E2E_BOT_ADMIN", "http://127.0.0.1:18081"),
		botHTTP:  envOr("E2E_BOT_HTTP", "http://127.0.0.1:18080"),
		secret:   envOr("BOT_WEBHOOK_SECRET", "devonly-webhook-secret"),
	}
	remindersConn, err := grpc.NewClient(remindersAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("e2e: соединение с reminders (%s): %v", remindersAddr, err)
	}
	defer func() { _ = remindersConn.Close() }()
	botConn, err := grpc.NewClient(botAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("e2e: соединение с bot (%s): %v", botAddr, err)
	}
	defer func() { _ = botConn.Close() }()
	e.reminders = remindersv1.NewIngestServiceClient(remindersConn)
	e.query = remindersv1.NewReminderQueryServiceClient(remindersConn)
	e.bot = botv1.NewMessagingServiceClient(botConn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	switch *caseName {
	case "main":
		e.caseMain(ctx)
	case "bot-down":
		e.caseBotDown(ctx)
	case "bot-recovered":
		e.caseBotRecovered(ctx)
	case "core":
		e.caseCore(ctx)
	case "core-pending":
		e.caseCorePending(ctx)
	case "core-recovered":
		e.caseCoreRecovered(ctx)
	case "core-bot-down":
		e.caseCoreBotDown(ctx)
	default:
		log.Fatalf("e2e: неизвестный сценарий %q", *caseName)
	}
}

// caseMain — сквозной путь: события core → план → передача в bot → доставка.
func (e *env) caseMain(ctx context.Context) {
	step("1. Профиль бота доступен через gRPC")
	profile, err := e.bot.GetBotProfile(ctx, &botv1.GetBotProfileRequest{})
	if err != nil {
		log.Fatalf("e2e: GetBotProfile: %v", err)
	}
	if profile.GetMode() != botv1.BotMode_BOT_MODE_STUB || profile.GetUsername() == "" {
		log.Fatalf("e2e: профиль бота: %+v", profile)
	}
	fmt.Printf("   режим %s, бот @%s, шаблон диплинка %s\n",
		profile.GetMode(), profile.GetUsername(), profile.GetOpenAppLinkTemplate())

	step("2. core передаёт события в reminders (ApplyEvents)")
	events := baseEvents(docFirst, perFirst, "Фискальный накопитель (тестовые данные)", 1)
	resp, err := e.reminders.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{Events: events, BatchId: "e2e-main"})
	if err != nil {
		log.Fatalf("e2e: ApplyEvents: %v", err)
	}
	for _, r := range resp.GetResults() {
		fmt.Printf("   событие %s: %s\n", r.GetEventId(), r.GetOutcome())
		if r.GetOutcome() == remindersv1.EventOutcome_EVENT_OUTCOME_REJECTED {
			log.Fatalf("e2e: событие отклонено: %s", r.GetMessage())
		}
	}

	step("3. reminders построил план и передал напоминание в bot")
	item := waitHandedOff(ctx, e.query, docFirst, 60*time.Second)
	fmt.Printf("   напоминание за %d дн. передано в %s\n", item.GetDaysBefore(),
		item.GetHandedOffAt().AsTime().Format(time.RFC3339))

	step("4. bot принял задание по ключу идемпотентности из контракта")
	key := fmt.Sprintf("rem:%s:%s:%d", perFirst, accID, item.GetDaysBefore())
	st, err := e.bot.GetNotificationStatus(ctx, &botv1.GetNotificationStatusRequest{IdempotencyKey: key})
	if err != nil {
		log.Fatalf("e2e: GetNotificationStatus(%s): %v", key, err)
	}
	fmt.Printf("   ключ %s: состояние %s, попыток %d\n", key, st.GetStatus(), st.GetAttempts())

	step("5. bot доставил сообщение через канал MAX (режим stub)")
	sent := waitStatus(ctx, e.bot, key, botv1.NotificationStatus_NOTIFICATION_STATUS_SENT, 30*time.Second)
	fmt.Printf("   отправлено в %s\n", sent.GetSentAt().AsTime().Format(time.RFC3339))
	msgs := e.stubMessages()
	msg := findMessage(msgs, maxUID, "Фискальный накопитель")
	if msg == nil {
		log.Fatalf("e2e: сообщение не найдено в буфере режима stub: %s", dump(msgs))
	}
	fmt.Printf("   текст: %s\n", msg.Text)
	if len(msg.Buttons) != 3 {
		log.Fatalf("e2e: ожидались три кнопки (открыть карточку, продлил, отложить), получено %d: %+v",
			len(msg.Buttons), msg.Buttons)
	}
	wantURL := strings.Replace(profile.GetOpenAppLinkTemplate(), "{payload}", "doc_"+docFirst, 1)
	wantRenewURL := strings.Replace(profile.GetOpenAppLinkTemplate(), "{payload}", "renew_"+docFirst, 1)
	if msg.Buttons[0].URL != wantURL {
		log.Fatalf("e2e: диплинк кнопки карточки %q, ожидался %q", msg.Buttons[0].URL, wantURL)
	}
	if msg.Buttons[1].URL != wantRenewURL {
		log.Fatalf("e2e: диплинк кнопки продления %q, ожидался %q", msg.Buttons[1].URL, wantRenewURL)
	}
	if msg.Buttons[2].Payload != "snooze:"+key {
		log.Fatalf("e2e: кнопка отложенного напоминания %+v, ожидался payload snooze:%s", msg.Buttons[2], key)
	}
	fmt.Printf("   кнопки: %s → %s | %s → %s | %s (отложить на неделю)\n",
		msg.Buttons[0].Text, msg.Buttons[0].URL, msg.Buttons[1].Text, msg.Buttons[1].URL, msg.Buttons[2].Text)

	step("6. Повторная постановка задания не создаёт второго сообщения")
	// Ключ идемпотентности того же вида, что у reminders-service, но с отдельным
	// периодом: проверяется поведение контракта при точном повторе запроса.
	replayKey := fmt.Sprintf("rem:%s:%s:14", perFirst, accID)
	replay := &botv1.EnqueueNotificationRequest{
		IdempotencyKey:     replayKey,
		Kind:               botv1.NotificationKind_NOTIFICATION_KIND_REMINDER,
		RecipientMaxUserId: maxUID,
		Text:               "Через 14 дней заканчивается срок: «Фискальный накопитель (тестовые данные)».",
		Buttons: []*botv1.Button{{Text: "Открыть документ",
			Action: &botv1.Button_OpenAppPayload{OpenAppPayload: "doc_" + docFirst}}},
		NotAfter: timestamppb.New(time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)),
	}
	first, err := e.bot.EnqueueNotification(ctx, replay)
	if err != nil {
		log.Fatalf("e2e: постановка задания: %v", err)
	}
	if first.GetDuplicate() {
		log.Fatal("e2e: первая постановка не должна быть дубликатом")
	}
	waitStatus(ctx, e.bot, replayKey, botv1.NotificationStatus_NOTIFICATION_STATUS_SENT, 30*time.Second)
	afterFirst := e.stubMessages()

	again, err := e.bot.EnqueueNotification(ctx, replay)
	if err != nil {
		log.Fatalf("e2e: точный повтор запроса: %v", err)
	}
	if !again.GetDuplicate() || again.GetNotificationId() != first.GetNotificationId() {
		log.Fatalf("e2e: повтор не распознан как дубликат: %+v", again)
	}
	time.Sleep(2 * time.Second)
	if after := len(e.stubMessages()); after != len(afterFirst) {
		log.Fatalf("e2e: после повтора сообщений стало %d, было %d", after, len(afterFirst))
	}
	fmt.Printf("   duplicate=true, сообщений в канале по-прежнему %d\n", len(afterFirst))

	// Тот же ключ с другим содержимым — конфликт (grpc-contract §1).
	conflict := proto.Clone(replay).(*botv1.EnqueueNotificationRequest)
	conflict.Text = "Изменённый текст того же напоминания"
	_, err = e.bot.EnqueueNotification(ctx, conflict)
	if status.Code(err) != codes.AlreadyExists {
		log.Fatalf("e2e: ожидался ALREADY_EXISTS при смене содержимого, получено %v", err)
	}
	fmt.Println("   смена содержимого при том же ключе отклонена кодом ALREADY_EXISTS")

	step("7. Событие webhook от MAX меняет состояние получателя и вызывает приветствие")
	body := fmt.Sprintf(`{"update_type":"bot_started","timestamp":%d,"user":{"user_id":%d,"is_bot":false}}`,
		time.Now().UnixMilli(), maxUID)
	if code := e.postWebhook(body, e.secret); code != http.StatusOK {
		log.Fatalf("e2e: webhook вернул %d", code)
	}
	if code := e.postWebhook(body, "wrong-secret"); code != http.StatusUnauthorized {
		log.Fatalf("e2e: запрос с чужим секретом должен отклоняться, получено %d", code)
	}
	recipient, err := e.bot.GetRecipientStatus(ctx, &botv1.GetRecipientStatusRequest{MaxUserId: maxUID})
	if err != nil {
		log.Fatalf("e2e: GetRecipientStatus: %v", err)
	}
	if recipient.GetState() != botv1.RecipientState_RECIPIENT_STATE_ACTIVE {
		log.Fatalf("e2e: состояние получателя %s, ожидалось ACTIVE", recipient.GetState())
	}
	welcome := waitMessage(e, maxUID, "Вовремя", 20*time.Second)
	fmt.Printf("   получатель: %s; приветствие доставлено: %s\n", recipient.GetState(), firstLine(welcome.Text))

	step("8. Повтор того же события webhook не создаёт второго приветствия")
	before := len(e.stubMessages())
	if code := e.postWebhook(body, e.secret); code != http.StatusOK {
		log.Fatalf("e2e: повтор webhook вернул %d", code)
	}
	time.Sleep(2 * time.Second)
	if after := len(e.stubMessages()); after != before {
		log.Fatalf("e2e: повтор события создал новые сообщения: было %d, стало %d", before, after)
	}
	fmt.Printf("   сообщений в канале: %d (без изменений)\n", before)

	step("9. «Напомнить через неделю»: нажатие в чате ставит повтор в план reminders-service")
	// Путь целиком: webhook bot-service → gRPC ReminderCommandService → строка плана (ADR-036).
	callback := fmt.Sprintf(`{"update_type":"message_callback","timestamp":%d,`+
		`"callback":{"callback_id":"e2e-snooze","payload":"snooze:%s","user":{"user_id":%d}}}`,
		time.Now().UnixMilli(), key, maxUID)
	if code := e.postWebhook(callback, e.secret); code != http.StatusOK {
		log.Fatalf("e2e: webhook с нажатием вернул %d", code)
	}
	weekAhead := time.Now().Add(6 * 24 * time.Hour)
	var snoozed *remindersv1.PlannedReminder
	for limit := time.Now().Add(20 * time.Second); snoozed == nil; time.Sleep(300 * time.Millisecond) {
		plan, err := e.query.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{DocumentId: docFirst})
		if err != nil {
			log.Fatalf("e2e: GetDocumentPlan: %v", err)
		}
		for _, it := range plan.GetItems() {
			if it.GetStatus() == remindersv1.ReminderStatus_REMINDER_STATUS_PLANNED && it.GetDueAt().AsTime().After(weekAhead) &&
				it.GetDueAt().AsTime().Before(time.Now().Add(8*24*time.Hour)) {
				snoozed = it
			}
		}
		if snoozed == nil && time.Now().After(limit) {
			log.Fatalf("e2e: повтор не появился в плане документа: %s", dump(plan.GetItems()))
		}
	}
	fmt.Printf("   повтор запланирован на %s, до срока останется %d дн.\n",
		snoozed.GetDueAt().AsTime().Format(time.RFC3339), snoozed.GetDaysBefore())
	fmt.Println("\nСценарий main пройден.")
}

// caseBotDown — bot-service недоступен: напоминание остаётся в плане,
// reminders продолжает работать и повторяет передачу.
func (e *env) caseBotDown(ctx context.Context) {
	step("1. bot-service остановлен: проверяем недоступность gRPC")
	_, err := e.bot.GetBotProfile(ctx, &botv1.GetBotProfileRequest{})
	if err == nil {
		log.Fatal("e2e: bot-service отвечает, хотя должен быть остановлен")
	}
	fmt.Printf("   вызов bot завершился ожидаемой ошибкой: %s\n", status.Code(err))

	step("2. Новый документ попадает в план при недоступном bot")
	events := baseEvents(docSecond, perSecond, "Разрешение на вывеску (тестовые данные)", 2)
	resp, err := e.reminders.ApplyEvents(ctx, &remindersv1.ApplyEventsRequest{Events: events, BatchId: "e2e-bot-down"})
	if err != nil {
		log.Fatalf("e2e: ApplyEvents при недоступном bot: %v", err)
	}
	for _, r := range resp.GetResults() {
		if r.GetOutcome() == remindersv1.EventOutcome_EVENT_OUTCOME_REJECTED {
			log.Fatalf("e2e: событие отклонено: %s", r.GetMessage())
		}
	}
	fmt.Println("   события приняты, план построен")

	step("3. Напоминание не теряется и ждёт повтора")
	deadline := time.Now().Add(25 * time.Second)
	var last *remindersv1.PlannedReminder
	for time.Now().Before(deadline) {
		plan, err := e.query.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{DocumentId: docSecond})
		if err != nil {
			log.Fatalf("e2e: GetDocumentPlan: %v", err)
		}
		if len(plan.GetItems()) == 0 {
			log.Fatal("e2e: план документа пуст")
		}
		last = plan.GetItems()[0]
		if last.GetStatus() == remindersv1.ReminderStatus_REMINDER_STATUS_HANDED_OFF {
			log.Fatal("e2e: напоминание передано, хотя bot-service остановлен")
		}
		time.Sleep(3 * time.Second)
	}
	fmt.Printf("   статус напоминания: %s, срок отправки %s — передача повторяется\n",
		last.GetStatus(), last.GetDueAt().AsTime().Format(time.RFC3339))
	fmt.Println("\nСценарий bot-down пройден.")
}

// caseBotRecovered — после возврата bot-service напоминание доставляется.
func (e *env) caseBotRecovered(ctx context.Context) {
	step("1. bot-service снова доступен")
	profile, err := e.bot.GetBotProfile(ctx, &botv1.GetBotProfileRequest{})
	if err != nil {
		log.Fatalf("e2e: GetBotProfile после перезапуска: %v", err)
	}

	step("2. Отложенное напоминание передано без вмешательства")
	item := waitHandedOff(ctx, e.query, docSecond, 90*time.Second)
	key := fmt.Sprintf("rem:%s:%s:%d", perSecond, accID, item.GetDaysBefore())
	fmt.Printf("   ключ %s, передано в %s\n", key, item.GetHandedOffAt().AsTime().Format(time.RFC3339))

	step("3. Сообщение доставлено получателю")
	sent := waitStatus(ctx, e.bot, key, botv1.NotificationStatus_NOTIFICATION_STATUS_SENT, 40*time.Second)
	fmt.Printf("   состояние: %s, отправлено в %s\n", sent.GetStatus(), sent.GetSentAt().AsTime().Format(time.RFC3339))
	msg := waitMessage(e, maxUID, "Разрешение на вывеску", 30*time.Second)
	wantURL := strings.Replace(profile.GetOpenAppLinkTemplate(), "{payload}", "doc_"+docSecond, 1)
	if len(msg.Buttons) != 3 || msg.Buttons[0].URL != wantURL {
		log.Fatalf("e2e: кнопки сообщения: %+v, первой ожидалась карточка по диплинку %s", msg.Buttons, wantURL)
	}
	fmt.Printf("   текст: %s\n", firstLine(msg.Text))
	fmt.Println("\nСценарий bot-recovered пройден.")
}

// baseEvents формирует пакет событий core для одного документа.
func baseEvents(documentID, periodID, title string, seq int) []*remindersv1.Event {
	now := time.Now().UTC()
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Fatalf("e2e: часовой пояс: %v", err)
	}
	// Время напоминания — 10 минут назад по часам организации: момент наступил,
	// но не просрочен (grace 24 ч), поэтому планировщик возьмёт его сразу.
	target := now.In(loc).Add(-10 * time.Minute)
	notifyMinutes := uint32(target.Hour()*60 + target.Minute())
	validUntil := target.AddDate(0, 0, 30).Format("2006-01-02")
	return []*remindersv1.Event{
		{
			EventId: fmt.Sprintf("5a000000-0000-4000-8000-0000000%05d", seq*10+1),
			Type:    remindersv1.EventType_EVENT_TYPE_ORGANIZATION_STATE, SchemaVersion: 1,
			SourceService: "core", AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_ORGANIZATION,
			AggregateId: orgID, AggregateVersion: uint64(seq), OccurredAt: timestamppb.New(now),
			Payload: &remindersv1.Event_OrganizationState{OrganizationState: &remindersv1.OrganizationState{
				OrganizationId: orgID, Name: "Кафе на Неве (тестовые данные)", Timezone: "Europe/Moscow"}},
		},
		{
			EventId: fmt.Sprintf("5a000000-0000-4000-8000-0000000%05d", seq*10+2),
			Type:    remindersv1.EventType_EVENT_TYPE_MEMBERSHIP_STATE, SchemaVersion: 1,
			SourceService: "core", AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_MEMBERSHIP,
			AggregateId: orgID + ":" + accID, AggregateVersion: uint64(seq), OccurredAt: timestamppb.New(now),
			Payload: &remindersv1.Event_MembershipState{MembershipState: &remindersv1.MembershipState{
				OrganizationId: orgID, AccountId: accID,
				AccountKind: remindersv1.AccountKind_ACCOUNT_KIND_MAX, MaxUserId: maxUID,
				NotifyEnabled: true, NotifyLocalMinutes: notifyMinutes}},
		},
		{
			EventId: fmt.Sprintf("5a000000-0000-4000-8000-0000000%05d", seq*10+3),
			Type:    remindersv1.EventType_EVENT_TYPE_DOCUMENT_STATE, SchemaVersion: 1,
			SourceService: "core", AggregateType: remindersv1.AggregateType_AGGREGATE_TYPE_DOCUMENT,
			AggregateId: documentID, AggregateVersion: 1, OccurredAt: timestamppb.New(now),
			Payload: &remindersv1.Event_DocumentState{DocumentState: &remindersv1.DocumentState{
				DocumentId: documentID, OrganizationId: orgID, Title: title,
				CurrentPeriod:       &remindersv1.DocumentPeriod{PeriodId: periodID, ValidUntil: validUntil},
				ReminderOffsetsDays: []uint32{30}}},
		},
	}
}

func waitHandedOff(ctx context.Context, q remindersv1.ReminderQueryServiceClient, documentID string, limit time.Duration) *remindersv1.PlannedReminder {
	deadline := time.Now().Add(limit)
	var last *remindersv1.PlannedReminder
	for time.Now().Before(deadline) {
		plan, err := q.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{DocumentId: documentID})
		if err == nil && len(plan.GetItems()) > 0 {
			last = plan.GetItems()[0]
			if last.GetStatus() == remindersv1.ReminderStatus_REMINDER_STATUS_HANDED_OFF {
				return last
			}
		}
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("e2e: напоминание документа %s не передано за %s (последнее состояние: %v)", documentID, limit, last)
	return nil
}

func waitStatus(ctx context.Context, bot botv1.MessagingServiceClient, key string,
	want botv1.NotificationStatus, limit time.Duration) *botv1.GetNotificationStatusResponse {
	deadline := time.Now().Add(limit)
	var last *botv1.GetNotificationStatusResponse
	for time.Now().Before(deadline) {
		st, err := bot.GetNotificationStatus(ctx, &botv1.GetNotificationStatusRequest{IdempotencyKey: key})
		if err != nil && status.Code(err) != codes.NotFound {
			log.Fatalf("e2e: GetNotificationStatus: %v", err)
		}
		if err == nil {
			last = st
			if st.GetStatus() == want {
				return st
			}
		}
		time.Sleep(time.Second)
	}
	log.Fatalf("e2e: сообщение %s не достигло состояния %s за %s (последнее: %v)", key, want, limit, last)
	return nil
}

type stubMessage struct {
	Recipient int64  `json:"recipient"`
	Text      string `json:"text"`
	MessageID string `json:"mid"`
	Buttons   []struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		URL     string `json:"url"`
		Payload string `json:"payload"`
	} `json:"buttons"`
}

func (e *env) stubMessages() []stubMessage {
	resp, err := http.Get(e.botAdmin + "/debug/stub/messages")
	if err != nil {
		log.Fatalf("e2e: запрос буфера stub: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("e2e: буфер stub вернул %d", resp.StatusCode)
	}
	var msgs []stubMessage
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		log.Fatalf("e2e: разбор буфера stub: %v", err)
	}
	return msgs
}

func waitMessage(e *env, recipient int64, substring string, limit time.Duration) stubMessage {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if msg := findMessage(e.stubMessages(), recipient, substring); msg != nil {
			return *msg
		}
		time.Sleep(time.Second)
	}
	log.Fatalf("e2e: сообщение со строкой %q не доставлено за %s", substring, limit)
	return stubMessage{}
}

func findMessage(msgs []stubMessage, recipient int64, substring string) *stubMessage {
	for i := range msgs {
		if msgs[i].Recipient == recipient && strings.Contains(msgs[i].Text, substring) {
			return &msgs[i]
		}
	}
	return nil
}

func (e *env) postWebhook(body, secret string) int {
	req, err := http.NewRequest(http.MethodPost, e.botHTTP+"/max/webhook", bytes.NewReader([]byte(body)))
	if err != nil {
		log.Fatalf("e2e: запрос webhook: %v", err)
	}
	req.Header.Set("X-Max-Bot-Api-Secret", secret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("e2e: webhook: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func step(name string) { fmt.Printf("\n== %s\n", name) }

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func dump(v any) string {
	out, _ := json.MarshalIndent(v, "", "  ")
	return string(out)
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
