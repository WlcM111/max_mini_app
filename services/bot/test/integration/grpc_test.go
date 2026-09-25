package integration_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	botv1 "vovremya/gen/go/vovremya/bot/v1"
	"vovremya/internal/platform/grpckit"
	"vovremya/internal/platform/metrics"
	"vovremya/services/bot/internal/adapters/grpcserver"
	"vovremya/services/bot/internal/domain"

	"google.golang.org/grpc/health"
)

func newGRPCClient(t *testing.T, s *stack) botv1.MessagingServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnaryInterceptor(grpckit.UnaryServerInterceptor(s.log, metrics.New(), 5*time.Second)))
	grpcserver.New(s.messaging, s.log).Register(srv)
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("подключение: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	})

	healthClient := grpc_health_v1.NewHealthClient(conn)
	hctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := healthClient.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil || resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("health: %v %v", err, resp.GetStatus())
	}
	return botv1.NewMessagingServiceClient(conn)
}

func enqueueRequest(s *stack, key string) *botv1.EnqueueNotificationRequest {
	return &botv1.EnqueueNotificationRequest{
		IdempotencyKey:     key,
		Kind:               botv1.NotificationKind_NOTIFICATION_KIND_REMINDER,
		RecipientMaxUserId: 1001,
		Text:               "Через 7 дней заканчивается срок: «Договор». Организация: Кафе. Срок до 28.09.2026.",
		Buttons: []*botv1.Button{{
			Text:   "Открыть документ",
			Action: &botv1.Button_OpenAppPayload{OpenAppPayload: "doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"},
		}},
		NotAfter: timestamppb.New(s.clock.Now().Add(24 * time.Hour)),
	}
}

func TestGRPCEnqueueAndStatus(t *testing.T) {
	s := newStack(t, stackOptions{})
	client := newGRPCClient(t, s)
	ctx := context.Background()
	req := enqueueRequest(s, "rem:6d1f0a73-2b8c-4e5f-9a10-3c7b5e2d8f04:9f3c7b21-4d6e-4a8b-b5c0-1e2f3a4b5c06:7")

	resp, err := client.EnqueueNotification(ctx, req)
	if err != nil {
		t.Fatalf("EnqueueNotification: %v", err)
	}
	if resp.GetNotificationId() == "" || resp.GetStatus() != botv1.NotificationStatus_NOTIFICATION_STATUS_QUEUED || resp.GetDuplicate() {
		t.Fatalf("ответ: %+v", resp)
	}

	// Повтор того же запроса — duplicate=true, идентификатор тот же.
	again, err := client.EnqueueNotification(ctx, req)
	if err != nil {
		t.Fatalf("повтор: %v", err)
	}
	if !again.GetDuplicate() || again.GetNotificationId() != resp.GetNotificationId() {
		t.Errorf("повтор: %+v", again)
	}

	// Тот же ключ с другим содержимым — ALREADY_EXISTS.
	conflict := enqueueRequest(s, req.GetIdempotencyKey())
	conflict.Text = "Другой текст"
	if _, err := client.EnqueueNotification(ctx, conflict); status.Code(err) != codes.AlreadyExists {
		t.Errorf("конфликт: ожидался ALREADY_EXISTS, получено %v", err)
	}

	st, err := client.GetNotificationStatus(ctx, &botv1.GetNotificationStatusRequest{IdempotencyKey: req.GetIdempotencyKey()})
	if err != nil {
		t.Fatalf("GetNotificationStatus: %v", err)
	}
	if st.GetStatus() != botv1.NotificationStatus_NOTIFICATION_STATUS_QUEUED || st.GetAttempts() != 0 {
		t.Errorf("состояние: %+v", st)
	}

	// После доставки статус SENT и заполнен момент отправки.
	if _, err := s.delivery.ProcessBatch(ctx); err != nil {
		t.Fatalf("доставка: %v", err)
	}
	st, err = client.GetNotificationStatus(ctx, &botv1.GetNotificationStatusRequest{IdempotencyKey: req.GetIdempotencyKey()})
	if err != nil {
		t.Fatalf("GetNotificationStatus после доставки: %v", err)
	}
	if st.GetStatus() != botv1.NotificationStatus_NOTIFICATION_STATUS_SENT || st.GetSentAt() == nil || st.GetAttempts() != 1 {
		t.Errorf("состояние после доставки: %+v", st)
	}
	if st.GetLastErrorCode() != "" {
		t.Errorf("код ошибки после успеха: %q", st.GetLastErrorCode())
	}

	if _, err := client.GetNotificationStatus(ctx, &botv1.GetNotificationStatusRequest{IdempotencyKey: "absent-key-0001"}); status.Code(err) != codes.NotFound {
		t.Errorf("отсутствующий ключ: ожидался NOT_FOUND, получено %v", err)
	}
}

