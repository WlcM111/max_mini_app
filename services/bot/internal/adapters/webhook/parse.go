// Package webhook принимает события MAX (spec §7): проверяет подлинность,
// разбирает тело запроса и передаёт событие сценарию.
package webhook

import (
	"encoding/json"
	"strconv"
	"time"

	"vovremya/services/bot/internal/app"
	"vovremya/services/bot/internal/domain"
)

// ParsedUpdate — результат разбора тела webhook (событие для сценария).
type ParsedUpdate = app.InboundUpdate

// envelope — верхний уровень события; поля читаются по отдельности,
// чтобы неизвестная или неожиданная форма не отменяла разбор остальных.
type envelope struct {
	UpdateType json.RawMessage `json:"update_type"`
	Timestamp  json.RawMessage `json:"timestamp"`
	User       json.RawMessage `json:"user"`
	Message    json.RawMessage `json:"message"`
}

type rawUser struct {
	UserID json.RawMessage `json:"user_id"`
	IsBot  *bool           `json:"is_bot"`
}

type rawMessage struct {
	Sender json.RawMessage `json:"sender"`
	Body   json.RawMessage `json:"body"`
}

type rawBody struct {
	Text json.RawMessage `json:"text"`
}

// ParseUpdate толерантно разбирает тело webhook. Отсутствие обязательных
// полей даёт событие с Parsed=false: оно записывается в журнал с исходом ignored.
func ParseUpdate(body []byte) app.InboundUpdate {
	in := app.InboundUpdate{Key: domain.NewDedupeKey(body)}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return in
	}
	if s, ok := parseString(env.UpdateType); ok {
		in.RawType = s
	}
	ms, okTime := parseInt(env.Timestamp)
	if in.RawType == "" || !okTime || ms <= 0 {
		return in
	}
	in.Parsed = true
	in.Update = domain.Update{
		Type:      domain.UpdateType(in.RawType),
		EventTime: time.UnixMilli(ms).UTC(),
	}

	if u, ok := parseUser(env.User); ok {
		in.Update.MaxUserID = u.id
		in.FromBot = u.isBot
	}
	if len(env.Message) > 0 {
		var msg rawMessage
		if err := json.Unmarshal(env.Message, &msg); err == nil {
			if in.Update.MaxUserID == 0 {
				if u, ok := parseUser(msg.Sender); ok {
					in.Update.MaxUserID = u.id
					in.FromBot = u.isBot
				}
			}
			if len(msg.Body) > 0 {
				var body rawBody
				if err := json.Unmarshal(msg.Body, &body); err == nil {
					if text, ok := parseString(body.Text); ok {
						in.Update.Text = text
					}
				}
			}
		}
	}
	return in
}

type userInfo struct {
	id    int64
	isBot bool
}

func parseUser(raw json.RawMessage) (userInfo, bool) {
	if len(raw) == 0 {
		return userInfo{}, false
	}
	var u rawUser
	if err := json.Unmarshal(raw, &u); err != nil {
		return userInfo{}, false
	}
	id, ok := parseInt(u.UserID)
	if !ok || id <= 0 {
		return userInfo{}, false
	}
	return userInfo{id: id, isBot: u.IsBot != nil && *u.IsBot}, true
}

func parseString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// parseInt читает целое число; допускается число в кавычках.
func parseInt(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		if v, err := n.Int64(); err == nil {
			return v, true
		}
		return 0, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
