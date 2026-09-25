package app

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"time"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// DocumentInput — входные данные создания и изменения документа.
type DocumentInput struct {
	PublicID         string
	DocumentTypeCode string
	Title            string
	Number           string
	Issuer           string
	ResponsibleLabel string
	Notes            string
	ReferenceURL     string
	ValidFrom        *time.Time
	ValidUntil       *time.Time
	Offsets          []int
	ExpectedVersion  int

	SetType        bool
	SetTitle       bool
	SetNumber      bool
	SetIssuer      bool
	SetResponsible bool
	SetNotes       bool
	SetReference   bool
	SetValidFrom   bool
	SetValidUntil  bool
	SetOffsets     bool
}

// DocumentView — документ для ответа API вместе с вычисляемыми полями.
type DocumentView struct {
	Document       domain.Document
	OrganizationID string
	Status         domain.DeadlineStatus
	DaysLeft       *int
	NextReminderAt *time.Time
	RemindersState domain.RemindersState
	CanEdit        bool
	RenewalSteps   []string
	DataStatus     string
	Created        bool
}

// DocumentPage — страница реестра документов.
type DocumentPage struct {
	Items      []DocumentView
	NextCursor string
}

// ListDocuments возвращает страницу реестра документов организации.
func (a *App) ListDocuments(ctx context.Context, actor Actor, orgPublicID string,
	status domain.DeadlineStatus, query, cursor string, limit int) (DocumentPage, error) {
	org, m, err := a.authorize(ctx, actor, orgPublicID, domain.RoleViewer)
	if err != nil {
		return DocumentPage{}, err
	}
	if status != "" && !status.Valid() {
		return DocumentPage{}, domain.ValidationFor("status", domain.CodeUnknownValue, "неизвестное состояние срока")
	}
	if len(query) > 100 {
		return DocumentPage{}, domain.ValidationFor("q", domain.CodeTooLong, "не более 100 символов")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		return DocumentPage{}, domain.ValidationFor("limit", domain.CodeOutOfRange, "ожидается 1…100")
	}
	afterUntil, afterID, err := decodeCursor(cursor)
	if err != nil {
		return DocumentPage{}, err
	}
	today := domain.TodayIn(a.Clock.Now(), a.timezone(org))
	rows, err := a.Docs.List(ctx, ports.DocumentFilter{
		OrganizationID: org.ID, Status: status, Query: query, Today: today,
		AfterUntil: afterUntil, AfterID: afterID, Limit: limit + 1,
	})
	if err != nil {
		return DocumentPage{}, err
	}
	page := DocumentPage{}
	if len(rows) > limit {
		last := rows[limit-1]
		page.NextCursor = encodeCursor(last.CurrentPeriod.ValidUntil, last.PublicID)
		rows = rows[:limit]
	}
	views, err := a.decorate(ctx, actor, org, rows, m.Role, today)
	if err != nil {
		return DocumentPage{}, err
	}
	page.Items = views
	return page, nil
}

// GetDocument возвращает карточку документа.
func (a *App) GetDocument(ctx context.Context, actor Actor, docPublicID string) (DocumentView, error) {
	doc, org, role, err := a.documentWithAccess(ctx, actor, docPublicID, domain.RoleViewer)
	if err != nil {
		return DocumentView{}, err
	}
	full, err := a.Docs.GetFull(ctx, doc.ID)
	if err != nil {
		return DocumentView{}, err
	}
	today := domain.TodayIn(a.Clock.Now(), a.timezone(org))
	views, err := a.decorate(ctx, actor, org, []domain.Document{full}, role, today)
	if err != nil {
		return DocumentView{}, err
	}
	return views[0], nil
}

// CreateDocument добавляет документ; повтор с тем же id возвращает текущее состояние.
func (a *App) CreateDocument(ctx context.Context, actor Actor, orgPublicID string, in DocumentInput) (DocumentView, error) {
	views, err := a.CreateDocuments(ctx, actor, orgPublicID, []DocumentInput{in})
	if err != nil {
		return DocumentView{}, err
	}
	return views[0], nil
}

