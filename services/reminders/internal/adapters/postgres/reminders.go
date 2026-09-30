package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/internal/ports"

	"github.com/jackc/pgx/v5"
)

// ReminderRepo хранит план напоминаний сервиса.
type ReminderRepo struct{ base }

// NewReminderRepo создаёт репозиторий плана.
func NewReminderRepo(pool *pgkit.Pool, m *metrics.Registry) *ReminderRepo {
	return &ReminderRepo{base{pool: pool, metrics: m}}
}

const upsertReminderSQL = `
INSERT INTO reminders.reminders AS r (
    document_id, organization_id, period_id, account_id, days_before,
    due_at, status, attempts, next_attempt_at, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, 'planned', 0, $6, now(), now())
ON CONFLICT (period_id, account_id, days_before) WHERE kind = 'offset' DO UPDATE
SET document_id = excluded.document_id,
    organization_id = excluded.organization_id,
    due_at = excluded.due_at,
    next_attempt_at = excluded.due_at,
    status = 'planned',
    attempts = 0,
    last_error_code = NULL,
    bot_notification_id = NULL,
    handed_off_at = NULL,
    updated_at = now()
WHERE r.status <> 'handed_off'`

// Upsert добавляет или обновляет напоминание плана.
// Уже переданные в bot-service напоминания (handed_off) не изменяются:
// сообщение принято очередью и повторная отправка не предполагается.
func (r *ReminderRepo) Upsert(ctx context.Context, rem domain.Reminder) error {
	started := time.Now()
	defer r.observe("reminder_upsert", started)
	_, err := r.db(ctx).Exec(ctx, upsertReminderSQL,
		rem.DocumentID, rem.OrganizationID, rem.Key.PeriodID, rem.Key.AccountID,
		int16(rem.Key.DaysBefore), rem.DueAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert reminder: %w", err)
	}
	return nil
}

