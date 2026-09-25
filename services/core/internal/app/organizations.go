package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"vovremya/services/core/internal/domain"
)

// OrganizationInput — входные данные создания и изменения организации.
type OrganizationInput struct {
	PublicID             string
	Name                 string
	BusinessCategoryCode string
	RegionCode           string
	Timezone             string
	FeatureCodes         []string
	ExpectedVersion      int
	SetName              bool
	SetCategory          bool
	SetRegion            bool
	SetTimezone          bool
	SetFeatures          bool
}

// OrganizationResult — организация с ролью пользователя и сводкой документов.
type OrganizationResult struct {
	Organization domain.Organization
	MyRole       domain.Role
	Stats        domain.DocumentStats
	Created      bool
}

// CreateOrganization создаёт организацию; повтор с тем же id возвращает текущее состояние.
func (a *App) CreateOrganization(ctx context.Context, actor Actor, in OrganizationInput) (OrganizationResult, error) {
	catalog := a.CatalogSnapshot()
	v := &domain.Validator{}
	if !domain.IsUUIDv4(in.PublicID) {
		v.Add("id", domain.CodeInvalidFormat, "ожидается UUID версии 4")
	}
	domain.ValidateOrganization(domain.OrganizationInput{
		Name: in.Name, BusinessCategoryCode: in.BusinessCategoryCode, RegionCode: in.RegionCode,
		Timezone: in.Timezone, FeatureCodes: in.FeatureCodes,
	}, catalog, v)
	if err := v.Err(); err != nil {
		return OrganizationResult{}, err
	}

	now := a.Clock.Now()
	var (
		org     domain.Organization
		created bool
	)
	err := a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		existing, err := a.Orgs.LockByPublicID(ctx, in.PublicID)
		switch {
		case err == nil && existing.CreatedByAccountID == actor.Account.ID:
			org = existing
			return nil
		case err == nil:
			return domain.ErrConflictIDReused
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		count, err := a.Orgs.CountByAccount(ctx, actor.Account.ID)
		if err != nil {
			return err
		}
		if count >= domain.MaxOrganizationsPerAccount {
			return domain.ErrQuotaExceeded
		}
		org, err = a.Orgs.Create(ctx, domain.Organization{
			PublicID: in.PublicID, Name: strings.TrimSpace(in.Name),
			BusinessCategoryCode: in.BusinessCategoryCode, RegionCode: in.RegionCode,
			Timezone: in.Timezone, FeatureCodes: in.FeatureCodes,
		}, actor.Account.ID, now)
		if err != nil {
			return err
		}
		if _, err := a.Orgs.CreateMembership(ctx, domain.Membership{
			OrganizationID: org.ID, AccountID: actor.Account.ID, Role: domain.RoleOwner,
			NotifyEnabled: true, NotifyLocalTime: "09:00",
		}, now); err != nil {
			return err
		}
		if err := a.appendOrganizationEvent(ctx, org); err != nil {
			return err
		}
		if err := a.appendMembershipEvent(ctx, org, actor.Account, domain.RoleOwner, true, 9*60, 1); err != nil {
			return err
		}
		created = true
		return a.Audit.Write(ctx, domain.AuditOrganizationCreated, actor.Account.ID, org.ID, org.PublicID, now)
	})
	if err != nil {
		return OrganizationResult{}, err
	}
	if created {
		a.flushAfterCommit(ctx)
	}
	stats, err := a.Orgs.Stats(ctx, org.ID, domain.TodayIn(now, a.timezone(org)))
	if err != nil {
		return OrganizationResult{}, err
	}
	return OrganizationResult{Organization: org, MyRole: domain.RoleOwner, Stats: stats, Created: created}, nil
}

// GetOrganization возвращает организацию и сводку статусов документов.
func (a *App) GetOrganization(ctx context.Context, actor Actor, publicID string) (OrganizationResult, error) {
	org, m, err := a.authorize(ctx, actor, publicID, domain.RoleViewer)
	if err != nil {
		return OrganizationResult{}, err
	}
	stats, err := a.Orgs.Stats(ctx, org.ID, domain.TodayIn(a.Clock.Now(), a.timezone(org)))
	if err != nil {
		return OrganizationResult{}, err
	}
	return OrganizationResult{Organization: org, MyRole: m.Role, Stats: stats}, nil
}