// CreateDocuments добавляет пакет документов в одной транзакции (всё или ничего).
func (a *App) CreateDocuments(ctx context.Context, actor Actor, orgPublicID string, items []DocumentInput) ([]DocumentView, error) {
	org, m, err := a.authorize(ctx, actor, orgPublicID, domain.RoleEditor)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 || len(items) > domain.MaxDocumentsPerBatch {
		return nil, domain.ValidationFor("items", domain.CodeOutOfRange, "ожидается от 1 до 30 документов")
	}
	catalog := a.CatalogSnapshot()
	v := &domain.Validator{}
	seen := make(map[string]struct{}, len(items))
	for _, in := range items {
		domain.ValidateDocument(domain.DocumentInput{
			PublicID: in.PublicID, DocumentTypeCode: in.DocumentTypeCode, Title: in.Title,
			Number: in.Number, Issuer: in.Issuer, ResponsibleLabel: in.ResponsibleLabel,
			Notes: in.Notes, ReferenceURL: in.ReferenceURL,
			ValidFrom: in.ValidFrom, ValidUntil: in.ValidUntil, Offsets: in.Offsets,
		}, catalog, v)
		if _, dup := seen[in.PublicID]; dup {
			v.Add("items.id", domain.CodeNotUnique, "идентификаторы в пакете не должны повторяться")
		}
		seen[in.PublicID] = struct{}{}
	}
	if err := v.Err(); err != nil {
		return nil, err
	}

	now := a.Clock.Now()
	created := make([]domain.Document, 0, len(items))
	createdFlags := make([]bool, 0, len(items))
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		created = created[:0]
		createdFlags = createdFlags[:0]
		if _, err := a.Orgs.LockByPublicID(ctx, orgPublicID); err != nil {
			return err
		}
		count, err := a.Docs.Count(ctx, org.ID)
		if err != nil {
			return err
		}
		fresh := 0
		for _, in := range items {
			existing, err := a.Docs.GetByPublicID(ctx, in.PublicID)
			switch {
			case err == nil && existing.OrganizationID == org.ID && existing.CreatedByID == actor.Account.ID:
				full, err := a.Docs.GetFull(ctx, existing.ID)
				if err != nil {
					return err
				}
				created = append(created, full)
				createdFlags = append(createdFlags, false)
				continue
			case err == nil:
				return domain.ErrConflictIDReused
			case !errors.Is(err, domain.ErrNotFound):
				return err
			}
			fresh++
			if count+fresh > domain.MaxDocumentsPerOrganization {
				return domain.ErrQuotaExceeded
			}
			offsets := in.Offsets
			if offsets == nil {
				offsets = catalog.DefaultOffsets(in.DocumentTypeCode)
			}
			doc := domain.Document{
				PublicID: in.PublicID, OrganizationID: org.ID, DocumentTypeCode: in.DocumentTypeCode,
				Title: strings.TrimSpace(in.Title), Number: in.Number, Issuer: in.Issuer,
				ResponsibleLabel: in.ResponsibleLabel, Notes: in.Notes, ReferenceURL: in.ReferenceURL,
				ReminderOffsets: domain.NormalizeOffsets(offsets),
				CurrentPeriod: domain.Period{
					PublicID: a.Random.UUID(), ValidFrom: in.ValidFrom, ValidUntil: in.ValidUntil, IsCurrent: true,
				},
			}
			stored, err := a.Docs.Create(ctx, doc, actor.Account.ID, now)
			if err != nil {
				return err
			}
			if err := a.appendDocumentEvent(ctx, org.PublicID, stored); err != nil {
				return err
			}
			created = append(created, stored)
			createdFlags = append(createdFlags, true)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	a.flushAfterCommit(ctx)

	today := domain.TodayIn(now, a.timezone(org))
	views, err := a.decorate(ctx, actor, org, created, m.Role, today)
	if err != nil {
		return nil, err
	}
	for i := range views {
		views[i].Created = createdFlags[i]
	}
	return views, nil
}

// UpdateDocument изменяет реквизиты документа, даты текущего периода и отступы.
func (a *App) UpdateDocument(ctx context.Context, actor Actor, docPublicID string, in DocumentInput) (DocumentView, error) {
	doc, org, role, err := a.documentWithAccess(ctx, actor, docPublicID, domain.RoleEditor)
	if err != nil {
		return DocumentView{}, err
	}
	catalog := a.CatalogSnapshot()
	now := a.Clock.Now()
	var updated domain.Document
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := a.Orgs.LockByPublicID(ctx, org.PublicID); err != nil {
			return err
		}
		current, err := a.Docs.GetFull(ctx, doc.ID)
		if err != nil {
			return err
		}
		if current.Version != in.ExpectedVersion {
			return domain.ErrConflictVersion
		}
		next := current
		if in.SetTitle {
			next.Title = strings.TrimSpace(in.Title)
		}
		if in.SetType {
			next.DocumentTypeCode = in.DocumentTypeCode
		}
		if in.SetNumber {
			next.Number = in.Number
		}
		if in.SetIssuer {
			next.Issuer = in.Issuer
		}
		if in.SetResponsible {
			next.ResponsibleLabel = in.ResponsibleLabel
		}
		if in.SetNotes {
			next.Notes = in.Notes
		}
		if in.SetReference {
			next.ReferenceURL = in.ReferenceURL
		}
		validFrom := current.CurrentPeriod.ValidFrom
		validUntil := current.CurrentPeriod.ValidUntil
		if in.SetValidFrom {
			validFrom = in.ValidFrom
		}
		if in.SetValidUntil {
			validUntil = in.ValidUntil
		}
		offsets := current.ReminderOffsets
		if in.SetOffsets {
			offsets = domain.NormalizeOffsets(in.Offsets)
		}
		v := &domain.Validator{}
		domain.ValidateDocument(domain.DocumentInput{
			DocumentTypeCode: next.DocumentTypeCode, Title: next.Title, Number: next.Number,
			Issuer: next.Issuer, ResponsibleLabel: next.ResponsibleLabel, Notes: next.Notes,
			ReferenceURL: next.ReferenceURL, ValidFrom: validFrom, ValidUntil: validUntil, Offsets: offsets,
		}, catalog, v)
		if err := v.Err(); err != nil {
			return err
		}
		updated, err = a.Docs.Update(ctx, next, actor.Account.ID, now)
		if err != nil {
			return err
		}
		if in.SetValidFrom || in.SetValidUntil {
			if err := a.Docs.UpdateCurrentPeriod(ctx, current.CurrentPeriod.ID, validFrom, validUntil); err != nil {
				return err
			}
			updated.CurrentPeriod.ValidFrom = validFrom
			updated.CurrentPeriod.ValidUntil = validUntil
		}
		if in.SetOffsets {
			if err := a.Docs.ReplaceOffsets(ctx, current.ID, offsets); err != nil {
				return err
			}
			updated.ReminderOffsets = offsets
		}
		return a.appendDocumentEvent(ctx, org.PublicID, updated)
	})
	if err != nil {
		return DocumentView{}, err
	}
	a.flushAfterCommit(ctx)
	full, err := a.Docs.GetFull(ctx, doc.ID)
	if err != nil {
		return DocumentView{}, err
	}
	views, err := a.decorate(ctx, actor, org, []domain.Document{full}, role, domain.TodayIn(now, a.timezone(org)))
	if err != nil {
		return DocumentView{}, err
	}
	return views[0], nil
}

