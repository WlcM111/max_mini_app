package domain

import (
	"testing"
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
	// Повтор из плана reminders-service снова можно отложить: кнопка несёт его полный ключ.
	snoozed := Notification{IdempotencyKey: "rem:p:a:23:snz:20000", Message: Message{Kind: KindReminder}}
	if b := snoozed.WithSnoozeButton().Buttons; len(b) != 1 || b[0].Payload != SnoozePayloadPrefix+"rem:p:a:23:snz:20000" {
		t.Fatalf("кнопка повтора: %+v", b)
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
