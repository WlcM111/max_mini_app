// Package domain — предметная модель bot-service: сообщения очереди доставки,
// их состояния и повторы, состояния диалогов получателей, реакция бота на
// события MAX и собственные тексты бота. Пакет зависит только от стандартной
// библиотеки: формат JSON MAX, gRPC и SQL находятся в адаптерах.
package domain

import "errors"

var (
	// ErrValidation — нарушение контракта во входных данных.
	ErrValidation = errors.New("validation failed")
	// ErrNotFound — запрошенный объект отсутствует.
	ErrNotFound = errors.New("not found")
	// ErrIdempotencyConflict — ключ идемпотентности уже использован с другим содержимым.
	ErrIdempotencyConflict = errors.New("idempotency key reused with different content")
	// ErrQueueFull — глубина очереди превысила допустимый предел (backpressure).
	ErrQueueFull = errors.New("outbound queue limit exceeded")
	// ErrProfileUnavailable — профиль бота ещё не загружен из MAX.
	ErrProfileUnavailable = errors.New("bot profile is not loaded")
)

// ValidationError описывает нарушенное поле контракта.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Reason }

// Unwrap позволяет сопоставлять ошибку с ErrValidation.
func (e *ValidationError) Unwrap() error { return ErrValidation }

func invalid(field, reason string) error { return &ValidationError{Field: field, Reason: reason} }
