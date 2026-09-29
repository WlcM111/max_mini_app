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

func TestSnoozeButtonNeedsMoreThanWeek(t *testing.T) {
	cases := []struct {
		key  string
		want int
	}{
		{"rem:p:a:30", 1},
		{"rem:p:a:8", 1},
		{"rem:p:a:7", 0},
		{"rem:p:a:1", 0},
		{"rem:p:a:0", 0},
		{"rem:p:a:30:snz:20000", 1},
		{"rem:p:a:3:snz:20000", 0},
		{"delivery-ok-0001", 1},
	}
	for _, tc := range cases {
		n := Notification{IdempotencyKey: tc.key, Message: Message{Kind: KindReminder}}
		if got := len(n.WithSnoozeButton().Buttons); got != tc.want {
			t.Errorf("%s: кнопок %d, ожидалось %d", tc.key, got, tc.want)
		}
	}
}

func TestSnoozedCopyText(t *testing.T) {
	now := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	n := Notification{IdempotencyKey: "rem:p:a:30", Message: Message{Kind: KindReminder,
		Text: "Через 30 дней заканчивается срок: «Лицензия». Организация: Кафе. Срок до 28.10.2026."}}
	want := "Напоминаю ещё раз. Заканчивается срок: «Лицензия». Организация: Кафе. Срок до 28.10.2026."
	c := n.SnoozedCopy(now)
	if c.Text != want {
		t.Fatalf("текст копии: %q", c.Text)
	}
	if again := c.SnoozedCopy(now.Add(SnoozeDelay)); again.Text != want {
		t.Fatalf("текст второй копии: %q", again.Text)
	}
	if !c.CreatedAt.Equal(now) {
		t.Fatalf("время создания копии: %v", c.CreatedAt)
	}
}

func TestSnoozeAllowed(t *testing.T) {
	now := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	until := func(date string) Notification {
		return Notification{Message: Message{Kind: KindReminder,
			Text: "Через 30 дней заканчивается срок: «Лицензия». Организация: Кафе. Срок до " + date + "."}}
	}
	if !until("28.10.2026").SnoozeAllowed(now) || !until("07.10.2026").SnoozeAllowed(now) {
		t.Error("копия успевает до дня окончания срока — перенос разрешён")
	}
	if until("06.10.2026").SnoozeAllowed(now) || until("01.10.2026").SnoozeAllowed(now) {
		t.Error("копия пришла бы в день окончания срока или позже — перенос запрещён")
	}
	if !(Notification{Message: Message{Text: "Через 30 дней…"}}).SnoozeAllowed(now) {
		t.Error("текст без даты перенос не ограничивает")
	}
}
