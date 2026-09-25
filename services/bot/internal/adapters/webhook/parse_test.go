package webhook_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"vovremya/services/bot/internal/adapters/webhook"
	"vovremya/services/bot/internal/domain"
)

func load(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("чтение образца %s: %v", name, err)
	}
	return data
}

func TestParseSamples(t *testing.T) {
	started := webhook.ParseUpdate(load(t, "bot_started.json"))
	if !started.Parsed || started.Update.Type != domain.UpdateBotStarted || started.Update.MaxUserID != 1001 {
		t.Fatalf("bot_started разобран неверно: %+v", started)
	}
	if want := time.UnixMilli(1790000000000).UTC(); !started.Update.EventTime.Equal(want) {
		t.Errorf("время события: ожидалось %s, получено %s", want, started.Update.EventTime)
	}
	if started.Update.Reply() != domain.ReplyWelcome {
		t.Error("на bot_started ожидается приветствие")
	}

	message := webhook.ParseUpdate(load(t, "message_created.json"))
	if !message.Parsed || message.Update.MaxUserID != 1001 || message.Update.Text != "/help" {
		t.Fatalf("message_created разобран неверно: %+v", message.Update)
	}
	if message.FromBot {
		t.Error("отправитель не бот")
	}
	if message.Update.Reply() != domain.ReplyHelp {
		t.Error("на /help ожидается справка")
	}

	muted := webhook.ParseUpdate(load(t, "dialog_muted.json"))
	if !muted.Parsed || muted.Update.Type != domain.UpdateDialogMuted || muted.Update.MaxUserID != 1001 {
		t.Fatalf("dialog_muted разобран неверно: %+v", muted.Update)
	}
	if muted.Update.Reply() != domain.ReplyNone {
		t.Error("на dialog_muted ответ не отправляется")
	}
}

func TestParseTolerance(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		parsed bool
		check  func(t *testing.T, in webhook.ParsedUpdate)
	}{
		{name: "пустое тело", body: ``},
		{name: "не объект", body: `[1,2,3]`},
		{name: "нет типа", body: `{"timestamp":1790000000000,"user":{"user_id":5}}`},
		{name: "нет времени", body: `{"update_type":"bot_started","user":{"user_id":5}}`},
		{name: "время строкой", body: `{"update_type":"bot_started","timestamp":"1790000000000","user":{"user_id":5}}`, parsed: true,
			check: func(t *testing.T, in webhook.ParsedUpdate) {
				if in.Update.MaxUserID != 5 {
					t.Errorf("идентификатор пользователя: %d", in.Update.MaxUserID)
				}
			}},
		{name: "время нулевое", body: `{"update_type":"bot_started","timestamp":0,"user":{"user_id":5}}`},
		{name: "неизвестный тип и лишние поля", body: `{"update_type":"comment_created","timestamp":1790000000000,"extra":{"a":1}}`, parsed: true,
			check: func(t *testing.T, in webhook.ParsedUpdate) {
				if in.Update.Type.AffectsRecipient() {
					t.Error("comment_created не должен влиять на состояние получателя")
				}
			}},
		{name: "идентификатор из sender", body: `{"update_type":"message_created","timestamp":1790000000000,
			"message":{"sender":{"user_id":77,"is_bot":true},"body":{"text":"привет"}}}`, parsed: true,
			check: func(t *testing.T, in webhook.ParsedUpdate) {
				if in.Update.MaxUserID != 77 || !in.FromBot {
					t.Errorf("sender разобран неверно: %+v", in)
				}
				if in.Update.Text != "привет" {
					t.Errorf("текст сообщения: %q", in.Update.Text)
				}
			}},
		{name: "текст неверного типа", body: `{"update_type":"message_created","timestamp":1790000000000,
			"message":{"sender":{"user_id":77},"body":{"text":42}}}`, parsed: true,
			check: func(t *testing.T, in webhook.ParsedUpdate) {
				if in.Update.MaxUserID != 77 {
					t.Errorf("идентификатор потерян: %+v", in.Update)
				}
			}},
		{name: "отрицательный идентификатор", body: `{"update_type":"bot_started","timestamp":1790000000000,"user":{"user_id":-3}}`, parsed: true,
			check: func(t *testing.T, in webhook.ParsedUpdate) {
				if in.Update.MaxUserID != 0 {
					t.Errorf("отрицательный идентификатор должен игнорироваться: %d", in.Update.MaxUserID)
				}
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := webhook.ParseUpdate([]byte(c.body))
			if in.Parsed != c.parsed {
				t.Fatalf("Parsed: ожидалось %t, получено %t", c.parsed, in.Parsed)
			}
			if in.Key == (domain.DedupeKey{}) {
				t.Error("ключ дедупликации должен вычисляться всегда")
			}
			if c.check != nil {
				c.check(t, in)
			}
		})
	}
}

func TestDedupeKeyDependsOnBody(t *testing.T) {
	a := webhook.ParseUpdate([]byte(`{"update_type":"bot_started","timestamp":1}`))
	b := webhook.ParseUpdate([]byte(`{"update_type":"bot_started","timestamp":2}`))
	if a.Key == b.Key {
		t.Error("разные тела должны давать разные ключи дедупликации")
	}
	again := webhook.ParseUpdate([]byte(`{"update_type":"bot_started","timestamp":1}`))
	if a.Key != again.Key {
		t.Error("одно и то же тело должно давать один ключ")
	}
}
