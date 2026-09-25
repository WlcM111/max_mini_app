package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vovremya/services/bot/internal/adapters/webhook"
	"vovremya/services/bot/internal/domain"
)

const testSecret = "devonly-webhook-secret"

func newWebhookServer(t *testing.T, s *stack) *httptest.Server {
	t.Helper()
	h := webhook.NewHandler(s.webhook, testSecret, 256*1024, 5*time.Second, s.log, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, srv *httptest.Server, secret, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+webhook.Path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("запрос: %v", err)
	}
	if secret != "" {
		req.Header.Set("X-Max-Bot-Api-Secret", secret)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("отправка: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func countRows(t *testing.T, s *stack, query string, args ...any) int {
	t.Helper()
	ctx := context.Background()
	var n int
	if err := s.pool.DB(ctx).QueryRow(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("подсчёт: %v", err)
	}
	return n
}

func TestWebhookRejectsWithoutSecret(t *testing.T) {
	s := newStack(t, stackOptions{})
	srv := newWebhookServer(t, s)
	body := `{"update_type":"bot_started","timestamp":1790000000000,"user":{"user_id":1001}}`

	if resp := post(t, srv, "", body); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("без секрета: ожидался 401, получен %d", resp.StatusCode)
	}
	if resp := post(t, srv, "wrong-secret", body); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("с чужим секретом: ожидался 401, получен %d", resp.StatusCode)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.inbound_updates`); n != 0 {
		t.Errorf("отклонённые запросы не должны писать в журнал: %d строк", n)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages`); n != 0 {
		t.Errorf("отклонённые запросы не должны ставить сообщения: %d", n)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+webhook.Path, nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET: ожидался 405, получен %d", resp.StatusCode)
	}
}

func TestWebhookBodyLimit(t *testing.T) {
	s := newStack(t, stackOptions{})
	h := webhook.NewHandler(s.webhook, testSecret, 1024, 5*time.Second, s.log, nil)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)

	big := `{"update_type":"bot_started","timestamp":1790000000000,"padding":"` + strings.Repeat("a", 2048) + `"}`
	if resp := post(t, srv, testSecret, big); resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("большое тело: ожидался 413, получен %d", resp.StatusCode)
	}
}

func TestWebhookWelcomeAndDeduplication(t *testing.T) {
	s := newStack(t, stackOptions{})
	srv := newWebhookServer(t, s)
	ctx := context.Background()
	body := `{"update_type":"bot_started","timestamp":1790000000000,"user":{"user_id":1001,"is_bot":false}}`

	resp := post(t, srv, testSecret, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bot_started: %d", resp.StatusCode)
	}
	if data, _ := io.ReadAll(resp.Body); strings.TrimSpace(string(data)) != "{}" {
		t.Errorf("тело ответа: %q", data)
	}
	rec, found, err := s.recipients.Get(ctx, 1001)
	if err != nil || !found || rec.State != domain.RecipientActive {
		t.Fatalf("состояние получателя: %+v %t %v", rec, found, err)
	}
	key := "reply:" + domain.NewDedupeKey([]byte(body)).Hex()
	welcome := s.status(t, key)
	if welcome.Kind != domain.KindWelcome || welcome.Text != domain.TextWelcome {
		t.Errorf("приветствие: %+v", welcome)
	}
	if len(welcome.Buttons) != 1 || welcome.Buttons[0].Payload != "" || welcome.Buttons[0].Text != domain.ButtonOpenApp {
		t.Errorf("кнопка приветствия: %+v", welcome.Buttons)
	}
	if !welcome.NotAfter.Equal(s.clock.Now().Add(time.Hour)) {
		t.Errorf("срок ответа: %s", welcome.NotAfter)
	}

	// Повтор того же тела не даёт повторных эффектов (AC-MAX-08).
	if resp := post(t, srv, testSecret, body); resp.StatusCode != http.StatusOK {
		t.Fatalf("повтор: %d", resp.StatusCode)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages`); n != 1 {
		t.Errorf("после повтора сообщений: %d, ожидалось 1", n)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.inbound_updates`); n != 1 {
		t.Errorf("после повтора записей журнала: %d, ожидалось 1", n)
	}
}

