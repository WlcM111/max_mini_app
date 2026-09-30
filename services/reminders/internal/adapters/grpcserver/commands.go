package grpcserver

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
	"vovremya/services/reminders/internal/app"
	"vovremya/services/reminders/internal/domain"
)

// maxKeyLength — предел длины ключа напоминания (совпадает с ограничением bot-service).
const maxKeyLength = 200

// CommandServer реализует ReminderCommandService.
type CommandServer struct {
	remindersv1.UnimplementedReminderCommandServiceServer

	snooze *app.SnoozeService
	log    *slog.Logger
}

// NewCommandServer создаёт gRPC-сервер команд над напоминаниями.
func NewCommandServer(snooze *app.SnoozeService, log *slog.Logger) *CommandServer {
	return &CommandServer{snooze: snooze, log: log}
}

// SnoozeReminder откладывает напоминание на неделю по нажатию кнопки в чате.
func (s *CommandServer) SnoozeReminder(ctx context.Context,
	req *remindersv1.SnoozeReminderRequest) (*remindersv1.SnoozeReminderResponse, error) {
	key := strings.TrimSpace(req.GetIdempotencyKey())
	if key == "" || utf8.RuneCountInString(key) > maxKeyLength {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key: ожидается непустой ключ до 200 символов")
	}
	res, err := s.snooze.Snooze(ctx, key, req.GetRecipientMaxUserId())
	if err != nil {
		s.log.Warn("snooze failed", slog.Any("error", err))
		return nil, mapError(err)
	}
	resp := &remindersv1.SnoozeReminderResponse{Outcome: toProtoOutcome(res.Outcome)}
	if !res.DueAt.IsZero() {
		resp.DueAt = timestamppb.New(res.DueAt)
	}
	return resp, nil
}

func toProtoOutcome(o domain.SnoozeOutcome) remindersv1.SnoozeOutcome {
	switch o {
	case domain.SnoozeSnoozed:
		return remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_SNOOZED
	case domain.SnoozeAlreadySnoozed:
		return remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_ALREADY_SNOOZED
	case domain.SnoozeTooLate:
		return remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_TOO_LATE
	case domain.SnoozeStale:
		return remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_STALE
	case domain.SnoozeNotFound:
		return remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_NOT_FOUND
	case domain.SnoozeForbidden:
		return remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_FORBIDDEN
	default:
		return remindersv1.SnoozeOutcome_SNOOZE_OUTCOME_UNSPECIFIED
	}
}
