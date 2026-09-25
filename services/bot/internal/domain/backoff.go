package domain

import "time"

// Backoff — выдержка перед повтором доставки: U(0; min(Max, Base × 2^(n−1))).
type Backoff struct {
	Base time.Duration
	Max  time.Duration
}

// DefaultBackoff — значения спецификации (spec §8): 5 с, предел 10 мин.
var DefaultBackoff = Backoff{Base: 5 * time.Second, Max: 10 * time.Minute}

// Ceiling возвращает верхнюю границу выдержки перед попыткой n (n ≥ 1).
func (b Backoff) Ceiling(n int) time.Duration {
	c := b.Base
	for i := 1; i < n && c < b.Max; i++ {
		c *= 2
	}
	if c > b.Max {
		c = b.Max
	}
	return c
}

// Delay возвращает выдержку для попытки n; rnd — случайное число из [0; 1).
func (b Backoff) Delay(n int, rnd float64) time.Duration {
	if rnd < 0 {
		rnd = 0
	}
	if rnd >= 1 {
		rnd = 0.999999
	}
	return time.Duration(float64(b.Ceiling(n)) * rnd)
}
