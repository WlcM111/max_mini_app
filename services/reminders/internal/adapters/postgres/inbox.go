package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/reminders/internal/ports"
)

// InboxRepo хранит журнал принятых событий core (дедупликация доставки).
type InboxRepo struct{ base }

// NewInboxRepo создаёт репозиторий журнала событий.
func NewInboxRepo(pool *pgkit.Pool, m *metrics.Registry) *InboxRepo {
	return &InboxRepo{base{pool: pool, metrics: m}}
}

const insertInboxSQL = `
INSERT INTO reminders.inbox_events (
    event_id, event_type, aggregate_type, aggregate_id, aggregate_version,
    schema_version, source_service, snapshot, outcome, occurred_at)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (event_id) DO NOTHING
RETURNING event_id::text`

// Insert регистрирует событие; inserted=false означает повторную доставку.
func (r *InboxRepo) Insert(ctx context.Context, rec ports.InboxRecord) (bool, error) {
	started := time.Now()
	defer r.observe("inbox_insert", started)
	var id string
	err := r.db(ctx).QueryRow(ctx, insertInboxSQL,
		rec.EventID, rec.EventType, rec.AggregateType, rec.AggregateID, int64(rec.AggregateVersion),
		int32(rec.SchemaVersion), rec.SourceService, rec.Snapshot, rec.Outcome, rec.OccurredAt.UTC()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("insert inbox event: %w", err)
	}
	return true, nil
}

// DeleteOlderThan удаляет записи журнала, полученные раньше указанного момента.
func (r *InboxRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	started := time.Now()
	defer r.observe("inbox_delete_old", started)
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM reminders.inbox_events WHERE received_at < $1`, before.UTC())
	if err != nil {
		return 0, fmt.Errorf("delete inbox events: %w", err)
	}
	return tag.RowsAffected(), nil
}

// LastAppliedAt возвращает момент последнего применённого события.
func (r *InboxRepo) LastAppliedAt(ctx context.Context) (time.Time, bool, error) {
	started := time.Now()
	defer r.observe("inbox_last_applied", started)
	var at *time.Time
	err := r.db(ctx).QueryRow(ctx,
		`SELECT max(occurred_at) FROM reminders.inbox_events WHERE outcome = 'applied'`).Scan(&at)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("last applied: %w", err)
	}
	if at == nil {
		return time.Time{}, false, nil
	}
	return *at, true, nil
}
