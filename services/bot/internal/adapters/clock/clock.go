// Package clock — системные часы и источник случайных чисел для bot-service.
package clock

import (
	"math/rand/v2"
	"time"

	"vovremya/services/bot/internal/ports"
)

// System — часы операционной системы в UTC.
type System struct{}

var _ ports.Clock = System{}

// Now возвращает текущее время в UTC.
func (System) Now() time.Time { return time.Now().UTC() }

// Random — источник случайных чисел для выдержки повторов.
type Random struct{}

var _ ports.Random = Random{}

// Float64 возвращает случайное число из [0; 1).
func (Random) Float64() float64 { return rand.Float64() }
