package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"vovremya/services/core/internal/app"
	"vovremya/services/core/internal/domain"
)

const maxBodyBytes = 1 << 20

// decodeBody разбирает тело запроса и возвращает набор переданных полей.
// Неизвестные поля отклоняются (DisallowUnknownFields).
func decodeBody(r *http.Request, dst any) (map[string]json.RawMessage, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return nil, domain.ValidationFor("body", domain.CodeInvalidFormat, "не удалось прочитать тело запроса")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return nil, domain.ValidationFor("body", domain.CodeInvalidFormat, "тело запроса не соответствует схеме")
	}
	present := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &present); err != nil {
		return nil, domain.ValidationFor("body", domain.CodeInvalidFormat, "ожидается объект JSON")
	}
	return present, nil
}

func ptr[T any](v T) *T { return &v }

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func dateOrNil(t *time.Time) *string {
	if t == nil {
		return nil
	}
	return ptr(domain.FormatLocalDate(*t))
}

func parseDatePtr(value *string, field string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	d, err := domain.ParseLocalDate(*value)
	if err != nil {
		return nil, domain.ValidationFor(field, domain.CodeInvalidFormat, "ожидается дата YYYY-MM-DD")
	}
	return &d, nil
}

type accountDTO struct {
	ID        string  `json:"id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name"`
	Username  *string `json:"username"`
}

func toAccountDTO(a domain.Account) accountDTO {
	return accountDTO{ID: a.PublicID, FirstName: a.FirstName,
		LastName: nullable(a.LastName), Username: nullable(a.Username)}
}

type startTargetDTO struct {
	Kind           string  `json:"kind"`
	DocumentID     *string `json:"document_id,omitempty"`
	OrganizationID *string `json:"organization_id,omitempty"`
	InviteToken    *string `json:"invite_token,omitempty"`
}

type sessionDTO struct {
	Token     string         `json:"token"`
	ExpiresAt string         `json:"expires_at"`
	Account   accountDTO     `json:"account"`
	Start     startTargetDTO `json:"start"`
}

type membershipSummaryDTO struct {
	OrganizationID   string `json:"organization_id"`
	OrganizationName string `json:"organization_name"`
	Role             string `json:"role"`
	NotifyEnabled    bool   `json:"notify_enabled"`
}

type remindersChannelDTO struct {
	State      string  `json:"state"`
	BotChatURL *string `json:"bot_chat_url"`
}

type limitsDTO struct {
	MaxOrganizations            int `json:"max_organizations"`
	MaxDocumentsPerOrganization int `json:"max_documents_per_organization"`
	MaxMembersPerOrganization   int `json:"max_members_per_organization"`
	MaxReminderOffsets          int `json:"max_reminder_offsets"`
}

type meDTO struct {
	Account          accountDTO             `json:"account"`
	Memberships      []membershipSummaryDTO `json:"memberships"`
	RemindersChannel remindersChannelDTO    `json:"reminders_channel"`
	Limits           limitsDTO              `json:"limits"`
}

type catalogSourceDTO struct {
	Title     string  `json:"title"`
	URL       string  `json:"url"`
	CheckedOn *string `json:"checked_on"`
}

