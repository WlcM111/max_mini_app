package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vovremya/services/core/internal/domain"
)

// CalendarExportResult — созданная одноразовая ссылка на файл календаря.
type CalendarExportResult struct {
	DownloadURL string
	FileName    string
	ExpiresAt   time.Time
}

// CreateCalendarExport готовит одноразовую ссылку на ICS организации.
func (a *App) CreateCalendarExport(ctx context.Context, actor Actor, orgPublicID string) (CalendarExportResult, error) {
	org, _, err := a.authorize(ctx, actor, orgPublicID, domain.RoleViewer)
	if err != nil {
		return CalendarExportResult{}, err
	}
	token, err := a.Random.Token()
	if err != nil {
		return CalendarExportResult{}, err
	}
	now := a.Clock.Now()
	expiresAt := now.Add(a.Settings.ExportTTL)
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := a.Exports.Create(ctx, TokenHash(token), org.ID, actor.Account.ID, now, expiresAt); err != nil {
			return err
		}
		return a.Audit.Write(ctx, domain.AuditExportCreated, actor.Account.ID, org.ID, org.PublicID, now)
	})
	if err != nil {
		return CalendarExportResult{}, err
	}
	return CalendarExportResult{
		DownloadURL: strings.TrimSuffix(a.Settings.PublicBaseURL, "/") + "/api/v1/downloads/" + token,
		FileName:    fmt.Sprintf("vovremya-%s.ics", strings.ReplaceAll(org.PublicID, "-", "")[:8]),
		ExpiresAt:   expiresAt,
	}, nil
}

// CalendarData — данные для формирования файла календаря.
type CalendarData struct {
	Organization domain.Organization
	Documents    []domain.Document
	Offsets      map[string][]int
	GeneratedAt  time.Time
}

// ConsumeCalendarExport проверяет одноразовую ссылку и возвращает данные календаря.
func (a *App) ConsumeCalendarExport(ctx context.Context, token string) (CalendarData, error) {
	if !domain.IsOpaqueToken(token) {
		return CalendarData{}, domain.ErrNotFound
	}
	now := a.Clock.Now()
	hash := TokenHash(token)
	orgID, err := a.Exports.Consume(ctx, hash, now)
	if err != nil {
		if exists, existsErr := a.Exports.Exists(ctx, hash); existsErr == nil && exists {
			// Ссылка есть, но просрочена или исчерпана.
			return CalendarData{}, domain.ErrLinkGone
		}
		return CalendarData{}, domain.ErrNotFound
	}
	org, err := a.Orgs.GetByID(ctx, orgID)
	if err != nil {
		return CalendarData{}, err
	}
	docs, err := a.Docs.ListCurrent(ctx, orgID)
	if err != nil {
		return CalendarData{}, err
	}
	offsets := make(map[string][]int, len(docs))
	for _, d := range docs {
		offsets[d.PublicID] = d.ReminderOffsets
	}
	return CalendarData{Organization: org, Documents: docs, Offsets: offsets, GeneratedAt: now}, nil
}
