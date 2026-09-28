package domain

import (
	"regexp"
	"strings"
)

// StartKind — вид цели запуска мини-приложения.
type StartKind string

// Виды цели запуска (spec §5).
const (
	StartNone         StartKind = "none"
	StartDocument     StartKind = "document"
	StartOrganization StartKind = "organization"
	StartInvite       StartKind = "invite"
	StartRenew        StartKind = "renew" // экран продления: кнопка «Продлил» в напоминании
)

// StartTarget — разобранный start_param.
type StartTarget struct {
	Kind           StartKind
	DocumentID     string
	OrganizationID string
	InviteToken    string
}

var (
	uuidV4Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	tokenRe  = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

// IsUUIDv4 проверяет строку на формат UUID версии 4 в нижнем регистре.
func IsUUIDv4(s string) bool { return uuidV4Re.MatchString(s) }

// IsOpaqueToken проверяет формат непрозрачного токена (43 символа base64url).
func IsOpaqueToken(s string) bool { return tokenRe.MatchString(s) }

// ParseStartTarget разбирает start_param по грамматике spec §5.
// Значение вне грамматики не является ошибкой и даёт kind=none.
func ParseStartTarget(param string) StartTarget {
	switch {
	case strings.HasPrefix(param, "doc_") && IsUUIDv4(param[4:]):
		return StartTarget{Kind: StartDocument, DocumentID: param[4:]}
	case strings.HasPrefix(param, "org_") && IsUUIDv4(param[4:]):
		return StartTarget{Kind: StartOrganization, OrganizationID: param[4:]}
	case strings.HasPrefix(param, "inv_") && IsOpaqueToken(param[4:]):
		return StartTarget{Kind: StartInvite, InviteToken: param[4:]}
	case strings.HasPrefix(param, "renew_") && IsUUIDv4(param[6:]):
		return StartTarget{Kind: StartRenew, DocumentID: param[6:]}
	default:
		return StartTarget{Kind: StartNone}
	}
}
