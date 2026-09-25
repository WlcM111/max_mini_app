package domain

import (
	"strings"
	"time"
)

// AccountKind — тип аккаунта получателя. Сообщения бота получают только аккаунты MAX.
type AccountKind string

const (
	AccountKindMax    AccountKind = "max"
	AccountKindReview AccountKind = "review"
)

// Ограничения проекций повторяют ограничения соответствующих полей core
// (docs/database/data-model.md) и проверяются до записи в БД.
const (
	MaxOrganizationNameLen = 100
	MaxDocumentTitleLen    = 200
	MaxTimezoneLen         = 64
	MaxOffsetDays          = 365
	MaxOffsetsPerDocument  = 5
	MinutesPerDay          = 24 * 60
)

// Organization — проекция организации core: нужна часовой пояс и название для текста.
type Organization struct {
	ID       string
	Name     string
	Timezone string
	Version  uint64
	Deleted  bool
}

// Validate проверяет инварианты проекции организации.
func (o Organization) Validate() error {
	if !IsUUID(o.ID) {
		return invalid("organization_id", "ожидается UUID v4")
	}
	name := strings.TrimSpace(o.Name)
	if name == "" || len([]rune(name)) > MaxOrganizationNameLen {
		return invalid("name", "длина 1..100 символов")
	}
	if len(o.Timezone) == 0 || len(o.Timezone) > MaxTimezoneLen {
		return invalid("timezone", "длина 1..64 символа")
	}
	if _, err := time.LoadLocation(o.Timezone); err != nil {
		return invalid("timezone", "неизвестный часовой пояс IANA")
	}
	return nil
}

// Location возвращает часовой пояс организации.
func (o Organization) Location() (*time.Location, error) {
	loc, err := time.LoadLocation(o.Timezone)
	if err != nil {
		return nil, invalid("timezone", "неизвестный часовой пояс IANA")
	}
	return loc, nil
}

// Member — проекция участия: получатель напоминаний и его настройки.
type Member struct {
	OrganizationID string
	AccountID      string
	Kind           AccountKind
	MaxUserID      int64
	NotifyEnabled  bool
	// NotifyLocalMinutes — время напоминаний в минутах от полуночи в поясе организации.
	NotifyLocalMinutes int
	Version            uint64
	Removed            bool
}

// Validate проверяет инварианты проекции участия.
func (m Member) Validate() error {
	if !IsUUID(m.OrganizationID) {
		return invalid("organization_id", "ожидается UUID v4")
	}
	if !IsUUID(m.AccountID) {
		return invalid("account_id", "ожидается UUID v4")
	}
	switch m.Kind {
	case AccountKindMax:
		if m.MaxUserID <= 0 {
			return invalid("max_user_id", "для аккаунта MAX требуется положительный идентификатор")
		}
	case AccountKindReview:
		if m.MaxUserID != 0 {
			return invalid("max_user_id", "служебный аккаунт не имеет идентификатора MAX")
		}
	default:
		return invalid("account_kind", "допустимо max или review")
	}
	if m.NotifyLocalMinutes < 0 || m.NotifyLocalMinutes >= MinutesPerDay {
		return invalid("notify_local_minutes", "допустимо 0..1439")
	}
	return nil
}

// Receives сообщает, может ли участник получить напоминание.
func (m Member) Receives() bool {
	return !m.Removed && m.NotifyEnabled && m.Kind == AccountKindMax && m.MaxUserID > 0
}

// Period — текущий период действия документа.
type Period struct {
	ID         string
	ValidFrom  *Date
	ValidUntil *Date
}

// Document — проекция документа core с текущим периодом и отступами напоминаний.
type Document struct {
	ID             string
	OrganizationID string
	Title          string
	Period         Period
	OffsetsDays    []int
	Version        uint64
	Deleted        bool
}

// Validate проверяет инварианты проекции документа.
func (d Document) Validate() error {
	if !IsUUID(d.ID) {
		return invalid("document_id", "ожидается UUID v4")
	}
	if !IsUUID(d.OrganizationID) {
		return invalid("organization_id", "ожидается UUID v4")
	}
	title := strings.TrimSpace(d.Title)
	if title == "" || len([]rune(title)) > MaxDocumentTitleLen {
		return invalid("title", "длина 1..200 символов")
	}
	if d.Period.ID != "" && !IsUUID(d.Period.ID) {
		return invalid("period_id", "ожидается UUID v4")
	}
	if d.Period.ValidFrom != nil && d.Period.ValidUntil != nil && d.Period.ValidUntil.Before(*d.Period.ValidFrom) {
		return invalid("valid_until", "дата окончания раньше даты начала")
	}
	if len(d.OffsetsDays) > MaxOffsetsPerDocument {
		return invalid("reminder_offsets_days", "не более 5 отступов")
	}
	seen := make(map[int]struct{}, len(d.OffsetsDays))
	for _, days := range d.OffsetsDays {
		if days < 0 || days > MaxOffsetDays {
			return invalid("reminder_offsets_days", "допустимо 0..365")
		}
		if _, dup := seen[days]; dup {
			return invalid("reminder_offsets_days", "значения не должны повторяться")
		}
		seen[days] = struct{}{}
	}
	return nil
}

// Plannable сообщает, может ли документ порождать напоминания.
func (d Document) Plannable() bool {
	return !d.Deleted && d.Period.ID != "" && d.Period.ValidUntil != nil && len(d.OffsetsDays) > 0
}
