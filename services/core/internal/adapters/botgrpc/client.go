// Package botgrpc — клиент MessagingService bot-service (grpc-contract §1).
package botgrpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	botv1 "vovremya/gen/go/vovremya/bot/v1"
	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// Client — клиент bot-service.
type Client struct {
	conn   *grpc.ClientConn
	client botv1.MessagingServiceClient
}

var _ ports.BotGateway = (*Client)(nil)

// New открывает соединение с bot-service.
func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("bot grpc client: %w", err)
	}
	return &Client{conn: conn, client: botv1.NewMessagingServiceClient(conn)}, nil
}

// Close закрывает соединение.
func (c *Client) Close() error { return c.conn.Close() }

// EnqueueNotification ставит сообщение в очередь доставки bot-service.
func (c *Client) EnqueueNotification(ctx context.Context, req ports.NotificationRequest) error {
	kind := botv1.NotificationKind_NOTIFICATION_KIND_UNSPECIFIED
	switch req.Kind {
	case "reminder":
		kind = botv1.NotificationKind_NOTIFICATION_KIND_REMINDER
	case "member_joined":
		kind = botv1.NotificationKind_NOTIFICATION_KIND_MEMBER_JOINED
	}
	buttons := make([]*botv1.Button, 0, len(req.Buttons))
	for _, b := range req.Buttons {
		button := &botv1.Button{Text: b.Text}
		if b.URL != "" {
			button.Action = &botv1.Button_Url{Url: b.URL}
		} else {
			button.Action = &botv1.Button_OpenAppPayload{OpenAppPayload: b.OpenAppPayload}
		}
		buttons = append(buttons, button)
	}
	_, err := c.client.EnqueueNotification(ctx, &botv1.EnqueueNotificationRequest{
		IdempotencyKey:     req.IdempotencyKey,
		Kind:               kind,
		RecipientMaxUserId: req.RecipientMax,
		Text:               req.Text,
		Buttons:            buttons,
		NotAfter:           timestamppb.New(req.NotAfter),
	})
	if err != nil {
		// Повтор с тем же ключом и другим содержимым — не ошибка доставки.
		if status.Code(err) == codes.AlreadyExists {
			return nil
		}
		return fmt.Errorf("EnqueueNotification: %w", err)
	}
	return nil
}

// GetRecipientStatus возвращает состояние канала напоминаний пользователя.
func (c *Client) GetRecipientStatus(ctx context.Context, maxUserID int64) (ports.RecipientStatus, error) {
	resp, err := c.client.GetRecipientStatus(ctx, &botv1.GetRecipientStatusRequest{MaxUserId: maxUserID})
	if err != nil {
		return ports.RecipientStatus{}, fmt.Errorf("GetRecipientStatus: %w", err)
	}
	return ports.RecipientStatus{State: recipientState(resp.GetState()), BotChatURL: resp.GetBotChatUrl()}, nil
}

// GetBotProfile возвращает профиль бота и шаблон диплинка.
func (c *Client) GetBotProfile(ctx context.Context) (ports.BotProfile, error) {
	resp, err := c.client.GetBotProfile(ctx, &botv1.GetBotProfileRequest{})
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			return ports.BotProfile{}, domain.ErrDependencyUnavailable
		}
		return ports.BotProfile{}, fmt.Errorf("GetBotProfile: %w", err)
	}
	return ports.BotProfile{
		Username:            resp.GetUsername(),
		DisplayName:         resp.GetDisplayName(),
		ChatURL:             resp.GetChatUrl(),
		OpenAppLinkTemplate: resp.GetOpenAppLinkTemplate(),
	}, nil
}

func recipientState(s botv1.RecipientState) string {
	switch s {
	case botv1.RecipientState_RECIPIENT_STATE_ACTIVE:
		return "active"
	case botv1.RecipientState_RECIPIENT_STATE_MUTED:
		return "muted"
	case botv1.RecipientState_RECIPIENT_STATE_STOPPED:
		return "stopped"
	case botv1.RecipientState_RECIPIENT_STATE_UNREACHABLE:
		return "unreachable"
	default:
		return "unknown"
	}
}

var _ = errors.Is
var _ = time.Second
