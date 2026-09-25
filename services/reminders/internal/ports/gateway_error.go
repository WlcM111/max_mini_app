package ports

import "fmt"

// GatewayError — ошибка исходящего вызова соседнего сервиса.
// Retryable различает временную недоступность и окончательный отказ:
// от этого зависит, будет ли напоминание повторено или помечено как пропущенное.
type GatewayError struct {
	Code      string
	Retryable bool
	Err       error
}

func (e *GatewayError) Error() string {
	return fmt.Sprintf("gateway error %s (retryable=%t): %v", e.Code, e.Retryable, e.Err)
}

// Unwrap возвращает исходную ошибку транспорта.
func (e *GatewayError) Unwrap() error { return e.Err }
