package domain

import "time"

// Коды last_error_code (messaging.proto, GetNotificationStatusResponse).
const (
	CodeMax4xx               = "max_4xx"
	CodeMax401               = "max_401"
	CodeMax429               = "max_429"
	CodeMax5xx               = "max_5xx"
	CodeTimeout              = "timeout"
	CodeNetwork              = "network"
	CodeRecipientUnreachable = "recipient_unreachable"
	CodeRecipientStopped     = "recipient_stopped"
	CodeExpired              = "expired"
)

// SendFailure — неуспешная попытка отправки в терминах предметной области.
// Перевод ответа MAX в SendFailure выполняет адаптер Bot API.
type SendFailure struct {
	Code                 string
	Retryable            bool
	RecipientUnreachable bool
	RetryAfter           time.Duration // пожелание MAX (Retry-After), 0 — нет
}

// Transition — следующее состояние сообщения.
type Transition struct {
	Status        Status
	NextAttemptAt time.Time
	ErrorCode     string
}

// RetryPolicy — правила повторов доставки.
type RetryPolicy struct {
	MaxAttempts int
	Backoff     Backoff
}

// AfterFailure определяет состояние после неуспешной попытки attempt (1..N).
// Повторяемая ошибка ведёт в retry_wait с выдержкой, после исчерпания попыток — в failed.
func (p RetryPolicy) AfterFailure(f SendFailure, attempt int, now time.Time, rnd float64) Transition {
	if !f.Retryable || attempt >= p.MaxAttempts {
		return Transition{Status: StatusFailed, ErrorCode: f.Code}
	}
	delay := p.Backoff.Delay(attempt, rnd)
	if f.RetryAfter > delay {
		delay = f.RetryAfter
	}
	return Transition{Status: StatusRetryWait, NextAttemptAt: now.Add(delay), ErrorCode: f.Code}
}

// PreSendCheck выполняет проверки перед вызовом MAX: просроченное сообщение
// не отправляется, остановившему бота получателю сообщение не отправляется.
func PreSendCheck(notAfter time.Time, recipient RecipientState, now time.Time) (Transition, bool) {
	if now.After(notAfter) {
		return Transition{Status: StatusExpired, ErrorCode: CodeExpired}, true
	}
	if recipient == RecipientStopped {
		return Transition{Status: StatusFailed, ErrorCode: CodeRecipientStopped}, true
	}
	return Transition{}, false
}
