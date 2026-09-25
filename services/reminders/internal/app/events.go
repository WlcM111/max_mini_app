// Package app содержит сценарии reminders-service: приём изменений из core,
// перепланирование, планировщик отправки, запросы плана и очистку данных.
// Слой зависит только от domain и ports.
package app

import (
	"time"

	"vovremya/services/reminders/internal/domain"
)

// EventType — тип принимаемого изменения. Значения соответствуют
// vovremya.reminders.v1.EventType нормативного контракта.
type EventType string

const (
	EventOrganizationState   EventType = "organization_state"
	EventOrganizationDeleted EventType = "organization_deleted"
	EventMembershipState     EventType = "membership_state"
	EventMembershipRemoved   EventType = "membership_removed"
	EventDocumentState       EventType = "document_state"
	EventDocumentDeleted     EventType = "document_deleted"
	EventAccountDeleted      EventType = "account_deleted"
)

// Типы агрегатов, по которым ведётся версионирование проекций.
const (
	AggregateOrganization = "organization"
	AggregateMembership   = "membership"
	AggregateDocument     = "document"
	AggregateAccount      = "account"
)

// SchemaVersion — поддерживаемая версия схемы payload событий.
const SchemaVersion = 1

// Event — принимаемое изменение предметного агрегата core.
// Полезная нагрузка строго типизирована: произвольный JSON не используется.
type Event struct {
	ID               string
	Type             EventType
	SchemaVersion    uint32
	SourceService    string
	AggregateType    string
	AggregateID      string
	AggregateVersion uint64
	OccurredAt       time.Time
	Snapshot         bool

	Organization *domain.Organization
	Member       *domain.Member
	Document     *domain.Document
	AccountID    string
}

// Outcome — результат обработки события.
type Outcome string

const (
	OutcomeApplied   Outcome = "applied"
	OutcomeDuplicate Outcome = "duplicate"
	OutcomeStale     Outcome = "stale"
	OutcomeRejected  Outcome = "rejected"
)

// Result — исход обработки одного события.
type Result struct {
	EventID        string
	Outcome        Outcome
	AppliedVersion uint64
	Message        string
}

// Validate проверяет конверт события и полезную нагрузку.
// Ошибка означает REJECTED: повтор такого события не изменит результат.
func (e Event) Validate() error {
	if !domain.IsUUID(e.ID) {
		return domain.ValidationError{Field: "event_id", Reason: "ожидается UUID v4"}
	}
	if e.SchemaVersion != SchemaVersion {
		return domain.ValidationError{Field: "schema_version", Reason: "поддерживается версия 1"}
	}
	if e.SourceService == "" {
		return domain.ValidationError{Field: "source_service", Reason: "обязательное поле"}
	}
	if e.OccurredAt.IsZero() {
		return domain.ValidationError{Field: "occurred_at", Reason: "обязательное поле"}
	}
	if e.AggregateVersion == 0 {
		return domain.ValidationError{Field: "aggregate_version", Reason: "версия агрегата начинается с 1"}
	}
	switch e.Type {
	case EventOrganizationState:
		if e.AggregateType != AggregateOrganization || e.Organization == nil {
			return domain.ValidationError{Field: "organization_state", Reason: "не заполнено состояние организации"}
		}
		if e.Organization.ID != e.AggregateID {
			return domain.ValidationError{Field: "aggregate_id", Reason: "не совпадает с организацией"}
		}
		return e.Organization.Validate()
	case EventOrganizationDeleted:
		if e.AggregateType != AggregateOrganization || !domain.IsUUID(e.AggregateID) {
			return domain.ValidationError{Field: "aggregate_id", Reason: "ожидается UUID организации"}
		}
		return nil
	case EventMembershipState:
		if e.AggregateType != AggregateMembership || e.Member == nil {
			return domain.ValidationError{Field: "membership_state", Reason: "не заполнено состояние участия"}
		}
		if e.AggregateID != MembershipAggregateID(e.Member.OrganizationID, e.Member.AccountID) {
			return domain.ValidationError{Field: "aggregate_id", Reason: "ожидается <organization_id>:<account_id>"}
		}
		return e.Member.Validate()
	case EventMembershipRemoved:
		if e.AggregateType != AggregateMembership {
			return domain.ValidationError{Field: "aggregate_type", Reason: "ожидается membership"}
		}
		if _, _, ok := SplitMembershipAggregateID(e.AggregateID); !ok {
			return domain.ValidationError{Field: "aggregate_id", Reason: "ожидается <organization_id>:<account_id>"}
		}
		return nil
	case EventDocumentState:
		if e.AggregateType != AggregateDocument || e.Document == nil {
			return domain.ValidationError{Field: "document_state", Reason: "не заполнено состояние документа"}
		}
		if e.Document.ID != e.AggregateID {
			return domain.ValidationError{Field: "aggregate_id", Reason: "не совпадает с документом"}
		}
		return e.Document.Validate()
	case EventDocumentDeleted:
		if e.AggregateType != AggregateDocument || !domain.IsUUID(e.AggregateID) {
			return domain.ValidationError{Field: "aggregate_id", Reason: "ожидается UUID документа"}
		}
		return nil
	case EventAccountDeleted:
		if e.AggregateType != AggregateAccount || !domain.IsUUID(e.AggregateID) {
			return domain.ValidationError{Field: "aggregate_id", Reason: "ожидается UUID аккаунта"}
		}
		return nil
	default:
		return domain.ValidationError{Field: "type", Reason: "неизвестный тип события"}
	}
}

// MembershipAggregateID формирует идентификатор агрегата участия.
func MembershipAggregateID(orgID, accountID string) string { return orgID + ":" + accountID }

// SplitMembershipAggregateID разбирает идентификатор агрегата участия.
func SplitMembershipAggregateID(id string) (orgID, accountID string, ok bool) {
	for i := 0; i < len(id); i++ {
		if id[i] == ':' {
			orgID, accountID = id[:i], id[i+1:]
			return orgID, accountID, domain.IsUUID(orgID) && domain.IsUUID(accountID)
		}
	}
	return "", "", false
}
