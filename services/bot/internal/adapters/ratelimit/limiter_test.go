package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"vovremya/services/bot/internal/adapters/ratelimit"
)

// TestPerRecipientInterval — не чаще одного сообщения в интервал на получателя (F-50).
func TestPerRecipientInterval(t *testing.T) {
	l := ratelimit.New(1000, 120*time.Millisecond)
	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := l.Wait(ctx, 42); err != nil {
			t.Fatalf("ожидание %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 240*time.Millisecond*9/10 {
		t.Errorf("три сообщения одному получателю заняли %s, ожидалось не меньше 240ms", elapsed)
	}
	// Другой получатель не ждёт очереди первого.
	start = time.Now()
	if err := l.Wait(ctx, 43); err != nil {
		t.Fatalf("другой получатель: %v", err)
	}
	if d := time.Since(start); d > 60*time.Millisecond {
		t.Errorf("другой получатель ждал %s", d)
	}
}

// TestGlobalRate — глобальная скорость ограничивает суммарный поток (F-43).
func TestGlobalRate(t *testing.T) {
	l := ratelimit.New(20, time.Millisecond)
	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 11; i++ {
		if err := l.Wait(ctx, int64(i)); err != nil {
			t.Fatalf("ожидание %d: %v", i, err)
		}
	}
	// 20 rps: одиннадцатая отправка не раньше 500 мс от первой (допуск 10 %).
	if d := time.Since(start); d < 450*time.Millisecond {
		t.Errorf("11 отправок при 20 rps заняли %s, ожидалось не меньше 450ms", d)
	}
}

// TestPenalizeHalvesRate — ответ 429 снижает скорость вдвое (spec §8).
func TestPenalizeHalvesRate(t *testing.T) {
	now := time.Now()
	l := ratelimit.NewWithClock(20, time.Millisecond, func() time.Time { return now })
	if got := l.Limit(); got != 20 {
		t.Fatalf("исходная скорость: %v", got)
	}
	l.Penalize()
	if got := l.Limit(); got != 10 {
		t.Errorf("после 429 ожидалось 10 rps, получено %v", got)
	}
	now = now.Add(61 * time.Second)
	if got := l.Limit(); got != 20 {
		t.Errorf("через минуту скорость должна восстановиться, получено %v", got)
	}
}

// TestWaitRespectsDeadline — истечение аренды прекращает ожидание.
func TestWaitRespectsDeadline(t *testing.T) {
	l := ratelimit.New(1000, 500*time.Millisecond)
	ctx := context.Background()
	if err := l.Wait(ctx, 7); err != nil {
		t.Fatalf("первое ожидание: %v", err)
	}
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := l.Wait(short, 7); err == nil {
		t.Fatal("ожидание должно прерваться по сроку контекста")
	}
	// Отменённая бронь освобождает интервал: следующий вызов ждёт не дольше интервала.
	start := time.Now()
	if err := l.Wait(ctx, 7); err != nil {
		t.Fatalf("повторное ожидание: %v", err)
	}
	if d := time.Since(start); d > 600*time.Millisecond {
		t.Errorf("повторное ожидание заняло %s", d)
	}
}
