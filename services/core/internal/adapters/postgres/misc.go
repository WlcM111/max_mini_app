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

// ExportRepo — одноразовые ссылки на файл календаря.
type ExportRepo struct{ base }

// NewExportRepo создаёт репозиторий экспортов.
func NewExportRepo(pool *pgkit.Pool, m *metrics.Registry) *ExportRepo {
	return &ExportRepo{base{pool: pool, metrics: m}}
}

var _ ports.ExportRepo = (*ExportRepo)(nil)

// Create сохраняет одноразовую ссылку.
func (r *ExportRepo) Create(ctx context.Context, tokenHash []byte, orgID, accountID int64,
	createdAt, expiresAt time.Time) error {
	_, err := r.db(ctx).Exec(ctx, `INSERT INTO core.calendar_exports
		(token_hash, organization_id, account_id, created_at, expires_at) VALUES ($1, $2, $3, $4, $5)`,
		tokenHash, orgID, accountID, createdAt, expiresAt)
	if err != nil {
		return fmt.Errorf("create calendar export: %w", err)
	}
	return nil
}

// Consume увеличивает счётчик скачиваний и возвращает организацию.
func (r *ExportRepo) Consume(ctx context.Context, tokenHash []byte, now time.Time) (int64, error) {
	const q = `UPDATE core.calendar_exports SET download_count = download_count + 1
		WHERE token_hash = $1 AND expires_at > $2 AND download_count < 3
		RETURNING organization_id`
	var orgID int64
	if err := r.db(ctx).QueryRow(ctx, q, tokenHash, now).Scan(&orgID); err != nil {
		return 0, notFound(err)
	}
	return orgID, nil
}

