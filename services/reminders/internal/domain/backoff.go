package domain

import (
	"math"
	"math/rand/v2"
	"time"
)

// BackoffParams — параметры повторов передачи напоминания в bot-service.
type BackoffParams struct {
	Base   time.Duration // базовая задержка первой повторной попытки
	Max    time.Duration // верхняя граница задержки
	Jitter time.Duration // верхняя граница случайной добавки
}

// DefaultBackoff — значения исходного ТЗ: min(5 мин, 10 с × 2^attempts) + U(0; 5 с).
var DefaultBackoff = BackoffParams{Base: 10 * time.Second, Max: 5 * time.Minute, Jitter: 5 * time.Second}

// Backoff возвращает задержку перед попыткой номер attempts (0 — первая неудача).
// Детерминированная часть ограничена Max, случайная добавка снижает синхронизацию
// повторов между напоминаниями одного пакета.
func (p BackoffParams) Backoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	exp := float64(p.Base) * math.Pow(2, float64(attempts))
	d := time.Duration(math.Min(exp, float64(p.Max)))
	if p.Jitter > 0 {
		d += time.Duration(rand.Int64N(int64(p.Jitter)))
	}
	return d
}

// MaxDelay возвращает верхнюю границу задержки с учётом случайной добавки.
func (p BackoffParams) MaxDelay() time.Duration { return p.Max + p.Jitter }
