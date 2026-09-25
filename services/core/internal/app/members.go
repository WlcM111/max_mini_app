package app

import (
	"context"
	"errors"

	"vovremya/services/core/internal/domain"
)

// ListMembers возвращает участников организации.
func (a *App) ListMembers(ctx context.Context, actor Actor, orgPublicID string) ([]domain.Membership, error) {
	org, _, err := a.authorize(ctx, actor, orgPublicID, domain.RoleViewer)
	if err != nil {
		return nil, err
	}
	return a.Orgs.ListMembers(ctx, org.ID)
}

// UpdateMemberRole меняет роль участника; роль владельца не меняется.
func (a *App) UpdateMemberRole(ctx context.Context, actor Actor, orgPublicID, accountPublicID string,
	role domain.Role) (domain.Membership, error) {
	org, _, err := a.authorize(ctx, actor, orgPublicID, domain.RoleOwner)
	if err != nil {
		return domain.Membership{}, err
	}
	if !role.Invitable() {
		return domain.Membership{}, domain.ValidationFor("role", domain.CodeUnknownValue, "допустимо editor или viewer")
	}
	target, err := a.Accounts.GetByPublicID(ctx, accountPublicID)
	if err != nil {
		return domain.Membership{}, err
	}
	now := a.Clock.Now()
	var updated domain.Membership
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := a.Orgs.LockByPublicID(ctx, orgPublicID); err != nil {
			return err
		}
		m, err := a.Orgs.GetMembership(ctx, org.ID, target.ID)
		if err != nil {
			return err
		}
		if m.Role == domain.RoleOwner {
			return domain.ErrForbidden
		}
		m.Role = role
		m.Version++
		updated, err = a.Orgs.UpdateMembership(ctx, m)
		if err != nil {
			return err
		}
		if err := a.appendMembershipEvent(ctx, org, target, role, m.NotifyEnabled, m.NotifyLocalMinutes(), m.Version); err != nil {
			return err
		}
		return a.Audit.Write(ctx, domain.AuditMemberRoleChanged, actor.Account.ID, org.ID, target.PublicID, now)
	})
	if err != nil {
		return domain.Membership{}, err
	}
	a.flushAfterCommit(ctx)
	updated.AccountPublicID = target.PublicID
	updated.AccountFirstName = target.FirstName
	updated.AccountLastName = target.LastName
	return updated, nil
}

// RemoveMember исключает участника или выводит пользователя из организации.
func (a *App) RemoveMember(ctx context.Context, actor Actor, orgPublicID, accountPublicID string) error {
	org, mine, err := a.authorize(ctx, actor, orgPublicID, domain.RoleViewer)
	if err != nil {
		return err
	}
	target, err := a.Accounts.GetByPublicID(ctx, accountPublicID)
	if err != nil {
		return err
	}
	self := target.ID == actor.Account.ID
	if self && mine.Role == domain.RoleOwner {
		// Владелец не может выйти из организации: нужно удалить организацию.
		return domain.ErrForbidden
	}
	if !self && !mine.Role.Allows(domain.RoleOwner) {
		return domain.ErrForbidden
	}
	now := a.Clock.Now()
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := a.Orgs.LockByPublicID(ctx, orgPublicID); err != nil {
			return err
		}
		m, err := a.Orgs.GetMembership(ctx, org.ID, target.ID)
		if err != nil {
			return err
		}
		if !self && m.Role == domain.RoleOwner {
			return domain.ErrForbidden
		}
		if err := a.Orgs.DeleteMembership(ctx, org.ID, target.ID); err != nil {
			return err
		}
		if err := a.appendMembershipRemoved(ctx, org.PublicID, target.PublicID, m.Version+1); err != nil {
			return err
		}
		return a.Audit.Write(ctx, domain.AuditMemberRemoved, actor.Account.ID, org.ID, target.PublicID, now)
	})
	if err != nil {
		return err
	}
	a.flushAfterCommit(ctx)
	return nil
}

// GetNotificationSettings возвращает настройки напоминаний пользователя.
func (a *App) GetNotificationSettings(ctx context.Context, actor Actor, orgPublicID string) (domain.Membership, error) {
	_, m, err := a.authorize(ctx, actor, orgPublicID, domain.RoleViewer)
	if err != nil {
		return domain.Membership{}, err
	}
	return m, nil
}

// PutNotificationSettings изменяет настройки напоминаний пользователя.
func (a *App) PutNotificationSettings(ctx context.Context, actor Actor, orgPublicID string,
	enabled bool, localTime string) (domain.Membership, error) {
	org, _, err := a.authorize(ctx, actor, orgPublicID, domain.RoleViewer)
	if err != nil {
		return domain.Membership{}, err
	}
	v := &domain.Validator{}
	domain.ValidateNotifyLocalTime(localTime, v)
	if err := v.Err(); err != nil {
		return domain.Membership{}, err
	}
	var updated domain.Membership
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := a.Orgs.LockByPublicID(ctx, orgPublicID); err != nil {
			return err
		}
		m, err := a.Orgs.GetMembership(ctx, org.ID, actor.Account.ID)
		if err != nil {
			return err
		}
		m.NotifyEnabled = enabled
		m.NotifyLocalTime = localTime
		m.Version++
		updated, err = a.Orgs.UpdateMembership(ctx, m)
		if err != nil {
			return err
		}
		return a.appendMembershipEvent(ctx, org, actor.Account, m.Role, enabled, m.NotifyLocalMinutes(), m.Version)
	})
	if err != nil {
		return domain.Membership{}, err
	}
	a.flushAfterCommit(ctx)
	return updated, nil
}

var _ = errors.Is
