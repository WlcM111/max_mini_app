package domain

import (
	"fmt"
	"time"
)

// Status — состояние напоминания в плане.
type Status string

const (
	// StatusPlanned — запланировано, ожидает наступления due_at.
	StatusPlanned Status = "planned"
	// StatusHandedOff — принято bot-service в очередь доставки.
	StatusHandedOff Status = "handed_off"
	// StatusCancelled — отменено перепланированием (изменение исходных данных).
	StatusCancelled Status = "cancelled"
	// StatusSkipped — не будет отправлено: устарело или отвергнуто получателем контракта.
	StatusSkipped Status = "skipped"
)

// Valid сообщает, что значение статуса допустимо.
func (s Status) Valid() bool {
	switch s {
	case StatusPlanned, StatusHandedOff, StatusCancelled, StatusSkipped:
		return true
	}
	return false
}

// CanTransitionTo проверяет допустимость перехода состояния.
// Терминальные состояния не меняются; повторное перепланирование возвращает
// отменённое напоминание в planned (ключ тот же, момент мог измениться).
func (s Status) CanTransitionTo(next Status) bool {
	switch s {
	case StatusPlanned:
		return next == StatusHandedOff || next == StatusCancelled || next == StatusSkipped || next == StatusPlanned
	case StatusCancelled:
		return next == StatusPlanned
	case StatusHandedOff, StatusSkipped:
		return false
	}
	return false
}

// Reminder — напоминание плана с состоянием доставки.
type Reminder struct {
	ID                int64
	DocumentID        string
	OrganizationID    string
	Key               PlanKey
	DueAt             time.Time
	Status            Status
	Attempts          int
	NextAttemptAt     time.Time
	LastErrorCode     string
	BotNotificationID string
	HandedOffAt       *time.Time
}

// IdempotencyKey возвращает ключ идемпотентности передачи в bot-service.
// Формат зафиксирован исходным ТЗ: rem:<period_uuid>:<account_uuid>:<days>.
func (k PlanKey) IdempotencyKey() string {
	return fmt.Sprintf("rem:%s:%s:%d", k.PeriodID, k.AccountID, k.DaysBefore)
}

// Expired сообщает, что момент напоминания просрочен более чем на grace
// и отправлять его уже не нужно.
func (r Reminder) Expired(now time.Time, grace time.Duration) bool {
	return now.After(r.DueAt.Add(grace))
}

// NotAfter возвращает момент, после которого сообщение не должно отправляться.
// Передаётся в bot-service: просроченные сообщения там переходят в expired.
func (r Reminder) NotAfter(grace time.Duration) time.Time { return r.DueAt.Add(grace) }