// RenewDocument создаёт новый текущий период документа.
func (a *App) RenewDocument(ctx context.Context, actor Actor, docPublicID, periodPublicID string,
	from, until *time.Time) (DocumentView, error) {
	doc, org, role, err := a.documentWithAccess(ctx, actor, docPublicID, domain.RoleEditor)
	if err != nil {
		return DocumentView{}, err
	}
	v := &domain.Validator{}
	if !domain.IsUUIDv4(periodPublicID) {
		v.Add("id", domain.CodeInvalidFormat, "ожидается UUID версии 4")
	}
	domain.ValidatePeriodDates(from, until, v)
	if err := v.Err(); err != nil {
		return DocumentView{}, err
	}

	now := a.Clock.Now()
	created := false
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := a.Orgs.LockByPublicID(ctx, org.PublicID); err != nil {
			return err
		}
		existing, err := a.Docs.FindPeriodByPublicID(ctx, periodPublicID)
		switch {
		case err == nil && existing.DocumentID == doc.ID:
			return nil // повтор идемпотентного запроса
		case err == nil:
			return domain.ErrConflictIDReused
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		current, err := a.Docs.GetFull(ctx, doc.ID)
		if err != nil {
			return err
		}
		if _, err := a.Docs.AddPeriod(ctx, doc.ID, domain.Period{
			PublicID: periodPublicID, ValidFrom: from, ValidUntil: until, IsCurrent: true,
		}, actor.Account.ID, now); err != nil {
			return err
		}
		current.Version++
		updated, err := a.Docs.Update(ctx, current, actor.Account.ID, now)
		if err != nil {
			return err
		}
		updated.CurrentPeriod = domain.Period{PublicID: periodPublicID, ValidFrom: from, ValidUntil: until, IsCurrent: true}
		created = true
		return a.appendDocumentEvent(ctx, org.PublicID, updated)
	})
	if err != nil {
		return DocumentView{}, err
	}
	if created {
		a.flushAfterCommit(ctx)
	}
	full, err := a.Docs.GetFull(ctx, doc.ID)
	if err != nil {
		return DocumentView{}, err
	}
	views, err := a.decorate(ctx, actor, org, []domain.Document{full}, role, domain.TodayIn(now, a.timezone(org)))
	if err != nil {
		return DocumentView{}, err
	}
	views[0].Created = created
	return views[0], nil
}

