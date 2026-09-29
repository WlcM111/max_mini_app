package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"vovremya/services/core/internal/adapters/ics"
	"vovremya/services/core/internal/adapters/xlsx"
	"vovremya/services/core/internal/app"
	"vovremya/services/core/internal/domain"
)

// handleCreateSession — POST /sessions.
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	if !s.sessionByIP.Allow(s.clientIP(r), now) {
		s.app.Metrics.SessionCreate.WithLabelValues("rate_limited").Inc()
		s.fail(w, r, domain.ErrRateLimited)
		return
	}
	var req struct {
		InitData   string `json:"init_data"`
		Platform   string `json:"platform"`
		AppVersion string `json:"app_version"`
	}
	if _, err := decodeBody(r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	// Лимит по пользователю проверяется до записи сессии.
	result, err := s.app.CreateSessionLimited(r.Context(), req.InitData, req.Platform, req.AppVersion,
		func(maxUserID int64) bool { return s.sessionByUser.Allow(strconv.FormatInt(maxUserID, 10), now) })
	if err != nil {
		s.fail(w, r, err)
		return
	}
	start := startTargetDTO{Kind: string(result.Start.Kind)}
	switch result.Start.Kind {
	case domain.StartDocument, domain.StartRenew:
		start.DocumentID = ptr(result.Start.DocumentID)
	case domain.StartOrganization:
		start.OrganizationID = ptr(result.Start.OrganizationID)
	case domain.StartInvite:
		start.InviteToken = ptr(result.Start.InviteToken)
	}
	s.writeJSON(w, http.StatusCreated, sessionDTO{
		Token: result.Token, ExpiresAt: rfc3339(result.ExpiresAt),
		Account: toAccountDTO(result.Account), Start: start,
	})
}

// handleDeleteSession — DELETE /sessions/current.
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if err := s.app.DeleteSession(r.Context(), actor); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetMe — GET /me.
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	me, err := s.app.GetMe(r.Context(), actor)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	memberships := make([]membershipSummaryDTO, 0, len(me.Memberships))
	for _, m := range me.Memberships {
		memberships = append(memberships, membershipSummaryDTO{
			OrganizationID: m.Organization.PublicID, OrganizationName: m.Organization.Name,
			Role: string(m.Role), NotifyEnabled: m.NotifyEnabled,
		})
	}
	s.writeJSON(w, http.StatusOK, meDTO{
		Account:     toAccountDTO(me.Account),
		Memberships: memberships,
		RemindersChannel: remindersChannelDTO{
			State: me.Channel.State, BotChatURL: nullable(me.Channel.BotChatURL),
		},
		Limits: limitsDTO{
			MaxOrganizations:            domain.MaxOrganizationsPerAccount,
			MaxDocumentsPerOrganization: domain.MaxDocumentsPerOrganization,
			MaxMembersPerOrganization:   domain.MaxMembersPerOrganization,
			MaxReminderOffsets:          domain.MaxReminderOffsets,
		},
		AssistantEnabled: s.app.AssistantEnabled(),
	})
}

