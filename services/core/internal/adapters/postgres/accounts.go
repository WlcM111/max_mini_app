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

// AccountRepo — аккаунты пользователей.
type AccountRepo struct{ base }

// NewAccountRepo создаёт репозиторий аккаунтов.
func NewAccountRepo(pool *pgkit.Pool, m *metrics.Registry) *AccountRepo {
	return &AccountRepo{base{pool: pool, metrics: m}}
}

var _ ports.AccountRepo = (*AccountRepo)(nil)

const accountColumns = `id, public_id::text, kind, coalesce(max_user_id, 0), coalesce(review_login, ''),
	first_name, coalesce(last_name, ''), coalesce(username, ''), coalesce(language_code, ''), created_at`

func scanAccount(scan func(dest ...any) error) (domain.Account, error) {
	var (
		a    domain.Account
		kind string
	)
	err := scan(&a.ID, &a.PublicID, &kind, &a.MaxUserID, &a.ReviewLogin, &a.FirstName,
		&a.LastName, &a.Username, &a.LanguageCode, &a.CreatedAt)
	a.Kind = domain.AccountKind(kind)
	return a, err
}

// UpsertMax создаёт или обновляет аккаунт пользователя MAX.
func (r *AccountRepo) UpsertMax(ctx context.Context, id ports.LaunchIdentity, now time.Time) (domain.Account, error) {
	started := time.Now()
	const q = `INSERT INTO core.accounts (kind, max_user_id, first_name, last_name, username, language_code, created_at, updated_at)
		VALUES ('max', $1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (max_user_id) DO UPDATE
		SET first_name = excluded.first_name, last_name = excluded.last_name,
		    username = excluded.username, language_code = excluded.language_code, updated_at = excluded.updated_at
		RETURNING ` + accountColumns
	acc, err := scanAccount(r.db(ctx).QueryRow(ctx, q, id.MaxUserID, id.FirstName,
		nilIfEmpty(id.LastName), nilIfEmpty(id.Username), nilIfEmpty(id.LanguageCode), now).Scan)
	r.observe("accounts.upsert_max", started)
	if err != nil {
		return domain.Account{}, fmt.Errorf("upsert max account: %w", err)
	}
	return acc, nil
}

// UpsertReview создаёт или обновляет учётную запись проверяющего.
func (r *AccountRepo) UpsertReview(ctx context.Context, login, firstName string, now time.Time) (domain.Account, error) {
	started := time.Now()
	const q = `INSERT INTO core.accounts (kind, review_login, first_name, created_at, updated_at)
		VALUES ('review', $1, $2, $3, $3)
		ON CONFLICT (review_login) DO UPDATE SET first_name = excluded.first_name, updated_at = excluded.updated_at
		RETURNING ` + accountColumns
	acc, err := scanAccount(r.db(ctx).QueryRow(ctx, q, login, firstName, now).Scan)
	r.observe("accounts.upsert_review", started)
	if err != nil {
		return domain.Account{}, fmt.Errorf("upsert review account: %w", err)
	}
	return acc, nil
}

// GetByID читает аккаунт по внутреннему идентификатору.
func (r *AccountRepo) GetByID(ctx context.Context, id int64) (domain.Account, error) {
	const q = `SELECT ` + accountColumns + ` FROM core.accounts WHERE id = $1`
	acc, err := scanAccount(r.db(ctx).QueryRow(ctx, q, id).Scan)
	if err != nil {
		return domain.Account{}, notFound(err)
	}
	return acc, nil
}

// GetByPublicID читает аккаунт по публичному идентификатору.
func (r *AccountRepo) GetByPublicID(ctx context.Context, publicID string) (domain.Account, error) {
	if !domain.IsUUIDv4(publicID) {
		return domain.Account{}, domain.ErrNotFound
	}
	const q = `SELECT ` + accountColumns + ` FROM core.accounts WHERE public_id = $1`
	acc, err := scanAccount(r.db(ctx).QueryRow(ctx, q, publicID).Scan)
	if err != nil {
		return domain.Account{}, notFound(err)
	}
	return acc, nil
}

// GetByReviewLogin читает учётную запись проверяющего.
func (r *AccountRepo) GetByReviewLogin(ctx context.Context, login string) (domain.Account, error) {
	const q = `SELECT ` + accountColumns + ` FROM core.accounts WHERE review_login = $1`
	acc, err := scanAccount(r.db(ctx).QueryRow(ctx, q, login).Scan)
	if err != nil {
		return domain.Account{}, notFound(err)
	}
	return acc, nil
}