// DeleteDocument удаляет документ вместе с периодами.
func (a *App) DeleteDocument(ctx context.Context, actor Actor, docPublicID string) error {
	doc, org, _, err := a.documentWithAccess(ctx, actor, docPublicID, domain.RoleEditor)
	if err != nil {
		return err
	}
	now := a.Clock.Now()
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := a.Orgs.LockByPublicID(ctx, org.PublicID); err != nil {
			return err
		}
		current, err := a.Docs.GetFull(ctx, doc.ID)
		if err != nil {
			return err
		}
		if err := a.Audit.Write(ctx, domain.AuditDocumentDeleted, actor.Account.ID, org.ID, current.PublicID, now); err != nil {
			return err
		}
		if err := a.appendDocumentDeleted(ctx, org.PublicID, current); err != nil {
			return err
		}
		return a.Docs.Delete(ctx, current.ID)
	})
	if err != nil {
		return err
	}
	a.flushAfterCommit(ctx)
	return nil
}

// documentWithAccess находит документ и проверяет доступ к его организации.
func (a *App) documentWithAccess(ctx context.Context, actor Actor, docPublicID string,
	min domain.Role) (domain.Document, domain.Organization, domain.Role, error) {
	if !domain.IsUUIDv4(docPublicID) {
		return domain.Document{}, domain.Organization{}, "", domain.ErrNotFound
	}
	doc, err := a.Docs.GetByPublicID(ctx, docPublicID)
	if err != nil {
		return domain.Document{}, domain.Organization{}, "", err
	}
	org, m, err := a.authorize(ctx, actor, doc.OrganizationPubID, min)
	if err != nil {
		return domain.Document{}, domain.Organization{}, "", err
	}
	return doc, org, m.Role, nil
}