type documentTypeDTO struct {
	Code           string            `json:"code"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	DataStatus     string            `json:"data_status"`
	Source         *catalogSourceDTO `json:"source"`
	DefaultOffsets []int             `json:"default_reminder_offsets_days"`
	RenewalSteps   []string          `json:"renewal_steps"`
}

type catalogDTO struct {
	Version       string            `json:"version"`
	Categories    []categoryDTO     `json:"business_categories"`
	Regions       []regionDTO       `json:"regions"`
	Features      []featureDTO      `json:"features"`
	DocumentTypes []documentTypeDTO `json:"document_types"`
}

type categoryDTO struct {
	Code  string `json:"code"`
	Title string `json:"title"`
}

type regionDTO struct {
	Code            string `json:"code"`
	Title           string `json:"title"`
	DefaultTimezone string `json:"default_timezone"`
}

type featureDTO struct {
	Code     string  `json:"code"`
	Question string  `json:"question"`
	Hint     *string `json:"hint"`
}

func toCatalogDTO(c *domain.Catalog) catalogDTO {
	out := catalogDTO{Version: c.Version,
		Categories:    make([]categoryDTO, 0, len(c.Categories)),
		Regions:       make([]regionDTO, 0, len(c.Regions)),
		Features:      make([]featureDTO, 0, len(c.Features)),
		DocumentTypes: make([]documentTypeDTO, 0, len(c.DocumentTypes)),
	}
	for _, v := range c.Categories {
		out.Categories = append(out.Categories, categoryDTO{Code: v.Code, Title: v.Title})
	}
	for _, v := range c.Regions {
		out.Regions = append(out.Regions, regionDTO{Code: v.Code, Title: v.Title, DefaultTimezone: v.DefaultTimezone})
	}
	for _, v := range c.Features {
		out.Features = append(out.Features, featureDTO{Code: v.Code, Question: v.Question, Hint: nullable(v.Hint)})
	}
	for _, t := range c.DocumentTypes {
		dto := documentTypeDTO{Code: t.Code, Title: t.Title, Description: t.Description,
			DataStatus: t.DataStatus, DefaultOffsets: t.DefaultOffsets, RenewalSteps: t.RenewalSteps}
		if dto.DefaultOffsets == nil {
			dto.DefaultOffsets = []int{}
		}
		if dto.RenewalSteps == nil {
			dto.RenewalSteps = []string{}
		}
		if t.Source != nil {
			src := catalogSourceDTO{Title: t.Source.Title, URL: t.Source.URL}
			if t.Source.CheckedOn != "" {
				src.CheckedOn = ptr(t.Source.CheckedOn)
			}
			dto.Source = &src
		}
		out.DocumentTypes = append(out.DocumentTypes, dto)
	}
	return out
}

type statsDTO struct {
	Total          int     `json:"total"`
	Expired        int     `json:"expired"`
	Expiring       int     `json:"expiring"`
	Valid          int     `json:"valid"`
	NoExpiry       int     `json:"no_expiry"`
	NextValidUntil *string `json:"next_valid_until"`
}

type organizationDTO struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	BusinessCategoryCode string   `json:"business_category_code"`
	RegionCode           string   `json:"region_code"`
	Timezone             string   `json:"timezone"`
	FeatureCodes         []string `json:"feature_codes"`
	MyRole               string   `json:"my_role"`
	Version              int      `json:"version"`
	Stats                statsDTO `json:"stats"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
}

func toOrganizationDTO(r app.OrganizationResult) organizationDTO {
	codes := r.Organization.FeatureCodes
	if codes == nil {
		codes = []string{}
	}
	return organizationDTO{
		ID: r.Organization.PublicID, Name: r.Organization.Name,
		BusinessCategoryCode: r.Organization.BusinessCategoryCode,
		RegionCode:           r.Organization.RegionCode,
		Timezone:             r.Organization.Timezone,
		FeatureCodes:         codes,
		MyRole:               string(r.MyRole),
		Version:              r.Organization.Version,
		Stats: statsDTO{Total: r.Stats.Total, Expired: r.Stats.Expired, Expiring: r.Stats.Expiring,
			Valid: r.Stats.Valid, NoExpiry: r.Stats.NoExpiry, NextValidUntil: dateOrNil(r.Stats.NextValidUntil)},
		CreatedAt: rfc3339(r.Organization.CreatedAt), UpdatedAt: rfc3339(r.Organization.UpdatedAt),
	}
}

type periodDTO struct {
	ID         string  `json:"id"`
	ValidFrom  *string `json:"valid_from"`
	ValidUntil *string `json:"valid_until"`
	IsCurrent  bool    `json:"is_current"`
	CreatedAt  string  `json:"created_at"`
}

type documentListItemDTO struct {
	ID               string  `json:"id"`
	OrganizationID   string  `json:"organization_id"`
	DocumentTypeCode *string `json:"document_type_code"`
	Title            string  `json:"title"`
	ResponsibleLabel *string `json:"responsible_label"`
	ValidUntil       *string `json:"valid_until"`
	Status           string  `json:"status"`
	DaysLeft         *int    `json:"days_left"`
	NextReminderAt   *string `json:"next_reminder_at"`
	RemindersState   string  `json:"reminders_state"`
}

type documentPageDTO struct {
	Items      []documentListItemDTO `json:"items"`
	NextCursor *string               `json:"next_cursor"`
}

type documentDTO struct {
	ID               string      `json:"id"`
	OrganizationID   string      `json:"organization_id"`
	DocumentTypeCode *string     `json:"document_type_code"`
	Title            string      `json:"title"`
	Number           *string     `json:"number"`
	Issuer           *string     `json:"issuer"`
	ResponsibleLabel *string     `json:"responsible_label"`
	Notes            *string     `json:"notes"`
	ReferenceURL     *string     `json:"reference_url"`
	CurrentPeriod    periodDTO   `json:"current_period"`
	Periods          []periodDTO `json:"periods"`
	Status           string      `json:"status"`
	DaysLeft         *int        `json:"days_left"`
	ReminderOffsets  []int       `json:"reminder_offsets_days"`
	NextReminderAt   *string     `json:"next_reminder_at"`
	RenewalSteps     []string    `json:"renewal_steps"`
	DataStatus       *string     `json:"data_status"`
	RemindersState   string      `json:"reminders_state"`
	Version          int         `json:"version"`
	CanEdit          bool        `json:"can_edit"`
	CreatedAt        string      `json:"created_at"`
	UpdatedAt        string      `json:"updated_at"`
}

