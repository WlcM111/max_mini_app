package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/reminders/internal/app"
	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"
)

// ProjectionRepo хранит локальные проекции данных core-service.
type ProjectionRepo struct{ base }

// NewProjectionRepo создаёт репозиторий проекций.
func NewProjectionRepo(pool *pgkit.Pool, m *metrics.Registry) *ProjectionRepo {
	return &ProjectionRepo{base{pool: pool, metrics: m}}
}

// GetOrganization возвращает проекцию организации.
func (r *ProjectionRepo) GetOrganization(ctx context.Context, id string) (domain.Organization, error) {
	started := time.Now()
	defer r.observe("organization_get", started)
	var org domain.Organization
	var version int64
	err := r.db(ctx).QueryRow(ctx, `
SELECT organization_id::text, name, timezone, version, deleted
FROM reminders.organizations WHERE organization_id = $1::uuid`, id).
		Scan(&org.ID, &org.Name, &org.Timezone, &version, &org.Deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Organization{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Organization{}, fmt.Errorf("get organization: %w", err)
	}
	org.Version = uint64(version)
	return org, nil
}

// UpsertOrganization записывает состояние организации.
func (r *ProjectionRepo) UpsertOrganization(ctx context.Context, org domain.Organization) error {
	started := time.Now()
	defer r.observe("organization_upsert", started)
	_, err := r.db(ctx).Exec(ctx, `
INSERT INTO reminders.organizations (organization_id, name, timezone, version, deleted, applied_at)
VALUES ($1::uuid, $2, $3, $4, false, now())
ON CONFLICT (organization_id) DO UPDATE
SET name = excluded.name, timezone = excluded.timezone, version = excluded.version,
    deleted = false, applied_at = now()`,
		org.ID, org.Name, org.Timezone, int64(org.Version))
	if err != nil {
		return fmt.Errorf("upsert organization: %w", err)
	}
	return nil
}

// MarkOrganizationDeleted помечает организацию удалённой.
// Если проекции ещё не было, создаётся запись-надгробие: она хранит только
// версию удаления и защищает от применения устаревших событий.
func (r *ProjectionRepo) MarkOrganizationDeleted(ctx context.Context, id string, version uint64) error {
	started := time.Now()
	defer r.observe("organization_delete", started)
	_, err := r.db(ctx).Exec(ctx, `
INSERT INTO reminders.organizations (organization_id, name, timezone, version, deleted, applied_at)
VALUES ($1::uuid, '(удалена)', 'UTC', $2, true, now())
ON CONFLICT (organization_id) DO UPDATE
SET deleted = true, version = excluded.version, applied_at = now()`, id, int64(version))
	if err != nil {
		return fmt.Errorf("mark organization deleted: %w", err)
	}
	return nil
}

// GetMember возвращает проекцию участия.
func (r *ProjectionRepo) GetMember(ctx context.Context, orgID, accountID string) (domain.Member, error) {
	started := time.Now()
	defer r.observe("member_get", started)
	var m domain.Member
	var kind string
	var version int64
	var minutes int16
	err := r.db(ctx).QueryRow(ctx, `
SELECT organization_id::text, account_id::text, account_kind, max_user_id,
       notify_enabled, notify_local_minutes, version, removed
FROM reminders.members WHERE organization_id = $1::uuid AND account_id = $2::uuid`, orgID, accountID).
		Scan(&m.OrganizationID, &m.AccountID, &kind, &m.MaxUserID, &m.NotifyEnabled, &minutes, &version, &m.Removed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Member{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Member{}, fmt.Errorf("get member: %w", err)
	}
	m.Kind = domain.AccountKind(kind)
	m.NotifyLocalMinutes = int(minutes)
	m.Version = uint64(version)
	return m, nil
}

// UpsertMember записывает состояние участия.
func (r *ProjectionRepo) UpsertMember(ctx context.Context, m domain.Member) error {
	started := time.Now()
	defer r.observe("member_upsert", started)
	_, err := r.db(ctx).Exec(ctx, `
INSERT INTO reminders.members (organization_id, account_id, account_kind, max_user_id,
                               notify_enabled, notify_local_minutes, version, removed, applied_at)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, false, now())
ON CONFLICT (organization_id, account_id) DO UPDATE
SET account_kind = excluded.account_kind, max_user_id = excluded.max_user_id,
    notify_enabled = excluded.notify_enabled, notify_local_minutes = excluded.notify_local_minutes,
    version = excluded.version, removed = false, applied_at = now()`,
		m.OrganizationID, m.AccountID, string(m.Kind), m.MaxUserID, m.NotifyEnabled,
		int16(m.NotifyLocalMinutes), int64(m.Version))
	if err != nil {
		return fmt.Errorf("upsert member: %w", err)
	}
	return nil
}

// MarkMemberRemoved помечает участие прекращённым.
func (r *ProjectionRepo) MarkMemberRemoved(ctx context.Context, orgID, accountID string, version uint64) error {
	started := time.Now()
	defer r.observe("member_remove", started)
	_, err := r.db(ctx).Exec(ctx, `
INSERT INTO reminders.members (organization_id, account_id, account_kind, max_user_id,
                               notify_enabled, notify_local_minutes, version, removed, applied_at)
VALUES ($1::uuid, $2::uuid, 'review', 0, false, 0, $3, true, now())
ON CONFLICT (organization_id, account_id) DO UPDATE
SET removed = true, notify_enabled = false, version = excluded.version, applied_at = now()`,
		orgID, accountID, int64(version))
	if err != nil {
		return fmt.Errorf("mark member removed: %w", err)
	}
	return nil
}

// ListMembers возвращает действующих участников организации.
func (r *ProjectionRepo) ListMembers(ctx context.Context, orgID string) ([]domain.Member, error) {
	started := time.Now()
	defer r.observe("member_list", started)
	rows, err := r.db(ctx).Query(ctx, `
SELECT organization_id::text, account_id::text, account_kind, max_user_id,
       notify_enabled, notify_local_minutes, version
FROM reminders.members WHERE organization_id = $1::uuid AND removed = false
ORDER BY account_id`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	var out []domain.Member
	for rows.Next() {
		var m domain.Member
		var kind string
		var minutes int16
		var version int64
		if err := rows.Scan(&m.OrganizationID, &m.AccountID, &kind, &m.MaxUserID,
			&m.NotifyEnabled, &minutes, &version); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		m.Kind = domain.AccountKind(kind)
		m.NotifyLocalMinutes = int(minutes)
		m.Version = uint64(version)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate members: %w", err)
	}
	return out, nil
}

// GetDocument возвращает проекцию документа с отступами напоминаний.
func (r *ProjectionRepo) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	started := time.Now()
	defer r.observe("document_get", started)
	var d domain.Document
	var periodID *string
	var validFrom, validUntil *time.Time
	var version int64
	err := r.db(ctx).QueryRow(ctx, `
SELECT document_id::text, organization_id::text, title, period_id::text, valid_from, valid_until, version, deleted,
       coalesce(responsible_account_id::text, '')
FROM reminders.documents WHERE document_id = $1::uuid`, id).
		Scan(&d.ID, &d.OrganizationID, &d.Title, &periodID, &validFrom, &validUntil, &version, &d.Deleted,
			&d.ResponsibleAccountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Document{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Document{}, fmt.Errorf("get document: %w", err)
	}
	d.Version = uint64(version)
	if periodID != nil {
		d.Period.ID = *periodID
	}
	d.Period.ValidFrom = toDate(validFrom)
	d.Period.ValidUntil = toDate(validUntil)

	rows, err := r.db(ctx).Query(ctx,
		`SELECT days_before FROM reminders.document_offsets WHERE document_id = $1::uuid ORDER BY days_before DESC`, id)
	if err != nil {
		return domain.Document{}, fmt.Errorf("get offsets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var days int16
		if err := rows.Scan(&days); err != nil {
			return domain.Document{}, fmt.Errorf("scan offset: %w", err)
		}
		d.OffsetsDays = append(d.OffsetsDays, int(days))
	}
	if err := rows.Err(); err != nil {
		return domain.Document{}, fmt.Errorf("iterate offsets: %w", err)
	}
	return d, nil
}

// UpsertDocument записывает состояние документа и его отступы напоминаний.
func (r *ProjectionRepo) UpsertDocument(ctx context.Context, d domain.Document) error {
	started := time.Now()
	defer r.observe("document_upsert", started)
	db := r.db(ctx)
	_, err := db.Exec(ctx, `
INSERT INTO reminders.documents (document_id, organization_id, title, period_id, valid_from, valid_until,
                                 version, deleted, applied_at, responsible_account_id)
VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5::date, $6::date, $7, false, now(), $8::uuid)
ON CONFLICT (document_id) DO UPDATE
SET organization_id = excluded.organization_id, title = excluded.title, period_id = excluded.period_id,
    valid_from = excluded.valid_from, valid_until = excluded.valid_until,
    version = excluded.version, deleted = false, applied_at = now(),
    responsible_account_id = excluded.responsible_account_id`,
		d.ID, d.OrganizationID, d.Title, nullString(d.Period.ID),
		dateString(d.Period.ValidFrom), dateString(d.Period.ValidUntil), int64(d.Version),
		nullString(d.ResponsibleAccountID))
	if err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM reminders.document_offsets WHERE document_id = $1::uuid`, d.ID); err != nil {
		return fmt.Errorf("clear offsets: %w", err)
	}
	for _, days := range d.OffsetsDays {
		if _, err := db.Exec(ctx, `
INSERT INTO reminders.document_offsets (document_id, days_before) VALUES ($1::uuid, $2)
ON CONFLICT DO NOTHING`, d.ID, int16(days)); err != nil {
			return fmt.Errorf("insert offset: %w", err)
		}
	}
	return nil
}

// MarkDocumentDeleted помечает документ удалённым.
func (r *ProjectionRepo) MarkDocumentDeleted(ctx context.Context, id string, version uint64) error {
	started := time.Now()
	defer r.observe("document_delete", started)
	tag, err := r.db(ctx).Exec(ctx, `
UPDATE reminders.documents
SET deleted = true, version = $2, period_id = NULL, valid_until = NULL, applied_at = now()
WHERE document_id = $1::uuid`, id, int64(version))
	if err != nil {
		return fmt.Errorf("mark document deleted: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Документ не был спроецирован: сохраняем надгробие, чтобы отбросить
		// устаревшее событие состояния, если оно придёт после удаления.
		_, err = r.db(ctx).Exec(ctx, `
INSERT INTO reminders.documents (document_id, organization_id, title, version, deleted, applied_at)
VALUES ($1::uuid, '00000000-0000-4000-8000-000000000000'::uuid, '(удалён)', $2, true, now())
ON CONFLICT (document_id) DO UPDATE SET deleted = true, version = excluded.version, applied_at = now()`,
			id, int64(version))
		if err != nil {
			return fmt.Errorf("insert document tombstone: %w", err)
		}
	}
	return nil
}

// ListDocumentIDs возвращает действующие документы организации.
func (r *ProjectionRepo) ListDocumentIDs(ctx context.Context, orgID string) ([]string, error) {
	started := time.Now()
	defer r.observe("document_list_ids", started)
	rows, err := r.db(ctx).Query(ctx,
		`SELECT document_id::text FROM reminders.documents WHERE organization_id = $1::uuid AND deleted = false`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list document ids: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan document id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate document ids: %w", err)
	}
	return out, nil
}

// AggregateVersion возвращает применённую версию агрегата проекции.
func (r *ProjectionRepo) AggregateVersion(ctx context.Context, aggregateType, aggregateID string) (ports.AggregateState, error) {
	started := time.Now()
	defer r.observe("aggregate_version", started)
	var (
		version   int64
		deleted   bool
		appliedAt time.Time
		err       error
	)
	switch aggregateType {
	case app.AggregateOrganization:
		err = r.db(ctx).QueryRow(ctx,
			`SELECT version, deleted, applied_at FROM reminders.organizations WHERE organization_id = $1::uuid`,
			aggregateID).Scan(&version, &deleted, &appliedAt)
	case app.AggregateDocument:
		err = r.db(ctx).QueryRow(ctx,
			`SELECT version, deleted, applied_at FROM reminders.documents WHERE document_id = $1::uuid`,
			aggregateID).Scan(&version, &deleted, &appliedAt)
	case app.AggregateMembership:
		orgID, accountID, ok := app.SplitMembershipAggregateID(aggregateID)
		if !ok {
			return ports.AggregateState{}, domain.ValidationError{Field: "aggregate_id", Reason: "ожидается <organization_id>:<account_id>"}
		}
		err = r.db(ctx).QueryRow(ctx, `
SELECT version, removed, applied_at FROM reminders.members
WHERE organization_id = $1::uuid AND account_id = $2::uuid`, orgID, accountID).Scan(&version, &deleted, &appliedAt)
	case app.AggregateAccount:
		// Удаление аккаунта — терминальное событие без проекции: повторная
		// доставка отсекается журналом входящих событий по event_id.
		return ports.AggregateState{}, nil
	default:
		return ports.AggregateState{}, domain.ValidationError{Field: "aggregate_type", Reason: "неизвестный тип агрегата"}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.AggregateState{}, nil
	}
	if err != nil {
		return ports.AggregateState{}, fmt.Errorf("aggregate version: %w", err)
	}
	return ports.AggregateState{Exists: true, Version: uint64(version), Deleted: deleted, AppliedAt: appliedAt}, nil
}

// ListOrganizationsOfAccount возвращает организации, где аккаунт — участник.
func (r *ProjectionRepo) ListOrganizationsOfAccount(ctx context.Context, accountID string) ([]string, error) {
	started := time.Now()
	defer r.observe("member_orgs", started)
	rows, err := r.db(ctx).Query(ctx,
		`SELECT organization_id::text FROM reminders.members WHERE account_id = $1::uuid`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list organizations of account: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan organization id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// RemoveMemberships удаляет проекции участий аккаунта (аккаунт удалён в core).
func (r *ProjectionRepo) RemoveMemberships(ctx context.Context, accountID string) error {
	started := time.Now()
	defer r.observe("member_remove_all", started)
	if _, err := r.db(ctx).Exec(ctx,
		`DELETE FROM reminders.members WHERE account_id = $1::uuid`, accountID); err != nil {
		return fmt.Errorf("remove memberships: %w", err)
	}
	return nil
}

func toDate(t *time.Time) *domain.Date {
	if t == nil {
		return nil
	}
	d := domain.Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
	return &d
}

func dateString(d *domain.Date) any {
	if d == nil {
		return nil
	}
	return d.String()
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var _ ports.ProjectionRepo = (*ProjectionRepo)(nil)
var _ ports.InboxRepo = (*InboxRepo)(nil)
