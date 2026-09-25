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

// MessageRepo — очередь исходящих сообщений.
type MessageRepo struct{ base }

// NewMessageRepo создаёт репозиторий очереди.
func NewMessageRepo(pool *pgkit.Pool, m *metrics.Registry) *MessageRepo {
	return &MessageRepo{base{pool: pool, metrics: m}}
}

var _ ports.MessageRepo = (*MessageRepo)(nil)

const messageColumns = `id, public_id::text, idempotency_key, request_hash, kind, recipient_max_user_id,
	text, silent, status, attempts, next_attempt_at, not_after, locked_until,
	max_message_id, last_error_code, created_at, sent_at, updated_at`

type messageRow struct {
	id            int64
	publicID      string
	key           string
	hash          []byte
	kind          string
	recipient     int64
	text          string
	silent        bool
	status        string
	attempts      int16
	nextAttemptAt time.Time
	notAfter      time.Time
	lockedUntil   *time.Time
	maxMessageID  *string
	lastError     *string
	createdAt     time.Time
	sentAt        *time.Time
	updatedAt     time.Time
}

func (r messageRow) toDomain() domain.Notification {
	return domain.Notification{
		ID:             r.id,
		PublicID:       r.publicID,
		IdempotencyKey: r.key,
		RequestHash:    r.hash,
		Message: domain.Message{
			Kind:               domain.Kind(r.kind),
			RecipientMaxUserID: r.recipient,
			Text:               r.text,
			Silent:             r.silent,
		},
		NotAfter:      r.notAfter,
		Status:        domain.Status(r.status),
		Attempts:      int(r.attempts),
		NextAttemptAt: r.nextAttemptAt,
		LockedUntil:   r.lockedUntil,
		MaxMessageID:  str(r.maxMessageID),
		LastErrorCode: str(r.lastError),
		CreatedAt:     r.createdAt,
		SentAt:        r.sentAt,
		UpdatedAt:     r.updatedAt,
	}
}

// scanMessage читает строку сообщения.
func scanMessage(scan func(dest ...any) error) (domain.Notification, error) {
	var row messageRow
	d := []any{&row.id, &row.publicID, &row.key, &row.hash, &row.kind, &row.recipient, &row.text, &row.silent,
		&row.status, &row.attempts, &row.nextAttemptAt, &row.notAfter, &row.lockedUntil, &row.maxMessageID,
		&row.lastError, &row.createdAt, &row.sentAt, &row.updatedAt}
	if err := scan(d...); err != nil {
		return domain.Notification{}, err
	}
	return row.toDomain(), nil
}

// Insert ставит сообщение в очередь вместе с кнопками.
func (r *MessageRepo) Insert(ctx context.Context, n domain.Notification) (domain.Notification, bool, error) {
	started := time.Now()
	const q = `INSERT INTO bot.outbound_messages
		(idempotency_key, request_hash, kind, recipient_max_user_id, text, silent, status,
		 attempts, next_attempt_at, not_after, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'queued', 0, $7, $8, $7, $7)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING ` + messageColumns
	stored, err := scanMessage(r.db(ctx).QueryRow(ctx, q, n.IdempotencyKey, n.RequestHash, string(n.Kind),
		n.RecipientMaxUserID, n.Text, n.Silent, n.NextAttemptAt, n.NotAfter).Scan)
	r.observe("messages.insert", started)
	if err != nil {
		if err = notFound(err); err == domain.ErrNotFound {
			return domain.Notification{}, false, nil // ключ уже существует
		}
		return domain.Notification{}, false, fmt.Errorf("insert outbound message: %w", err)
	}
	for i, b := range n.Buttons {
		const qb = `INSERT INTO bot.outbound_buttons (message_id, position, text, action, open_app_payload, url)
			VALUES ($1, $2, $3, $4, $5, $6)`
		var payload, url *string
		if b.Action == domain.ActionOpenApp {
			p := b.Payload
			payload = &p
		} else {
			u := b.URL
			url = &u
		}
		if _, err := r.db(ctx).Exec(ctx, qb, stored.ID, i+1, b.Text, string(b.Action), payload, url); err != nil {
			return domain.Notification{}, false, fmt.Errorf("insert outbound button: %w", err)
		}
	}
	stored.Buttons = n.Buttons
	return stored, true, nil
}