func toPeriodDTO(p domain.Period) periodDTO {
	return periodDTO{ID: p.PublicID, ValidFrom: dateOrNil(p.ValidFrom), ValidUntil: dateOrNil(p.ValidUntil),
		IsCurrent: p.IsCurrent, CreatedAt: rfc3339(p.CreatedAt)}
}

func toDocumentListItem(v app.DocumentView) documentListItemDTO {
	item := documentListItemDTO{
		ID: v.Document.PublicID, OrganizationID: v.OrganizationID,
		DocumentTypeCode: nullable(v.Document.DocumentTypeCode),
		Title:            v.Document.Title,
		ResponsibleLabel: nullable(v.Document.ResponsibleLabel),
		ValidUntil:       dateOrNil(v.Document.CurrentPeriod.ValidUntil),
		Status:           string(v.Status), DaysLeft: v.DaysLeft,
		RemindersState: string(v.RemindersState),
	}
	if v.NextReminderAt != nil {
		item.NextReminderAt = ptr(rfc3339(*v.NextReminderAt))
	}
	return item
}

func toDocumentDTO(v app.DocumentView) documentDTO {
	periods := make([]periodDTO, 0, len(v.Document.Periods))
	for _, p := range v.Document.Periods {
		periods = append(periods, toPeriodDTO(p))
	}
	offsets := v.Document.ReminderOffsets
	if offsets == nil {
		offsets = []int{}
	}
	steps := v.RenewalSteps
	if steps == nil {
		steps = []string{}
	}
	doc := documentDTO{
		ID: v.Document.PublicID, OrganizationID: v.OrganizationID,
		DocumentTypeCode: nullable(v.Document.DocumentTypeCode), Title: v.Document.Title,
		Number: nullable(v.Document.Number), Issuer: nullable(v.Document.Issuer),
		ResponsibleLabel: nullable(v.Document.ResponsibleLabel), Notes: nullable(v.Document.Notes),
		ReferenceURL:  nullable(v.Document.ReferenceURL),
		CurrentPeriod: toPeriodDTO(v.Document.CurrentPeriod), Periods: periods,
		Status: string(v.Status), DaysLeft: v.DaysLeft, ReminderOffsets: offsets,
		RenewalSteps: steps, DataStatus: nullable(v.DataStatus),
		RemindersState: string(v.RemindersState), Version: v.Document.Version, CanEdit: v.CanEdit,
		CreatedAt: rfc3339(v.Document.CreatedAt), UpdatedAt: rfc3339(v.Document.UpdatedAt),
	}
	if v.NextReminderAt != nil {
		doc.NextReminderAt = ptr(rfc3339(*v.NextReminderAt))
	}
	return doc
}

type memberDTO struct {
	AccountID string  `json:"account_id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name"`
	Role      string  `json:"role"`
	JoinedAt  string  `json:"joined_at"`
	IsMe      bool    `json:"is_me"`
}

func toMemberDTO(m domain.Membership, meID string) memberDTO {
	return memberDTO{AccountID: m.AccountPublicID, FirstName: m.AccountFirstName,
		LastName: nullable(m.AccountLastName), Role: string(m.Role),
		JoinedAt: rfc3339(m.JoinedAt), IsMe: m.AccountPublicID == meID}
}

type notificationSettingsDTO struct {
	Enabled   bool   `json:"enabled"`
	LocalTime string `json:"local_time"`
}

type inviteCreatedDTO struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expires_at"`
	LinkURL   string `json:"link_url"`
	ShareText string `json:"share_text"`
}

type inviteSummaryDTO struct {
	ID                 string  `json:"id"`
	Role               string  `json:"role"`
	CreatedAt          string  `json:"created_at"`
	ExpiresAt          string  `json:"expires_at"`
	CreatedByFirstName *string `json:"created_by_first_name"`
}

type invitePreviewDTO struct {
	OrganizationName string  `json:"organization_name"`
	Role             string  `json:"role"`
	InviterFirstName *string `json:"inviter_first_name"`
	ExpiresAt        string  `json:"expires_at"`
}

type inviteAcceptedDTO struct {
	OrganizationID string `json:"organization_id"`
	Role           string `json:"role"`
}

type calendarExportDTO struct {
	DownloadURL string `json:"download_url"`
	FileName    string `json:"file_name"`
	ExpiresAt   string `json:"expires_at"`
}
