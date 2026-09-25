package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// DocumentRepo — документы и периоды действия.
type DocumentRepo struct{ base }

// NewDocumentRepo создаёт репозиторий документов.
func NewDocumentRepo(pool *pgkit.Pool, m *metrics.Registry) *DocumentRepo {
	return &DocumentRepo{base{pool: pool, metrics: m}}
}

var _ ports.DocumentRepo = (*DocumentRepo)(nil)

const docColumns = `d.id, d.public_id::text, d.organization_id, o.public_id::text,
	coalesce(d.document_type_code, ''), d.title, coalesce(d.number, ''), coalesce(d.issuer, ''),
	coalesce(d.responsible_label, ''), coalesce(d.notes, ''), coalesce(d.reference_url, ''),
	d.version, coalesce(d.created_by, 0), d.created_at, d.updated_at,
	p.id, p.public_id::text, p.valid_from, p.valid_until, p.created_at`

func scanDocument(scan func(dest ...any) error) (domain.Document, error) {
	var d domain.Document
	err := scan(&d.ID, &d.PublicID, &d.OrganizationID, &d.OrganizationPubID, &d.DocumentTypeCode,
		&d.Title, &d.Number, &d.Issuer, &d.ResponsibleLabel, &d.Notes, &d.ReferenceURL,
		&d.Version, &d.CreatedByID, &d.CreatedAt, &d.UpdatedAt,
		&d.CurrentPeriod.ID, &d.CurrentPeriod.PublicID, &d.CurrentPeriod.ValidFrom,
		&d.CurrentPeriod.ValidUntil, &d.CurrentPeriod.CreatedAt)
	d.CurrentPeriod.DocumentID = d.ID
	d.CurrentPeriod.IsCurrent = true
	return d, err
}

func (r *DocumentRepo) loadOffsets(ctx context.Context, docIDs []int64) (map[int64][]int, error) {
	out := make(map[int64][]int, len(docIDs))
	if len(docIDs) == 0 {
		return out, nil
	}
	rows, err := r.db(ctx).Query(ctx, `SELECT document_id, days_before FROM core.document_reminder_offsets
		WHERE document_id = ANY($1) ORDER BY document_id, days_before DESC`, docIDs)
	if err != nil {
		return nil, fmt.Errorf("load reminder offsets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id   int64
			days int16
		)
		if err := rows.Scan(&id, &days); err != nil {
			return nil, err
		}
		out[id] = append(out[id], int(days))
	}
	return out, rows.Err()
}

func (r *DocumentRepo) attachOffsets(ctx context.Context, docs []domain.Document) error {
	ids := make([]int64, 0, len(docs))
	for _, d := range docs {
		ids = append(ids, d.ID)
	}
	offsets, err := r.loadOffsets(ctx, ids)
	if err != nil {
		return err
	}
	for i := range docs {
		docs[i].ReminderOffsets = offsets[docs[i].ID]
	}
	return nil
}

// GetByPublicID читает документ с текущим периодом.
func (r *DocumentRepo) GetByPublicID(ctx context.Context, publicID string) (domain.Document, error) {
	started := time.Now()
	const q = `SELECT ` + docColumns + ` FROM core.documents d
		JOIN core.organizations o ON o.id = d.organization_id
		JOIN core.document_periods p ON p.document_id = d.id AND p.is_current
		WHERE d.public_id = $1`
	doc, err := scanDocument(r.db(ctx).QueryRow(ctx, q, publicID).Scan)
	r.observe("documents.get", started)
	if err != nil {
		return domain.Document{}, notFound(err)
	}
	docs := []domain.Document{doc}
	if err := r.attachOffsets(ctx, docs); err != nil {
		return domain.Document{}, err
	}
	return docs[0], nil
}

