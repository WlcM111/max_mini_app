// Package clock — системные часы и источник случайных значений core-service.
package clock

import (
	"crypto/rand"
	"encoding/base64"
	mrand "math/rand/v2"
	"time"

	"github.com/google/uuid"

	"vovremya/services/core/internal/ports"
)

// System — часы операционной системы в UTC.
type System struct{}

var _ ports.Clock = System{}

// Now возвращает текущее время в UTC.
func (System) Now() time.Time { return time.Now().UTC() }

// Random — источник случайных значений.
type Random struct{}

var _ ports.Random = Random{}

// Float64 возвращает случайное число из [0; 1).
func (Random) Float64() float64 { return mrand.Float64() }

// Token возвращает непрозрачный токен: 32 случайных байта в base64url.
func (Random) Token() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// UUID возвращает новый UUID версии 4.
func (Random) UUID() string { return uuid.NewString() }