func TestGRPCValidation(t *testing.T) {
	s := newStack(t, stackOptions{})
	client := newGRPCClient(t, s)
	ctx := context.Background()

	cases := []struct {
		name   string
		mutate func(r *botv1.EnqueueNotificationRequest)
	}{
		{"ключ не по шаблону", func(r *botv1.EnqueueNotificationRequest) { r.IdempotencyKey = "BAD KEY" }},
		{"вид не задан", func(r *botv1.EnqueueNotificationRequest) {
			r.Kind = botv1.NotificationKind_NOTIFICATION_KIND_UNSPECIFIED
		}},
		{"получатель не задан", func(r *botv1.EnqueueNotificationRequest) { r.RecipientMaxUserId = 0 }},
		{"пустой текст", func(r *botv1.EnqueueNotificationRequest) { r.Text = "" }},
		{"четыре кнопки", func(r *botv1.EnqueueNotificationRequest) {
			b := r.Buttons[0]
			r.Buttons = []*botv1.Button{b, b, b, b}
		}},
		{"кнопка без действия", func(r *botv1.EnqueueNotificationRequest) {
			r.Buttons = []*botv1.Button{{Text: "Кнопка"}}
		}},
		{"payload с недопустимым символом", func(r *botv1.EnqueueNotificationRequest) {
			r.Buttons = []*botv1.Button{{Text: "Кнопка", Action: &botv1.Button_OpenAppPayload{OpenAppPayload: "doc/1"}}}
		}},
		{"ссылка не https", func(r *botv1.EnqueueNotificationRequest) {
			r.Buttons = []*botv1.Button{{Text: "Кнопка", Action: &botv1.Button_Url{Url: "http://example.org"}}}
		}},
		{"not_after не задан", func(r *botv1.EnqueueNotificationRequest) { r.NotAfter = nil }},
		{"not_after в прошлом", func(r *botv1.EnqueueNotificationRequest) {
			r.NotAfter = timestamppb.New(s.clock.Now().Add(-time.Minute))
		}},
		{"not_after дальше 7 суток", func(r *botv1.EnqueueNotificationRequest) {
			r.NotAfter = timestamppb.New(s.clock.Now().Add(8 * 24 * time.Hour))
		}},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := enqueueRequest(s, "validation-key-"+string(rune('a'+i)))
			c.mutate(req)
			_, err := client.EnqueueNotification(ctx, req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("ожидался INVALID_ARGUMENT, получено %v", err)
			}
			if msg := status.Convert(err).Message(); msg == "" {
				t.Error("сообщение об ошибке должно называть поле")
			}
		})
	}

	if _, err := client.GetRecipientStatus(ctx, &botv1.GetRecipientStatusRequest{MaxUserId: 0}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("GetRecipientStatus(0): %v", err)
	}
	if _, err := client.GetNotificationStatus(ctx, &botv1.GetNotificationStatusRequest{IdempotencyKey: "BAD"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("GetNotificationStatus(BAD): %v", err)
	}
}

