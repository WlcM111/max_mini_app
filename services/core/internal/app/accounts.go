package app

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"time"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

var reviewLoginRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)

// MembershipSummary — организация пользователя в ответе GET /me.
type MembershipSummary struct {
	Organization  domain.Organization
	Role          domain.Role
	NotifyEnabled bool
}

// MeResult — ответ GET /me.
type MeResult struct {
	Account     domain.Account
	Memberships []MembershipSummary
	Channel     ports.RecipientStatus
}

// GetMe возвращает пользователя, его организации и состояние канала напоминаний.
func (a *App) GetMe(ctx context.Context, actor Actor) (MeResult, error) {
	memberships, orgs, err := a.Orgs.ListMemberships(ctx, actor.Account.ID)
	if err != nil {
		return MeResult{}, err
	}
	byID := make(map[int64]domain.Organization, len(orgs))
	for _, o := range orgs {
		byID[o.ID] = o
	}
	out := make([]MembershipSummary, 0, len(memberships))
	for _, m := range memberships {
		out = append(out, MembershipSummary{
			Organization:  byID[m.OrganizationID],
			Role:          m.Role,
			NotifyEnabled: m.NotifyEnabled,
		})
	}

	channel := ports.RecipientStatus{State: "unknown"}
	if actor.Account.Kind == domain.AccountMax {
		callCtx, cancel := context.WithTimeout(ctx, a.Settings.BotRPCTimeout)
		status, err := a.Bot.GetRecipientStatus(callCtx, actor.Account.MaxUserID)
		cancel()
		if err != nil {
			a.Log.Warn("recipient status unavailable", slog.Any("error", err))
			channel = ports.RecipientStatus{State: "unavailable"}
			if p, perr := a.botProfile(ctx); perr == nil {
				channel.BotChatURL = p.ChatURL
			}
		} else {
			channel = status
		}
	}
	return MeResult{Account: actor.Account, Memberships: out, Channel: channel}, nil
}

// DeleteMe удаляет аккаунт: организации, где пользователь владелец, удаляются
// целиком; для reminders-service формируются события удаления.
func (a *App) DeleteMe(ctx context.Context, actor Actor) error {
	now := a.Clock.Now()
	err := a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		owned, err := a.Orgs.ListOwnedOrganizations(ctx, actor.Account.ID)
		if err != nil {
			return err
		}
		for _, org := range owned {
			if err := a.Audit.Write(ctx, domain.AuditOrganizationDeleted, actor.Account.ID, org.ID, org.PublicID, now); err != nil {
				return err
			}
			if err := a.appendOrganizationDeleted(ctx, org); err != nil {
				return err
			}
			if err := a.Orgs.Delete(ctx, org.ID); err != nil {
				return err
			}
		}
		memberships, orgs, err := a.Orgs.ListMemberships(ctx, actor.Account.ID)
		if err != nil {
			return err
		}
		byID := make(map[int64]domain.Organization, len(orgs))
		for _, o := range orgs {
			byID[o.ID] = o
		}
		for _, m := range memberships {
			org, ok := byID[m.OrganizationID]
			if !ok {
				continue
			}
			if err := a.appendMembershipRemoved(ctx, org.PublicID, actor.Account.PublicID, m.Version+1); err != nil {
				return err
			}
		}
		if err := a.appendAccountDeleted(ctx, actor.Account); err != nil {
			return err
		}
		if err := a.Audit.Write(ctx, domain.AuditAccountDeleted, 0, 0, actor.Account.PublicID, now); err != nil {
			return err
		}
		return a.Accounts.Delete(ctx, actor.Account.ID)
	})
	if err != nil {
		return err
	}
	a.flushAfterCommit(ctx)
	return nil
}

// ClientEventsInput — пакет технических событий мини-приложения.
type ClientEventsInput struct {
	Events []ClientEvent
}

// ClientEvent — одно техническое событие клиента.
type ClientEvent struct {
	Name       string
	Platform   string
	AppVersion string
	DurationMs int
	Code       string
}

var allowedClientCodes = map[string]struct{}{
	"not_in_max": {}, "launch_invalid": {}, "launch_expired": {}, "network": {},
	"server_5xx": {}, "bridge_download_failed": {}, "bridge_share_failed": {},
	"bridge_qr_failed": {}, "bridge_unsupported": {},
}

var allowedClientNames = map[string]struct{}{
	"bootstrap_completed": {}, "bootstrap_failed": {}, "bridge_error": {}, "api_error_shown": {},
}

// AcceptClientEvents принимает технические события клиента (телеметрия).
func (a *App) AcceptClientEvents(ctx context.Context, in ClientEventsInput) error {
	if len(in.Events) == 0 || len(in.Events) > 20 {
		return domain.ValidationFor("events", domain.CodeOutOfRange, "ожидается от 1 до 20 событий")
	}
	for _, e := range in.Events {
		if _, ok := allowedClientNames[e.Name]; !ok {
			return domain.ValidationFor("events.name", domain.CodeUnknownValue, "неизвестное событие")
		}
		if e.DurationMs < 0 || e.DurationMs > 120000 {
			return domain.ValidationFor("events.duration_ms", domain.CodeOutOfRange, "ожидается 0…120000")
		}
		code := e.Code
		if code != "" {
			if _, ok := allowedClientCodes[code]; !ok {
				code = "other"
			}
		}
		a.Metrics.ClientEvents.WithLabelValues(e.Name, code).Inc()
		a.Log.Info("client event",
			slog.String("event", e.Name),
			slog.String("code", code),
			slog.String("platform", e.Platform),
			slog.Int("duration_ms", e.DurationMs))
	}
	return nil
}

// RunRetention удаляет устаревшие технические данные по расписанию.
func (a *App) RunRetention(ctx context.Context) error {
	ticker := time.NewTicker(a.Settings.RetentionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		if err := a.RetentionOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.Log.Warn("retention failed", slog.Any("error", err))
		}
	}
}

// RetentionOnce выполняет один проход очистки.
func (a *App) RetentionOnce(ctx context.Context) error {
	now := a.Clock.Now()
	if _, err := a.Sessions.DeleteExpiredBefore(ctx, now.Add(-a.Settings.SessionRetention)); err != nil {
		return err
	}
	if _, err := a.Invites.DeleteExpiredBefore(ctx, now.Add(-a.Settings.SessionRetention)); err != nil {
		return err
	}
	if _, err := a.Exports.DeleteExpiredBefore(ctx, now.Add(-24*time.Hour)); err != nil {
		return err
	}
	if _, err := a.Outbox.DeleteSentBefore(ctx, now.Add(-a.Settings.OutboxRetention)); err != nil {
		return err
	}
	if _, err := a.Audit.DeleteBefore(ctx, now.Add(-a.Settings.AuditRetention)); err != nil {
		return err
	}
	return nil
}
