package app

import (
	"context"
	"log/slog"
	"strings"

	"vovremya/services/bot/internal/domain"
)

// callbackAnswerer — клиент MAX, умеющий отвечать на нажатия callback-кнопок.
type callbackAnswerer interface {
	AnswerCallback(ctx context.Context, callbackID, notification string) error
}

// UseCallbackAnswers подключает ответы на нажатия кнопок, если клиент MAX их поддерживает.
func (s *WebhookService) UseCallbackAnswers(client any) {
	if a, ok := client.(callbackAnswerer); ok {
		s.answerer = a
	}
}

// snooze ставит копию напоминания в очередь на +7 дней и отвечает всплывающим уведомлением.
// Повторная доставка того же события безопасна: ключ копии включает день нажатия.
func (s *WebhookService) snooze(ctx context.Context, in InboundUpdate) {
	key := strings.TrimPrefix(in.CallbackPayload, domain.SnoozePayloadPrefix)
	answer := "Напомню через неделю"
	original, err := s.messages.GetByKey(ctx, key)
	switch {
	case err != nil:
		answer = "Не получилось отложить — откройте документ в приложении"
	case in.Update.MaxUserID != 0 && original.RecipientMaxUserID != in.Update.MaxUserID:
		answer = "Это напоминание адресовано другому пользователю"
	default:
		if _, inserted, insErr := s.messages.Insert(ctx, original.SnoozedCopy(s.clock.Now())); insErr != nil {
			answer = "Не получилось отложить — попробуйте ещё раз"
			s.log.Warn("snooze insert failed", slog.Any("error", insErr))
		} else if !inserted {
			answer = "Уже напомню через неделю"
		}
	}
	if s.answerer == nil {
		return
	}
	if err := s.answerer.AnswerCallback(ctx, in.CallbackID, answer); err != nil {
		s.log.Warn("answer callback failed", slog.Any("error", err))
	}
}