// CancelOutsideKeys отменяет запланированные напоминания документа,
// которых нет в новом плане.
func (r *ReminderRepo) CancelOutsideKeys(ctx context.Context, documentID string, keep []domain.PlanKey) (int64, error) {
	started := time.Now()
	defer r.observe("reminder_cancel_outside", started)
	keys := make([]string, 0, len(keep))
	recipients := make([]string, 0, len(keep))
	seen := make(map[string]bool, len(keep))
	for _, k := range keep {
		keys = append(keys, fmt.Sprintf("%s:%s:%d", k.PeriodID, k.AccountID, k.DaysBefore))
		pair := k.PeriodID + ":" + k.AccountID
		if !seen[pair] {
			seen[pair] = true
			recipients = append(recipients, pair)
		}
	}
	// Отложенный повтор сохраняется, пока получатель остаётся в плане текущего периода:
	// правка названия его не отменяет, а продление, исключение из плана или отключение
	// уведомлений — отменяют (ADR-036).
	tag, err := r.db(ctx).Exec(ctx, `
UPDATE reminders.reminders r
SET status = 'cancelled', updated_at = now()
WHERE r.document_id = $1::uuid
  AND r.status = 'planned'
  AND CASE WHEN r.kind = 'snooze'
           THEN (r.period_id::text || ':' || r.account_id::text) <> ALL($3::text[])
           ELSE (r.period_id::text || ':' || r.account_id::text || ':' || r.days_before::text) <> ALL($2::text[])
      END`,
		documentID, keys, recipients)
	if err != nil {
		return 0, fmt.Errorf("cancel obsolete reminders: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *ReminderRepo) cancelBy(ctx context.Context, name, where string, args ...any) (int64, error) {
	started := time.Now()
	defer r.observe(name, started)
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE reminders.reminders r SET status = 'cancelled', updated_at = now() WHERE r.status = 'planned' AND `+where, args...)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return tag.RowsAffected(), nil
}

// CancelByDocument отменяет все запланированные напоминания документа.
func (r *ReminderRepo) CancelByDocument(ctx context.Context, documentID string) (int64, error) {
	return r.cancelBy(ctx, "reminder_cancel_document", "r.document_id = $1::uuid", documentID)
}

// CancelByOrganization отменяет запланированные напоминания организации.
func (r *ReminderRepo) CancelByOrganization(ctx context.Context, orgID string) (int64, error) {
	return r.cancelBy(ctx, "reminder_cancel_organization", "r.organization_id = $1::uuid", orgID)
}

// CancelByMember отменяет запланированные напоминания участника организации.
func (r *ReminderRepo) CancelByMember(ctx context.Context, orgID, accountID string) (int64, error) {
	return r.cancelBy(ctx, "reminder_cancel_member",
		"r.organization_id = $1::uuid AND r.account_id = $2::uuid", orgID, accountID)
}

// CancelByAccount отменяет запланированные напоминания получателя везде.
func (r *ReminderRepo) CancelByAccount(ctx context.Context, accountID string) (int64, error) {
	return r.cancelBy(ctx, "reminder_cancel_account", "r.account_id = $1::uuid", accountID)
}

const claimDueSQL = `
UPDATE reminders.reminders r
SET next_attempt_at = $2, updated_at = now()
FROM (
    SELECT id FROM reminders.reminders
    WHERE status = 'planned' AND next_attempt_at <= $1
    ORDER BY next_attempt_at
    LIMIT $3
    FOR UPDATE SKIP LOCKED
) s
WHERE r.id = s.id
RETURNING r.id, r.document_id::text, r.organization_id::text, r.period_id::text,
          r.account_id::text, r.days_before, coalesce(r.snooze_day, 0), r.due_at, r.status, r.attempts,
          r.next_attempt_at, coalesce(r.last_error_code, '')`

// ClaimDue захватывает пакет готовых напоминаний и продлевает аренду.
// FOR UPDATE SKIP LOCKED исключает одновременный захват одной строки
// несколькими экземплярами сервиса; аренда возвращает строку в работу,
// если обработчик завершился аварийно.
func (r *ReminderRepo) ClaimDue(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Reminder, error) {
	started := time.Now()
	defer r.observe("reminder_claim_due", started)
	rows, err := r.db(ctx).Query(ctx, claimDueSQL, now.UTC(), now.UTC().Add(lease), limit)
	if err != nil {
		return nil, fmt.Errorf("claim due reminders: %w", err)
	}
	defer rows.Close()
	var out []domain.Reminder
	for rows.Next() {
		rem, err := scanReminder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rem)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due reminders: %w", err)
	}
	return out, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanReminder(row scanner) (domain.Reminder, error) {
	var (
		rem    domain.Reminder
		days   int16
		snooze int32
		status string
		att    int16
	)
	if err := row.Scan(&rem.ID, &rem.DocumentID, &rem.OrganizationID, &rem.Key.PeriodID,
		&rem.Key.AccountID, &days, &snooze, &rem.DueAt, &status, &att, &rem.NextAttemptAt, &rem.LastErrorCode); err != nil {
		return domain.Reminder{}, fmt.Errorf("scan reminder: %w", err)
	}
	rem.Key.DaysBefore = int(days)
	rem.Key.SnoozeDay = int(snooze)
	rem.Status = domain.Status(status)
	rem.Attempts = int(att)
	return rem, nil
}

// MarkHandedOff фиксирует приём напоминания bot-service.
// Возвращает false, если строка изменилась параллельно (например, отменена).
func (r *ReminderRepo) MarkHandedOff(ctx context.Context, id int64, notificationID string, at time.Time) (bool, error) {
	started := time.Now()
	defer r.observe("reminder_handed_off", started)
	tag, err := r.db(ctx).Exec(ctx, `
UPDATE reminders.reminders
SET status = 'handed_off', handed_off_at = $3, bot_notification_id = $2::uuid,
    last_error_code = NULL, updated_at = now()
WHERE id = $1 AND status = 'planned'`, id, nullString(notificationID), at.UTC())
	if err != nil {
		return false, fmt.Errorf("mark handed off: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Reschedule откладывает повтор передачи после временной ошибки.
func (r *ReminderRepo) Reschedule(ctx context.Context, id int64, nextAttemptAt time.Time, errorCode string) (bool, error) {
	started := time.Now()
	defer r.observe("reminder_reschedule", started)
	tag, err := r.db(ctx).Exec(ctx, `
UPDATE reminders.reminders
SET attempts = attempts + 1, next_attempt_at = $2, last_error_code = $3, updated_at = now()
WHERE id = $1 AND status = 'planned'`, id, nextAttemptAt.UTC(), errorCode)
	if err != nil {
		return false, fmt.Errorf("reschedule reminder: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// MarkSkipped помечает напоминание как неотправляемое.
func (r *ReminderRepo) MarkSkipped(ctx context.Context, id int64, errorCode string) (bool, error) {
	started := time.Now()
	defer r.observe("reminder_skip", started)
	tag, err := r.db(ctx).Exec(ctx, `
UPDATE reminders.reminders
SET status = 'skipped', last_error_code = $2, updated_at = now()
WHERE id = $1 AND status = 'planned'`, id, errorCode)
	if err != nil {
		return false, fmt.Errorf("mark skipped: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// NextForAccount возвращает ближайшее запланированное напоминание по документам.
func (r *ReminderRepo) NextForAccount(ctx context.Context, accountID string, documentIDs []string) (map[string]domain.Reminder, error) {
	started := time.Now()
	defer r.observe("reminder_next_for_account", started)
	rows, err := r.db(ctx).Query(ctx, `
SELECT DISTINCT ON (document_id)
       id, document_id::text, organization_id::text, period_id::text, account_id::text,
       days_before, coalesce(snooze_day, 0), due_at, status, attempts, next_attempt_at, coalesce(last_error_code, '')
FROM reminders.reminders
WHERE account_id = $1::uuid AND document_id = ANY($2::uuid[]) AND status = 'planned'
ORDER BY document_id, due_at`, accountID, documentIDs)
	if err != nil {
		return nil, fmt.Errorf("next reminders: %w", err)
	}
	defer rows.Close()
	out := make(map[string]domain.Reminder)
	for rows.Next() {
		var (
			rem    domain.Reminder
			days   int16
			snooze int32
			status string
			att    int16
		)
		if err := rows.Scan(&rem.ID, &rem.DocumentID, &rem.OrganizationID, &rem.Key.PeriodID,
			&rem.Key.AccountID, &days, &snooze, &rem.DueAt, &status, &att, &rem.NextAttemptAt, &rem.LastErrorCode); err != nil {
			return nil, fmt.Errorf("scan next reminder: %w", err)
		}
		rem.Key.DaysBefore = int(days)
		rem.Key.SnoozeDay = int(snooze)
		rem.Status = domain.Status(status)
		rem.Attempts = int(att)
		out[rem.DocumentID] = rem
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate next reminders: %w", err)
	}
	return out, nil
}

// ListByDocument возвращает полный план документа.
func (r *ReminderRepo) ListByDocument(ctx context.Context, documentID string) ([]domain.Reminder, error) {
	started := time.Now()
	defer r.observe("reminder_list_document", started)
	rows, err := r.db(ctx).Query(ctx, `
SELECT id, document_id::text, organization_id::text, period_id::text, account_id::text,
       days_before, coalesce(snooze_day, 0), due_at, status, attempts, next_attempt_at, coalesce(last_error_code, ''), handed_off_at
FROM reminders.reminders
WHERE document_id = $1::uuid
ORDER BY due_at, account_id, days_before DESC`, documentID)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()
	var out []domain.Reminder
	for rows.Next() {
		var (
			rem    domain.Reminder
			days   int16
			snooze int32
			status string
			att    int16
			handed *time.Time
		)
		if err := rows.Scan(&rem.ID, &rem.DocumentID, &rem.OrganizationID, &rem.Key.PeriodID,
			&rem.Key.AccountID, &days, &snooze, &rem.DueAt, &status, &att, &rem.NextAttemptAt,
			&rem.LastErrorCode, &handed); err != nil {
			return nil, fmt.Errorf("scan reminder: %w", err)
		}
		rem.Key.DaysBefore = int(days)
		rem.Key.SnoozeDay = int(snooze)
		rem.Status = domain.Status(status)
		rem.Attempts = int(att)
		rem.HandedOffAt = handed
		out = append(out, rem)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reminders: %w", err)
	}
	return out, nil
}

// CountPlanned возвращает число запланированных напоминаний.
func (r *ReminderRepo) CountPlanned(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db(ctx).QueryRow(ctx,
		`SELECT count(*) FROM reminders.reminders WHERE status = 'planned'`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count planned: %w", err)
	}
	return n, nil
}

// CountDue возвращает число напоминаний, готовых к отправке.
func (r *ReminderRepo) CountDue(ctx context.Context, now time.Time) (int64, error) {
	var n int64
	if err := r.db(ctx).QueryRow(ctx,
		`SELECT count(*) FROM reminders.reminders WHERE status = 'planned' AND next_attempt_at <= $1`,
		now.UTC()).Scan(&n); err != nil {
		return 0, fmt.Errorf("count due: %w", err)
	}
	return n, nil
}

// DeleteFinalizedBefore удаляет неактивные напоминания старше момента.
func (r *ReminderRepo) DeleteFinalizedBefore(ctx context.Context, before time.Time) (int64, error) {
	started := time.Now()
	defer r.observe("reminder_delete_finalized", started)
	tag, err := r.db(ctx).Exec(ctx, `
DELETE FROM reminders.reminders
WHERE status IN ('handed_off', 'cancelled', 'skipped') AND updated_at < $1`, before.UTC())
	if err != nil {
		return 0, fmt.Errorf("delete finalized reminders: %w", err)
	}
	return tag.RowsAffected(), nil
}

var _ ports.ReminderRepo = (*ReminderRepo)(nil)

// FindByKey возвращает напоминание по ключу плана — обычное или отложенный повтор.
func (r *ReminderRepo) FindByKey(ctx context.Context, key domain.PlanKey) (domain.Reminder, error) {
	started := time.Now()
	defer r.observe("reminder_find_by_key", started)
	row := r.db(ctx).QueryRow(ctx, `
SELECT id, document_id::text, organization_id::text, period_id::text, account_id::text,
       days_before, coalesce(snooze_day, 0), due_at, status, attempts, next_attempt_at, coalesce(last_error_code, '')
FROM reminders.reminders
WHERE period_id = $1::uuid AND account_id = $2::uuid
  AND ((kind = 'offset' AND days_before = $3 AND $4::integer = 0)
       OR (kind = 'snooze' AND snooze_day = $4::integer))`,
		key.PeriodID, key.AccountID, int16(key.DaysBefore), int32(key.SnoozeDay))
	rem, err := scanReminder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Reminder{}, domain.ErrNotFound
	}
	return rem, err
}

// InsertSnooze добавляет отложенный повтор в план. Возвращает false, если повтор
// этого дня для получателя и периода уже запланирован.
func (r *ReminderRepo) InsertSnooze(ctx context.Context, rem domain.Reminder) (bool, error) {
	started := time.Now()
	defer r.observe("reminder_insert_snooze", started)
	tag, err := r.db(ctx).Exec(ctx, `
INSERT INTO reminders.reminders (
    document_id, organization_id, period_id, account_id, days_before, kind, snooze_day,
    due_at, status, attempts, next_attempt_at, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, 'snooze', $6, $7, 'planned', 0, $7, now(), now())
ON CONFLICT (period_id, account_id, snooze_day) WHERE kind = 'snooze' DO NOTHING`,
		rem.DocumentID, rem.OrganizationID, rem.Key.PeriodID, rem.Key.AccountID,
		int16(rem.Key.DaysBefore), int32(rem.Key.SnoozeDay), rem.DueAt.UTC())
	if err != nil {
		return false, fmt.Errorf("insert snooze: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