// GetFull читает документ вместе с историей периодов.
func (r *DocumentRepo) GetFull(ctx context.Context, id int64) (domain.Document, error) {
	const q = `SELECT ` + docColumns + ` FROM core.documents d
		JOIN core.organizations o ON o.id = d.organization_id
		JOIN core.document_periods p ON p.document_id = d.id AND p.is_current
		WHERE d.id = $1`
	doc, err := scanDocument(r.db(ctx).QueryRow(ctx, q, id).Scan)
	if err != nil {
		return domain.Document{}, notFound(err)
	}
	docs := []domain.Document{doc}
	if err := r.attachOffsets(ctx, docs); err != nil {
		return domain.Document{}, err
	}
	doc = docs[0]

	rows, err := r.db(ctx).Query(ctx, `SELECT id, public_id::text, valid_from, valid_until, is_current, created_at
		FROM core.document_periods WHERE document_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`,
		id, domain.MaxPeriodsInDocumentResponse)
	if err != nil {
		return domain.Document{}, fmt.Errorf("load periods: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.Period
		if err := rows.Scan(&p.ID, &p.PublicID, &p.ValidFrom, &p.ValidUntil, &p.IsCurrent, &p.CreatedAt); err != nil {
			return domain.Document{}, err
		}
		p.DocumentID = id
		doc.Periods = append(doc.Periods, p)
	}
	return doc, rows.Err()
}

// List возвращает страницу реестра документов.
func (r *DocumentRepo) List(ctx context.Context, f ports.DocumentFilter) ([]domain.Document, error) {
	started := time.Now()
	const q = `SELECT ` + docColumns + ` FROM core.documents d
		JOIN core.organizations o ON o.id = d.organization_id
		JOIN core.document_periods p ON p.document_id = d.id AND p.is_current
		WHERE d.organization_id = $1
		  AND ($2::text IS NULL
		       OR ($2 = 'expired'   AND p.valid_until < $3::date)
		       OR ($2 = 'expiring'  AND p.valid_until >= $3::date AND p.valid_until <= $3::date + 30)
		       OR ($2 = 'valid'     AND p.valid_until > $3::date + 30)
		       OR ($2 = 'no_expiry' AND p.valid_until IS NULL))
		  AND ($4::text IS NULL OR d.title ILIKE '%' || $4 || '%' ESCAPE '\')
		  AND (coalesce(p.valid_until, 'infinity'::date), d.public_id) > ($5::date, $6::uuid)
		ORDER BY coalesce(p.valid_until, 'infinity'::date), d.public_id
		LIMIT $7`
	var status *string
	if f.Status != "" {
		s := string(f.Status)
		status = &s
	}
	var query *string
	if f.Query != "" {
		escaped := escapeLike(f.Query)
		query = &escaped
	}
	rows, err := r.db(ctx).Query(ctx, q, f.OrganizationID, status, f.Today, query, f.AfterUntil, f.AfterID, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()
	var out []domain.Document
	for rows.Next() {
		doc, err := scanDocument(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	r.observe("documents.list", started)
	if err := r.attachOffsets(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListCurrent возвращает все документы организации с текущими периодами.
func (r *DocumentRepo) ListCurrent(ctx context.Context, orgID int64) ([]domain.Document, error) {
	const q = `SELECT ` + docColumns + ` FROM core.documents d
		JOIN core.organizations o ON o.id = d.organization_id
		JOIN core.document_periods p ON p.document_id = d.id AND p.is_current
		WHERE d.organization_id = $1
		ORDER BY coalesce(p.valid_until, 'infinity'::date), d.public_id`
	rows, err := r.db(ctx).Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("list current documents: %w", err)
	}
	defer rows.Close()
	var out []domain.Document
	for rows.Next() {
		doc, err := scanDocument(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachOffsets(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Count возвращает число документов организации.
func (r *DocumentRepo) Count(ctx context.Context, orgID int64) (int, error) {
	var n int
	if err := r.db(ctx).QueryRow(ctx,
		`SELECT count(*) FROM core.documents WHERE organization_id = $1`, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count documents: %w", err)
	}
	return n, nil
}

// Create добавляет документ с текущим периодом и отступами.
func (r *DocumentRepo) Create(ctx context.Context, doc domain.Document, createdBy int64, now time.Time) (domain.Document, error) {
	started := time.Now()
	const qd = `INSERT INTO core.documents (public_id, organization_id, document_type_code, title, number,
		issuer, responsible_label, notes, reference_url, version, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, $10, $10, $11, $11) RETURNING id`
	var id int64
	err := r.db(ctx).QueryRow(ctx, qd, doc.PublicID, doc.OrganizationID, nilIfEmpty(doc.DocumentTypeCode),
		doc.Title, nilIfEmpty(doc.Number), nilIfEmpty(doc.Issuer), nilIfEmpty(doc.ResponsibleLabel),
		nilIfEmpty(doc.Notes), nilIfEmpty(doc.ReferenceURL), createdBy, now).Scan(&id)
	r.observe("documents.create", started)
	if err != nil {
		return domain.Document{}, fmt.Errorf("create document: %w", err)
	}
	const qp = `INSERT INTO core.document_periods (public_id, document_id, valid_from, valid_until, is_current, created_by, created_at)
		VALUES ($1, $2, $3, $4, true, $5, $6)`
	if _, err := r.db(ctx).Exec(ctx, qp, doc.CurrentPeriod.PublicID, id, doc.CurrentPeriod.ValidFrom,
		doc.CurrentPeriod.ValidUntil, createdBy, now); err != nil {
		return domain.Document{}, fmt.Errorf("create period: %w", err)
	}
	if err := r.ReplaceOffsets(ctx, id, doc.ReminderOffsets); err != nil {
		return domain.Document{}, err
	}
	return r.GetFull(ctx, id)
}

// Update сохраняет реквизиты документа и увеличивает версию.
func (r *DocumentRepo) Update(ctx context.Context, doc domain.Document, updatedBy int64, now time.Time) (domain.Document, error) {
	started := time.Now()
	const q = `UPDATE core.documents SET document_type_code = $2, title = $3, number = $4, issuer = $5,
		responsible_label = $6, notes = $7, reference_url = $8, version = version + 1,
		updated_by = $9, updated_at = $10 WHERE id = $1 RETURNING version`
	var version int
	err := r.db(ctx).QueryRow(ctx, q, doc.ID, nilIfEmpty(doc.DocumentTypeCode), doc.Title,
		nilIfEmpty(doc.Number), nilIfEmpty(doc.Issuer), nilIfEmpty(doc.ResponsibleLabel),
		nilIfEmpty(doc.Notes), nilIfEmpty(doc.ReferenceURL), updatedBy, now).Scan(&version)
	r.observe("documents.update", started)
	if err != nil {
		return domain.Document{}, fmt.Errorf("update document: %w", err)
	}
	doc.Version = version
	doc.UpdatedAt = now
	return doc, nil
}

// ReplaceOffsets заменяет набор отступов напоминаний документа.
func (r *DocumentRepo) ReplaceOffsets(ctx context.Context, docID int64, offsets []int) error {
	if _, err := r.db(ctx).Exec(ctx,
		`DELETE FROM core.document_reminder_offsets WHERE document_id = $1`, docID); err != nil {
		return fmt.Errorf("clear offsets: %w", err)
	}
	for _, d := range offsets {
		if _, err := r.db(ctx).Exec(ctx,
			`INSERT INTO core.document_reminder_offsets (document_id, days_before) VALUES ($1, $2)`,
			docID, d); err != nil {
			return fmt.Errorf("insert offset: %w", err)
		}
	}
	return nil
}

// UpdateCurrentPeriod исправляет даты текущего периода.
func (r *DocumentRepo) UpdateCurrentPeriod(ctx context.Context, periodID int64, from, until *time.Time) error {
	_, err := r.db(ctx).Exec(ctx,
		`UPDATE core.document_periods SET valid_from = $2, valid_until = $3 WHERE id = $1`, periodID, from, until)
	if err != nil {
		return fmt.Errorf("update period: %w", err)
	}
	return nil
}

// AddPeriod делает прежний период историческим и добавляет новый текущий.
func (r *DocumentRepo) AddPeriod(ctx context.Context, docID int64, p domain.Period, createdBy int64, now time.Time) (domain.Period, error) {
	if _, err := r.db(ctx).Exec(ctx,
		`UPDATE core.document_periods SET is_current = false WHERE document_id = $1 AND is_current`, docID); err != nil {
		return domain.Period{}, fmt.Errorf("close current period: %w", err)
	}
	const q = `INSERT INTO core.document_periods (public_id, document_id, valid_from, valid_until, is_current, created_by, created_at)
		VALUES ($1, $2, $3, $4, true, $5, $6) RETURNING id, created_at`
	var stored domain.Period
	err := r.db(ctx).QueryRow(ctx, q, p.PublicID, docID, p.ValidFrom, p.ValidUntil, createdBy, now).
		Scan(&stored.ID, &stored.CreatedAt)
	if err != nil {
		return domain.Period{}, fmt.Errorf("insert period: %w", err)
	}
	stored.PublicID = p.PublicID
	stored.DocumentID = docID
	stored.ValidFrom = p.ValidFrom
	stored.ValidUntil = p.ValidUntil
	stored.IsCurrent = true
	return stored, nil
}

// FindPeriodByPublicID ищет период по публичному идентификатору.
func (r *DocumentRepo) FindPeriodByPublicID(ctx context.Context, publicID string) (domain.Period, error) {
	const q = `SELECT id, public_id::text, document_id, valid_from, valid_until, is_current, created_at
		FROM core.document_periods WHERE public_id = $1`
	var p domain.Period
	err := r.db(ctx).QueryRow(ctx, q, publicID).Scan(&p.ID, &p.PublicID, &p.DocumentID,
		&p.ValidFrom, &p.ValidUntil, &p.IsCurrent, &p.CreatedAt)
	if err != nil {
		return domain.Period{}, notFound(err)
	}
	return p, nil
}

// Delete удаляет документ вместе с периодами и отступами.
func (r *DocumentRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db(ctx).Exec(ctx, `DELETE FROM core.documents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}

// escapeLike экранирует служебные символы поиска подстроки.
func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}