// UpdateOrganization изменяет профиль организации с оптимистичной блокировкой.
func (a *App) UpdateOrganization(ctx context.Context, actor Actor, publicID string, in OrganizationInput) (OrganizationResult, error) {
	org, m, err := a.authorize(ctx, actor, publicID, domain.RoleEditor)
	if err != nil {
		return OrganizationResult{}, err
	}
	catalog := a.CatalogSnapshot()
	now := a.Clock.Now()
	var updated domain.Organization
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := a.Orgs.LockByPublicID(ctx, publicID)
		if err != nil {
			return err
		}
		if locked.Version != in.ExpectedVersion {
			return domain.ErrConflictVersion
		}
		next := locked
		if in.SetName {
			next.Name = strings.TrimSpace(in.Name)
		}
		if in.SetCategory {
			next.BusinessCategoryCode = in.BusinessCategoryCode
		}
		if in.SetRegion {
			next.RegionCode = in.RegionCode
		}
		if in.SetTimezone {
			next.Timezone = in.Timezone
		}
		if in.SetFeatures {
			next.FeatureCodes = in.FeatureCodes
		}
		v := &domain.Validator{}
		domain.ValidateOrganization(domain.OrganizationInput{
			Name: next.Name, BusinessCategoryCode: next.BusinessCategoryCode,
			RegionCode: next.RegionCode, Timezone: next.Timezone, FeatureCodes: next.FeatureCodes,
		}, catalog, v)
		if err := v.Err(); err != nil {
			return err
		}
		updated, err = a.Orgs.Update(ctx, next, now)
		if err != nil {
			return err
		}
		// Смена названия или пояса меняет тексты и время напоминаний: событие
		// уходит в reminders-service, который перестраивает план.
		return a.appendOrganizationEvent(ctx, updated)
	})
	if err != nil {
		return OrganizationResult{}, err
	}
	a.flushAfterCommit(ctx)
	stats, err := a.Orgs.Stats(ctx, updated.ID, domain.TodayIn(now, a.timezone(updated)))
	if err != nil {
		return OrganizationResult{}, err
	}
	_ = org
	return OrganizationResult{Organization: updated, MyRole: m.Role, Stats: stats}, nil
}

// DeleteOrganization удаляет организацию со всеми данными.
func (a *App) DeleteOrganization(ctx context.Context, actor Actor, publicID string) error {
	org, _, err := a.authorize(ctx, actor, publicID, domain.RoleOwner)
	if err != nil {
		return err
	}
	now := a.Clock.Now()
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := a.Orgs.LockByPublicID(ctx, publicID)
		if err != nil {
			return err
		}
		if err := a.Audit.Write(ctx, domain.AuditOrganizationDeleted, actor.Account.ID, locked.ID, locked.PublicID, now); err != nil {
			return err
		}
		if err := a.appendOrganizationDeleted(ctx, locked); err != nil {
			return err
		}
		return a.Orgs.Delete(ctx, locked.ID)
	})
	if err != nil {
		return err
	}
	a.flushAfterCommit(ctx)
	_ = org
	return nil
}

// ListSuggestions возвращает типовые документы профиля, ещё не добавленные в реестр.
func (a *App) ListSuggestions(ctx context.Context, actor Actor, publicID string) ([]domain.DocumentType, error) {
	org, _, err := a.authorize(ctx, actor, publicID, domain.RoleViewer)
	if err != nil {
		return nil, err
	}
	docs, err := a.Docs.ListCurrent(ctx, org.ID)
	if err != nil {
		return nil, err
	}
	present := make(map[string]struct{}, len(docs))
	for _, d := range docs {
		if d.DocumentTypeCode != "" {
			present[d.DocumentTypeCode] = struct{}{}
		}
	}
	applicable := a.CatalogSnapshot().Applicable(org.BusinessCategoryCode, org.FeatureCodes)
	out := make([]domain.DocumentType, 0, len(applicable))
	for _, t := range applicable {
		if _, used := present[t.Code]; used {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

var _ = time.Time{}
