package postgres

import (
	"context"
	"fmt"
	"time"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// OrganizationRepo — организации и участники.
type OrganizationRepo struct{ base }

// NewOrganizationRepo создаёт репозиторий организаций.
func NewOrganizationRepo(pool *pgkit.Pool, m *metrics.Registry) *OrganizationRepo {
	return &OrganizationRepo{base{pool: pool, metrics: m}}
}

var _ ports.OrganizationRepo = (*OrganizationRepo)(nil)

const orgColumns = `id, public_id::text, name, business_category_code, region_code, timezone, version,
	coalesce(created_by, 0), created_at, updated_at`

func scanOrg(scan func(dest ...any) error) (domain.Organization, error) {
	var o domain.Organization
	err := scan(&o.ID, &o.PublicID, &o.Name, &o.BusinessCategoryCode, &o.RegionCode, &o.Timezone,
		&o.Version, &o.CreatedByAccountID, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

func (r *OrganizationRepo) loadFeatures(ctx context.Context, orgIDs []int64) (map[int64][]string, error) {
	out := make(map[int64][]string, len(orgIDs))
	if len(orgIDs) == 0 {
		return out, nil
	}
	rows, err := r.db(ctx).Query(ctx,
		`SELECT organization_id, feature_code FROM core.organization_features
		 WHERE organization_id = ANY($1) ORDER BY organization_id, feature_code`, orgIDs)
	if err != nil {
		return nil, fmt.Errorf("load organization features: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id   int64
			code string
		)
		if err := rows.Scan(&id, &code); err != nil {
			return nil, err
		}
		out[id] = append(out[id], code)
	}
	return out, rows.Err()
}

func (r *OrganizationRepo) withFeatures(ctx context.Context, org domain.Organization) (domain.Organization, error) {
	features, err := r.loadFeatures(ctx, []int64{org.ID})
	if err != nil {
		return domain.Organization{}, err
	}
	org.FeatureCodes = features[org.ID]
	return org, nil
}

// GetByPublicID читает организацию по публичному идентификатору.
func (r *OrganizationRepo) GetByPublicID(ctx context.Context, publicID string) (domain.Organization, error) {
	started := time.Now()
	const q = `SELECT ` + orgColumns + ` FROM core.organizations WHERE public_id = $1`
	org, err := scanOrg(r.db(ctx).QueryRow(ctx, q, publicID).Scan)
	r.observe("organizations.get", started)
	if err != nil {
		return domain.Organization{}, notFound(err)
	}
	return r.withFeatures(ctx, org)
}

// GetByID читает организацию по внутреннему идентификатору.
func (r *OrganizationRepo) GetByID(ctx context.Context, id int64) (domain.Organization, error) {
	const q = `SELECT ` + orgColumns + ` FROM core.organizations WHERE id = $1`
	org, err := scanOrg(r.db(ctx).QueryRow(ctx, q, id).Scan)
	if err != nil {
		return domain.Organization{}, notFound(err)
	}
	return r.withFeatures(ctx, org)
}

// LockByPublicID блокирует строку организации на время транзакции.
func (r *OrganizationRepo) LockByPublicID(ctx context.Context, publicID string) (domain.Organization, error) {
	started := time.Now()
	const q = `SELECT ` + orgColumns + ` FROM core.organizations WHERE public_id = $1 FOR UPDATE`
	org, err := scanOrg(r.db(ctx).QueryRow(ctx, q, publicID).Scan)
	r.observe("organizations.lock", started)
	if err != nil {
		return domain.Organization{}, notFound(err)
	}
	return r.withFeatures(ctx, org)
}

// Create создаёт организацию с признаками.
func (r *OrganizationRepo) Create(ctx context.Context, org domain.Organization, createdBy int64, now time.Time) (domain.Organization, error) {
	started := time.Now()
	const q = `INSERT INTO core.organizations
		(public_id, name, business_category_code, region_code, timezone, version, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $7) RETURNING ` + orgColumns
	stored, err := scanOrg(r.db(ctx).QueryRow(ctx, q, org.PublicID, org.Name, org.BusinessCategoryCode,
		org.RegionCode, org.Timezone, createdBy, now).Scan)
	r.observe("organizations.create", started)
	if err != nil {
		return domain.Organization{}, fmt.Errorf("create organization: %w", err)
	}
	if err := r.replaceFeatures(ctx, stored.ID, org.FeatureCodes); err != nil {
		return domain.Organization{}, err
	}
	stored.FeatureCodes = org.FeatureCodes
	return stored, nil
}

func (r *OrganizationRepo) replaceFeatures(ctx context.Context, orgID int64, codes []string) error {
	if _, err := r.db(ctx).Exec(ctx, `DELETE FROM core.organization_features WHERE organization_id = $1`, orgID); err != nil {
		return fmt.Errorf("clear organization features: %w", err)
	}
	for _, code := range codes {
		if _, err := r.db(ctx).Exec(ctx,
			`INSERT INTO core.organization_features (organization_id, feature_code) VALUES ($1, $2)`,
			orgID, code); err != nil {
			return fmt.Errorf("insert organization feature: %w", err)
		}
	}
	return nil
}

// Update сохраняет профиль организации и увеличивает версию.
func (r *OrganizationRepo) Update(ctx context.Context, org domain.Organization, now time.Time) (domain.Organization, error) {
	started := time.Now()
	const q = `UPDATE core.organizations
		SET name = $2, business_category_code = $3, region_code = $4, timezone = $5,
		    version = version + 1, updated_at = $6
		WHERE id = $1 RETURNING ` + orgColumns
	stored, err := scanOrg(r.db(ctx).QueryRow(ctx, q, org.ID, org.Name, org.BusinessCategoryCode,
		org.RegionCode, org.Timezone, now).Scan)
	r.observe("organizations.update", started)
	if err != nil {
		return domain.Organization{}, fmt.Errorf("update organization: %w", err)
	}
	if err := r.replaceFeatures(ctx, org.ID, org.FeatureCodes); err != nil {
		return domain.Organization{}, err
	}
	stored.FeatureCodes = org.FeatureCodes
	return stored, nil
}

// Delete удаляет организацию каскадом.
func (r *OrganizationRepo) Delete(ctx context.Context, id int64) error {
	started := time.Now()
	_, err := r.db(ctx).Exec(ctx, `DELETE FROM core.organizations WHERE id = $1`, id)
	r.observe("organizations.delete", started)
	if err != nil {
		return fmt.Errorf("delete organization: %w", err)
	}
	return nil
}

// CountByAccount возвращает число организаций пользователя.
func (r *OrganizationRepo) CountByAccount(ctx context.Context, accountID int64) (int, error) {
	var n int
	err := r.db(ctx).QueryRow(ctx,
		`SELECT count(*) FROM core.memberships WHERE account_id = $1`, accountID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count memberships: %w", err)
	}
	return n, nil
}

// Stats возвращает сводку статусов документов организации.
func (r *OrganizationRepo) Stats(ctx context.Context, orgID int64, today time.Time) (domain.DocumentStats, error) {
	started := time.Now()
	const q = `SELECT count(*),
		count(*) FILTER (WHERE p.valid_until < $2::date),
		count(*) FILTER (WHERE p.valid_until >= $2::date AND p.valid_until <= $2::date + 30),
		count(*) FILTER (WHERE p.valid_until > $2::date + 30),
		count(*) FILTER (WHERE p.valid_until IS NULL),
		min(p.valid_until) FILTER (WHERE p.valid_until >= $2::date)
		FROM core.documents d
		JOIN core.document_periods p ON p.document_id = d.id AND p.is_current
		WHERE d.organization_id = $1`
	var (
		s    domain.DocumentStats
		next *time.Time
	)
	err := r.db(ctx).QueryRow(ctx, q, orgID, today).Scan(&s.Total, &s.Expired, &s.Expiring, &s.Valid, &s.NoExpiry, &next)
	r.observe("organizations.stats", started)
	if err != nil {
		return domain.DocumentStats{}, fmt.Errorf("document stats: %w", err)
	}
	s.NextValidUntil = next
	return s, nil
}

const membershipColumns = `m.organization_id, m.account_id, m.role, m.notify_enabled,
	to_char(m.notify_local_time, 'HH24:MI'), m.version, m.joined_at,
	a.public_id::text, a.first_name, coalesce(a.last_name, ''), a.kind, coalesce(a.max_user_id, 0)`

func scanMembership(scan func(dest ...any) error) (domain.Membership, error) {
	var (
		m    domain.Membership
		role string
		kind string
	)
	err := scan(&m.OrganizationID, &m.AccountID, &role, &m.NotifyEnabled, &m.NotifyLocalTime,
		&m.Version, &m.JoinedAt, &m.AccountPublicID, &m.AccountFirstName, &m.AccountLastName, &kind, &m.MaxUserID)
	m.Role = domain.Role(role)
	m.AccountKind = domain.AccountKind(kind)
	return m, err
}

// GetMembership читает участие пользователя в организации.
func (r *OrganizationRepo) GetMembership(ctx context.Context, orgID, accountID int64) (domain.Membership, error) {
	started := time.Now()
	const q = `SELECT ` + membershipColumns + ` FROM core.memberships m
		JOIN core.accounts a ON a.id = m.account_id
		WHERE m.organization_id = $1 AND m.account_id = $2`
	m, err := scanMembership(r.db(ctx).QueryRow(ctx, q, orgID, accountID).Scan)
	r.observe("memberships.get", started)
	if err != nil {
		return domain.Membership{}, notFound(err)
	}
	return m, nil
}

// ListMemberships возвращает участия пользователя и соответствующие организации.
func (r *OrganizationRepo) ListMemberships(ctx context.Context, accountID int64) ([]domain.Membership, []domain.Organization, error) {
	started := time.Now()
	const q = `SELECT ` + membershipColumns + ` FROM core.memberships m
		JOIN core.accounts a ON a.id = m.account_id
		WHERE m.account_id = $1 ORDER BY m.joined_at`
	rows, err := r.db(ctx).Query(ctx, q, accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("list memberships: %w", err)
	}
	var (
		memberships []domain.Membership
		ids         []int64
	)
	for rows.Next() {
		m, err := scanMembership(rows.Scan)
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		memberships = append(memberships, m)
		ids = append(ids, m.OrganizationID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	orgs := make([]domain.Organization, 0, len(ids))
	if len(ids) > 0 {
		rows, err := r.db(ctx).Query(ctx, `SELECT `+orgColumns+` FROM core.organizations WHERE id = ANY($1)`, ids)
		if err != nil {
			return nil, nil, fmt.Errorf("list organizations: %w", err)
		}
		for rows.Next() {
			o, err := scanOrg(rows.Scan)
			if err != nil {
				rows.Close()
				return nil, nil, err
			}
			orgs = append(orgs, o)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
	}
	r.observe("memberships.list", started)
	return memberships, orgs, nil
}

// ListMembers возвращает участников организации.
func (r *OrganizationRepo) ListMembers(ctx context.Context, orgID int64) ([]domain.Membership, error) {
	const q = `SELECT ` + membershipColumns + ` FROM core.memberships m
		JOIN core.accounts a ON a.id = m.account_id
		WHERE m.organization_id = $1 ORDER BY m.joined_at`
	rows, err := r.db(ctx).Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	var out []domain.Membership
	for rows.Next() {
		m, err := scanMembership(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CountMembers возвращает число участников организации.
func (r *OrganizationRepo) CountMembers(ctx context.Context, orgID int64) (int, error) {
	var n int
	err := r.db(ctx).QueryRow(ctx, `SELECT count(*) FROM core.memberships WHERE organization_id = $1`, orgID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count members: %w", err)
	}
	return n, nil
}

// CreateMembership добавляет участника организации.
func (r *OrganizationRepo) CreateMembership(ctx context.Context, m domain.Membership, now time.Time) (domain.Membership, error) {
	started := time.Now()
	const q = `INSERT INTO core.memberships
		(organization_id, account_id, role, notify_enabled, notify_local_time, version, joined_at)
		VALUES ($1, $2, $3, $4, $5::time, 1, $6)`
	if _, err := r.db(ctx).Exec(ctx, q, m.OrganizationID, m.AccountID, string(m.Role),
		m.NotifyEnabled, m.NotifyLocalTime, now); err != nil {
		return domain.Membership{}, fmt.Errorf("create membership: %w", err)
	}
	r.observe("memberships.create", started)
	return r.GetMembership(ctx, m.OrganizationID, m.AccountID)
}

// UpdateMembership сохраняет роль и настройки уведомлений участника.
func (r *OrganizationRepo) UpdateMembership(ctx context.Context, m domain.Membership) (domain.Membership, error) {
	started := time.Now()
	const q = `UPDATE core.memberships
		SET role = $3, notify_enabled = $4, notify_local_time = $5::time, version = $6
		WHERE organization_id = $1 AND account_id = $2`
	tag, err := r.db(ctx).Exec(ctx, q, m.OrganizationID, m.AccountID, string(m.Role),
		m.NotifyEnabled, m.NotifyLocalTime, m.Version)
	r.observe("memberships.update", started)
	if err != nil {
		return domain.Membership{}, fmt.Errorf("update membership: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Membership{}, domain.ErrNotFound
	}
	return r.GetMembership(ctx, m.OrganizationID, m.AccountID)
}

// DeleteMembership удаляет участника организации.
func (r *OrganizationRepo) DeleteMembership(ctx context.Context, orgID, accountID int64) error {
	tag, err := r.db(ctx).Exec(ctx,
		`DELETE FROM core.memberships WHERE organization_id = $1 AND account_id = $2`, orgID, accountID)
	if err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListOwnedOrganizations возвращает организации, где пользователь владелец.
func (r *OrganizationRepo) ListOwnedOrganizations(ctx context.Context, accountID int64) ([]domain.Organization, error) {
	const q = `SELECT ` + orgColumns + ` FROM core.organizations
		WHERE id IN (SELECT organization_id FROM core.memberships WHERE account_id = $1 AND role = 'owner')`
	rows, err := r.db(ctx).Query(ctx, q, accountID)
	if err != nil {
		return nil, fmt.Errorf("list owned organizations: %w", err)
	}
	defer rows.Close()
	var out []domain.Organization
	for rows.Next() {
		o, err := scanOrg(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
