package domain

import "time"

// AccountKind — вид аккаунта.
type AccountKind string

// Виды аккаунтов: пользователь MAX и учётная запись проверяющего.
const (
	AccountMax    AccountKind = "max"
	AccountReview AccountKind = "review"
)

// Account — пользователь системы.
type Account struct {
	ID           int64
	PublicID     string
	Kind         AccountKind
	MaxUserID    int64
	ReviewLogin  string
	FirstName    string
	LastName     string
	Username     string
	LanguageCode string
	CreatedAt    time.Time
}

// Session — серверная непрозрачная сессия.
type Session struct {
	AccountID  int64
	Source     string
	QueryID    string
	Platform   string
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

// Organization — организация пользователя.
type Organization struct {
	ID                   int64
	PublicID             string
	Name                 string
	BusinessCategoryCode string
	RegionCode           string
	Timezone             string
	FeatureCodes         []string
	Version              int
	CreatedByAccountID   int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Location возвращает часовой пояс организации.
func (o Organization) Location() (*time.Location, error) { return time.LoadLocation(o.Timezone) }

// Membership — участие аккаунта в организации.
type Membership struct {
	OrganizationID   int64
	AccountID        int64
	Role             Role
	NotifyEnabled    bool
	NotifyLocalTime  string // HH:MM
	Version          int
	JoinedAt         time.Time
	AccountPublicID  string
	AccountFirstName string
	AccountLastName  string
	AccountKind      AccountKind
	MaxUserID        int64
}

// NotifyLocalMinutes возвращает время напоминаний в минутах от полуночи.
func (m Membership) NotifyLocalMinutes() int {
	h, mi, ok := parseHHMM(m.NotifyLocalTime)
	if !ok {
		return 9 * 60
	}
	return h*60 + mi
}

// Invite — приглашение в организацию.
type Invite struct {
	ID                 int64
	PublicID           string
	OrganizationID     int64
	Role               Role
	CreatedByAccountID int64
	CreatedByFirstName string
	CreatedAt          time.Time
	ExpiresAt          time.Time
	AcceptedByID       int64
	AcceptedAt         *time.Time
	RevokedAt          *time.Time
	OrganizationName   string
	OrganizationPubID  string
}

// Active сообщает, действует ли приглашение в момент now.
func (i Invite) Active(now time.Time) bool {
	return i.AcceptedAt == nil && i.RevokedAt == nil && now.Before(i.ExpiresAt)
}

// Period — период действия документа.
type Period struct {
	ID         int64
	PublicID   string
	DocumentID int64
	ValidFrom  *time.Time
	ValidUntil *time.Time
	IsCurrent  bool
	CreatedAt  time.Time
}

// Document — документ со сроком.
type Document struct {
	ID                int64
	PublicID          string
	OrganizationID    int64
	OrganizationPubID string
	DocumentTypeCode  string
	Title             string
	Number            string
	Issuer            string
	ResponsibleLabel  string
	Notes             string
	ReferenceURL      string
	Version           int
	CreatedByID       int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CurrentPeriod     Period
	Periods           []Period
	ReminderOffsets   []int
}

// DocumentStats — сводка статусов документов организации.
type DocumentStats struct {
	Total          int
	Expired        int
	Expiring       int
	Valid          int
	NoExpiry       int
	NextValidUntil *time.Time
}

// CalendarExport — одноразовая ссылка на файл календаря.
type CalendarExport struct {
	OrganizationID int64
	AccountID      int64
	ExpiresAt      time.Time
}

// Действия журнала аудита.
const (
	AuditSessionCreated      = "session.created"
	AuditOrganizationCreated = "organization.created"
	AuditOrganizationDeleted = "organization.deleted"
	AuditMemberRoleChanged   = "member.role_changed"
	AuditMemberRemoved       = "member.removed"
	AuditInviteCreated       = "invite.created"
	AuditInviteAccepted      = "invite.accepted"
	AuditInviteRevoked       = "invite.revoked"
	AuditDocumentDeleted     = "document.deleted"
	AuditExportCreated       = "export.created"
	AuditReviewTokenIssued   = "review_token.issued"
	AuditAccountDeleted      = "account.deleted"
)

func parseHHMM(s string) (int, int, bool) {
	if len(s) < 5 || s[2] != ':' {
		return 0, 0, false
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}
