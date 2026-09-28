package domain

import (
	"fmt"
	"strings"
)

// Тексты сообщений бота зафиксированы исходным ТЗ
// (docs/max/max-integration-spec.md §15). Формирование текста принадлежит
// reminders-service: bot-service не знает предметных правил сроков.
const (
	// ButtonOpenDocument — надпись кнопки напоминания.
	ButtonOpenDocument = "Открыть документ"
	ButtonRenewed      = "Продлил"
	ButtonOpenApp      = "Открыть «Вовремя»"
	// MaxMessageLen — ограничение MAX на длину текста сообщения.
	MaxMessageLen = 4000
)

// PluralDays возвращает форму слова «день» для числа n.
func PluralDays(n int) string {
	if n < 0 {
		n = -n
	}
	if n%100 >= 11 && n%100 <= 14 {
		return "дней"
	}
	switch n % 10 {
	case 1:
		return "день"
	case 2, 3, 4:
		return "дня"
	}
	return "дней"
}

// ReminderText формирует текст напоминания.
// daysBefore = 0 — срок заканчивается сегодня.
func ReminderText(documentTitle, organizationName string, daysBefore int, validUntil Date) string {
	title := strings.TrimSpace(documentTitle)
	org := strings.TrimSpace(organizationName)
	var text string
	if daysBefore == 0 {
		text = fmt.Sprintf("Сегодня заканчивается срок: «%s». Организация: %s. Срок до %s.",
			title, org, validUntil.Format())
	} else {
		text = fmt.Sprintf("Через %d %s заканчивается срок: «%s». Организация: %s. Срок до %s.",
			daysBefore, PluralDays(daysBefore), title, org, validUntil.Format())
	}
	if len([]rune(text)) > MaxMessageLen {
		runes := []rune(text)
		text = string(runes[:MaxMessageLen])
	}
	return text
}

// DeepLinkPayload возвращает payload диплинка мини-приложения для документа.
// Грамматика зафиксирована спецификацией MAX-интеграции §5: doc_<uuid>.
func DeepLinkPayload(documentID string) string { return "doc_" + documentID }

// RenewLinkPayload — диплинк экрана продления (кнопка «Продлил»).
func RenewLinkPayload(documentID string) string { return "renew_" + documentID }

// OrganizationLinkPayload — диплинк главной организации (кнопка в сводке).
func OrganizationLinkPayload(organizationID string) string { return "org_" + organizationID }
