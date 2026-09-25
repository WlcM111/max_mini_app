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

// InboundRepo — журнал дедупликации событий webhook.
type InboundRepo struct{ base }

// NewInboundRepo создаёт репозиторий журнала событий.
func NewInboundRepo(pool *pgkit.Pool, m *metrics.Registry) *InboundRepo {
	return &InboundRepo{base{pool: pool, metrics: m}}
}

var _ ports.InboundRepo = (*InboundRepo)(nil)

// Insert записывает событие; повтор того же тела не создаёт новой строки.
func (r *InboundRepo) Insert(ctx context.Context, rec domain.InboundRecord) (bool, error) {
	started := time.Now()
	const q = `INSERT INTO bot.inbound_updates (dedupe_key, update_type, event_time, max_user_id, outcome, received_at)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (dedupe_key) DO NOTHING`
	key := rec.Key
	tag, err := r.db(ctx).Exec(ctx, q, key[:], rec.UpdateType, rec.EventTime,
		nilIfZero(rec.MaxUserID), string(rec.Outcome), rec.ReceivedAt)
	r.observe("inbound.insert", started)
	if err != nil {
		return false, fmt.Errorf("insert inbound update: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// DeleteBefore удаляет записи журнала, полученные раньше before.
func (r *InboundRepo) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	started := time.Now()
	const q = `DELETE FROM bot.inbound_updates WHERE received_at < $1`
	tag, err := r.db(ctx).Exec(ctx, q, before)
	r.observe("inbound.delete_before", started)
	if err != nil {
		return 0, fmt.Errorf("delete inbound updates: %w", err)
	}
	return tag.RowsAffected(), nil
}
