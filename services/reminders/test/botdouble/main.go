// Команда botdouble — управляемый тестовый сервер bot-service для автономной
// проверки reminders-service. Реализует нормативный контракт
// vovremya.bot.v1.MessagingService, проверяет содержимое запросов и хранит
// принятые сообщения в памяти. Предназначен только для dev/test окружения и
// не входит в production-образ сервиса.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	botv1 "vovremya/gen/go/vovremya/bot/v1"
)

type accepted struct {
	IdempotencyKey string `json:"idempotency_key"`
	NotificationID string `json:"notification_id"`
	Recipient      int64  `json:"recipient_max_user_id"`
	Text           string `json:"text"`
	ButtonPayload  string `json:"button_payload"`
	NotAfter       string `json:"not_after"`
	ReceivedAt     string `json:"received_at"`
}

type server struct {
	botv1.UnimplementedMessagingServiceServer

	mu       sync.Mutex
	byKey    map[string]accepted
	order    []string
	failNext int // сколько следующих запросов отклонить
	failCode codes.Code
}

func (s *server) EnqueueNotification(_ context.Context, req *botv1.EnqueueNotificationRequest) (*botv1.EnqueueNotificationResponse, error) {
	// Проверка контракта: двойник не подтверждает любой запрос подряд.
	if req.GetIdempotencyKey() == "" || len(req.GetIdempotencyKey()) > 120 {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key: 1..120 символов")
	}
	if req.GetKind() == botv1.NotificationKind_NOTIFICATION_KIND_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "kind: обязательное поле")
	}
	if req.GetRecipientMaxUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "recipient_max_user_id: положительное число")
	}
	text := []rune(req.GetText())
	if len(text) == 0 || len(text) > 4000 {
		return nil, status.Error(codes.InvalidArgument, "text: 1..4000 символов")
	}
	if len(req.GetButtons()) > 3 {
		return nil, status.Error(codes.InvalidArgument, "buttons: не более трёх кнопок")
	}
	for _, b := range req.GetButtons() {
		if b.GetText() == "" {
			return nil, status.Error(codes.InvalidArgument, "buttons.text: обязательное поле")
		}
		if b.GetOpenAppPayload() == "" && b.GetUrl() == "" {
			return nil, status.Error(codes.InvalidArgument, "buttons: требуется url или open_app_payload")
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failNext > 0 {
		s.failNext--
		return nil, status.Error(s.failCode, "botdouble: сконфигурированный отказ")
	}
	if prev, ok := s.byKey[req.GetIdempotencyKey()]; ok {
		return &botv1.EnqueueNotificationResponse{NotificationId: prev.NotificationID, Duplicate: true}, nil
	}
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", len(s.order)+1)
	rec := accepted{
		IdempotencyKey: req.GetIdempotencyKey(),
		NotificationID: id,
		Recipient:      req.GetRecipientMaxUserId(),
		Text:           req.GetText(),
		ReceivedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if len(req.GetButtons()) > 0 {
		rec.ButtonPayload = req.GetButtons()[0].GetOpenAppPayload()
	}
	if req.GetNotAfter() != nil {
		rec.NotAfter = req.GetNotAfter().AsTime().Format(time.RFC3339)
	}
	s.byKey[rec.IdempotencyKey] = rec
	s.order = append(s.order, rec.IdempotencyKey)
	return &botv1.EnqueueNotificationResponse{NotificationId: id, Duplicate: false}, nil
}

func (s *server) GetRecipientStatus(_ context.Context, req *botv1.GetRecipientStatusRequest) (*botv1.GetRecipientStatusResponse, error) {
	if req.GetMaxUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "max_user_id: положительное число")
	}
	return &botv1.GetRecipientStatusResponse{State: botv1.RecipientState_RECIPIENT_STATE_ACTIVE}, nil
}

func (s *server) GetBotProfile(context.Context, *botv1.GetBotProfileRequest) (*botv1.GetBotProfileResponse, error) {
	return &botv1.GetBotProfileResponse{
		Username:            "vovremya_double_bot",
		DisplayName:         "Вовремя (двойник)",
		ChatUrl:             "https://max.ru/vovremya_double_bot",
		OpenAppLinkTemplate: "https://max.ru/vovremya_double_bot?startapp={payload}",
	}, nil
}

// GetNotificationStatus отдаёт состояние принятого сообщения по ключу идемпотентности.
func (s *server) GetNotificationStatus(_ context.Context, req *botv1.GetNotificationStatusRequest) (*botv1.GetNotificationStatusResponse, error) {
	if req.GetIdempotencyKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key: обязательное поле")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.byKey[req.GetIdempotencyKey()]
	if !ok {
		return nil, status.Error(codes.NotFound, "сообщение не найдено")
	}
	return &botv1.GetNotificationStatusResponse{
		NotificationId: rec.NotificationID,
		Status:         botv1.NotificationStatus_NOTIFICATION_STATUS_SENT,
	}, nil
}

// adminHandler даёт тестам доступ к принятым сообщениям и управлению отказами.
func (s *server) adminHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /messages", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		out := make([]accepted, 0, len(s.order))
		for _, k := range s.order {
			out = append(out, s.byKey[k])
		}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("POST /fail", func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("count"))
		code := codes.Unavailable
		if r.URL.Query().Get("code") == "invalid" {
			code = codes.InvalidArgument
		}
		s.mu.Lock()
		s.failNext, s.failCode = n, code
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func main() {
	grpcAddr := envOr("BOTDOUBLE_GRPC_ADDR", ":9090")
	adminAddr := envOr("BOTDOUBLE_ADMIN_ADDR", ":8090")
	s := &server{byKey: make(map[string]accepted)}

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("botdouble: listen %s: %v", grpcAddr, err)
	}
	srv := grpc.NewServer()
	botv1.RegisterMessagingServiceServer(srv, s)

	go func() {
		admin := &http.Server{Addr: adminAddr, Handler: s.adminHandler(), ReadHeaderTimeout: 5 * time.Second}
		if err := admin.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("botdouble: admin server: %v", err)
		}
	}()
	log.Printf("botdouble: grpc on %s, admin on %s", grpcAddr, adminAddr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("botdouble: serve: %v", err)
	}
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
