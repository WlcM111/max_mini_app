package domain

import "time"

// Типы событий для reminders-service (vovremya.reminders.v1.EventType).
const (
	EventOrganizationState   = "organization_state"
	EventOrganizationDeleted = "organization_deleted"
	EventMembershipState     = "membership_state"
	EventMembershipRemoved   = "membership_removed"
	EventDocumentState       = "document_state"
	EventDocumentDeleted     = "document_deleted"
	EventAccountDeleted      = "account_deleted"
)

// Типы агрегатов (vovremya.reminders.v1.AggregateType).
const (
	AggregateOrganization = "organization"
	AggregateMembership   = "membership"
	AggregateDocument     = "document"
	AggregateAccount      = "account"
)

// Состояния строки outbox.
const (
	OutboxPending = "pending"
	OutboxSent    = "sent"
	OutboxFailed  = "failed"
)

// OutboxEvent — событие изменения, доставляемое reminders-service.
// Пишется в одной транзакции с изменением предметных данных.
type OutboxEvent struct {
	ID               int64
	EventID          string
	EventType        string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	Payload          []byte // JSON по схеме нормативного .proto
	Snapshot         bool
	Status           string
	Attempts         int
	NextAttemptAt    time.Time
	LastErrorCode    string
	CreatedAt        time.Time
}

// OrganizationStatePayload — vovremya.reminders.v1.OrganizationState.
type OrganizationStatePayload struct {
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Timezone       string `json:"timezone"`
}

// MembershipStatePayload — vovremya.reminders.v1.MembershipState.
type MembershipStatePayload struct {
	OrganizationID     string `json:"organization_id"`
	AccountID          string `json:"account_id"`
	AccountKind        string `json:"account_kind"` // max | review
	MaxUserID          int64  `json:"max_user_id"`
	NotifyEnabled      bool   `json:"notify_enabled"`
	NotifyLocalMinutes int    `json:"notify_local_minutes"`
}

// DocumentPeriodPayload — vovremya.reminders.v1.DocumentPeriod.
type DocumentPeriodPayload struct {
	PeriodID   string `json:"period_id"`
	ValidFrom  string `json:"valid_from"`
	ValidUntil string `json:"valid_until"`
}

// DocumentStatePayload — vovremya.reminders.v1.DocumentState.
type DocumentStatePayload struct {
	DocumentID           string                `json:"document_id"`
	OrganizationID       string                `json:"organization_id"`
	Title                string                `json:"title"`
	CurrentPeriod        DocumentPeriodPayload `json:"current_period"`
	Offsets              []int                 `json:"reminder_offsets_days"`
	ResponsibleAccountID string                `json:"responsible_account_id,omitempty"`
}

// OrganizationDeletedPayload — vovremya.reminders.v1.OrganizationDeleted.
type OrganizationDeletedPayload struct {
	OrganizationID string `json:"organization_id"`
}

// MembershipRemovedPayload — vovremya.reminders.v1.MembershipRemoved.
type MembershipRemovedPayload struct {
	OrganizationID string `json:"organization_id"`
	AccountID      string `json:"account_id"`
}

// DocumentDeletedPayload — vovremya.reminders.v1.DocumentDeleted.
type DocumentDeletedPayload struct {
	DocumentID     string `json:"document_id"`
	OrganizationID string `json:"organization_id"`
}

// AccountDeletedPayload — vovremya.reminders.v1.AccountDeleted.
type AccountDeletedPayload struct {
	AccountID string `json:"account_id"`
}

// MembershipAggregateID собирает идентификатор агрегата участия.
func MembershipAggregateID(orgPublicID, accountPublicID string) string {
	return orgPublicID + ":" + accountPublicID
}

// RemindersState — состояние плана напоминаний для ответа публичного API.
type RemindersState string

// Значения состояния плана (OpenAPI 1.1.0).
const (
	RemindersActual      RemindersState = "actual"
	RemindersPending     RemindersState = "pending"
	RemindersUnavailable RemindersState = "unavailable"
)

// OutboxBackoff вычисляет момент следующей попытки доставки события:
// min(5 мин, 10 с × 2^attempts) плюс случайная добавка до 5 с.
func OutboxBackoff(now time.Time, attempts int, jitter float64) time.Time {
	delay := 10 * time.Second
	for i := 0; i < attempts && delay < 5*time.Minute; i++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	extra := time.Duration(jitter * float64(5*time.Second))
	return now.Add(delay + extra)
}
