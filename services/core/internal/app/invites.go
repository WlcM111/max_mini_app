package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// InviteCreated — созданное приглашение вместе со ссылкой (выдаётся один раз).
type InviteCreated struct {
	Invite    domain.Invite
	LinkURL   string
	ShareText string
}

// CreateInvite создаёт одноразовое приглашение и ссылку-диплинк.
func (a *App) CreateInvite(ctx context.Context, actor Actor, orgPublicID, publicID string, role domain.Role) (InviteCreated, error) {
	org, _, err := a.authorize(ctx, actor, orgPublicID, domain.RoleOwner)
	if err != nil {
		return InviteCreated{}, err
	}
	v := &domain.Validator{}
	if !domain.IsUUIDv4(publicID) {
		v.Add("id", domain.CodeInvalidFormat, "ожидается UUID версии 4")
	}
	if !role.Invitable() {
		v.Add("role", domain.CodeUnknownValue, "допустимо editor или viewer")
	}
	if err := v.Err(); err != nil {
		return InviteCreated{}, err
	}
	// Ссылка строится по шаблону диплинка из профиля бота: сервис не собирает её сам.
	profile, err := a.botProfile(ctx)
	if err != nil {
		return InviteCreated{}, domain.ErrDependencyUnavailable
	}
	token, err := a.Random.Token()
	if err != nil {
		return InviteCreated{}, err
	}
	now := a.Clock.Now()
	invite := domain.Invite{
		PublicID: publicID, OrganizationID: org.ID, Role: role,
		CreatedByAccountID: actor.Account.ID, CreatedAt: now, ExpiresAt: now.Add(a.Settings.InviteTTL),
	}
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := a.Orgs.LockByPublicID(ctx, orgPublicID); err != nil {
			return err
		}
		// Повторное использование идентификатора приглашения запрещено:
		// ссылка выдаётся один раз и не может быть получена повторно.
		if _, err := a.Invites.GetByPublicID(ctx, publicID); err == nil {
			return domain.ErrConflictIDReused
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		members, err := a.Orgs.CountMembers(ctx, org.ID)
		if err != nil {
			return err
		}
		if members >= domain.MaxMembersPerOrganization {
			return domain.ErrQuotaExceeded
		}
		active, err := a.Invites.CountActive(ctx, org.ID, now)
		if err != nil {
			return err
		}
		if active >= domain.MaxActiveInvites {
			return domain.ErrQuotaExceeded
		}
		invite, err = a.Invites.Create(ctx, invite, TokenHash(token))
		if err != nil {
			return err
		}
		return a.Audit.Write(ctx, domain.AuditInviteCreated, actor.Account.ID, org.ID, publicID, now)
	})
	if err != nil {
		return InviteCreated{}, err
	}
	link := strings.Replace(profile.OpenAppLinkTemplate, "{payload}", "inv_"+token, 1)
	return InviteCreated{
		Invite:    invite,
		LinkURL:   link,
		ShareText: domain.InviteShareText(org.Name),
	}, nil
}

// ListInvites возвращает активные приглашения организации.
func (a *App) ListInvites(ctx context.Context, actor Actor, orgPublicID string) ([]domain.Invite, error) {
	org, _, err := a.authorize(ctx, actor, orgPublicID, domain.RoleOwner)
	if err != nil {
		return nil, err
	}
	return a.Invites.ListActive(ctx, org.ID, a.Clock.Now())
}

// RevokeInvite отзывает приглашение.
func (a *App) RevokeInvite(ctx context.Context, actor Actor, invitePublicID string) error {
	if !domain.IsUUIDv4(invitePublicID) {
		return domain.ErrNotFound
	}
	invite, err := a.Invites.GetByPublicID(ctx, invitePublicID)
	if err != nil {
		return err
	}
	org, _, err := a.authorize(ctx, actor, invite.OrganizationPubID, domain.RoleOwner)
	if err != nil {
		return err
	}
	now := a.Clock.Now()
	if invite.AcceptedAt != nil {
		// Принятое приглашение отозвать нельзя (OpenAPI 1.1.0: 409).
		return domain.ErrInviteInvalid
	}
	if invite.RevokedAt != nil {
		return nil
	}
	return a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := a.Invites.MarkRevoked(ctx, invite.ID, now); err != nil {
			return err
		}
		return a.Audit.Write(ctx, domain.AuditInviteRevoked, actor.Account.ID, org.ID, invite.PublicID, now)
	})
}

// InvitePreview — данные приглашения до принятия.
type InvitePreview struct {
	OrganizationName  string
	Role              domain.Role
	InviterFirstName  string
	ExpiresAt         time.Time
	OrganizationPubID string
}

// PreviewInvite показывает, куда приглашают.
func (a *App) PreviewInvite(ctx context.Context, actor Actor, token string) (InvitePreview, error) {
	invite, err := a.inviteByToken(ctx, actor, token)
	if err != nil {
		return InvitePreview{}, err
	}
	return InvitePreview{
		OrganizationName:  invite.OrganizationName,
		Role:              invite.Role,
		InviterFirstName:  invite.CreatedByFirstName,
		ExpiresAt:         invite.ExpiresAt,
		OrganizationPubID: invite.OrganizationPubID,
	}, nil
}

