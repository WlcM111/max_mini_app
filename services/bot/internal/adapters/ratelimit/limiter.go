// Package ratelimit ограничивает скорость обращений к MAX Bot API:
// глобальный токен-бакет (F-43) и интервал на одного получателя (F-50).
package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"vovremya/services/bot/internal/ports"
)

// penaltyWindow — срок снижения скорости вдвое после ответа 429 (spec §8).
const penaltyWindow = time.Minute

// sweepInterval — период очистки состояний получателей.
const sweepInterval = time.Minute

// Limiter реализует ports.RateLimiter.
type Limiter struct {
	global   *rate.Limiter
	base     rate.Limit
	interval time.Duration
	now      func() time.Time

	mu           sync.Mutex
	next         map[int64]time.Time
	penaltyUntil time.Time
	lastSweep    time.Time
}

var _ ports.RateLimiter = (*Limiter)(nil)

// New создаёт ограничитель: rps вызовов в секунду и не чаще одного сообщения
// в interval одному получателю.
func New(rps float64, interval time.Duration) *Limiter {
	return NewWithClock(rps, interval, time.Now)
}

// NewWithClock позволяет задать источник времени (для тестов).
func NewWithClock(rps float64, interval time.Duration, now func() time.Time) *Limiter {
	limit := rate.Limit(rps)
	return &Limiter{
		global:   rate.NewLimiter(limit, 1),
		base:     limit,
		interval: interval,
		now:      now,
		next:     make(map[int64]time.Time),
	}
}

// Wait ожидает разрешения отправить сообщение получателю.
func (l *Limiter) Wait(ctx context.Context, recipient int64) error {
	slot := l.reserve(recipient)
	if wait := slot.Sub(l.now()); wait > 0 {
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			l.cancel(recipient, slot)
			return ctx.Err()
		case <-t.C:
		}
	}
	if err := l.global.Wait(ctx); err != nil {
		l.cancel(recipient, slot)
		return err
	}
	return nil
}

// Penalize снижает глобальную скорость вдвое на минуту (ответ MAX 429).
func (l *Limiter) Penalize() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.penaltyUntil = l.now().Add(penaltyWindow)
	l.global.SetLimit(l.base / 2)
}

// Limit возвращает действующую глобальную скорость (для тестов и диагностики).
func (l *Limiter) Limit() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.restore(l.now())
	return float64(l.global.Limit())
}

func (l *Limiter) reserve(recipient int64) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.restore(now)
	slot := l.next[recipient]
	if slot.Before(now) {
		slot = now
	}
	l.next[recipient] = slot.Add(l.interval)
	l.sweep(now)
	return slot
}

// cancel освобождает зарезервированный интервал, если после него не было новых броней.
func (l *Limiter) cancel(recipient int64, slot time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if next, ok := l.next[recipient]; ok && next.Equal(slot.Add(l.interval)) {
		if slot.After(l.now()) {
			delete(l.next, recipient)
			return
		}
		l.next[recipient] = slot
	}
}

func (l *Limiter) restore(now time.Time) {
	if !l.penaltyUntil.IsZero() && now.After(l.penaltyUntil) {
		l.penaltyUntil = time.Time{}
		l.global.SetLimit(l.base)
	}
}

func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < sweepInterval {
		return
	}
	l.lastSweep = now
	horizon := now.Add(-l.interval)
	for id, t := range l.next {
		if t.Before(horizon) {
			delete(l.next, id)
		}
	}
}
