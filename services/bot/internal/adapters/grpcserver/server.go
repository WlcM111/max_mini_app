// Package grpcserver реализует MessagingService (api/proto/vovremya/bot/v1).
package grpcserver

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	botv1 "vovremya/gen/go/vovremya/bot/v1"
	"vovremya/internal/platform/logging"
	"vovremya/services/bot/internal/app"
	"vovremya/services/bot/internal/domain"
)

// Server — gRPC-сервер сервиса сообщений.
type Server struct {
	botv1.UnimplementedMessagingServiceServer
	svc *app.MessagingService
	log *slog.Logger
}

// New создаёт сервер.
func New(svc *app.MessagingService, log *slog.Logger) *Server {
	return &Server{svc: svc, log: log}
}

// Register регистрирует сервис в gRPC-сервере.
func (s *Server) Register(gs *grpc.Server) { botv1.RegisterMessagingServiceServer(gs, s) }

// EnqueueNotification ставит сообщение в очередь доставки.
func (s *Server) EnqueueNotification(ctx context.Context, req *botv1.EnqueueNotificationRequest) (*botv1.EnqueueNotificationResponse, error) {
	hash, err := requestHash(req)
	if err != nil {
		return nil, status.Error(codes.Internal, "не удалось вычислить ключ содержимого")
	}
	domainReq, err := toDomainRequest(req)
	if err != nil {
		return nil, s.mapError(ctx, err, "EnqueueNotification")
	}
	res, err := s.svc.Enqueue(ctx, domainReq, hash)
	if err != nil {
		return nil, s.mapError(ctx, err, "EnqueueNotification")
	}
	return &botv1.EnqueueNotificationResponse{
		NotificationId: res.Notification.PublicID,
		Status:         statusToProto(res.Notification.Status),
		Duplicate:      res.Duplicate,
	}, nil
}

// GetNotificationStatus возвращает состояние доставки по ключу идемпотентности.
func (s *Server) GetNotificationStatus(ctx context.Context, req *botv1.GetNotificationStatusRequest) (*botv1.GetNotificationStatusResponse, error) {
	n, err := s.svc.NotificationStatus(ctx, req.GetIdempotencyKey())
	if err != nil {
		return nil, s.mapError(ctx, err, "GetNotificationStatus")
	}
	resp := &botv1.GetNotificationStatusResponse{
		NotificationId: n.PublicID,
		Status:         statusToProto(n.Status),
		Attempts:       int32(n.Attempts),
		LastErrorCode:  n.LastErrorCode,
	}
	if n.Status == domain.StatusSent && n.SentAt != nil {
		resp.SentAt = timestamppb.New(*n.SentAt)
	}
	return resp, nil
}

// GetRecipientStatus сообщает, может ли пользователь получать сообщения бота.
func (s *Server) GetRecipientStatus(ctx context.Context, req *botv1.GetRecipientStatusRequest) (*botv1.GetRecipientStatusResponse, error) {
	st, err := s.svc.RecipientStatus(ctx, req.GetMaxUserId())
	if err != nil {
		return nil, s.mapError(ctx, err, "GetRecipientStatus")
	}
	resp := &botv1.GetRecipientStatusResponse{
		State:      recipientStateToProto(st.Recipient.State),
		BotChatUrl: st.ChatURL,
	}
	if st.Known && !st.Recipient.StateChangedAt.IsZero() {
		resp.StateChangedAt = timestamppb.New(st.Recipient.StateChangedAt)
	}
	return resp, nil
}

// GetBotProfile возвращает профиль бота и шаблон диплинка.
func (s *Server) GetBotProfile(ctx context.Context, _ *botv1.GetBotProfileRequest) (*botv1.GetBotProfileResponse, error) {
	p, mode, err := s.svc.BotProfile()
	if err != nil {
		return nil, s.mapError(ctx, err, "GetBotProfile")
	}
	return &botv1.GetBotProfileResponse{
		Username:            p.Username,
		DisplayName:         p.DisplayName,
		ChatUrl:             p.ChatURL(),
		OpenAppLinkTemplate: p.OpenAppLinkTemplate(),
		Mode:                modeToProto(mode),
	}, nil
}

// requestHash — SHA-256 детерминированной сериализации запроса с очищенным
// ключом идемпотентности (grpc-contract §1).
func requestHash(req *botv1.EnqueueNotificationRequest) ([]byte, error) {
	clone, ok := proto.Clone(req).(*botv1.EnqueueNotificationRequest)
	if !ok {
		return nil, errors.New("clone request")
	}
	clone.IdempotencyKey = ""
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(clone)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}