// Delete удаляет аккаунт вместе с сессиями и членствами (каскад).
func (r *AccountRepo) Delete(ctx context.Context, id int64) error {
	started := time.Now()
	_, err := r.db(ctx).Exec(ctx, `DELETE FROM core.accounts WHERE id = $1`, id)
	r.observe("accounts.delete", started)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

// SessionRepo — серверные сессии.
type SessionRepo struct{ base }

// NewSessionRepo создаёт репозиторий сессий.
func NewSessionRepo(pool *pgkit.Pool, m *metrics.Registry) *SessionRepo {
	return &SessionRepo{base{pool: pool, metrics: m}}
}

var _ ports.SessionRepo = (*SessionRepo)(nil)

// Create сохраняет сессию.
func (r *SessionRepo) Create(ctx context.Context, tokenHash []byte, accountID int64,
	source, queryID, platform string, createdAt, expiresAt time.Time) error {
	started := time.Now()
	const q = `INSERT INTO core.sessions (token_hash, account_id, source, launch_query_id, platform,
		created_at, expires_at, last_seen_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $6)`
	_, err := r.db(ctx).Exec(ctx, q, tokenHash, accountID, source,
		nilIfEmpty(queryID), nilIfEmpty(platform), createdAt, expiresAt)
	r.observe("sessions.create", started)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// FindActive ищет действующую сессию по хешу токена.
func (r *SessionRepo) FindActive(ctx context.Context, tokenHash []byte, now time.Time) (domain.Account, domain.Session, error) {
	started := time.Now()
	// Столбцы аккаунта указываются с псевдонимом: в соединении с сессиями
	// имя id есть в обеих таблицах.
	const q = `SELECT a.id, a.public_id::text, a.kind, coalesce(a.max_user_id, 0), coalesce(a.review_login, ''),
		a.first_name, coalesce(a.last_name, ''), coalesce(a.username, ''), coalesce(a.language_code, ''), a.created_at,
		s.account_id, s.source, coalesce(s.launch_query_id, ''),
		coalesce(s.platform, ''), s.expires_at, s.last_seen_at
		FROM core.sessions s JOIN core.accounts a ON a.id = s.account_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > $2`
	var (
		acc     domain.Account
		kind    string
		session domain.Session
	)
	err := r.db(ctx).QueryRow(ctx, q, tokenHash, now).Scan(
		&acc.ID, &acc.PublicID, &kind, &acc.MaxUserID, &acc.ReviewLogin, &acc.FirstName,
		&acc.LastName, &acc.Username, &acc.LanguageCode, &acc.CreatedAt,
		&session.AccountID, &session.Source, &session.QueryID, &session.Platform,
		&session.ExpiresAt, &session.LastSeenAt)
	r.observe("sessions.find_active", started)
	if err != nil {
		return domain.Account{}, domain.Session{}, notFound(err)
	}
	acc.Kind = domain.AccountKind(kind)
	return acc, session, nil
}

// Touch обновляет момент последней активности сессии.
func (r *SessionRepo) Touch(ctx context.Context, tokenHash []byte, now time.Time) error {
	_, err := r.db(ctx).Exec(ctx, `UPDATE core.sessions SET last_seen_at = $2 WHERE token_hash = $1`, tokenHash, now)
	return err
}

// RevokeByHash отзывает сессию по хешу токена.
func (r *SessionRepo) RevokeByHash(ctx context.Context, tokenHash []byte, now time.Time) error {
	_, err := r.db(ctx).Exec(ctx,
		`UPDATE core.sessions SET revoked_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash, now)
	return err
}

// RevokeByAccount отзывает все сессии аккаунта.
func (r *SessionRepo) RevokeByAccount(ctx context.Context, accountID int64, now time.Time) error {
	_, err := r.db(ctx).Exec(ctx,
		`UPDATE core.sessions SET revoked_at = $2 WHERE account_id = $1 AND revoked_at IS NULL`, accountID, now)
	return err
}

// DeleteExpiredBefore удаляет просроченные сессии.
func (r *SessionRepo) DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM core.sessions WHERE expires_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}