func TestGRPCBackpressureAndProfile(t *testing.T) {
	s := newStack(t, stackOptions{queueLimit: 1})
	client := newGRPCClient(t, s)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		req := enqueueRequest(s, "backpressure-key-"+string(rune('a'+i)))
		if _, err := client.EnqueueNotification(ctx, req); err != nil {
			t.Fatalf("постановка %d: %v", i, err)
		}
	}
	if err := s.monitor.Refresh(ctx); err != nil {
		t.Fatalf("обновление глубины: %v", err)
	}
	if _, err := client.EnqueueNotification(ctx, enqueueRequest(s, "backpressure-key-c")); status.Code(err) != codes.ResourceExhausted {
		t.Errorf("переполнение очереди: ожидался RESOURCE_EXHAUSTED, получено %v", err)
	}

	profile, err := client.GetBotProfile(ctx, &botv1.GetBotProfileRequest{})
	if err != nil {
		t.Fatalf("GetBotProfile: %v", err)
	}
	if profile.GetUsername() != "vovremya_local_bot" ||
		profile.GetChatUrl() != "https://max.ru/vovremya_local_bot" ||
		profile.GetOpenAppLinkTemplate() != "https://max.ru/vovremya_local_bot?startapp={payload}" ||
		profile.GetMode() != botv1.BotMode_BOT_MODE_STUB {
		t.Errorf("профиль: %+v", profile)
	}

	empty := newStack(t, stackOptions{profile: &domain.Profile{}})
	emptyClient := newGRPCClient(t, empty)
	if _, err := emptyClient.GetBotProfile(ctx, &botv1.GetBotProfileRequest{}); status.Code(err) != codes.Unavailable {
		t.Errorf("профиль не загружен: ожидался UNAVAILABLE, получено %v", err)
	}
}

func TestGRPCRecipientStatus(t *testing.T) {
	s := newStack(t, stackOptions{})
	client := newGRPCClient(t, s)
	ctx := context.Background()

	unknown, err := client.GetRecipientStatus(ctx, &botv1.GetRecipientStatusRequest{MaxUserId: 9999})
	if err != nil {
		t.Fatalf("GetRecipientStatus: %v", err)
	}
	if unknown.GetState() != botv1.RecipientState_RECIPIENT_STATE_UNKNOWN || unknown.GetStateChangedAt() != nil {
		t.Errorf("незнакомый получатель: %+v", unknown)
	}
	if unknown.GetBotChatUrl() != "https://max.ru/vovremya_local_bot" {
		t.Errorf("ссылка на чат: %q", unknown.GetBotChatUrl())
	}

	err = s.pool.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.recipients.LockOrCreate(ctx, 1001, s.clock.Now())
		if err != nil {
			return err
		}
		next, _ := r.ApplyEvent(domain.UpdateBotStopped, s.clock.Now())
		return s.recipients.Save(ctx, next, s.clock.Now())
	})
	if err != nil {
		t.Fatalf("подготовка получателя: %v", err)
	}
	stopped, err := client.GetRecipientStatus(ctx, &botv1.GetRecipientStatusRequest{MaxUserId: 1001})
	if err != nil {
		t.Fatalf("GetRecipientStatus: %v", err)
	}
	if stopped.GetState() != botv1.RecipientState_RECIPIENT_STATE_STOPPED || stopped.GetStateChangedAt() == nil {
		t.Errorf("остановленный получатель: %+v", stopped)
	}
}

func TestGRPCDeadlineAndCancel(t *testing.T) {
	s := newStack(t, stackOptions{})
	client := newGRPCClient(t, s)

	expired, cancel := context.WithTimeout(context.Background(), -time.Second)
	defer cancel()
	_, err := client.EnqueueNotification(expired, enqueueRequest(s, "deadline-key-0001"))
	if status.Code(err) != codes.DeadlineExceeded {
		t.Errorf("истёкший срок: ожидался DEADLINE_EXCEEDED, получено %v", err)
	}

	canceled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	_, err = client.EnqueueNotification(canceled, enqueueRequest(s, "cancel-key-0001"))
	if status.Code(err) != codes.Canceled {
		t.Errorf("отменённый запрос: ожидался CANCELED, получено %v", err)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages`); n != 0 {
		t.Errorf("отменённые запросы не должны создавать сообщений: %d", n)
	}
}