// handleDeleteMe — DELETE /me.
func (s *Server) handleDeleteMe(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if err := s.app.DeleteMe(r.Context(), actor); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetCatalog — GET /catalog.
func (s *Server) handleGetCatalog(w http.ResponseWriter, r *http.Request, _ app.Actor) {
	catalog := s.app.CatalogSnapshot()
	if catalog == nil {
		s.fail(w, r, domain.ErrDependencyUnavailable)
		return
	}
	etag := catalog.ETag()
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	s.writeJSON(w, http.StatusOK, toCatalogDTO(catalog))
}

// handleCreateOrganization — POST /organizations.
func (s *Server) handleCreateOrganization(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	var req struct {
		ID                   string   `json:"id"`
		Name                 string   `json:"name"`
		BusinessCategoryCode string   `json:"business_category_code"`
		RegionCode           string   `json:"region_code"`
		Timezone             string   `json:"timezone"`
		FeatureCodes         []string `json:"feature_codes"`
	}
	if _, err := decodeBody(r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	result, err := s.app.CreateOrganization(r.Context(), actor, app.OrganizationInput{
		PublicID: req.ID, Name: req.Name, BusinessCategoryCode: req.BusinessCategoryCode,
		RegionCode: req.RegionCode, Timezone: req.Timezone, FeatureCodes: req.FeatureCodes,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	s.writeJSON(w, status, toOrganizationDTO(result))
}

// handleGetOrganization — GET /organizations/{organizationId}.
func (s *Server) handleGetOrganization(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	result, err := s.app.GetOrganization(r.Context(), actor, r.PathValue("organizationId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, toOrganizationDTO(result))
}

// handleUpdateOrganization — PATCH /organizations/{organizationId}.
func (s *Server) handleUpdateOrganization(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	var req struct {
		ExpectedVersion      int      `json:"expected_version"`
		Name                 string   `json:"name"`
		BusinessCategoryCode string   `json:"business_category_code"`
		RegionCode           string   `json:"region_code"`
		Timezone             string   `json:"timezone"`
		FeatureCodes         []string `json:"feature_codes"`
	}
	present, err := decodeBody(r, &req)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, ok := present["expected_version"]; !ok {
		s.fail(w, r, domain.ValidationFor("expected_version", domain.CodeRequired, "поле обязательно"))
		return
	}
	if len(present) < 2 {
		s.fail(w, r, domain.ValidationFor("body", domain.CodeInvalidCombination, "нужно хотя бы одно изменяемое поле"))
		return
	}
	in := app.OrganizationInput{
		ExpectedVersion: req.ExpectedVersion, Name: req.Name,
		BusinessCategoryCode: req.BusinessCategoryCode, RegionCode: req.RegionCode,
		Timezone: req.Timezone, FeatureCodes: req.FeatureCodes,
	}
	_, in.SetName = present["name"]
	_, in.SetCategory = present["business_category_code"]
	_, in.SetRegion = present["region_code"]
	_, in.SetTimezone = present["timezone"]
	_, in.SetFeatures = present["feature_codes"]
	result, err := s.app.UpdateOrganization(r.Context(), actor, r.PathValue("organizationId"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, toOrganizationDTO(result))
}

// handleDeleteOrganization — DELETE /organizations/{organizationId}.
func (s *Server) handleDeleteOrganization(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if err := s.app.DeleteOrganization(r.Context(), actor, r.PathValue("organizationId")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSuggestions — GET /organizations/{organizationId}/suggestions.
func (s *Server) handleSuggestions(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	types, err := s.app.ListSuggestions(r.Context(), actor, r.PathValue("organizationId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	type suggestion struct {
		DocumentTypeCode string `json:"document_type_code"`
		Title            string `json:"title"`
		Description      string `json:"description"`
		DataStatus       string `json:"data_status"`
	}
	items := make([]suggestion, 0, len(types))
	for _, t := range types {
		items = append(items, suggestion{DocumentTypeCode: t.Code, Title: t.Title,
			Description: t.Description, DataStatus: t.DataStatus})
	}
	s.writeJSON(w, http.StatusOK, struct {
		Items []suggestion `json:"items"`
	}{Items: items})
}

// handleListDocuments — GET /organizations/{organizationId}/documents.
func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	q := r.URL.Query()
	limit := 50
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			s.fail(w, r, domain.ValidationFor("limit", domain.CodeInvalidFormat, "ожидается целое число"))
			return
		}
		limit = parsed
	}
	// responsible=me — «Мои документы»: где ответственный — текущий пользователь.
	responsible := q.Get("responsible")
	if responsible == "me" {
		responsible = actor.Account.PublicID
	}
	page, err := s.app.ListDocuments(r.Context(), actor, r.PathValue("organizationId"),
		domain.DeadlineStatus(q.Get("status")), q.Get("q"), q.Get("cursor"), limit, responsible)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]documentListItemDTO, 0, len(page.Items))
	for _, v := range page.Items {
		items = append(items, toDocumentListItem(v))
	}
	s.writeJSON(w, http.StatusOK, documentPageDTO{Items: items, NextCursor: nullable(page.NextCursor)})
}

type documentBody struct {
	ID                   string  `json:"id"`
	DocumentTypeCode     *string `json:"document_type_code"`
	Title                string  `json:"title"`
	Number               *string `json:"number"`
	Issuer               *string `json:"issuer"`
	ResponsibleLabel     *string `json:"responsible_label"`
	ResponsibleAccountID *string `json:"responsible_account_id"`
	Notes                *string `json:"notes"`
	ReferenceURL         *string `json:"reference_url"`
	ValidFrom            *string `json:"valid_from"`
	ValidUntil           *string `json:"valid_until"`
	ReminderOffsets      []int   `json:"reminder_offsets_days"`
	ExpectedVersion      int     `json:"expected_version"`
}

func (b documentBody) toInput(present map[string]any) (app.DocumentInput, error) {
	from, err := parseDatePtr(b.ValidFrom, "valid_from")
	if err != nil {
		return app.DocumentInput{}, err
	}
	until, err := parseDatePtr(b.ValidUntil, "valid_until")
	if err != nil {
		return app.DocumentInput{}, err
	}
	in := app.DocumentInput{
		PublicID: b.ID, Title: b.Title, ValidFrom: from, ValidUntil: until,
		Offsets: b.ReminderOffsets, ExpectedVersion: b.ExpectedVersion,
	}
	if b.DocumentTypeCode != nil {
		in.DocumentTypeCode = *b.DocumentTypeCode
	}
	if b.Number != nil {
		in.Number = *b.Number
	}
	if b.Issuer != nil {
		in.Issuer = *b.Issuer
	}
	if b.ResponsibleLabel != nil {
		in.ResponsibleLabel = *b.ResponsibleLabel
	}
	if b.ResponsibleAccountID != nil {
		in.ResponsibleAccountID = *b.ResponsibleAccountID
	}
	if b.Notes != nil {
		in.Notes = *b.Notes
	}
	if b.ReferenceURL != nil {
		in.ReferenceURL = *b.ReferenceURL
	}
	_, in.SetType = present["document_type_code"]
	_, in.SetTitle = present["title"]
	_, in.SetNumber = present["number"]
	_, in.SetIssuer = present["issuer"]
	_, in.SetResponsible = present["responsible_label"]
	_, in.SetResponsibleAccount = present["responsible_account_id"]
	_, in.SetNotes = present["notes"]
	_, in.SetReference = present["reference_url"]
	_, in.SetValidFrom = present["valid_from"]
	_, in.SetValidUntil = present["valid_until"]
	_, in.SetOffsets = present["reminder_offsets_days"]
	return in, nil
}

// rawPresence переводит набор переданных полей в множество имён.
func rawPresence(raw map[string]json.RawMessage) map[string]any {
	out := make(map[string]any, len(raw))
	for k := range raw {
		out[k] = struct{}{}
	}
	return out
}

// handleCreateDocument — POST /organizations/{organizationId}/documents.
func (s *Server) handleCreateDocument(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	var body documentBody
	present, err := decodeBody(r, &body)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	in, err := body.toInput(rawPresence(present))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if !in.SetOffsets {
		in.Offsets = nil
	}
	view, err := s.app.CreateDocument(r.Context(), actor, r.PathValue("organizationId"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if view.Created {
		status = http.StatusCreated
	}
	s.writeJSON(w, status, toDocumentDTO(view))
}

// handleDraftDocument — POST /organizations/{organizationId}/documents/draft.
// Возвращает черновик карточки, распознанный языковым ассистентом (FR-21).
// Ничего не сохраняет: документ создаётся обычным POST после подтверждения.
func (s *Server) handleDraftDocument(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if !s.assistantLimiter.Allow("draft:"+actor.Account.PublicID, time.Now()) {
		s.fail(w, r, domain.ErrRateLimited)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	draft, err := s.app.DraftDocument(r.Context(), actor, r.PathValue("organizationId"), body.Text)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, documentDraftDTO{
		Title:            draft.Title,
		Number:           nullable(draft.Number),
		Issuer:           nullable(draft.Issuer),
		ValidFrom:        nullable(draft.ValidFrom),
		ValidUntil:       nullable(draft.ValidUntil),
		DocumentTypeCode: nullable(draft.DocumentTypeCode),
		Offsets:          intsOrEmpty(draft.Offsets),
		Confidence:       draft.Confidence,
	})
}

// handleMatchProfile — POST /profile-match.
// Сопоставляет свободное описание бизнеса с кодами справочника (FR-22).
func (s *Server) handleMatchProfile(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if !s.assistantLimiter.Allow("profile:"+actor.Account.PublicID, time.Now()) {
		s.fail(w, r, domain.ErrRateLimited)
		return
	}
	var body struct {
		Description string `json:"description"`
	}
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	match, err := s.app.MatchProfile(r.Context(), actor, body.Description)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, profileMatchDTO{
		BusinessCategoryCode: nullable(match.BusinessCategoryCode),
		FeatureCodes:         stringsOrEmpty(match.FeatureCodes),
		Confidence:           match.Confidence,
	})
}

// handleCreateDocumentsBatch — POST /organizations/{organizationId}/documents/batch.
func (s *Server) handleCreateDocumentsBatch(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	var body struct {
		Items []documentBody `json:"items"`
	}
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	inputs := make([]app.DocumentInput, 0, len(body.Items))
	for _, item := range body.Items {
		in, err := item.toInput(map[string]any{})
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if item.ReminderOffsets == nil {
			in.Offsets = nil
		} else {
			in.SetOffsets = true
		}
		inputs = append(inputs, in)
	}
	views, err := s.app.CreateDocuments(r.Context(), actor, r.PathValue("organizationId"), inputs)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]documentDTO, 0, len(views))
	for _, v := range views {
		items = append(items, toDocumentDTO(v))
	}
	s.writeJSON(w, http.StatusCreated, struct {
		Items []documentDTO `json:"items"`
	}{Items: items})
}

// handleGetDocument — GET /documents/{documentId}.
func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	view, err := s.app.GetDocument(r.Context(), actor, r.PathValue("documentId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, toDocumentDTO(view))
}

// handleUpdateDocument — PATCH /documents/{documentId}.
func (s *Server) handleUpdateDocument(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	var body documentBody
	present, err := decodeBody(r, &body)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, ok := present["expected_version"]; !ok {
		s.fail(w, r, domain.ValidationFor("expected_version", domain.CodeRequired, "поле обязательно"))
		return
	}
	if len(present) < 2 {
		s.fail(w, r, domain.ValidationFor("body", domain.CodeInvalidCombination, "нужно хотя бы одно изменяемое поле"))
		return
	}
	in, err := body.toInput(rawPresence(present))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	view, err := s.app.UpdateDocument(r.Context(), actor, r.PathValue("documentId"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, toDocumentDTO(view))
}

// handleDeleteDocument — DELETE /documents/{documentId}.
func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if err := s.app.DeleteDocument(r.Context(), actor, r.PathValue("documentId")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRenewDocument — POST /documents/{documentId}/renewals.
func (s *Server) handleRenewDocument(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	var body struct {
		ID         string  `json:"id"`
		ValidFrom  *string `json:"valid_from"`
		ValidUntil *string `json:"valid_until"`
	}
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	from, err := parseDatePtr(body.ValidFrom, "valid_from")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	until, err := parseDatePtr(body.ValidUntil, "valid_until")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	view, err := s.app.RenewDocument(r.Context(), actor, r.PathValue("documentId"), body.ID, from, until)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if view.Created {
		status = http.StatusCreated
	}
	s.writeJSON(w, status, toDocumentDTO(view))
}

// handleListMembers — GET /organizations/{organizationId}/members.
func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	members, err := s.app.ListMembers(r.Context(), actor, r.PathValue("organizationId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]memberDTO, 0, len(members))
	for _, m := range members {
		items = append(items, toMemberDTO(m, actor.Account.PublicID))
	}
	s.writeJSON(w, http.StatusOK, struct {
		Items []memberDTO `json:"items"`
	}{Items: items})
}

// handleUpdateMemberRole — PATCH /organizations/{organizationId}/members/{accountId}.
func (s *Server) handleUpdateMemberRole(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	var body struct {
		Role string `json:"role"`
	}
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	member, err := s.app.UpdateMemberRole(r.Context(), actor, r.PathValue("organizationId"),
		r.PathValue("accountId"), domain.Role(body.Role))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, toMemberDTO(member, actor.Account.PublicID))
}

// handleRemoveMember — DELETE /organizations/{organizationId}/members/{accountId}.
func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	err := s.app.RemoveMember(r.Context(), actor, r.PathValue("organizationId"), r.PathValue("accountId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetNotificationSettings — GET /organizations/{organizationId}/notification-settings.
func (s *Server) handleGetNotificationSettings(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	m, err := s.app.GetNotificationSettings(r.Context(), actor, r.PathValue("organizationId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, notificationSettingsDTO{Enabled: m.NotifyEnabled, LocalTime: m.NotifyLocalTime})
}

// handlePutNotificationSettings — PUT /organizations/{organizationId}/notification-settings.
func (s *Server) handlePutNotificationSettings(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	var body notificationSettingsDTO
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	m, err := s.app.PutNotificationSettings(r.Context(), actor, r.PathValue("organizationId"),
		body.Enabled, body.LocalTime)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, notificationSettingsDTO{Enabled: m.NotifyEnabled, LocalTime: m.NotifyLocalTime})
}

// handleListInvites — GET /organizations/{organizationId}/invites.
func (s *Server) handleListInvites(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	invites, err := s.app.ListInvites(r.Context(), actor, r.PathValue("organizationId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]inviteSummaryDTO, 0, len(invites))
	for _, inv := range invites {
		items = append(items, inviteSummaryDTO{
			ID: inv.PublicID, Role: string(inv.Role), CreatedAt: rfc3339(inv.CreatedAt),
			ExpiresAt: rfc3339(inv.ExpiresAt), CreatedByFirstName: nullable(inv.CreatedByFirstName),
		})
	}
	s.writeJSON(w, http.StatusOK, struct {
		Items []inviteSummaryDTO `json:"items"`
	}{Items: items})
}

// handleCreateInvite — POST /organizations/{organizationId}/invites.
func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	var body struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	}
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	created, err := s.app.CreateInvite(r.Context(), actor, r.PathValue("organizationId"),
		body.ID, domain.Role(body.Role))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, inviteCreatedDTO{
		ID: created.Invite.PublicID, Role: string(created.Invite.Role),
		ExpiresAt: rfc3339(created.Invite.ExpiresAt), LinkURL: created.LinkURL, ShareText: created.ShareText,
	})
}

// handleRevokeInvite — DELETE /invites/{inviteId}.
func (s *Server) handleRevokeInvite(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if err := s.app.RevokeInvite(r.Context(), actor, r.PathValue("inviteId")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePreviewInvite — POST /invites/preview.
func (s *Server) handlePreviewInvite(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if !s.inviteLimiter.Allow("preview:"+actor.Account.PublicID, time.Now()) {
		s.fail(w, r, domain.ErrRateLimited)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	preview, err := s.app.PreviewInvite(r.Context(), actor, body.Token)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, invitePreviewDTO{
		OrganizationName: preview.OrganizationName, Role: string(preview.Role),
		InviterFirstName: nullable(preview.InviterFirstName), ExpiresAt: rfc3339(preview.ExpiresAt),
	})
}

// handleAcceptInvite — POST /invites/accept.
func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if !s.inviteLimiter.Allow("accept:"+actor.Account.PublicID, time.Now()) {
		s.fail(w, r, domain.ErrRateLimited)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	accepted, err := s.app.AcceptInvite(r.Context(), actor, body.Token)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, inviteAcceptedDTO{
		OrganizationID: accepted.OrganizationPubID, Role: string(accepted.Role),
	})
}

// handleCreateExport — POST /organizations/{organizationId}/exports/calendar.
func (s *Server) handleCreateExport(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if !s.inviteLimiter.Allow("export:"+actor.Account.PublicID, time.Now()) {
		s.fail(w, r, domain.ErrRateLimited)
		return
	}
	result, err := s.app.CreateCalendarExport(r.Context(), actor, r.PathValue("organizationId"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, calendarExportDTO{
		DownloadURL: result.DownloadURL, FileName: result.FileName, ExpiresAt: rfc3339(result.ExpiresAt),
	})
}

// handleDownloadCalendar — GET /downloads/{downloadToken} (без Bearer).
func (s *Server) handleDownloadCalendar(w http.ResponseWriter, r *http.Request) {
	data, err := s.app.ConsumeCalendarExport(r.Context(), r.PathValue("downloadToken"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// ?format=xlsx — тот же одноразовый токен отдаёт реестр для бухгалтерии и проверок.
	if r.URL.Query().Get("format") == "xlsx" {
		book, err := xlsx.Registry(data)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="vovremya-reestr-`+firstEight(data.Organization.PublicID)+`.xlsx"`)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(book)
		return
	}
	body := ics.Render(data)
	fileName := "vovremya-" + firstEight(data.Organization.PublicID) + ".ics"
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

// maxImageBodyBytes — фото до 5 МБ в Base64 плюс поля JSON.
const maxImageBodyBytes = 7 << 20

// handleDraftDocumentImage — черновик карточки по фотографии документа (FR-23).
func (s *Server) handleDraftDocumentImage(w http.ResponseWriter, r *http.Request, actor app.Actor) {
	if !s.assistantLimiter.Allow("draft:"+actor.Account.PublicID, time.Now()) {
		s.fail(w, r, domain.ErrRateLimited)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxImageBodyBytes+1))
	if err != nil {
		// Обрыв соединения или истёкший срок чтения — это не «слишком большой файл».
		s.fail(w, r, domain.ValidationFor("image", domain.CodeInvalidFormat, "фотография получена не полностью — повторите отправку"))
		return
	}
	if len(raw) > maxImageBodyBytes {
		s.fail(w, r, domain.ValidationFor("image", domain.CodeTooLong, "фотография больше 5 МБ"))
		return
	}
	var body struct {
		Image    string `json:"image"`
		MimeType string `json:"mime_type"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		s.fail(w, r, domain.ValidationFor("image", domain.CodeInvalidFormat, "ожидается JSON с полями image и mime_type"))
		return
	}
	image, err := base64.StdEncoding.DecodeString(body.Image)
	if err != nil {
		s.fail(w, r, domain.ValidationFor("image", domain.CodeInvalidFormat, "изображение должно быть в Base64"))
		return
	}
	draft, err := s.app.DraftDocumentFromImage(r.Context(), actor, r.PathValue("organizationId"), image, body.MimeType)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, documentDraftDTO{
		Title:            draft.Title,
		Number:           nullable(draft.Number),
		Issuer:           nullable(draft.Issuer),
		ValidFrom:        nullable(draft.ValidFrom),
		ValidUntil:       nullable(draft.ValidUntil),
		DocumentTypeCode: nullable(draft.DocumentTypeCode),
		Offsets:          intsOrEmpty(draft.Offsets),
		Confidence:       draft.Confidence,
	})
}

// handleClientEvents — POST /client-events.
func (s *Server) handleClientEvents(w http.ResponseWriter, r *http.Request) {
	if !s.eventsLimiter.Allow(s.clientIP(r), time.Now()) {
		s.fail(w, r, domain.ErrRateLimited)
		return
	}
	var body struct {
		Events []struct {
			Name       string `json:"name"`
			Platform   string `json:"platform"`
			AppVersion string `json:"app_version"`
			DurationMs int    `json:"duration_ms"`
			Code       string `json:"code"`
		} `json:"events"`
	}
	if _, err := decodeBody(r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	events := make([]app.ClientEvent, 0, len(body.Events))
	for _, e := range body.Events {
		events = append(events, app.ClientEvent{Name: e.Name, Platform: e.Platform,
			AppVersion: e.AppVersion, DurationMs: e.DurationMs, Code: e.Code})
	}
	if err := s.app.AcceptClientEvents(r.Context(), app.ClientEventsInput{Events: events}); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func firstEight(uuidValue string) string {
	compact := ""
	for _, ch := range uuidValue {
		if ch != '-' {
			compact += string(ch)
		}
		if len(compact) == 8 {
			break
		}
	}
	return compact
}
