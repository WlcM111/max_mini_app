package postgres

import (
	"context"
	"fmt"
	"time"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// RecipientRepo — состояния диалогов с получателями.
type RecipientRepo struct{ base }

// NewRecipientRepo создаёт репозиторий получателей.
func NewRecipientRepo(pool *pgkit.Pool, m *metrics.Registry) *RecipientRepo {
	return &RecipientRepo{base{pool: pool, metrics: m}}
}

var _ ports.RecipientRepo = (*RecipientRepo)(nil)

const recipientColumns = `max_user_id, state, state_changed_at, last_event_time, last_delivery_at`

func scanRecipient(scan func(dest ...any) error) (domain.Recipient, error) {
	var (
		r     domain.Recipient
		state string
	)
	if err := scan(&r.MaxUserID, &state, &r.StateChangedAt, &r.LastEventTime, &r.LastDeliveryAt); err != nil {
		return domain.Recipient{}, err
	}
	r.State = domain.RecipientState(state)
	return r, nil
}

// Get читает состояние получателя.
func (r *RecipientRepo) Get(ctx context.Context, maxUserID int64) (domain.Recipient, bool, error) {
	started := time.Now()
	const q = `SELECT ` + recipientColumns + ` FROM bot.recipients WHERE max_user_id = $1`
	rec, err := scanRecipient(r.db(ctx).QueryRow(ctx, q, maxUserID).Scan)
	r.observe("recipients.get", started)
	if err != nil {
		if notFound(err) == domain.ErrNotFound {
			return domain.Recipient{MaxUserID: maxUserID, State: domain.RecipientUnknown}, false, nil
		}
		return domain.Recipient{}, false, fmt.Errorf("get recipient: %w", err)
	}
	return rec, true, nil
}

// LockOrCreate создаёт при необходимости строку получателя и блокирует её.
func (r *RecipientRepo) LockOrCreate(ctx context.Context, maxUserID int64, now time.Time) (domain.Recipient, error) {
	if !pgkit.InTx(ctx) {
		return domain.Recipient{}, errNoTx
	}
	started := time.Now()
	const qi = `INSERT INTO bot.recipients (max_user_id, state, state_changed_at, updated_at)
		VALUES ($1, 'unknown', $2, $2) ON CONFLICT (max_user_id) DO NOTHING`
	if _, err := r.db(ctx).Exec(ctx, qi, maxUserID, now); err != nil {
		return domain.Recipient{}, fmt.Errorf("create recipient: %w", err)
	}
	const qs = `SELECT ` + recipientColumns + ` FROM bot.recipients WHERE max_user_id = $1 FOR UPDATE`
	rec, err := scanRecipient(r.db(ctx).QueryRow(ctx, qs, maxUserID).Scan)
	r.observe("recipients.lock_or_create", started)
	if err != nil {
		return domain.Recipient{}, fmt.Errorf("lock recipient: %w", err)
	}
	return rec, nil
}

// Save записывает состояние получателя.
func (r *RecipientRepo) Save(ctx context.Context, rec domain.Recipient, now time.Time) error {
	started := time.Now()
	const q = `UPDATE bot.recipients
		SET state = $2, state_changed_at = $3, last_event_time = $4, last_delivery_at = $5, updated_at = $6
		WHERE max_user_id = $1`
	_, err := r.db(ctx).Exec(ctx, q, rec.MaxUserID, string(rec.State), rec.StateChangedAt,
		rec.LastEventTime, rec.LastDeliveryAt, now)
	r.observe("recipients.save", started)
	if err != nil {
		return fmt.Errorf("save recipient: %w", err)
	}
	return nil
}

// MarkDelivered фиксирует момент успешной доставки.
func (r *RecipientRepo) MarkDelivered(ctx context.Context, maxUserID int64, at time.Time) error {
	started := time.Now()
	const q = `INSERT INTO bot.recipients (max_user_id, state, state_changed_at, last_delivery_at, updated_at)
		VALUES ($1, 'unknown', $2, $2, $2)
		ON CONFLICT (max_user_id) DO UPDATE
		SET last_delivery_at = GREATEST(bot.recipients.last_delivery_at, excluded.last_delivery_at),
		    updated_at = excluded.updated_at`
	_, err := r.db(ctx).Exec(ctx, q, maxUserID, at)
	r.observe("recipients.mark_delivered", started)
	if err != nil {
		return fmt.Errorf("mark delivered: %w", err)
	}
	return nil
}

// DeleteStoppedInactive удаляет остановленные диалоги без событий после before.
func (r *RecipientRepo) DeleteStoppedInactive(ctx context.Context, before time.Time) (int64, error) {
	started := time.Now()
	const q = `DELETE FROM bot.recipients
		WHERE state = 'stopped' AND coalesce(last_event_time, state_changed_at) < $1`
	tag, err := r.db(ctx).Exec(ctx, q, before)
	r.observe("recipients.delete_stopped", started)
	if err != nil {
		return 0, fmt.Errorf("delete stopped recipients: %w", err)
	}
	return tag.RowsAffected(), nil
}
