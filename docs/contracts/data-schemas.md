# Сквозные схемы данных: API ↔ frontend ↔ domain ↔ PostgreSQL ↔ proto

Версия 1.0.0 · 20.09.2026. Типы frontend — `components['schemas'][...]` из `frontend/src/api/schema.d.ts`, сгенерированного из openapi.yaml; имена Go-типов — пакет `services/core/internal/domain`.

| OpenAPI / TS | Go domain (core) | PostgreSQL | Примечание |
|---|---|---|---|
| `Account` | `Account{PublicID, Kind, MaxUserID, ReviewLogin, FirstName, LastName, Username, LanguageCode}` | core.accounts | `id` = `public_id` |
| `Session` | `Session{TokenHash, AccountID, ExpiresAt}` + открытый токен при создании | core.sessions | токен не хранится |
| `StartTarget` | `StartTarget{Kind, DocumentID, OrganizationID, InviteToken}` | — | из `start_param` |
| `Organization` | `Organization{PublicID, Name, CategoryCode, RegionCode, Timezone, FeatureCodes, Version}` + `MyRole` + `Stats` | core.organizations, organization_features, memberships | `stats` вычисляется запросом |
| `DocumentStats` | `DocumentStats` | агрегат по текущим периодам | даты — в поясе организации |
| `MembershipSummary`, `Member` | `Membership{OrganizationID, AccountID, Role, NotifyEnabled, NotifyLocalTime, JoinedAt}` | core.memberships | |
| `Role`, `InvitableRole` | `Role` (`RoleOwner`, `RoleEditor`, `RoleViewer`) | `memberships.role`, `invites.role` | порядок прав viewer < editor < owner |
| `NotificationSettings` | `NotifySettings{Enabled, LocalTime}` | `memberships.notify_enabled`, `notify_local_time` | `"HH:MM"` ↔ `time` |
| `DocumentType`, `Catalog` | `Catalog`, `DocumentType{Code, Title, Description, DataStatus, Source, DefaultOffsets, RenewalSteps}` | справочные таблицы | загружается в память при старте |
| `Suggestion` | `DocumentType` + правило | applicability_rules | |
| `Document` | `Document{PublicID, OrganizationID, TypeCode, Title, Number, Issuer, ResponsibleLabel, Notes, ReferenceURL, Version, Offsets}` + `Period` + `Periods` | core.documents, document_reminder_offsets, document_periods | `status`, `days_left`, `can_edit` вычисляются |
| `DocumentListItem` | проекция `DocumentRow` | documents ⋈ текущий период | `next_reminder_at` — минимум `due_at` для текущего пользователя |
| `Period` | `Period{PublicID, ValidFrom, ValidUntil, IsCurrent, CreatedAt}` | core.document_periods | даты `civil date` без времени |
| `DeadlineStatus` | `DeadlineStatus` = `domain.StatusOf(validUntil, today)` | — | правило: < today → expired; ≤ today + 30 → expiring; иначе valid; NULL → no_expiry |
| `ReminderOffsets` | `[]int` (0–365, ≤ 5, уникальные, отсортированы по убыванию) | core.document_reminder_offsets | |
| `InviteCreated`, `InviteSummary`, `InvitePreview` | `Invite{PublicID, OrganizationID, Role, ExpiresAt, AcceptedAt, RevokedAt}` | core.invites | ссылка только в ответе создания |
| `CalendarExport` | `Export{TokenHash, ExpiresAt}` | core.calendar_exports | |
| `RemindersChannel.state` | `RecipientState` | — (из bot по gRPC) | proto `RECIPIENT_STATE_ACTIVE` → `active`; ошибка gRPC → `unavailable` |
| `ClientEvent` | `ClientEvent` | не хранится (метрики и логи) | |
| — | `ReminderPlanItem{PeriodID, AccountID, DaysBefore, DueAt}` | core.reminders | ↔ proto `EnqueueNotificationRequest` (ключ `rem:<period_uuid>:<account_uuid>:<days>`) |
