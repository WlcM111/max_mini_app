package httpapi

import "net/http"

// routes регистрирует пути публичного API (openapi.yaml 1.1.0).
func (s *Server) routes(mux *http.ServeMux) {
	const p = "/api/v1"

	mux.HandleFunc("POST "+p+"/sessions", s.handleCreateSession)
	mux.Handle("DELETE "+p+"/sessions/current", s.authenticated(s.handleDeleteSession))
	mux.Handle("GET "+p+"/me", s.authenticated(s.handleGetMe))
	mux.Handle("DELETE "+p+"/me", s.authenticated(s.handleDeleteMe))
	mux.Handle("GET "+p+"/catalog", s.authenticated(s.handleGetCatalog))

	mux.Handle("POST "+p+"/organizations", s.authenticated(s.handleCreateOrganization))
	mux.Handle("GET "+p+"/organizations/{organizationId}", s.authenticated(s.handleGetOrganization))
	mux.Handle("PATCH "+p+"/organizations/{organizationId}", s.authenticated(s.handleUpdateOrganization))
	mux.Handle("DELETE "+p+"/organizations/{organizationId}", s.authenticated(s.handleDeleteOrganization))
	mux.Handle("GET "+p+"/organizations/{organizationId}/suggestions", s.authenticated(s.handleSuggestions))

	mux.Handle("GET "+p+"/organizations/{organizationId}/documents", s.authenticated(s.handleListDocuments))
	mux.Handle("POST "+p+"/organizations/{organizationId}/documents", s.authenticated(s.handleCreateDocument))
	mux.Handle("POST "+p+"/organizations/{organizationId}/documents/batch", s.authenticated(s.handleCreateDocumentsBatch))
	mux.Handle("GET "+p+"/documents/{documentId}", s.authenticated(s.handleGetDocument))
	mux.Handle("PATCH "+p+"/documents/{documentId}", s.authenticated(s.handleUpdateDocument))
	mux.Handle("DELETE "+p+"/documents/{documentId}", s.authenticated(s.handleDeleteDocument))
	mux.Handle("POST "+p+"/documents/{documentId}/renewals", s.authenticated(s.handleRenewDocument))

	mux.Handle("GET "+p+"/organizations/{organizationId}/members", s.authenticated(s.handleListMembers))
	mux.Handle("PATCH "+p+"/organizations/{organizationId}/members/{accountId}", s.authenticated(s.handleUpdateMemberRole))
	mux.Handle("DELETE "+p+"/organizations/{organizationId}/members/{accountId}", s.authenticated(s.handleRemoveMember))
	mux.Handle("GET "+p+"/organizations/{organizationId}/notification-settings", s.authenticated(s.handleGetNotificationSettings))
	mux.Handle("PUT "+p+"/organizations/{organizationId}/notification-settings", s.authenticated(s.handlePutNotificationSettings))

	mux.Handle("GET "+p+"/organizations/{organizationId}/invites", s.authenticated(s.handleListInvites))
	mux.Handle("POST "+p+"/organizations/{organizationId}/invites", s.authenticated(s.handleCreateInvite))
	mux.Handle("DELETE "+p+"/invites/{inviteId}", s.authenticated(s.handleRevokeInvite))
	mux.Handle("POST "+p+"/invites/preview", s.authenticated(s.handlePreviewInvite))
	mux.Handle("POST "+p+"/invites/accept", s.authenticated(s.handleAcceptInvite))

	mux.Handle("POST "+p+"/organizations/{organizationId}/exports/calendar", s.authenticated(s.handleCreateExport))
	mux.HandleFunc("GET "+p+"/downloads/{downloadToken}", s.handleDownloadCalendar)
	mux.HandleFunc("POST "+p+"/client-events", s.handleClientEvents)
}