// AcceptInvite принимает приглашение и добавляет пользователя в организацию.
func (a *App) AcceptInvite(ctx context.Context, actor Actor, token string) (InvitePreview, error) {
	if !domain.IsOpaqueToken(token) {
		return InvitePreview{}, domain.ValidationFor("token", domain.CodeInvalidFormat, "ожидается 43 символа base64url")
	}
	now := a.Clock.Now()
	var (
		invite domain.Invite
		org    domain.Organization
		owner  domain.Membership
	)
	err := a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		invite, err = a.Invites.LockByTokenHash(ctx, TokenHash(token))
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return domain.ErrInviteInvalid
			}
			return err
		}
		if invite.RevokedAt != nil || invite.AcceptedAt != nil {
			return domain.ErrInviteInvalid
		}
		if !now.Before(invite.ExpiresAt) {
			return domain.ErrInviteExpired
		}
		org, err = a.Orgs.LockByPublicID(ctx, invite.OrganizationPubID)
		if err != nil {
			return err
		}
		if _, err := a.Orgs.GetMembership(ctx, org.ID, actor.Account.ID); err == nil {
			return domain.ErrAlreadyMember
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		members, err := a.Orgs.CountMembers(ctx, org.ID)
		if err != nil {
			return err
		}
		if members >= domain.MaxMembersPerOrganization {
			return domain.ErrQuotaExceeded
		}
		if _, err := a.Orgs.CreateMembership(ctx, domain.Membership{
			OrganizationID: org.ID, AccountID: actor.Account.ID, Role: invite.Role,
			NotifyEnabled: true, NotifyLocalTime: "09:00",
		}, now); err != nil {
			return err
		}
		if err := a.Invites.MarkAccepted(ctx, invite.ID, actor.Account.ID, now); err != nil {
			return err
		}
		if err := a.appendMembershipEvent(ctx, org, actor.Account, invite.Role, true, 9*60, 1); err != nil {
			return err
		}
		members2, err := a.Orgs.ListMembers(ctx, org.ID)
		if err != nil {
			return err
		}
		for _, m := range members2 {
			if m.Role == domain.RoleOwner {
				owner = m
			}
		}
		return a.Audit.Write(ctx, domain.AuditInviteAccepted, actor.Account.ID, org.ID, invite.PublicID, now)
	})
	if err != nil {
		return InvitePreview{}, err
	}
	a.flushAfterCommit(ctx)
	a.notifyOwnerAboutMember(ctx, org, owner, actor.Account, invite)
	return InvitePreview{
		OrganizationName:  org.Name,
		Role:              invite.Role,
		ExpiresAt:         invite.ExpiresAt,
		OrganizationPubID: org.PublicID,
	}, nil
}

// notifyOwnerAboutMember ставит владельцу сообщение о новом участнике.
// Ошибка доставки не отменяет принятие приглашения и только журналируется.
func (a *App) notifyOwnerAboutMember(ctx context.Context, org domain.Organization, owner domain.Membership,
	newMember domain.Account, invite domain.Invite) {
	if owner.AccountKind != domain.AccountMax || owner.MaxUserID == 0 || owner.AccountID == newMember.ID {
		return
	}
	callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.Settings.BotRPCTimeout)
	defer cancel()
	err := a.Bot.EnqueueNotification(callCtx, ports.NotificationRequest{
		IdempotencyKey: domain.MemberJoinedIdempotencyKey(invite.PublicID),
		Kind:           "member_joined",
		RecipientMax:   owner.MaxUserID,
		Text:           domain.MemberJoinedText(org.Name, newMember.FirstName, invite.Role),
		Buttons: []ports.NotificationButton{{
			Text:           domain.OpenOrganizationButton,
			OpenAppPayload: domain.OrganizationPayload(org.PublicID),
		}},
		NotAfter: a.Clock.Now().Add(24 * time.Hour).Truncate(time.Second),
	})
	if err != nil {
		a.Log.Warn("member joined notification not queued",
			slog.String("organization_id", org.PublicID), slog.Any("error", err))
	}
}

// inviteByToken выполняет общие проверки предпросмотра и принятия.
func (a *App) inviteByToken(ctx context.Context, actor Actor, token string) (domain.Invite, error) {
	if !domain.IsOpaqueToken(token) {
		return domain.Invite{}, domain.ValidationFor("token", domain.CodeInvalidFormat, "ожидается 43 символа base64url")
	}
	invite, err := a.Invites.FindByTokenHash(ctx, TokenHash(token))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Invite{}, domain.ErrInviteInvalid
		}
		return domain.Invite{}, err
	}
	if invite.RevokedAt != nil || invite.AcceptedAt != nil {
		return domain.Invite{}, domain.ErrInviteInvalid
	}
	if !a.Clock.Now().Before(invite.ExpiresAt) {
		return domain.Invite{}, domain.ErrInviteExpired
	}
	org, err := a.Orgs.GetByPublicID(ctx, invite.OrganizationPubID)
	if err != nil {
		return domain.Invite{}, err
	}
	if _, err := a.Orgs.GetMembership(ctx, org.ID, actor.Account.ID); err == nil {
		return domain.Invite{}, domain.ErrAlreadyMember
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.Invite{}, err
	}
	return invite, nil
}
