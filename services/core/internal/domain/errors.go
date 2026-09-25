// Package domain — предметная модель core-service: организации, участники,
// документы и периоды, приглашения, справочник и правила доступа.
// Пакет не зависит от HTTP, gRPC, PostgreSQL и SDK MAX.
package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Ошибки предметной области. Соответствие кодам HTTP — handoff core-service §14.
var (
	ErrValidation            = errors.New("ошибка валидации")
	ErrUnauthenticated       = errors.New("нет действующей сессии")
	ErrLaunchInvalid         = errors.New("данные запуска MAX недействительны")
	ErrLaunchExpired         = errors.New("данные запуска MAX просрочены")
	ErrForbidden             = errors.New("недостаточно прав")
	ErrNotFound              = errors.New("объект не найден")
	ErrConflictVersion       = errors.New("версия объекта изменилась")
	ErrConflictIDReused      = errors.New("идентификатор уже использован")
	ErrQuotaExceeded         = errors.New("превышена квота")
	ErrInviteInvalid         = errors.New("приглашение недействительно")
	ErrInviteExpired         = errors.New("срок приглашения истёк")
	ErrAlreadyMember         = errors.New("пользователь уже участник организации")
	ErrLinkGone              = errors.New("ссылка истекла или исчерпана")
	ErrRateLimited           = errors.New("превышен предел частоты запросов")
	ErrDependencyUnavailable = errors.New("зависимый сервис недоступен")
)

// Коды ошибок полей (OpenAPI FieldError.code).
const (
	CodeRequired           = "required"
	CodeTooLong            = "too_long"
	CodeTooShort           = "too_short"
	CodeInvalidFormat      = "invalid_format"
	CodeOutOfRange         = "out_of_range"
	CodeNotUnique          = "not_unique"
	CodeUnknownValue       = "unknown_value"
	CodeInvalidCombination = "invalid_combination"
)

// FieldError описывает одну ошибку поля запроса.
type FieldError struct {
	Field   string
	Code    string
	Message string
}

// ValidationError — набор ошибок полей; разворачивается в ErrValidation.
type ValidationError struct {
	Fields []FieldError
}

// Error возвращает текст ошибки.
func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s: %s", f.Field, f.Message))
	}
	return "ошибка валидации: " + strings.Join(parts, "; ")
}

// Unwrap позволяет сравнивать ошибку с ErrValidation.
func (e *ValidationError) Unwrap() error { return ErrValidation }

// Validator накапливает ошибки полей.
type Validator struct{ fields []FieldError }

// Add добавляет ошибку поля.
func (v *Validator) Add(field, code, message string) {
	v.fields = append(v.fields, FieldError{Field: field, Code: code, Message: message})
}

// Required отмечает отсутствующее обязательное поле.
func (v *Validator) Required(field string) {
	v.Add(field, CodeRequired, "поле обязательно")
}

// Err возвращает накопленную ошибку или nil.
func (v *Validator) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &ValidationError{Fields: v.fields}
}

// HasErrors сообщает, накоплены ли ошибки.
func (v *Validator) HasErrors() bool { return len(v.fields) > 0 }

// ValidationFor создаёт ошибку по одному полю.
func ValidationFor(field, code, message string) error {
	return &ValidationError{Fields: []FieldError{{Field: field, Code: code, Message: message}}}
}
