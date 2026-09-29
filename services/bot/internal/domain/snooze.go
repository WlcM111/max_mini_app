package domain

import (
	"fmt"
	"strconv"
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

// SnoozeAllowed сообщает, придёт ли копия раньше дня окончания срока из текста напоминания
// («Срок до ДД.ММ.ГГГГ.»). Текст без даты перенос не ограничивает.
func (n Notification) SnoozeAllowed(now time.Time) bool {
	until, ok := expiryFromText(n.Text)
	if !ok {
		return true
	}
	return now.Add(SnoozeDelay + 24*time.Hour).Before(until)
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
	msg.Text = snoozeTextPrefix + withoutRelativeTerm(strings.TrimPrefix(msg.Text, snoozeTextPrefix))
	if utf8.RuneCountInString(msg.Text) > MaxTextRunes {
		msg.Text = string([]rune(msg.Text)[:MaxTextRunes])
	}
	return Notification{
		IdempotencyKey: key,
		RequestHash:    n.RequestHash,
		Message:        msg,
		NextAttemptAt:  now.Add(SnoozeDelay),
		NotAfter:       now.Add(SnoozeDelay + 24*time.Hour),
		CreatedAt:      now,
	}
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

// expiryFromText находит дату «Срок до ДД.ММ.ГГГГ» в тексте напоминания reminders-service.
func expiryFromText(text string) (time.Time, bool) {
	const marker, layout = "Срок до ", "02.01.2006"
	i := strings.LastIndex(text, marker)
	if i < 0 || len(text) < i+len(marker)+len(layout) {
		return time.Time{}, false
	}
	t, err := time.Parse(layout, text[i+len(marker):i+len(marker)+len(layout)])
	return t, err == nil
}

// withoutRelativeTerm убирает относительный срок («Через N дней», «Сегодня»): к приходу копии
// он устареет, а дата окончания остаётся в тексте.
func withoutRelativeTerm(text string) string {
	const phrase = " заканчивается срок: "
	i := strings.Index(text, phrase)
	if i < 0 || !(strings.HasPrefix(text, "Через ") || strings.HasPrefix(text, "Сегодня")) {
		return text
	}
	return "Заканчивается срок: " + text[i+len(phrase):]
}
