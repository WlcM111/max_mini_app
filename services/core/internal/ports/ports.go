// Package ports описывает зависимости сценариев core-service: хранилище,
// проверка данных запуска MAX и вызовы соседних микросервисов.
// Реализации находятся в internal/adapters.
package ports

import (
	"context"
	"time"

	"vovremya/services/core/internal/domain"
)

// Clock — источник времени.
type Clock interface{ Now() time.Time }

// Random — источник случайных значений.
type Random interface {
	Float64() float64
	Token() (string, error) // 43 символа base64url
	UUID() string
}

// TxManager выполняет функцию в одной транзакции.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// LaunchIdentity — результат проверки данных запуска MAX.
type LaunchIdentity struct {
	MaxUserID    int64
	FirstName    string
	LastName     string
	Username     string
	LanguageCode string
	QueryID      string
	AuthDate     time.Time
	StartParam   string
}

// LaunchVerifier проверяет подпись init_data (spec §4).
type LaunchVerifier interface {
	Verify(initData string, now time.Time) (LaunchIdentity, error)
}

// AccountRepo — аккаунты пользователей.
type AccountRepo interface {
	UpsertMax(ctx context.Context, id LaunchIdentity, now time.Time) (domain.Account, error)
	UpsertReview(ctx context.Context, login, firstName string, now time.Time) (domain.Account, error)
	GetByID(ctx context.Context, id int64) (domain.Account, error)
	GetByPublicID(ctx context.Context, publicID string) (domain.Account, error)
	GetByReviewLogin(ctx context.Context, login string) (domain.Account, error)
	Delete(ctx context.Context, id int64) error
}

// SessionRepo — серверные сессии.
type SessionRepo interface {
	Create(ctx context.Context, tokenHash []byte, accountID int64, source, queryID, platform string,
		createdAt, expiresAt time.Time) error
	FindActive(ctx context.Context, tokenHash []byte, now time.Time) (domain.Account, domain.Session, error)
	Touch(ctx context.Context, tokenHash []byte, now time.Time) error
	RevokeByHash(ctx context.Context, tokenHash []byte, now time.Time) error
	RevokeByAccount(ctx context.Context, accountID int64, now time.Time) error
	DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error)
}

// CatalogRepo — чтение справочника.
type CatalogRepo interface {
	Load(ctx context.Context) (*domain.Catalog, error)
}

// OrganizationRepo — организации и участники.
type OrganizationRepo interface {
	GetByPublicID(ctx context.Context, publicID string) (domain.Organization, error)
	GetByID(ctx context.Context, id int64) (domain.Organization, error)
	LockByPublicID(ctx context.Context, publicID string) (domain.Organization, error)
	Create(ctx context.Context, org domain.Organization, createdBy int64, now time.Time) (domain.Organization, error)
	Update(ctx context.Context, org domain.Organization, now time.Time) (domain.Organization, error)
	Delete(ctx context.Context, id int64) error
	CountByAccount(ctx context.Context, accountID int64) (int, error)
	Stats(ctx context.Context, orgID int64, today time.Time) (domain.DocumentStats, error)

	GetMembership(ctx context.Context, orgID, accountID int64) (domain.Membership, error)
	ListMemberships(ctx context.Context, accountID int64) ([]domain.Membership, []domain.Organization, error)
	ListMembers(ctx context.Context, orgID int64) ([]domain.Membership, error)
	CountMembers(ctx context.Context, orgID int64) (int, error)
	CreateMembership(ctx context.Context, m domain.Membership, now time.Time) (domain.Membership, error)
	UpdateMembership(ctx context.Context, m domain.Membership) (domain.Membership, error)
	DeleteMembership(ctx context.Context, orgID, accountID int64) error
	ListOwnedOrganizations(ctx context.Context, accountID int64) ([]domain.Organization, error)
}

// DocumentFilter — параметры выборки реестра документов.
type DocumentFilter struct {
	OrganizationID int64
	Status         domain.DeadlineStatus
	Query          string
	Today          time.Time
	AfterUntil     string // дата, 'infinity' или '-infinity'
	AfterID        string
	Limit          int
}

// DocumentRepo — документы и периоды.
type DocumentRepo interface {
	GetByPublicID(ctx context.Context, publicID string) (domain.Document, error)
	GetFull(ctx context.Context, id int64) (domain.Document, error)
	List(ctx context.Context, f DocumentFilter) ([]domain.Document, error)
	ListCurrent(ctx context.Context, orgID int64) ([]domain.Document, error)
	Count(ctx context.Context, orgID int64) (int, error)
	Create(ctx context.Context, doc domain.Document, createdBy int64, now time.Time) (domain.Document, error)
	Update(ctx context.Context, doc domain.Document, updatedBy int64, now time.Time) (domain.Document, error)
	ReplaceOffsets(ctx context.Context, docID int64, offsets []int) error
	UpdateCurrentPeriod(ctx context.Context, periodID int64, from, until *time.Time) error
	AddPeriod(ctx context.Context, docID int64, p domain.Period, createdBy int64, now time.Time) (domain.Period, error)
	FindPeriodByPublicID(ctx context.Context, publicID string) (domain.Period, error)
	Delete(ctx context.Context, id int64) error
}

