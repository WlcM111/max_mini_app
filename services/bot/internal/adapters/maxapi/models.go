// Package maxapi реализует клиент MAX Bot API (spec §8–§10, факты F-40…F-55)
// и его локальный режим stub. Формат JSON MAX known только этому пакету.
package maxapi

import (
	"fmt"

	"vovremya/services/bot/internal/domain"
)

// ButtonKind — вид кнопки для диплинка мини-приложения (BOT_OPEN_APP_BUTTON_KIND).
type ButtonKind string

const (
	// ButtonKindLink — официальный диплинк https://max.ru/<bot>?startapp=<payload> (F-05).
	ButtonKindLink ButtonKind = "link"
	// ButtonKindOpenApp — кнопка open_app (X-01, включается после проверки MAX-02).
	ButtonKindOpenApp ButtonKind = "open_app"
)

// sendMessageBody — тело POST /messages (F-49). Поле format не передаётся:
// текст сообщения обычный, разметка не применяется (spec §8).
type sendMessageBody struct {
	Text        string       `json:"text"`
	Notify      bool         `json:"notify"`
	Attachments []attachment `json:"attachments,omitempty"`
}

type attachment struct {
	Type    string          `json:"type"`
	Payload keyboardPayload `json:"payload"`
}

type keyboardPayload struct {
	Buttons [][]keyboardButton `json:"buttons"`
}

// keyboardButton — кнопка inline-клавиатуры (F-51).
type keyboardButton struct {
	Type    string  `json:"type"`
	Text    string  `json:"text"`
	URL     string  `json:"url,omitempty"`
	WebApp  string  `json:"web_app,omitempty"`
	Payload *string `json:"payload,omitempty"`
}

// sendMessageResult — ответ POST /messages; идентификатор берётся из body.mid (X-02).
type sendMessageResult struct {
	Message struct {
		Body struct {
			Mid string `json:"mid"`
		} `json:"body"`
	} `json:"message"`
}

// meResult — ответ GET /me (F-52).
type meResult struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	IsBot    bool   `json:"is_bot"`
}

// subscriptionList — ответ GET /subscriptions (F-47).
type subscriptionList struct {
	Subscriptions []struct {
		URL string `json:"url"`
	} `json:"subscriptions"`
}

// subscribeBody — тело POST /subscriptions (F-47).
type subscribeBody struct {
	URL         string   `json:"url"`
	UpdateTypes []string `json:"update_types,omitempty"`
	Secret      string   `json:"secret,omitempty"`
}

// simpleResult — ответ POST /subscriptions.
type simpleResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// botCommand — элемент PATCH /me/commands (F-53).
type botCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// renderSendBody формирует тело запроса к MAX по сообщению очереди (spec §8).
func renderSendBody(msg domain.Message, profile domain.Profile, kind ButtonKind) (sendMessageBody, error) {
	body := sendMessageBody{Text: msg.Text, Notify: !msg.Silent}
	if len(msg.Buttons) == 0 {
		return body, nil
	}
	rows := make([][]keyboardButton, 0, len(msg.Buttons))
	for _, b := range msg.Buttons {
		btn, err := renderButton(b, profile, kind)
		if err != nil {
			return sendMessageBody{}, err
		}
		rows = append(rows, []keyboardButton{btn})
	}
	body.Attachments = []attachment{{Type: "inline_keyboard", Payload: keyboardPayload{Buttons: rows}}}
	return body, nil
}

func renderButton(b domain.Button, profile domain.Profile, kind ButtonKind) (keyboardButton, error) {
	switch b.Action {
	case domain.ActionURL:
		return keyboardButton{Type: "link", Text: b.Text, URL: b.URL}, nil
	case domain.ActionOpenApp:
		if profile.Username == "" {
			return keyboardButton{}, fmt.Errorf("диплинк кнопки требует загруженного профиля бота")
		}
		if kind == ButtonKindOpenApp {
			payload := b.Payload
			return keyboardButton{Type: "open_app", Text: b.Text, WebApp: profile.Username, Payload: &payload}, nil
		}
		return keyboardButton{Type: "link", Text: b.Text, URL: profile.OpenAppLink(b.Payload)}, nil
	default:
		return keyboardButton{}, fmt.Errorf("неизвестное действие кнопки: %q", b.Action)
	}
}