// decorate дополняет документы вычисляемыми полями и данными плана напоминаний.
func (a *App) decorate(ctx context.Context, actor Actor, org domain.Organization, docs []domain.Document,
	role domain.Role, today time.Time) ([]DocumentView, error) {
	ids := make([]string, 0, len(docs))
	for _, d := range docs {
		ids = append(ids, d.PublicID)
	}
	next := map[string]time.Time{}
	planAvailable := true
	if len(ids) > 0 {
		callCtx, cancel := context.WithTimeout(ctx, a.Settings.RemindersRPCTimeout)
		items, err := a.Reminders.GetNextReminders(callCtx, actor.Account.PublicID, ids)
		cancel()
		if err != nil {
			planAvailable = false
			a.Metrics.RemindersRead.WithLabelValues("unavailable").Inc()
			a.Log.Warn("reminders plan unavailable", slog.Any("error", err))
		} else {
			a.Metrics.RemindersRead.WithLabelValues("ok").Inc()
			for _, it := range items {
				next[it.DocumentID] = it.DueAt
			}
		}
	}
	pendingDocs := map[string]bool{}
	orgPending := false
	if planAvailable && len(ids) > 0 {
		var err error
		pendingDocs, err = a.Outbox.PendingForAggregates(ctx, domain.AggregateDocument, ids)
		if err != nil {
			return nil, err
		}
		orgPending, err = a.Outbox.HasPending(ctx, domain.AggregateOrganization, org.PublicID)
		if err != nil {
			return nil, err
		}
	}

	catalog := a.CatalogSnapshot()
	out := make([]DocumentView, 0, len(docs))
	for _, d := range docs {
		view := DocumentView{
			Document:       d,
			OrganizationID: org.PublicID,
			Status:         domain.StatusOf(d.CurrentPeriod.ValidUntil, today),
			DaysLeft:       domain.DaysLeft(d.CurrentPeriod.ValidUntil, today),
			CanEdit:        role.Allows(domain.RoleEditor),
			RemindersState: domain.RemindersActual,
		}
		if due, ok := next[d.PublicID]; ok {
			t := due
			view.NextReminderAt = &t
		}
		switch {
		case !planAvailable:
			view.RemindersState = domain.RemindersUnavailable
		case orgPending || pendingDocs[d.PublicID]:
			view.RemindersState = domain.RemindersPending
		}
		if d.DocumentTypeCode != "" {
			if t, ok := catalog.DocumentType(d.DocumentTypeCode); ok {
				view.RenewalSteps = t.RenewalSteps
				view.DataStatus = t.DataStatus
			}
		}
		out = append(out, view)
	}
	return out, nil
}

// encodeCursor кодирует позицию страницы реестра.
func encodeCursor(validUntil *time.Time, publicID string) string {
	key := "inf"
	if validUntil != nil {
		key = domain.FormatLocalDate(*validUntil)
	}
	return base64.RawURLEncoding.EncodeToString([]byte(key + "|" + publicID))
}

// decodeCursor разбирает позицию страницы реестра в границы запроса.
func decodeCursor(cursor string) (string, string, error) {
	if cursor == "" {
		return "-infinity", "00000000-0000-0000-0000-000000000000", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", domain.ValidationFor("cursor", domain.CodeInvalidFormat, "курсор повреждён")
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || !domain.IsUUIDv4(parts[1]) {
		return "", "", domain.ValidationFor("cursor", domain.CodeInvalidFormat, "курсор повреждён")
	}
	if parts[0] == "inf" {
		return "infinity", parts[1], nil
	}
	if _, err := domain.ParseLocalDate(parts[0]); err != nil {
		return "", "", domain.ValidationFor("cursor", domain.CodeInvalidFormat, "курсор повреждён")
	}
	return parts[0], parts[1], nil
}
