// Package ports объявляет интерфейсы, которые нужны сценариям reminders-service.
// Реализации находятся в adapters; направление зависимостей — внутрь (ADR-025).
package ports

import (
	"context"
	"time"

	"vovremya/services/reminders/internal/domain"
)

// Clock — источник времени; в тестах подменяется управляемой реализацией.
type Clock interface {
	Now() time.Time
}

// TxManager задаёт транзакционную границу внутри сервиса.
// Транзакция передаётся через контекст и не пересекает границу микросервиса.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// InboxRepo хранит журнал принятых событий core.
type InboxRepo interface {
	// Insert регистрирует событие. inserted=false означает повторную доставку.
	Insert(ctx context.Context, rec InboxRecord) (inserted bool, err error)
	// DeleteOlderThan удаляет записи старше указанного момента; возвращает число строк.
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
	// LastAppliedAt возвращает момент последнего успешно применённого события.
	LastAppliedAt(ctx context.Context) (time.Time, bool, error)
}

// InboxRecord — запись журнала входящих событий.
type InboxRecord struct {
	EventID          string
	EventType        string
	AggregateType    string
	AggregateID      string
	AggregateVersion uint64
	SchemaVersion    uint32
	SourceService    string
	Snapshot         bool
	Outcome          string
	OccurredAt       time.Time
}

// ProjectionRepo хранит локальные проекции данных core.
type ProjectionRepo interface {
	GetOrganization(ctx context.Context, id string) (domain.Organization, error)
	UpsertOrganization(ctx context.Context, org domain.Organization) error
	MarkOrganizationDeleted(ctx context.Context, id string, version uint64) error

	GetMember(ctx context.Context, orgID, accountID string) (domain.Member, error)
	UpsertMember(ctx context.Context, m domain.Member) error
	MarkMemberRemoved(ctx context.Context, orgID, accountID string, version uint64) error
	ListMembers(ctx context.Context, orgID string) ([]domain.Member, error)

	GetDocument(ctx context.Context, id string) (domain.Document, error)
	UpsertDocument(ctx context.Context, d domain.Document) error
	MarkDocumentDeleted(ctx context.Context, id string, version uint64) error
	ListDocumentIDs(ctx context.Context, orgID string) ([]string, error)

	// AggregateVersion возвращает применённую версию агрегата и признак удаления.
	AggregateVersion(ctx context.Context, aggregateType, aggregateID string) (AggregateState, error)
	// ListOrganizationsOfAccount возвращает организации, где аккаунт — участник.
	ListOrganizationsOfAccount(ctx context.Context, accountID string) ([]string, error)
	// RemoveMemberships удаляет проекции участий аккаунта (удаление аккаунта в core).
	RemoveMemberships(ctx context.Context, accountID string) error
}

// AggregateState — применённое состояние агрегата в проекции.
type AggregateState struct {
	Exists    bool
	Version   uint64
	Deleted   bool
	AppliedAt time.Time
}

// ReminderRepo хранит план напоминаний.
type ReminderRepo interface {
	// Upsert вставляет или обновляет напоминание плана; не трогает handed_off.
	Upsert(ctx context.Context, r domain.Reminder) error
	// CancelOutsideKeys отменяет planned-напоминания документа, отсутствующие в keep.
	CancelOutsideKeys(ctx context.Context, documentID string, keep []domain.PlanKey) (int64, error)
	// CancelByDocument отменяет все planned-напоминания документа.
	CancelByDocument(ctx context.Context, documentID string) (int64, error)
	// CancelByOrganization отменяет planned-напоминания организации.
	CancelByOrganization(ctx context.Context, orgID string) (int64, error)
	// CancelByMember отменяет planned-напоминания участника в организации.
	CancelByMember(ctx context.Context, orgID, accountID string) (int64, error)
	// CancelByAccount отменяет planned-напоминания получателя во всех организациях.
	CancelByAccount(ctx context.Context, accountID string) (int64, error)

	// ClaimDue захватывает пакет готовых к отправке напоминаний, продлевая аренду.
	ClaimDue(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Reminder, error)
	// MarkHandedOff фиксирует приём напоминания bot-service.
	MarkHandedOff(ctx context.Context, id int64, notificationID string, at time.Time) (bool, error)
	// Reschedule откладывает повтор после ошибки.
	Reschedule(ctx context.Context, id int64, nextAttemptAt time.Time, errorCode string) (bool, error)
	// MarkSkipped помечает напоминание как неотправляемое.
	MarkSkipped(ctx context.Context, id int64, errorCode string) (bool, error)

	// NextForAccount возвращает ближайшее planned-напоминание по каждому документу.
	NextForAccount(ctx context.Context, accountID string, documentIDs []string) (map[string]domain.Reminder, error)
	// ListByDocument возвращает план документа.
	ListByDocument(ctx context.Context, documentID string) ([]domain.Reminder, error)
	// CountPlanned возвращает число запланированных напоминаний.
	CountPlanned(ctx context.Context) (int64, error)
	// CountDue возвращает число напоминаний, готовых к отправке.
	CountDue(ctx context.Context, now time.Time) (int64, error)
	// DeleteFinalizedBefore удаляет неактивные напоминания старше момента.
	DeleteFinalizedBefore(ctx context.Context, before time.Time) (int64, error)
}

// NotificationRequest — задание на доставку сообщения получателю MAX.
type NotificationRequest struct {
	IdempotencyKey     string
	RecipientMaxUserID int64
	Text               string
	ButtonText         string
	ButtonPayload      string
	// Buttons — кнопки под сообщением (до 3); если пусто — используется ButtonText/ButtonPayload.
	Buttons  []NotificationButton
	NotAfter time.Time
}

// NotificationButton — кнопка открытия мини-приложения с payload диплинка.
type NotificationButton struct {
	Text    string
	Payload string
}

// DigestRecipient — получатель недельной сводки в организации.
type DigestRecipient struct {
	OrganizationID     string
	AccountID          string
	MaxUserID          int64
	NotifyLocalMinutes int
	OrganizationName   string
	Timezone           string
}

// DigestDocument — документ для недельной сводки.
type DigestDocument struct {
	Title                string
	ValidUntil           time.Time
	ResponsibleAccountID string
}

// DigestRepo — выборки для недельной сводки (реализует postgres.ProjectionRepo).
type DigestRepo interface {
	ListDigestRecipients(ctx context.Context) ([]DigestRecipient, error)
	ListDigestDocuments(ctx context.Context, organizationID string, until time.Time) ([]DigestDocument, error)
}

// NotificationResult — результат приёма задания bot-service.
type NotificationResult struct {
	NotificationID string
	Duplicate      bool
}

// MessagingGateway — исходящий порт к bot-service (доставка сообщений MAX).
type MessagingGateway interface {
	Enqueue(ctx context.Context, req NotificationRequest) (NotificationResult, error)
}
