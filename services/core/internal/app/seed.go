package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"vovremya/services/core/internal/domain"
)

// DemoData — структура файла demo/demo-data.json.
type DemoData struct {
	Version int `json:"version"`
	Owner   struct {
		ReviewLogin string `json:"review_login"`
		FirstName   string `json:"first_name"`
	} `json:"owner"`
	Organization struct {
		ID                   string   `json:"id"`
		Name                 string   `json:"name"`
		BusinessCategoryCode string   `json:"business_category_code"`
		RegionCode           string   `json:"region_code"`
		Timezone             string   `json:"timezone"`
		FeatureCodes         []string `json:"feature_codes"`
	} `json:"organization"`
	Documents []struct {
		ID                   string `json:"id"`
		DocumentTypeCode     string `json:"document_type_code"`
		Title                string `json:"title"`
		Number               string `json:"number"`
		Issuer               string `json:"issuer"`
		ResponsibleLabel     string `json:"responsible_label"`
		Notes                string `json:"notes"`
		ValidUntilOffsetDays *int   `json:"valid_until_offset_days"`
		ValidFromOffsetDays  *int   `json:"valid_from_offset_days"`
		ReminderOffsetsDays  []int  `json:"reminder_offsets_days"`
	} `json:"documents"`
}

// SeedDemo идемпотентно загружает демонстрационные данные.
func (a *App) SeedDemo(ctx context.Context, raw []byte) (string, error) {
	var data DemoData
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", err
	}
	now := a.Clock.Now()
	var owner domain.Account
	err := a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		owner, err = a.Accounts.UpsertReview(ctx, data.Owner.ReviewLogin, data.Owner.FirstName, now)
		if err != nil {
			return err
		}
		org, err := a.Orgs.LockByPublicID(ctx, data.Organization.ID)
		if errors.Is(err, domain.ErrNotFound) {
			org, err = a.Orgs.Create(ctx, domain.Organization{
				PublicID:             data.Organization.ID,
				Name:                 data.Organization.Name,
				BusinessCategoryCode: data.Organization.BusinessCategoryCode,
				RegionCode:           data.Organization.RegionCode,
				Timezone:             data.Organization.Timezone,
				FeatureCodes:         data.Organization.FeatureCodes,
			}, owner.ID, now)
			if err != nil {
				return err
			}
			if _, err := a.Orgs.CreateMembership(ctx, domain.Membership{
				OrganizationID: org.ID, AccountID: owner.ID, Role: domain.RoleOwner,
				NotifyEnabled: true, NotifyLocalTime: "09:00",
			}, now); err != nil {
				return err
			}
			if err := a.appendOrganizationEvent(ctx, org); err != nil {
				return err
			}
			if err := a.appendMembershipEvent(ctx, org, owner, domain.RoleOwner, true, 9*60, 1); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		loc, err := time.LoadLocation(org.Timezone)
		if err != nil {
			loc = time.UTC
		}
		today := domain.TodayIn(now, loc)
		for _, d := range data.Documents {
			if _, err := a.Docs.GetByPublicID(ctx, d.ID); err == nil {
				continue
			} else if !errors.Is(err, domain.ErrNotFound) {
				return err
			}
			var from, until *time.Time
			if d.ValidFromOffsetDays != nil {
				t := today.AddDate(0, 0, *d.ValidFromOffsetDays)
				from = &t
			}
			if d.ValidUntilOffsetDays != nil {
				t := today.AddDate(0, 0, *d.ValidUntilOffsetDays)
				until = &t
			}
			offsets := d.ReminderOffsetsDays
			if len(offsets) == 0 {
				offsets = a.CatalogSnapshot().DefaultOffsets(d.DocumentTypeCode)
			}
			doc, err := a.Docs.Create(ctx, domain.Document{
				PublicID: d.ID, OrganizationID: org.ID, DocumentTypeCode: d.DocumentTypeCode,
				Title: d.Title, Number: d.Number, Issuer: d.Issuer,
				ResponsibleLabel: d.ResponsibleLabel, Notes: d.Notes,
				ReminderOffsets: domain.NormalizeOffsets(offsets),
				CurrentPeriod: domain.Period{
					PublicID: a.Random.UUID(), ValidFrom: from, ValidUntil: until, IsCurrent: true,
				},
			}, owner.ID, now)
			if err != nil {
				return err
			}
			if err := a.appendDocumentEvent(ctx, org.PublicID, doc); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	a.flushAfterCommit(ctx)
	return data.Organization.ID, nil
}
