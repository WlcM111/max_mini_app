package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// SessionResult — созданная сессия.
type SessionResult struct {
	Token     string
	ExpiresAt time.Time
	Account   domain.Account
	Start     domain.StartTarget
}

// TokenHash возвращает хеш токена сессии для хранения и поиска.
func TokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

const sessionTokenPrefix = "vvs_"

// CreateSessionLimited — CreateSession с ограничением частоты по пользователю MAX:
// allow вызывается после проверки подписи и до записи сессии.
func (a *App) CreateSessionLimited(ctx context.Context, initData, platform, appVersion string,
	allow func(maxUserID int64) bool) (SessionResult, error) {
	now := a.Clock.Now()
	// Длина проверяется в символах: OpenAPI задаёт minLength/maxLength в символах.
	if l := utf8.RuneCountInString(initData); l < 16 || l > 4096 {
		a.Metrics.SessionCreate.WithLabelValues("malformed").Inc()
		return SessionResult{}, domain.ValidationFor("init_data", domain.CodeInvalidFormat,
			"ожидается строка длиной от 16 до 4096 символов")
	}
	if platform != "" && platform != "ios" && platform != "android" && platform != "desktop" && platform != "web" {
		return SessionResult{}, domain.ValidationFor("platform", domain.CodeUnknownValue, "неизвестная платформа")
	}
	if len(appVersion) > 32 {
		return SessionResult{}, domain.ValidationFor("app_version", domain.CodeTooLong, "не более 32 символов")
	}

	identity, err := a.Launch.Verify(initData, now)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrLaunchExpired):
			a.Metrics.SessionCreate.WithLabelValues("expired").Inc()
		case errors.Is(err, domain.ErrLaunchInvalid):
			a.Metrics.SessionCreate.WithLabelValues("invalid_signature").Inc()
		default:
			a.Metrics.SessionCreate.WithLabelValues("malformed").Inc()
		}
		return SessionResult{}, err
	}
	if allow != nil && !allow(identity.MaxUserID) {
		a.Metrics.SessionCreate.WithLabelValues("rate_limited").Inc()
		return SessionResult{}, domain.ErrRateLimited
	}

	token, err := a.Random.Token()
	if err != nil {
		return SessionResult{}, err
	}
	token = sessionTokenPrefix + token
	expiresAt := now.Add(a.Settings.SessionTTL)

	var account domain.Account
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		account, err = a.Accounts.UpsertMax(ctx, identity, now)
		if err != nil {
			return err
		}
		if err := a.Sessions.Create(ctx, TokenHash(token), account.ID, "max_launch",
			identity.QueryID, platform, now, expiresAt); err != nil {
			return err
		}
		return a.Audit.Write(ctx, domain.AuditSessionCreated, account.ID, 0, account.PublicID, now)
	})
	if err != nil {
		return SessionResult{}, err
	}
	a.Metrics.SessionCreate.WithLabelValues("ok").Inc()
	a.Log.Info("session created",
		slog.String("account_id", account.PublicID),
		slog.String("platform", platform),
		slog.String("app_version", appVersion))
	return SessionResult{
		Token:     token,
		ExpiresAt: expiresAt,
		Account:   account,
		Start:     domain.ParseStartTarget(identity.StartParam),
	}, nil
}

// Authenticate проверяет токен сессии и возвращает действующего пользователя.
func (a *App) Authenticate(ctx context.Context, bearer string) (Actor, error) {
	token := strings.TrimSpace(bearer)
	if !strings.HasPrefix(token, sessionTokenPrefix) || !domain.IsOpaqueToken(strings.TrimPrefix(token, sessionTokenPrefix)) {
		return Actor{}, domain.ErrUnauthenticated
	}
	now := a.Clock.Now()
	hash := TokenHash(token)
	account, session, err := a.Sessions.FindActive(ctx, hash, now)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return Actor{}, domain.ErrUnauthenticated
		}
		return Actor{}, err
	}
	if now.Sub(session.LastSeenAt) > 5*time.Minute {
		if err := a.Sessions.Touch(ctx, hash, now); err != nil {
			a.Log.Warn("session touch failed", slog.Any("error", err))
		}
	}
	return Actor{Account: account, TokenHash: hash}, nil
}

// DeleteSession отзывает текущую сессию.
func (a *App) DeleteSession(ctx context.Context, actor Actor) error {
	return a.Sessions.RevokeByHash(ctx, actor.TokenHash, a.Clock.Now())
}

// IssueReviewToken выдаёт сессию учётной записи проверяющего (CLI).
func (a *App) IssueReviewToken(ctx context.Context, login string, role domain.Role, ttl time.Duration,
	demoOrgPublicID string) (string, time.Time, error) {
	now := a.Clock.Now()
	if !reviewLoginRe.MatchString(login) {
		return "", time.Time{}, domain.ValidationFor("login", domain.CodeInvalidFormat,
			"ожидается ^[a-z][a-z0-9_]{2,31}$")
	}
	if !role.Invitable() {
		return "", time.Time{}, domain.ValidationFor("role", domain.CodeUnknownValue, "допустимо editor или viewer")
	}
	if ttl <= 0 || ttl > 720*time.Hour {
		return "", time.Time{}, domain.ValidationFor("ttl", domain.CodeOutOfRange, "допустимо до 720h")
	}
	token, err := a.Random.Token()
	if err != nil {
		return "", time.Time{}, err
	}
	token = sessionTokenPrefix + token
	expiresAt := now.Add(ttl)
	err = a.Tx.WithinTx(ctx, func(ctx context.Context) error {
		account, err := a.Accounts.UpsertReview(ctx, login, "Проверяющий", now)
		if err != nil {
			return err
		}
		org, err := a.Orgs.LockByPublicID(ctx, demoOrgPublicID)
		if err != nil {
			return err
		}
		if _, err := a.Orgs.GetMembership(ctx, org.ID, account.ID); err != nil {
			if !errors.Is(err, domain.ErrNotFound) {
				return err
			}
			if _, err := a.Orgs.CreateMembership(ctx, domain.Membership{
				OrganizationID: org.ID, AccountID: account.ID, Role: role,
				NotifyEnabled: false, NotifyLocalTime: "09:00",
			}, now); err != nil {
				return err
			}
			if err := a.appendMembershipEvent(ctx, org, account, role, false, 9*60, 1); err != nil {
				return err
			}
		}
		if err := a.Sessions.Create(ctx, TokenHash(token), account.ID, "review_cli", "", "", now, expiresAt); err != nil {
			return err
		}
		return a.Audit.Write(ctx, domain.AuditReviewTokenIssued, account.ID, org.ID, account.PublicID, now)
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// RevokeReviewTokens отзывает все сессии учётной записи проверяющего.
func (a *App) RevokeReviewTokens(ctx context.Context, login string) error {
	account, err := a.Accounts.GetByReviewLogin(ctx, login)
	if err != nil {
		return err
	}
	return a.Sessions.RevokeByAccount(ctx, account.ID, a.Clock.Now())
}

var _ = ports.LaunchIdentity{}
