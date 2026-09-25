package httpapi

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// keyedLimiter — ограничитель частоты по ключу (аккаунт, IP).
type keyedLimiter struct {
	mu       sync.Mutex
	limiters map[string]*entry
	rps      rate.Limit
	burst    int
	ttl      time.Duration
}

type entry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func newKeyedLimiter(rps float64, burst int) *keyedLimiter {
	return &keyedLimiter{
		limiters: make(map[string]*entry),
		rps:      rate.Limit(rps),
		burst:    burst,
		ttl:      10 * time.Minute,
	}
}

// Allow сообщает, разрешён ли запрос по ключу.
func (k *keyedLimiter) Allow(key string, now time.Time) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	e, ok := k.limiters[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(k.rps, k.burst)}
		k.limiters[key] = e
	}
	e.lastUsed = now
	if len(k.limiters) > 10000 {
		for id, v := range k.limiters {
			if now.Sub(v.lastUsed) > k.ttl {
				delete(k.limiters, id)
			}
		}
	}
	return e.limiter.AllowN(now, 1)
}

// windowLimiter — счётчик запросов в фиксированном окне одной минуты.
type windowLimiter struct {
	mu     sync.Mutex
	counts map[string]*windowCount
	limit  int
}

type windowCount struct {
	windowStart time.Time
	count       int
}

func newWindowLimiter(limit int) *windowLimiter {
	return &windowLimiter{counts: make(map[string]*windowCount), limit: limit}
}

// Allow сообщает, не превышен ли предел запросов за минуту.
func (w *windowLimiter) Allow(key string, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	window := now.Truncate(time.Minute)
	c, ok := w.counts[key]
	if !ok || c.windowStart.Before(window) {
		c = &windowCount{windowStart: window}
		w.counts[key] = c
	}
	if len(w.counts) > 10000 {
		for id, v := range w.counts {
			if v.windowStart.Before(window) {
				delete(w.counts, id)
			}
		}
	}
	c.count++
	return c.count <= w.limit
}