func TestWebhookRepliesAndCooldown(t *testing.T) {
	s := newStack(t, stackOptions{})
	srv := newWebhookServer(t, s)

	help := `{"update_type":"message_created","timestamp":1790000100000,
		"message":{"sender":{"user_id":1001},"body":{"text":"/help"}}}`
	if resp := post(t, srv, testSecret, help); resp.StatusCode != http.StatusOK {
		t.Fatalf("/help: %d", resp.StatusCode)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages WHERE kind = 'help'`); n != 1 {
		t.Fatalf("справка не поставлена: %d", n)
	}

	// Подсказка на произвольный текст не чаще раза в 10 минут.
	s.clock.Advance(time.Minute)
	hint := `{"update_type":"message_created","timestamp":1790000200000,
		"message":{"sender":{"user_id":1001},"body":{"text":"привет"}}}`
	if resp := post(t, srv, testSecret, hint); resp.StatusCode != http.StatusOK {
		t.Fatalf("подсказка: %d", resp.StatusCode)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages WHERE kind = 'help'`); n != 1 {
		t.Errorf("подсказка внутри интервала 10 минут не должна ставиться: %d", n)
	}

	s.clock.Advance(11 * time.Minute)
	hint2 := `{"update_type":"message_created","timestamp":1790000900000,
		"message":{"sender":{"user_id":1001},"body":{"text":"как дела"}}}`
	if resp := post(t, srv, testSecret, hint2); resp.StatusCode != http.StatusOK {
		t.Fatalf("подсказка после паузы: %d", resp.StatusCode)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages WHERE kind = 'help'`); n != 2 {
		t.Errorf("после паузы подсказка должна ставиться: %d", n)
	}
	var text string
	ctx := context.Background()
	if err := s.pool.DB(ctx).QueryRow(ctx,
		`SELECT text FROM bot.outbound_messages WHERE kind = 'help' ORDER BY id DESC LIMIT 1`).Scan(&text); err != nil {
		t.Fatalf("чтение текста: %v", err)
	}
	if text != domain.TextHint {
		t.Errorf("текст подсказки: %q", text)
	}
}

func TestWebhookStateTransitionsAndOrdering(t *testing.T) {
	s := newStack(t, stackOptions{})
	srv := newWebhookServer(t, s)
	ctx := context.Background()

	events := []struct {
		body  string
		state domain.RecipientState
	}{
		{`{"update_type":"bot_started","timestamp":1790000000000,"user":{"user_id":2002}}`, domain.RecipientActive},
		{`{"update_type":"dialog_muted","timestamp":1790000100000,"user":{"user_id":2002}}`, domain.RecipientMuted},
		{`{"update_type":"dialog_unmuted","timestamp":1790000200000,"user":{"user_id":2002}}`, domain.RecipientActive},
		{`{"update_type":"bot_stopped","timestamp":1790000300000,"user":{"user_id":2002}}`, domain.RecipientStopped},
	}
	for _, e := range events {
		if resp := post(t, srv, testSecret, e.body); resp.StatusCode != http.StatusOK {
			t.Fatalf("событие: %d", resp.StatusCode)
		}
		rec, found, err := s.recipients.Get(ctx, 2002)
		if err != nil || !found || rec.State != e.state {
			t.Fatalf("после события ожидалось %s, получено %+v", e.state, rec)
		}
	}

	// Событие с меньшим временем не меняет состояние (spec §7).
	old := `{"update_type":"bot_started","timestamp":1789999000000,"user":{"user_id":2002}}`
	if resp := post(t, srv, testSecret, old); resp.StatusCode != http.StatusOK {
		t.Fatalf("устаревшее событие: %d", resp.StatusCode)
	}
	rec, _, _ := s.recipients.Get(ctx, 2002)
	if rec.State != domain.RecipientStopped {
		t.Errorf("устаревшее событие изменило состояние: %s", rec.State)
	}
}

func TestWebhookIgnoresUnknownAndOwnMessages(t *testing.T) {
	s := newStack(t, stackOptions{})
	srv := newWebhookServer(t, s)

	cases := []string{
		`{"update_type":"message_edited","timestamp":1790000000000,"message":{"sender":{"user_id":3003}}}`,
		`{"update_type":"message_created","timestamp":1790000000000,"message":{"sender":{"user_id":500100},"body":{"text":"привет"}}}`,
		`{"update_type":"message_created","timestamp":1790000000000,"message":{"sender":{"user_id":3003,"is_bot":true},"body":{"text":"привет"}}}`,
		`{"timestamp":1790000000000,"user":{"user_id":3003}}`,
		`не json`,
	}
	for i, body := range cases {
		resp := post(t, srv, testSecret, body)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("случай %d: ожидался 200, получен %d", i, resp.StatusCode)
		}
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.outbound_messages`); n != 0 {
		t.Errorf("на игнорируемые события ответы не ставятся: %d", n)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.recipients`); n != 0 {
		t.Errorf("игнорируемые события не создают получателей: %d", n)
	}
	if n := countRows(t, s, `SELECT count(*) FROM bot.inbound_updates WHERE outcome = 'ignored'`); n != len(cases) {
		t.Errorf("в журнале %d записей ignored, ожидалось %d", n, len(cases))
	}
}

func TestWebhookReturns503WhenStorageUnavailable(t *testing.T) {
	s := newStack(t, stackOptions{})
	srv := newWebhookServer(t, s)
	s.pool.Close() // имитация недоступной PostgreSQL

	body := fmt.Sprintf(`{"update_type":"bot_started","timestamp":%d,"user":{"user_id":4004}}`, time.Now().UnixMilli())
	if resp := post(t, srv, testSecret, body); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("при недоступной БД ожидался 503, получен %d", resp.StatusCode)
	}
}
