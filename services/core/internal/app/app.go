// Package app реализует сценарии core-service поверх портов: сессии,
// организации, документы, участники, приглашения, экспорт и доставку событий
// в reminders-service. Пакет не знает о HTTP, gRPC и SQL.
package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// Settings — параметры сценариев, приходящие из конфигурации сервиса.
type Settings struct {
	SessionTTL             time.Duration
	InviteTTL              time.Duration
	ExportTTL              time.Duration
	PublicBaseURL          string
	BotProfileCacheTTL     time.Duration
	BotRPCTimeout          time.Duration
	RemindersRPCTimeout    time.Duration
	SyncFlushTimeout       time.Duration
	AssistantTimeout       time.Duration
	AssistantMaxInputChars int
	OutboxBatch            int
	OutboxLease            time.Duration
	RelayInterval          time.Duration
	RelayConcurrency       int
	RetentionInterval      time.Duration
	SessionRetention       time.Duration
	AuditRetention         time.Duration
	OutboxRetention        time.Duration
}

// Deps — зависимости сценариев.
type Deps struct {
	Tx        ports.TxManager
	Accounts  ports.AccountRepo
	Sessions  ports.SessionRepo
	Catalog   ports.CatalogRepo
	Orgs      ports.OrganizationRepo
	Docs      ports.DocumentRepo
	Invites   ports.InviteRepo
	Exports   ports.ExportRepo
	Audit     ports.AuditRepo
	Outbox    ports.OutboxRepo
	Launch    ports.LaunchVerifier
	Bot       ports.BotGateway
	Reminders ports.RemindersGateway
	Assistant ports.DraftAssistant
	Clock     ports.Clock
	Random    ports.Random
	Log       *slog.Logger
	Metrics   *Metrics
	Settings  Settings
}

// App — сценарии сервиса.
type App struct {
	Deps
	catalog   *domain.Catalog
	catalogMu sync.RWMutex

	profileMu      sync.Mutex
	profile        ports.BotProfile
	profileFetched time.Time

	relaySignal chan struct{}
}

// New создаёт сценарии.
func New(d Deps) *App {
	return &App{Deps: d, relaySignal: make(chan struct{}, 1)}
}

// LoadCatalog читает справочник из хранилища и кэширует его в памяти.
func (a *App) LoadCatalog(ctx context.Context) error {
	c, err := a.Catalog.Load(ctx)
	if err != nil {
		return err
	}
	a.catalogMu.Lock()
	a.catalog = c
	a.catalogMu.Unlock()
	return nil
}

// CatalogSnapshot возвращает загруженный справочник.
func (a *App) CatalogSnapshot() *domain.Catalog {
	a.catalogMu.RLock()
	defer a.catalogMu.RUnlock()
	return a.catalog
}

// Actor — действующий пользователь запроса.
type Actor struct {
	Account   domain.Account
	TokenHash []byte
}

// authorize проверяет доступ к организации: отсутствие членства неотличимо от
// отсутствия организации (404), недостаточная роль — 403.
func (a *App) authorize(ctx context.Context, actor Actor, orgPublicID string, min domain.Role) (domain.Organization, domain.Membership, error) {
	if !domain.IsUUIDv4(orgPublicID) {
		return domain.Organization{}, domain.Membership{}, domain.ErrNotFound
	}
	org, err := a.Orgs.GetByPublicID(ctx, orgPublicID)
	if err != nil {
		return domain.Organization{}, domain.Membership{}, err
	}
	m, err := a.Orgs.GetMembership(ctx, org.ID, actor.Account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Organization{}, domain.Membership{}, domain.ErrNotFound
		}
		return domain.Organization{}, domain.Membership{}, err
	}
	if !m.Role.Allows(min) {
		return domain.Organization{}, domain.Membership{}, domain.ErrForbidden
	}
	return org, m, nil
}

// botProfile возвращает профиль бота из кэша либо запрашивает его у bot-service.
func (a *App) botProfile(ctx context.Context) (ports.BotProfile, error) {
	now := a.Clock.Now()
	a.profileMu.Lock()
	cached := a.profile
	fresh := !a.profileFetched.IsZero() && now.Sub(a.profileFetched) < a.Settings.BotProfileCacheTTL
	a.profileMu.Unlock()
	if fresh {
		return cached, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, a.Settings.BotRPCTimeout)
	defer cancel()
	p, err := a.Bot.GetBotProfile(callCtx)
	if err != nil {
		if cached.Username != "" {
			return cached, nil
		}
		return ports.BotProfile{}, err
	}
	a.profileMu.Lock()
	a.profile = p
	a.profileFetched = now
	a.profileMu.Unlock()
	return p, nil
}

// timezone возвращает часовой пояс организации; при неизвестном поясе — UTC.
func (a *App) timezone(org domain.Organization) *time.Location {
	loc, err := org.Location()
	if err != nil {
		a.Log.Warn("unknown organization timezone", slog.String("timezone", org.Timezone))
		return time.UTC
	}
	return loc
}
