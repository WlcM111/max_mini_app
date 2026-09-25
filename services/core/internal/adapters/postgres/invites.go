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

// InviteRepo — приглашения в организацию.
type InviteRepo struct{ base }

// NewInviteRepo создаёт репозиторий приглашений.
func NewInviteRepo(pool *pgkit.Pool, m *metrics.Registry) *InviteRepo {
	return &InviteRepo{base{pool: pool, metrics: m}}
}

var _ ports.InviteRepo = (*InviteRepo)(nil)

const inviteColumns = `i.id, i.public_id::text, i.organization_id, o.public_id::text, o.name, i.role,
	coalesce(i.created_by, 0), coalesce(a.first_name, ''), i.created_at, i.expires_at,
	coalesce(i.accepted_by, 0), i.accepted_at, i.revoked_at`

func scanInvite(scan func(dest ...any) error) (domain.Invite, error) {
	var (
		inv  domain.Invite
		role string
	)
	err := scan(&inv.ID, &inv.PublicID, &inv.OrganizationID, &inv.OrganizationPubID, &inv.OrganizationName,
		&role, &inv.CreatedByAccountID, &inv.CreatedByFirstName, &inv.CreatedAt, &inv.ExpiresAt,
		&inv.AcceptedByID, &inv.AcceptedAt, &inv.RevokedAt)
	inv.Role = domain.Role(role)
	return inv, err
}

const inviteFrom = ` FROM core.invites i
	JOIN core.organizations o ON o.id = i.organization_id
	LEFT JOIN core.accounts a ON a.id = i.created_by `

// GetByPublicID читает приглашение по публичному идентификатору.
func (r *InviteRepo) GetByPublicID(ctx context.Context, publicID string) (domain.Invite, error) {
	const q = `SELECT ` + inviteColumns + inviteFrom + `WHERE i.public_id = $1`
	inv, err := scanInvite(r.db(ctx).QueryRow(ctx, q, publicID).Scan)
	if err != nil {
		return domain.Invite{}, notFound(err)
	}
	return inv, nil
}

// FindByTokenHash ищет приглашение по хешу токена.
func (r *InviteRepo) FindByTokenHash(ctx context.Context, tokenHash []byte) (domain.Invite, error) {
	const q = `SELECT ` + inviteColumns + inviteFrom + `WHERE i.token_hash = $1`
	inv, err := scanInvite(r.db(ctx).QueryRow(ctx, q, tokenHash).Scan)
	if err != nil {
		return domain.Invite{}, notFound(err)
	}
	return inv, nil
}

// LockByTokenHash блокирует приглашение на время транзакции.
func (r *InviteRepo) LockByTokenHash(ctx context.Context, tokenHash []byte) (domain.Invite, error) {
	const q = `SELECT ` + inviteColumns + inviteFrom + `WHERE i.token_hash = $1 FOR UPDATE OF i`
	inv, err := scanInvite(r.db(ctx).QueryRow(ctx, q, tokenHash).Scan)
	if err != nil {
		return domain.Invite{}, notFound(err)
	}
	return inv, nil
}

// ListActive возвращает действующие приглашения организации.
func (r *InviteRepo) ListActive(ctx context.Context, orgID int64, now time.Time) ([]domain.Invite, error) {
	const q = `SELECT ` + inviteColumns + inviteFrom + `WHERE i.organization_id = $1
		AND i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > $2
		ORDER BY i.created_at DESC`
	rows, err := r.db(ctx).Query(ctx, q, orgID, now)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	defer rows.Close()
	var out []domain.Invite
	for rows.Next() {
		inv, err := scanInvite(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// CountActive возвращает число действующих приглашений организации.
func (r *InviteRepo) CountActive(ctx context.Context, orgID int64, now time.Time) (int, error) {
	var n int
	err := r.db(ctx).QueryRow(ctx, `SELECT count(*) FROM core.invites
		WHERE organization_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > $2`,
		orgID, now).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count invites: %w", err)
	}
	return n, nil
}

// Create сохраняет приглашение.
func (r *InviteRepo) Create(ctx context.Context, inv domain.Invite, tokenHash []byte) (domain.Invite, error) {
	started := time.Now()
	const q = `INSERT INTO core.invites (public_id, organization_id, token_hash, role, created_by, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	var id int64
	err := r.db(ctx).QueryRow(ctx, q, inv.PublicID, inv.OrganizationID, tokenHash, string(inv.Role),
		nilIfZero(inv.CreatedByAccountID), inv.CreatedAt, inv.ExpiresAt).Scan(&id)
	r.observe("invites.create", started)
	if err != nil {
		return domain.Invite{}, fmt.Errorf("create invite: %w", err)
	}
	inv.ID = id
	return inv, nil
}

// MarkAccepted отмечает приглашение принятым.
func (r *InviteRepo) MarkAccepted(ctx context.Context, id, accountID int64, now time.Time) error {
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE core.invites SET accepted_by = $2, accepted_at = $3
		 WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL`, id, accountID, now)
	if err != nil {
		return fmt.Errorf("accept invite: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInviteInvalid
	}
	return nil
}

// MarkRevoked отмечает приглашение отозванным.
func (r *InviteRepo) MarkRevoked(ctx context.Context, id int64, now time.Time) error {
	_, err := r.db(ctx).Exec(ctx,
		`UPDATE core.invites SET revoked_at = $2 WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL`,
		id, now)
	if err != nil {
		return fmt.Errorf("revoke invite: %w", err)
	}
	return nil
}

// DeleteExpiredBefore удаляет давно истёкшие приглашения.
func (r *InviteRepo) DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM core.invites WHERE expires_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired invites: %w", err)
	}
	return tag.RowsAffected(), nil
}