// GetByKey читает сообщение по ключу идемпотентности.
func (r *MessageRepo) GetByKey(ctx context.Context, key string) (domain.Notification, error) {
	started := time.Now()
	const q = `SELECT ` + messageColumns + ` FROM bot.outbound_messages WHERE idempotency_key = $1`
	n, err := scanMessage(r.db(ctx).QueryRow(ctx, q, key).Scan)
	r.observe("messages.get_by_key", started)
	if err != nil {
		return domain.Notification{}, notFound(err)
	}
	buttons, err := r.loadButtons(ctx, []int64{n.ID})
	if err != nil {
		return domain.Notification{}, err
	}
	n.Buttons = buttons[n.ID]
	return n, nil
}

// CountActive возвращает число сообщений в незавершённых состояниях.
func (r *MessageRepo) CountActive(ctx context.Context) (int64, error) {
	started := time.Now()
	const q = `SELECT count(*) FROM bot.outbound_messages WHERE status IN ('queued', 'sending', 'retry_wait')`
	var n int64
	err := r.db(ctx).QueryRow(ctx, q).Scan(&n)
	r.observe("messages.count_active", started)
	if err != nil {
		return 0, fmt.Errorf("count active messages: %w", err)
	}
	return n, nil
}

// ClaimReady захватывает готовые сообщения с арендой до now+lease.
func (r *MessageRepo) ClaimReady(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Notification, error) {
	started := time.Now()
	const q = `UPDATE bot.outbound_messages m
		SET status = 'sending', locked_until = $2, attempts = m.attempts + 1, updated_at = $1
		WHERE m.id IN (
			SELECT id FROM bot.outbound_messages
			WHERE status IN ('queued', 'retry_wait') AND next_attempt_at <= $1
			ORDER BY next_attempt_at
			LIMIT $3
			FOR UPDATE SKIP LOCKED)
		RETURNING ` + messageColumns
	rows, err := r.db(ctx).Query(ctx, q, now, now.Add(lease), limit)
	if err != nil {
		return nil, fmt.Errorf("claim messages: %w", err)
	}
	defer rows.Close()
	var out []domain.Notification
	ids := make([]int64, 0, limit)
	for rows.Next() {
		n, err := scanMessage(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan claimed message: %w", err)
		}
		out = append(out, n)
		ids = append(ids, n.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim messages: %w", err)
	}
	r.observe("messages.claim_ready", started)
	if len(ids) == 0 {
		return nil, nil
	}
	buttons, err := r.loadButtons(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Buttons = buttons[out[i].ID]
	}
	return out, nil
}

func (r *MessageRepo) loadButtons(ctx context.Context, ids []int64) (map[int64][]domain.Button, error) {
	started := time.Now()
	const q = `SELECT message_id, position, text, action, open_app_payload, url
		FROM bot.outbound_buttons WHERE message_id = ANY($1) ORDER BY message_id, position`
	rows, err := r.db(ctx).Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("load buttons: %w", err)
	}
	defer rows.Close()
	out := make(map[int64][]domain.Button, len(ids))
	for rows.Next() {
		var (
			id       int64
			position int16
			b        domain.Button
			action   string
			payload  *string
			url      *string
		)
		if err := rows.Scan(&id, &position, &b.Text, &action, &payload, &url); err != nil {
			return nil, fmt.Errorf("scan button: %w", err)
		}
		b.Action = domain.ButtonAction(action)
		b.Payload = str(payload)
		b.URL = str(url)
		out[id] = append(out[id], b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load buttons: %w", err)
	}
	r.observe("messages.load_buttons", started)
	return out, nil
}

// MarkSent фиксирует успешную отправку, если аренда не потеряна.
func (r *MessageRepo) MarkSent(ctx context.Context, id int64, lease time.Time, maxMessageID string, now time.Time) (bool, error) {
	started := time.Now()
	const q = `UPDATE bot.outbound_messages
		SET status = 'sent', sent_at = $4, max_message_id = $3, last_error_code = NULL,
		    locked_until = NULL, updated_at = $4
		WHERE id = $1 AND status = 'sending' AND locked_until = $2`
	tag, err := r.db(ctx).Exec(ctx, q, id, lease, nilIfEmpty(truncate(maxMessageID, 128)), now)
	r.observe("messages.mark_sent", started)
	if err != nil {
		return false, fmt.Errorf("mark message sent: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Finish переводит сообщение в конечное состояние или в ожидание повтора.
func (r *MessageRepo) Finish(ctx context.Context, id int64, lease time.Time, tr domain.Transition, now time.Time) (bool, error) {
	started := time.Now()
	const q = `UPDATE bot.outbound_messages
		SET status = $3,
		    next_attempt_at = CASE WHEN $3 = 'retry_wait' THEN $4::timestamptz ELSE next_attempt_at END,
		    last_error_code = $5, locked_until = NULL, updated_at = $6
		WHERE id = $1 AND status = 'sending' AND locked_until = $2`
	next := tr.NextAttemptAt
	if next.IsZero() {
		next = now
	}
	tag, err := r.db(ctx).Exec(ctx, q, id, lease, string(tr.Status), next, nilIfEmpty(tr.ErrorCode), now)
	r.observe("messages.finish", started)
	if err != nil {
		return false, fmt.Errorf("finish message: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Release возвращает сообщение в очередь, не засчитывая попытку.
func (r *MessageRepo) Release(ctx context.Context, id int64, lease time.Time, nextAttemptAt, now time.Time) (bool, error) {
	started := time.Now()
	const q = `UPDATE bot.outbound_messages
		SET status = CASE WHEN attempts - 1 <= 0 THEN 'queued' ELSE 'retry_wait' END,
		    attempts = GREATEST(attempts - 1, 0), next_attempt_at = $3,
		    locked_until = NULL, updated_at = $4
		WHERE id = $1 AND status = 'sending' AND locked_until = $2`
	tag, err := r.db(ctx).Exec(ctx, q, id, lease, nextAttemptAt, now)
	r.observe("messages.release", started)
	if err != nil {
		return false, fmt.Errorf("release message: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ReapExpiredLeases возвращает в очередь сообщения с истёкшей арендой.
func (r *MessageRepo) ReapExpiredLeases(ctx context.Context, now time.Time) (int64, error) {
	started := time.Now()
	const q = `UPDATE bot.outbound_messages
		SET status = 'retry_wait', locked_until = NULL, next_attempt_at = $1, updated_at = $1
		WHERE status = 'sending' AND locked_until < $1`
	tag, err := r.db(ctx).Exec(ctx, q, now)
	r.observe("messages.reap_leases", started)
	if err != nil {
		return 0, fmt.Errorf("reap leases: %w", err)
	}
	return tag.RowsAffected(), nil
}

// HasRecentKind сообщает, ставилось ли получателю сообщение вида kind после since.
func (r *MessageRepo) HasRecentKind(ctx context.Context, recipient int64, kind domain.Kind, since time.Time) (bool, error) {
	started := time.Now()
	const q = `SELECT EXISTS (SELECT 1 FROM bot.outbound_messages
		WHERE recipient_max_user_id = $1 AND kind = $2 AND created_at > $3)`
	var exists bool
	err := r.db(ctx).QueryRow(ctx, q, recipient, string(kind), since).Scan(&exists)
	r.observe("messages.has_recent_kind", started)
	if err != nil {
		return false, fmt.Errorf("check recent kind: %w", err)
	}
	return exists, nil
}

// DeleteFinalizedBefore удаляет завершённые сообщения старше before.
func (r *MessageRepo) DeleteFinalizedBefore(ctx context.Context, before time.Time) (int64, error) {
	started := time.Now()
	const q = `DELETE FROM bot.outbound_messages
		WHERE status IN ('sent', 'failed', 'expired') AND updated_at < $1`
	tag, err := r.db(ctx).Exec(ctx, q, before)
	r.observe("messages.delete_finalized", started)
	if err != nil {
		return 0, fmt.Errorf("delete finalized messages: %w", err)
	}
	return tag.RowsAffected(), nil
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
