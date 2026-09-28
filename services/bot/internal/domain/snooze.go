package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// «Напомнить через неделю»: callback-кнопка под напоминанием, обрабатывает сам bot-service.
const (
	SnoozeButtonText    = "Напомнить через неделю"
	SnoozePayloadPrefix = "snooze:"
	SnoozeDelay         = 7 * 24 * time.Hour
	snoozeTextPrefix    = "Напоминаю ещё раз. "
	maxIdempotencyKey   = 200
)

// WithSnoozeButton — сообщение к отправке: напоминанию добавляется кнопка отложенного повтора.
// Недельная сводка (ключ digest:) кнопку не получает.
func (n Notification) WithSnoozeButton() Message {
	msg := n.Message
	if msg.Kind != KindReminder || len(msg.Buttons) >= MaxButtons || strings.HasPrefix(n.IdempotencyKey, "digest:") {
		return msg
	}
	buttons := make([]Button, 0, len(msg.Buttons)+1)
	buttons = append(buttons, msg.Buttons...)
	msg.Buttons = append(buttons, Button{Text: SnoozeButtonText, Action: ActionCallback, Payload: SnoozePayloadPrefix + n.IdempotencyKey})
	return msg
}

// SnoozedCopy — копия напоминания к отправке через неделю. Ключ включает день нажатия:
// повторное нажатие в тот же день не создаёт второй копии.
func (n Notification) SnoozedCopy(now time.Time) Notification {
	base := n.IdempotencyKey
	if i := strings.Index(base, ":snz:"); i >= 0 {
		base = base[:i]
	}
	key := fmt.Sprintf("%s:snz:%d", base, now.Unix()/86400)
	if len(key) > maxIdempotencyKey {
		key = key[len(key)-maxIdempotencyKey:]
	}
	msg := n.Message
	msg.Buttons = append([]Button(nil), n.Buttons...)
	if !strings.HasPrefix(msg.Text, snoozeTextPrefix) {
		msg.Text = snoozeTextPrefix + msg.Text
	}
	if utf8.RuneCountInString(msg.Text) > MaxTextRunes {
		msg.Text = string([]rune(msg.Text)[:MaxTextRunes])
	}
	return Notification{
		IdempotencyKey: key,
		RequestHash:    n.RequestHash,
		Message:        msg,
		NextAttemptAt:  now.Add(SnoozeDelay),
		NotAfter:       now.Add(SnoozeDelay + 24*time.Hour),
	}
}