// Exists сообщает, существует ли ссылка (для различения 404 и 410).
func (r *ExportRepo) Exists(ctx context.Context, tokenHash []byte) (bool, error) {
	var exists bool
	err := r.db(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM core.calendar_exports WHERE token_hash = $1)`, tokenHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check export: %w", err)
	}
	return exists, nil
}

// DeleteExpiredBefore удаляет просроченные ссылки.
func (r *ExportRepo) DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM core.calendar_exports WHERE expires_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired exports: %w", err)
	}
	return tag.RowsAffected(), nil
}

// AuditRepo — журнал значимых действий.
type AuditRepo struct{ base }

// NewAuditRepo создаёт репозиторий аудита.
func NewAuditRepo(pool *pgkit.Pool, m *metrics.Registry) *AuditRepo {
	return &AuditRepo{base{pool: pool, metrics: m}}
}

var _ ports.AuditRepo = (*AuditRepo)(nil)

// Write добавляет запись журнала.
func (r *AuditRepo) Write(ctx context.Context, action string, accountID, orgID int64,
	targetPublicID string, now time.Time) error {
	_, err := r.db(ctx).Exec(ctx, `INSERT INTO core.audit_events
		(occurred_at, account_id, organization_id, action, target_public_id) VALUES ($1, $2, $3, $4, $5)`,
		now, nilIfZero(accountID), nilIfZero(orgID), action, nilIfEmpty(targetPublicID))
	if err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

// DeleteBefore удаляет записи журнала старше указанного момента.
func (r *AuditRepo) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM core.audit_events WHERE occurred_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete audit events: %w", err)
	}
	return tag.RowsAffected(), nil
}

// OutboxRepo — исходящие события для reminders-service.
type OutboxRepo struct{ base }

// NewOutboxRepo создаёт репозиторий outbox.
func NewOutboxRepo(pool *pgkit.Pool, m *metrics.Registry) *OutboxRepo {
	return &OutboxRepo{base{pool: pool, metrics: m}}
}

var _ ports.OutboxRepo = (*OutboxRepo)(nil)

const outboxColumns = `id, event_id::text, event_type, aggregate_type, aggregate_id, aggregate_version,
	payload, snapshot, status, attempts, next_attempt_at, coalesce(last_error_code, ''), created_at`

func scanOutbox(scan func(dest ...any) error) (domain.OutboxEvent, error) {
	var e domain.OutboxEvent
	err := scan(&e.ID, &e.EventID, &e.EventType, &e.AggregateType, &e.AggregateID, &e.AggregateVersion,
		&e.Payload, &e.Snapshot, &e.Status, &e.Attempts, &e.NextAttemptAt, &e.LastErrorCode, &e.CreatedAt)
	return e, err
}

// Append записывает событие; вызывается в транзакции изменения данных.
func (r *OutboxRepo) Append(ctx context.Context, e domain.OutboxEvent) error {
	if !pgkit.InTx(ctx) {
		return fmt.Errorf("событие outbox пишется только в транзакции изменения данных")
	}
	started := time.Now()
	const q = `INSERT INTO core.outbox_events
		(event_id, event_type, aggregate_type, aggregate_id, aggregate_version, payload, snapshot,
		 status, attempts, next_attempt_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', 0, $8, $9)`
	_, err := r.db(ctx).Exec(ctx, q, e.EventID, e.EventType, e.AggregateType, e.AggregateID,
		e.AggregateVersion, e.Payload, e.Snapshot, e.NextAttemptAt, e.CreatedAt)
	r.observe("outbox.append", started)
	if err != nil {
		return fmt.Errorf("append outbox event: %w", err)
	}
	return nil
}

// ClaimReady захватывает пакет готовых событий с арендой.
func (r *OutboxRepo) ClaimReady(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.OutboxEvent, error) {
	started := time.Now()
	const q = `UPDATE core.outbox_events e SET next_attempt_at = $2
		WHERE e.id IN (
			SELECT id FROM core.outbox_events
			WHERE status <> 'sent' AND next_attempt_at <= $1
			ORDER BY id
			LIMIT $3
			FOR UPDATE SKIP LOCKED)
		RETURNING ` + outboxColumns
	rows, err := r.db(ctx).Query(ctx, q, now, now.Add(lease), limit)
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer rows.Close()
	var out []domain.OutboxEvent
	for rows.Next() {
		e, err := scanOutbox(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	r.observe("outbox.claim", started)
	return out, nil
}

// MarkSent отмечает событие доставленным.
func (r *OutboxRepo) MarkSent(ctx context.Context, id int64, now time.Time) error {
	_, err := r.db(ctx).Exec(ctx,
		`UPDATE core.outbox_events SET status = 'sent', sent_at = $2, last_error_code = NULL WHERE id = $1`, id, now)
	if err != nil {
		return fmt.Errorf("mark outbox sent: %w", err)
	}
	return nil
}

// MarkFailed откладывает следующую попытку доставки.
func (r *OutboxRepo) MarkFailed(ctx context.Context, id int64, nextAttemptAt time.Time, errorCode string) error {
	_, err := r.db(ctx).Exec(ctx, `UPDATE core.outbox_events
		SET status = 'failed', attempts = attempts + 1, next_attempt_at = $2, last_error_code = $3
		WHERE id = $1 AND status <> 'sent'`, id, nextAttemptAt, errorCode)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}

// PendingCount возвращает число недоставленных событий.
func (r *OutboxRepo) PendingCount(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db(ctx).QueryRow(ctx,
		`SELECT count(*) FROM core.outbox_events WHERE status <> 'sent'`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count pending outbox: %w", err)
	}
	return n, nil
}

// PendingForAggregates сообщает, по каким агрегатам есть недоставленные события.
func (r *OutboxRepo) PendingForAggregates(ctx context.Context, aggregateType string, ids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db(ctx).Query(ctx, `SELECT DISTINCT aggregate_id FROM core.outbox_events
		WHERE status <> 'sent' AND aggregate_type = $1 AND aggregate_id = ANY($2)`, aggregateType, ids)
	if err != nil {
		return nil, fmt.Errorf("pending aggregates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// HasPending сообщает, есть ли недоставленные события по агрегату.
func (r *OutboxRepo) HasPending(ctx context.Context, aggregateType, aggregateID string) (bool, error) {
	var exists bool
	err := r.db(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM core.outbox_events
		WHERE status <> 'sent' AND aggregate_type = $1 AND aggregate_id = $2)`, aggregateType, aggregateID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has pending: %w", err)
	}
	return exists, nil
}

// DeleteSentBefore удаляет доставленные события старше указанного момента.
func (r *OutboxRepo) DeleteSentBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db(ctx).Exec(ctx,
		`DELETE FROM core.outbox_events WHERE status = 'sent' AND sent_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete sent outbox: %w", err)
	}
	return tag.RowsAffected(), nil
}
