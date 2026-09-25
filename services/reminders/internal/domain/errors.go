// Package domain содержит предметную модель напоминаний: проекции исходных
// данных core, правила построения плана, состояния напоминаний и тексты.
// Пакет не зависит от транспорта, SQL и инфраструктуры (только стандартная библиотека).
package domain

import "errors"

var (
	// ErrValidation — входные данные нарушают инварианты предметной модели.
	ErrValidation = errors.New("validation failed")
	// ErrNotFound — запрошенный объект отсутствует в проекции или плане.
	ErrNotFound = errors.New("not found")
	// ErrStaleVersion — версия агрегата не новее уже применённой.
	ErrStaleVersion = errors.New("stale aggregate version")
)

// ValidationError описывает нарушение инварианта с указанием поля.
type ValidationError struct {
	Field  string
	Reason string
}

func (e ValidationError) Error() string { return e.Field + ": " + e.Reason }

// Unwrap связывает ошибку с ErrValidation для errors.Is.
func (e ValidationError) Unwrap() error { return ErrValidation }

func invalid(field, reason string) error { return ValidationError{Field: field, Reason: reason} }
