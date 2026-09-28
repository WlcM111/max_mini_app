package domain

import (
	"strings"
	"testing"
	"time"
)

func TestDigestText(t *testing.T) {
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	text := DigestText("Кафе «Дружба»", today, []DigestItem{
		{Title: "Ключ ФН", ValidUntil: today.AddDate(0, 0, -3)},
		{Title: "КЭП", ValidUntil: today.AddDate(0, 0, 5)},
	})
	for _, want := range []string{"Сводка на неделю", "Просрочены:", "«Ключ ФН» — истёк 25.09.2026", "«КЭП» — 03.10.2026, через 5 дней"} {
		if !strings.Contains(text, want) {
			t.Errorf("в сводке нет %q:\n%s", want, text)
		}
	}
}