// InviteRepo — приглашения.
type InviteRepo interface {
	GetByPublicID(ctx context.Context, publicID string) (domain.Invite, error)
	FindByTokenHash(ctx context.Context, tokenHash []byte) (domain.Invite, error)
	LockByTokenHash(ctx context.Context, tokenHash []byte) (domain.Invite, error)
	ListActive(ctx context.Context, orgID int64, now time.Time) ([]domain.Invite, error)
	CountActive(ctx context.Context, orgID int64, now time.Time) (int, error)
	Create(ctx context.Context, inv domain.Invite, tokenHash []byte) (domain.Invite, error)
	MarkAccepted(ctx context.Context, id, accountID int64, now time.Time) error
	MarkRevoked(ctx context.Context, id int64, now time.Time) error
	DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error)
}

// ExportRepo — одноразовые ссылки на файл календаря.
type ExportRepo interface {
	Create(ctx context.Context, tokenHash []byte, orgID, accountID int64, createdAt, expiresAt time.Time) error
	Consume(ctx context.Context, tokenHash []byte, now time.Time) (int64, error)
	Exists(ctx context.Context, tokenHash []byte) (bool, error)
	DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error)
}

// AuditRepo — журнал значимых действий.
type AuditRepo interface {
	Write(ctx context.Context, action string, accountID, orgID int64, targetPublicID string, now time.Time) error
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

// OutboxRepo — исходящие события для reminders-service.
type OutboxRepo interface {
	Append(ctx context.Context, e domain.OutboxEvent) error
	ClaimReady(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.OutboxEvent, error)
	MarkSent(ctx context.Context, id int64, now time.Time) error
	MarkFailed(ctx context.Context, id int64, nextAttemptAt time.Time, errorCode string) error
	PendingCount(ctx context.Context) (int64, error)
	PendingForAggregates(ctx context.Context, aggregateType string, ids []string) (map[string]bool, error)
	HasPending(ctx context.Context, aggregateType, aggregateID string) (bool, error)
	DeleteSentBefore(ctx context.Context, before time.Time) (int64, error)
}

// RecipientStatus — состояние канала напоминаний пользователя.
type RecipientStatus struct {
	State      string
	BotChatURL string
}

// BotProfile — профиль бота MAX.
type BotProfile struct {
	Username            string
	DisplayName         string
	ChatURL             string
	OpenAppLinkTemplate string
}

// NotificationButton — кнопка сообщения бота.
type NotificationButton struct {
	Text           string
	OpenAppPayload string
	URL            string
}

// NotificationRequest — задание на отправку сообщения через bot-service.
type NotificationRequest struct {
	IdempotencyKey string
	Kind           string // reminder | member_joined
	RecipientMax   int64
	Text           string
	Buttons        []NotificationButton
	NotAfter       time.Time
}

// BotGateway — клиент MessagingService bot-service.
type BotGateway interface {
	EnqueueNotification(ctx context.Context, req NotificationRequest) error
	GetRecipientStatus(ctx context.Context, maxUserID int64) (RecipientStatus, error)
	GetBotProfile(ctx context.Context) (BotProfile, error)
}

// IngestEvent — событие, передаваемое reminders-service.
type IngestEvent struct {
	EventID          string
	EventType        string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	Payload          []byte
	Snapshot         bool
	OccurredAt       time.Time
}

// IngestResult — исход применения одного события.
type IngestResult struct {
	EventID   string
	Outcome   string // applied | duplicate | stale | rejected
	Message   string
	Retryable bool
}

// NextReminder — ближайшее напоминание по документу.
type NextReminder struct {
	DocumentID string
	DueAt      time.Time
	DaysBefore int
}

// RemindersGateway — клиент reminders-service (приём событий и чтение плана).
type RemindersGateway interface {
	ApplyEvents(ctx context.Context, batchID string, events []IngestEvent) ([]IngestResult, error)
	GetNextReminders(ctx context.Context, accountPublicID string, documentIDs []string) ([]NextReminder, error)
	GetSyncStatus(ctx context.Context, aggregateType, aggregateID string, expectedVersion int64) (bool, error)
}

// AssistantOption — элемент закрытого списка, из которого ассистент выбирает код.
type AssistantOption struct {
	Code  string
	Title string
	Hint  string
}

// AssistantCatalog — справочные списки, передаваемые ассистенту. Модель выбирает
// значения только из них: коды вне списка отбрасываются сценарием (ADR-032).
type AssistantCatalog struct {
	DocumentTypes []AssistantOption
	Categories    []AssistantOption
	Features      []AssistantOption
}

// DocumentDraft — распознанные реквизиты документа. Пустая строка означает,
// что значение в тексте не найдено; даты — YYYY-MM-DD.
type DocumentDraft struct {
	Title            string
	Number           string
	Issuer           string
	ValidFrom        string
	ValidUntil       string
	DocumentTypeCode string
	Confidence       float64
}

// ProfileMatch — профиль организации, распознанный по свободному описанию.
type ProfileMatch struct {
	BusinessCategoryCode string
	FeatureCodes         []string
	Confidence           float64
}

// DraftAssistant — внешний языковой сервис (GigaChat). Используется только для
// извлечения данных из текста пользователя и выбора кодов справочника; в
// критический путь напоминаний и статусов не входит (ADR-032).
type DraftAssistant interface {
	DraftDocument(ctx context.Context, text string, catalog AssistantCatalog) (DocumentDraft, error)
	MatchProfile(ctx context.Context, description string, catalog AssistantCatalog) (ProfileMatch, error)
}