func toDomainRequest(req *botv1.EnqueueNotificationRequest) (domain.EnqueueRequest, error) {
	out := domain.EnqueueRequest{
		IdempotencyKey: req.GetIdempotencyKey(),
		Message: domain.Message{
			Kind:               kindFromProto(req.GetKind()),
			RecipientMaxUserID: req.GetRecipientMaxUserId(),
			Text:               req.GetText(),
			Silent:             req.GetSilent(),
		},
	}
	if ts := req.GetNotAfter(); ts != nil {
		if err := ts.CheckValid(); err != nil {
			return domain.EnqueueRequest{}, &domain.ValidationError{Field: "not_after", Reason: "некорректная отметка времени"}
		}
		out.NotAfter = ts.AsTime().UTC()
	}
	for _, b := range req.GetButtons() {
		button := domain.Button{Text: b.GetText()}
		switch action := b.GetAction().(type) {
		case *botv1.Button_OpenAppPayload:
			button.Action = domain.ActionOpenApp
			button.Payload = action.OpenAppPayload
		case *botv1.Button_Url:
			button.Action = domain.ActionURL
			button.URL = action.Url
		}
		out.Buttons = append(out.Buttons, button)
	}
	return out, nil
}

func kindFromProto(k botv1.NotificationKind) domain.Kind {
	switch k {
	case botv1.NotificationKind_NOTIFICATION_KIND_REMINDER:
		return domain.KindReminder
	case botv1.NotificationKind_NOTIFICATION_KIND_MEMBER_JOINED:
		return domain.KindMemberJoined
	default:
		return ""
	}
}

func statusToProto(s domain.Status) botv1.NotificationStatus {
	switch s {
	case domain.StatusQueued:
		return botv1.NotificationStatus_NOTIFICATION_STATUS_QUEUED
	case domain.StatusSending:
		return botv1.NotificationStatus_NOTIFICATION_STATUS_SENDING
	case domain.StatusSent:
		return botv1.NotificationStatus_NOTIFICATION_STATUS_SENT
	case domain.StatusRetryWait:
		return botv1.NotificationStatus_NOTIFICATION_STATUS_RETRY_WAIT
	case domain.StatusFailed:
		return botv1.NotificationStatus_NOTIFICATION_STATUS_FAILED
	case domain.StatusExpired:
		return botv1.NotificationStatus_NOTIFICATION_STATUS_EXPIRED
	default:
		return botv1.NotificationStatus_NOTIFICATION_STATUS_UNSPECIFIED
	}
}

func recipientStateToProto(s domain.RecipientState) botv1.RecipientState {
	switch s {
	case domain.RecipientActive:
		return botv1.RecipientState_RECIPIENT_STATE_ACTIVE
	case domain.RecipientMuted:
		return botv1.RecipientState_RECIPIENT_STATE_MUTED
	case domain.RecipientStopped:
		return botv1.RecipientState_RECIPIENT_STATE_STOPPED
	case domain.RecipientUnreachable:
		return botv1.RecipientState_RECIPIENT_STATE_UNREACHABLE
	default:
		return botv1.RecipientState_RECIPIENT_STATE_UNKNOWN
	}
}

func modeToProto(m domain.Mode) botv1.BotMode {
	if m == domain.ModeLive {
		return botv1.BotMode_BOT_MODE_LIVE
	}
	return botv1.BotMode_BOT_MODE_STUB
}

// mapError переводит ошибки сценариев в коды gRPC (grpc-contract §1).
func (s *Server) mapError(ctx context.Context, err error, method string) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		return status.Errorf(codes.InvalidArgument, "%s: %s", ve.Field, ve.Reason)
	case errors.Is(err, domain.ErrValidation):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return status.Error(codes.AlreadyExists, "ключ идемпотентности уже использован с другим содержимым")
	case errors.Is(err, domain.ErrQueueFull):
		return status.Error(codes.ResourceExhausted, "очередь доставки переполнена")
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "сообщение не найдено")
	case errors.Is(err, domain.ErrProfileUnavailable):
		return status.Error(codes.Unavailable, "профиль бота ещё не загружен")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "запрос отменён")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "истёк срок запроса")
	default:
		// Внутренние ошибки хранилища наружу не раскрываются.
		logging.From(ctx, s.log).Error("grpc request failed",
			slog.String("method", method), slog.Any("error", err))
		return status.Error(codes.Unavailable, "хранилище временно недоступно")
	}
}
