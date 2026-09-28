package postgres

import (
	"context"
	"fmt"
	"time"

	"vovremya/services/reminders/internal/ports"
)

// ListDigestRecipients — участники, получающие напоминания, с названием и поясом организации.
func (r *ProjectionRepo) ListDigestRecipients(ctx context.Context) ([]ports.DigestRecipient, error) {
	rows, err := r.db(ctx).Query(ctx, `
SELECT m.organization_id::text, m.account_id::text, m.max_user_id, m.notify_local_minutes, o.name, o.timezone
FROM reminders.members m
JOIN reminders.organizations o ON o.organization_id = m.organization_id
WHERE NOT m.removed AND m.notify_enabled AND m.account_kind = 'max' AND m.max_user_id > 0 AND NOT o.deleted`)
	if err != nil {
		return nil, fmt.Errorf("list digest recipients: %w", err)
	}
	defer rows.Close()
	var out []ports.DigestRecipient
	for rows.Next() {
		var d ports.DigestRecipient
		var minutes int16
		if err := rows.Scan(&d.OrganizationID, &d.AccountID, &d.MaxUserID, &minutes, &d.OrganizationName, &d.Timezone); err != nil {
			return nil, fmt.Errorf("scan digest recipient: %w", err)
		}
		d.NotifyLocalMinutes = int(minutes)
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListDigestDocuments — документы организации со сроком не позже until (включая просроченные).
func (r *ProjectionRepo) ListDigestDocuments(ctx context.Context, organizationID string, until time.Time) ([]ports.DigestDocument, error) {
	rows, err := r.db(ctx).Query(ctx, `
SELECT title, valid_until, coalesce(responsible_account_id::text, '')
FROM reminders.documents
WHERE organization_id = $1::uuid AND NOT deleted AND valid_until IS NOT NULL AND valid_until <= $2::date
ORDER BY valid_until, title
LIMIT 50`, organizationID, until.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list digest documents: %w", err)
	}
	defer rows.Close()
	var out []ports.DigestDocument
	for rows.Next() {
		var d ports.DigestDocument
		if err := rows.Scan(&d.Title, &d.ValidUntil, &d.ResponsibleAccountID); err != nil {
			return nil, fmt.Errorf("scan digest document: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
