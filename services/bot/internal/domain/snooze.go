package domain

import (
	"strconv"
	"strings"
	"time"
)

// «Напомнить через неделю»: callback-кнопка под напоминанием. Нажатие bot-service передаёт
// в reminders-service, который ставит повтор в план (ADR-036).
const (
	SnoozeButtonText    = "Напомнить через неделю"
	SnoozePayloadPrefix = "snooze:"
	SnoozeDelay         = 7 * 24 * time.Hour
)

// snoozeDays — при отступе не больше недели копия пришла бы в день окончания срока или позже.
const snoozeDays = int(SnoozeDelay / (24 * time.Hour))

// WithSnoozeButton — сообщение к отправке: напоминанию добавляется кнопка отложенного повтора.
// Недельная сводка (ключ digest:) и напоминание за неделю и меньше до срока кнопку не получают.
func (n Notification) WithSnoozeButton() Message {
	msg := n.Message
	if msg.Kind != KindReminder || len(msg.Buttons) >= MaxButtons || strings.HasPrefix(n.IdempotencyKey, "digest:") {
		return msg
	}
	if days, ok := reminderDays(n.IdempotencyKey); ok && days <= snoozeDays {
		return msg
	}
	buttons := make([]Button, 0, len(msg.Buttons)+1)
	buttons = append(buttons, msg.Buttons...)
	msg.Buttons = append(buttons, Button{Text: SnoozeButtonText, Action: ActionCallback, Payload: SnoozePayloadPrefix + n.IdempotencyKey})
	return msg
}

// reminderDays возвращает отступ из ключа rem:<период>:<получатель>:<дни>[:snz:<день>].
func reminderDays(key string) (int, bool) {
	if i := strings.Index(key, ":snz:"); i >= 0 {
		key = key[:i]
	}
	if !strings.HasPrefix(key, "rem:") {
		return 0, false
	}
	days, err := strconv.Atoi(key[strings.LastIndex(key, ":")+1:])
	return days, err == nil
}
