package domain

import (
	"fmt"
	"strings"
	"time"
)

// DigestItem — строка недельной сводки.
type DigestItem struct {
	Title      string
	ValidUntil time.Time // дата окончания (полночь UTC)
}

const digestMaxItems = 12

// DigestText собирает недельную сводку: сначала просроченные, затем истекающие в ближайшие 7 дней.
func DigestText(organizationName string, today time.Time, items []DigestItem) string {
	var expired, soon []string
	for _, it := range items {
		days := int(it.ValidUntil.Sub(today).Hours() / 24)
		title := strings.TrimSpace(it.Title)
		date := it.ValidUntil.Format("02.01.2006")
		switch {
		case days < 0:
			expired = append(expired, fmt.Sprintf("• «%s» — истёк %s", title, date))
		case days == 0:
			soon = append(soon, fmt.Sprintf("• «%s» — сегодня, %s", title, date))
		default:
			soon = append(soon, fmt.Sprintf("• «%s» — %s, через %d %s", title, date, days, PluralDays(days)))
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Сводка на неделю. Организация: %s.", strings.TrimSpace(organizationName))
	shown := 0
	write := func(header string, lines []string) {
		if len(lines) == 0 {
			return
		}
		b.WriteString("\n\n" + header)
		for _, line := range lines {
			if shown == digestMaxItems {
				break
			}
			b.WriteString("\n" + line)
			shown++
		}
	}
	write("Просрочены:", expired)
	write("Истекают в ближайшие 7 дней:", soon)
	if rest := len(expired) + len(soon) - shown; rest > 0 {
		fmt.Fprintf(&b, "\n…и ещё %d — откройте приложение.", rest)
	}
	text := b.String()
	if runes := []rune(text); len(runes) > MaxMessageLen {
		text = string(runes[:MaxMessageLen])
	}
	return text
}
