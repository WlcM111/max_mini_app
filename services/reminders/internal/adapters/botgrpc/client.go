// Package botgrpc — исходящий адаптер к bot-service (нормативный контракт
// vovremya.bot.v1.MessagingService). Здесь и только здесь коды gRPC
// превращаются в решение «повторить или отказаться».
package botgrpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	botv1 "vovremya/gen/go/vovremya/bot/v1"
	"vovremya/services/reminders/internal/ports"
)

// Client — клиент MessagingService.
type Client struct {
	api     botv1.MessagingServiceClient
	timeout time.Duration
}

// NewClient создаёт клиент поверх готового соединения.
func NewClient(conn grpc.ClientConnInterface, timeout time.Duration) *Client {
	return &Client{api: botv1.NewMessagingServiceClient(conn), timeout: timeout}
}

// Enqueue ставит напоминание в очередь доставки bot-service.
// Идемпотентность обеспечивается ключом запроса: повтор после таймаута
// не создаёт второе сообщение (duplicate=true в ответе).
func (c *Client) Enqueue(ctx context.Context, req ports.NotificationRequest) (ports.NotificationResult, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	pbReq := &botv1.EnqueueNotificationRequest{
		IdempotencyKey:     req.IdempotencyKey,
		Kind:               botv1.NotificationKind_NOTIFICATION_KIND_REMINDER,
		RecipientMaxUserId: req.RecipientMaxUserID,
		Text:               req.Text,
		Silent:             false,
		NotAfter:           timestamppb.New(req.NotAfter.UTC()),
	}
	for _, b := range req.Buttons {
		pbReq.Buttons = append(pbReq.Buttons, &botv1.Button{
			Text:   b.Text,
			Action: &botv1.Button_OpenAppPayload{OpenAppPayload: b.Payload},
		})
	}
	if len(req.Buttons) == 0 && req.ButtonText != "" {
		pbReq.Buttons = []*botv1.Button{{
			Text:   req.ButtonText,
			Action: &botv1.Button_OpenAppPayload{OpenAppPayload: req.ButtonPayload},
		}}
	}

	resp, err := c.api.EnqueueNotification(callCtx, pbReq)
	if err != nil {
		return ports.NotificationResult{}, classify(err)
	}
	return ports.NotificationResult{
		NotificationID: resp.GetNotificationId(),
		Duplicate:      resp.GetDuplicate(),
	}, nil
}

// classify переводит код gRPC в решение о повторе.
// Повторяем только то, что может измениться со временем: недоступность,
// исчерпание лимитов, срыв срока, внутреннюю ошибку получателя.
// Ошибки контракта повторять бессмысленно — напоминание помечается пропущенным.
func classify(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return &ports.GatewayError{Code: "transport", Retryable: true, Err: err}
	}
	switch st.Code() {
	case codes.Unavailable:
		return &ports.GatewayError{Code: "unavailable", Retryable: true, Err: err}
	case codes.DeadlineExceeded:
		return &ports.GatewayError{Code: "deadline_exceeded", Retryable: true, Err: err}
	case codes.ResourceExhausted:
		return &ports.GatewayError{Code: "resource_exhausted", Retryable: true, Err: err}
	case codes.Aborted:
		return &ports.GatewayError{Code: "aborted", Retryable: true, Err: err}
	case codes.Internal, codes.Unknown:
		return &ports.GatewayError{Code: "bot_internal", Retryable: true, Err: err}
	case codes.InvalidArgument:
		return &ports.GatewayError{Code: "invalid_argument", Retryable: false, Err: err}
	case codes.FailedPrecondition:
		return &ports.GatewayError{Code: "failed_precondition", Retryable: false, Err: err}
	case codes.AlreadyExists:
		return &ports.GatewayError{Code: "already_exists", Retryable: false, Err: err}
	case codes.PermissionDenied, codes.Unauthenticated:
		return &ports.GatewayError{Code: "permission_denied", Retryable: false, Err: err}
	case codes.Unimplemented:
		return &ports.GatewayError{Code: "unimplemented", Retryable: false, Err: err}
	case codes.NotFound:
		return &ports.GatewayError{Code: "not_found", Retryable: false, Err: err}
	default:
		return &ports.GatewayError{Code: fmt.Sprintf("grpc_%s", st.Code().String()), Retryable: true, Err: err}
	}
}

var _ ports.MessagingGateway = (*Client)(nil)
