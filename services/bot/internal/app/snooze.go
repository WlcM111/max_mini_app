package app

import (
	"context"
	"log/slog"
	"strings"

	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// callbackAnswerer — клиент MAX, умеющий отвечать на нажатия callback-кнопок.
type callbackAnswerer interface {
	AnswerCallback(ctx context.Context, callbackID, notification string) error
}

// Ответы на нажатие «Напомнить через неделю».
const (
	snoozeAnswerDone     = "Напомню через неделю"
	snoozeAnswerAlready  = "Уже напомню через неделю"
	snoozeAnswerTooLate  = "До окончания срока меньше недели — продлите документ в приложении"
	snoozeAnswerStale    = "Документ изменился — откройте его в приложении"
	snoozeAnswerNotFound = "Не получилось отложить — откройте документ в приложении"
	snoozeAnswerForeign  = "Это напоминание адресовано другому пользователю"
	snoozeAnswerRetry    = "Не получилось отложить — попробуйте ещё раз"
)

// UseCallbackAnswers подключает ответы на нажатия кнопок, если клиент MAX их поддерживает.
func (s *WebhookService) UseCallbackAnswers(client any) {
	if a, ok := client.(callbackAnswerer); ok {
		s.answerer = a
	}
}

// UseSnoozer подключает reminders-service, который ведёт отложенные повторы (ADR-036).
func (s *WebhookService) UseSnoozer(c ports.ReminderCommands) { s.snoozer = c }

// snooze передаёт нажатие «Напомнить через неделю» в reminders-service и отвечает
// всплывающим уведомлением. Повтор становится строкой плана reminders-service, поэтому
// его отменяют продление и удаление документа, исключение участника и удаление аккаунта.
func (s *WebhookService) snooze(ctx context.Context, in InboundUpdate) {
	key := strings.TrimPrefix(in.CallbackPayload, domain.SnoozePayloadPrefix)
	answer := s.snoozeAnswer(ctx, key, in.Update.MaxUserID)
	if s.answerer == nil {
		return
	}
	if err := s.answerer.AnswerCallback(ctx, in.CallbackID, answer); err != nil {
		s.log.Warn("answer callback failed", slog.Any("error", err))
	}
}

func (s *WebhookService) snoozeAnswer(ctx context.Context, key string, maxUserID int64) string {
	// Без отправителя нельзя проверить, что кнопку нажал адресат напоминания.
	if maxUserID <= 0 || key == "" || s.snoozer == nil {
		return snoozeAnswerNotFound
	}
	outcome, _, err := s.snoozer.SnoozeReminder(ctx, key, maxUserID)
	if err != nil {
		s.log.Warn("snooze via reminders-service failed", slog.Any("error", err))
		return snoozeAnswerRetry
	}
	switch outcome {
	case ports.SnoozeSnoozed:
		return snoozeAnswerDone
	case ports.SnoozeAlreadySnoozed:
		return snoozeAnswerAlready
	case ports.SnoozeTooLate:
		return snoozeAnswerTooLate
	case ports.SnoozeStale:
		return snoozeAnswerStale
	case ports.SnoozeForbidden:
		return snoozeAnswerForeign
	case ports.SnoozeNotFound:
		return snoozeAnswerNotFound
	default:
		return snoozeAnswerRetry
	}
}
