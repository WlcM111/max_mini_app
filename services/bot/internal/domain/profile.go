package domain

import (
	"regexp"
	"strings"
)

// Mode — режим работы канала MAX.
type Mode string

const (
	ModeStub Mode = "stub"
	ModeLive Mode = "live"
)

// Profile — профиль бота из GET /me.
type Profile struct {
	UserID      int64
	Username    string
	DisplayName string
}

// Ник подставляется в путь ссылки https://max.ru/<username> без экранирования,
// поэтому допускаются только символы, безопасные для пути URL.
var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// Validate проверяет, что из профиля можно построить диплинк.
func (p Profile) Validate() error {
	if !usernameRe.MatchString(p.Username) {
		return invalid("username", "ожидается непустой ник из латиницы, цифр, «_», «.», «-»")
	}
	return nil
}

// ChatURL — ссылка на чат с ботом: https://max.ru/<username> (spec §10).
func (p Profile) ChatURL() string { return "https://max.ru/" + p.Username }

// OpenAppLinkTemplate — шаблон диплинка с подстановкой {payload} (F-05).
func (p Profile) OpenAppLinkTemplate() string { return p.ChatURL() + "?startapp={payload}" }

// OpenAppLink — диплинк мини-приложения; для пустого payload — «…?startapp».
func (p Profile) OpenAppLink(payload string) string {
	if payload == "" {
		return p.ChatURL() + "?startapp"
	}
	return strings.Replace(p.OpenAppLinkTemplate(), "{payload}", payload, 1)
}
