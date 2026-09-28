package domain

import (
	"strings"
	"testing"
	"time"
)

func TestSnooze(t *testing.T) {
	n := Notification{IdempotencyKey: "rem:abc:30", Message: Message{Kind: KindReminder, Text: "Через 30 дней…",
		Buttons: []Button{{Text: "Открыть документ", Action: ActionOpenApp, Payload: "doc_x"}}}}
	msg := n.WithSnoozeButton()
	if len(msg.Buttons) != 2 || msg.Buttons[1].Action != ActionCallback || msg.Buttons[1].Payload != "snooze:rem:abc:30" {
		t.Fatalf("кнопка не добавлена: %+v", msg.Buttons)
	}
	if len(n.Buttons) != 1 {
		t.Fatal("исходные кнопки изменены")
	}
	now := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	c := n.SnoozedCopy(now)
	if !c.NextAttemptAt.Equal(now.Add(SnoozeDelay)) || !strings.HasPrefix(c.Text, "Напоминаю ещё раз.") {
		t.Fatalf("копия: %+v", c)
	}
	if again := c.SnoozedCopy(now.Add(8 * 24 * time.Hour)); strings.Count(again.IdempotencyKey, ":snz:") != 1 {
		t.Fatalf("ключ растёт: %s", again.IdempotencyKey)
	}
	if d := (Notification{IdempotencyKey: "digest:o:a:202640", Message: Message{Kind: KindReminder}}).WithSnoozeButton(); len(d.Buttons) != 0 {
		t.Fatal("сводке кнопка не нужна")
	}
}
