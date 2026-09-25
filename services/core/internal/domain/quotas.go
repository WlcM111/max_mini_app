package domain

// Квоты продукта (handoff core-service §7, OpenAPI Limits).
const (
	MaxOrganizationsPerAccount   = 20
	MaxDocumentsPerOrganization  = 500
	MaxMembersPerOrganization    = 30
	MaxActiveInvites             = 50
	MaxReminderOffsets           = 5
	MaxDocumentsPerBatch         = 30
	MaxPeriodsInDocumentResponse = 20
)
