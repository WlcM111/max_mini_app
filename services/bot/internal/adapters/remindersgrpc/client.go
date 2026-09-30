// Package remindersgrpc — клиент команд reminders-service для bot-service (ADR-036).
package remindersgrpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
	"vovremya/services/bot/internal/ports"
)

// Client вызывает ReminderCommandService.
type Client struct {
	api     remindersv1.ReminderCommandServiceClient
	timeout time.Duration
}

// NewClient создаёт клиент поверх соединения gRPC.
func NewClient(conn grpc.ClientConnInterface, timeout time.Duration) *Client {
	return &Client{api: remindersv1.NewReminderCommandServiceClient(conn), timeout: timeout}
}

// SnoozeReminder откладывает напоминание на неделю от имени нажавшего кнопку.
func (c *Client) SnoozeReminder(ctx context.Context, idempotencyKey string,
	recipientMaxUserID int64) (ports.SnoozeOutcome, time.Time, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.api.SnoozeReminder(callCtx, &remindersv1.SnoozeReminderRequest{
		IdempotencyKey:     idempotencyKey,
		RecipientMaxUserId: recipientMaxUserID,
	})
	if err != nil {
		return ports.SnoozeUnknown, time.Time{}, fmt.Errorf("reminders snooze: %w", err)
	}
	var due time.Time
	if resp.GetDueAt() != nil {
		due = resp.GetDueAt().AsTime()
	}
	return fromProto(resp.GetOutcome()), due, nil
}

func fromProto(o remindersv1.SnoozeOutcome) ports.SnoozeOutcome {
	switch o {
	case remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_SNOOZED:
		return ports.SnoozeSnoozed
	case remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_ALREADY_SNOOZED:
		return ports.SnoozeAlreadySnoozed
	case remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_TOO_LATE:
		return ports.SnoozeTooLate
	case remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_STALE:
		return ports.SnoozeStale
	case remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_NOT_FOUND:
		return ports.SnoozeNotFound
	case remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_FORBIDDEN:
		return ports.SnoozeForbidden
	default:
		return ports.SnoozeUnknown
	}
}
