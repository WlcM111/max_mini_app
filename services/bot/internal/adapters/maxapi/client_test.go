package maxapi_test

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vovremya/services/bot/internal/adapters/maxapi"
	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

const token = "devonly-local-bot-token"

type captured struct {
	method string
	path   string
	query  string
	auth   string
	body   map[string]any
	raw    string
}

// fakeMAX — двойник платформы MAX: отвечает по сценарию и записывает запросы.
type fakeMAX struct {
	t        *testing.T
	status   int
	response string
	headers  map[string]string
	delay    time.Duration
	requests []captured
}

func (f *fakeMAX) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	c := captured{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery,
		auth: r.Header.Get("Authorization"), raw: string(raw)}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &c.body)
	}
	f.requests = append(f.requests, c)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	for k, v := range f.headers {
		w.Header().Set(k, v)
	}
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if f.response != "" {
		_, _ = io.WriteString(w, f.response)
	}
}

func newClient(t *testing.T, srv *httptest.Server, kind maxapi.ButtonKind) *maxapi.Client {
	t.Helper()
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatalf("запись сертификата: %v", err)
	}
	client, err := maxapi.New(maxapi.Config{
		BaseURL: srv.URL, Token: token, Timeout: 5 * time.Second, ButtonKind: kind,
		ExtraCAFile: caFile, RequireExtraCA: true,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	if err != nil {
		t.Fatalf("создание клиента: %v", err)
	}
	return client
}

var profile = domain.Profile{UserID: 500100, Username: "vovremya_bot", DisplayName: "Вовремя"}

func reminder() domain.Message {
	return domain.Message{
		Kind: domain.KindReminder, RecipientMaxUserID: 1001,
		Text:    "Через 7 дней заканчивается срок: «Договор».",
		Buttons: []domain.Button{{Text: "Открыть документ", Action: domain.ActionOpenApp, Payload: "doc_42"}},
	}
}

func TestSendMessageRequestFormat(t *testing.T) {
	fake := &fakeMAX{t: t, response: `{"message":{"body":{"mid":"mid.000001"},"timestamp":1790000000000}}`}
	srv := httptest.NewTLSServer(fake)
	defer srv.Close()
	client := newClient(t, srv, maxapi.ButtonKindLink)

	res, err := client.SendMessage(context.Background(), reminder(), profile)
	if err != nil {
		t.Fatalf("отправка: %v", err)
	}
	if res.MessageID != "mid.000001" {
		t.Errorf("идентификатор сообщения: %q", res.MessageID)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("запросов: %d", len(fake.requests))
	}
	req := fake.requests[0]
	if req.method != http.MethodPost || req.path != "/messages" || req.query != "user_id=1001" {
		t.Errorf("запрос: %s %s?%s", req.method, req.path, req.query)
	}
	// Токен передаётся только заголовком Authorization (F-40, PLAT-06).
	if req.auth != token {
		t.Errorf("заголовок Authorization: %q", req.auth)
	}
	if req.query != "user_id=1001" || contains(req.raw, token) {
		t.Error("токен не должен попадать в URL или тело запроса")
	}
	if _, ok := req.body["format"]; ok {
		t.Error("поле format не передаётся: текст обычный (spec §8)")
	}
	if req.body["notify"] != true {
		t.Errorf("notify: %v", req.body["notify"])
	}
	attachments, _ := req.body["attachments"].([]any)
	if len(attachments) != 1 {
		t.Fatalf("вложения: %v", req.body["attachments"])
	}
	attachment, _ := attachments[0].(map[string]any)
	if attachment["type"] != "inline_keyboard" {
		t.Errorf("тип вложения: %v", attachment["type"])
	}
	payload, _ := attachment["payload"].(map[string]any)
	rows, _ := payload["buttons"].([]any)
	if len(rows) != 1 {
		t.Fatalf("рядов кнопок: %d", len(rows))
	}
	row, _ := rows[0].([]any)
	button, _ := row[0].(map[string]any)
	if button["type"] != "link" || button["url"] != "https://max.ru/vovremya_bot?startapp=doc_42" {
		t.Errorf("кнопка link: %v", button)
	}
}

func TestSendMessageOpenAppButton(t *testing.T) {
	fake := &fakeMAX{t: t, response: `{"message":{"body":{"mid":"mid.2"}}}`}
	srv := httptest.NewTLSServer(fake)
	defer srv.Close()
	client := newClient(t, srv, maxapi.ButtonKindOpenApp)

	msg := reminder()
	msg.Buttons = append(msg.Buttons, domain.Button{Text: "Источник", Action: domain.ActionURL, URL: "https://example.org/doc"})
	if _, err := client.SendMessage(context.Background(), msg, profile); err != nil {
		t.Fatalf("отправка: %v", err)
	}
	attachments, _ := fake.requests[0].body["attachments"].([]any)
	attachment, _ := attachments[0].(map[string]any)
	payload, _ := attachment["payload"].(map[string]any)
	rows, _ := payload["buttons"].([]any)
	if len(rows) != 2 {
		t.Fatalf("каждая кнопка занимает отдельный ряд: %d", len(rows))
	}
	first, _ := rows[0].([]any)
	button, _ := first[0].(map[string]any)
	if button["type"] != "open_app" || button["web_app"] != "vovremya_bot" || button["payload"] != "doc_42" {
		t.Errorf("кнопка open_app: %v", button)
	}
	second, _ := rows[1].([]any)
	link, _ := second[0].(map[string]any)
	if link["type"] != "link" || link["url"] != "https://example.org/doc" {
		t.Errorf("кнопка со ссылкой: %v", link)
	}
}

func TestSendMessageStatusClassification(t *testing.T) {
	cases := []struct {
		status      int
		headers     map[string]string
		code        string
		retryable   bool
		unreachable bool
		retryAfter  time.Duration
	}{
		{status: 400, code: domain.CodeMax4xx},
		{status: 405, code: domain.CodeMax4xx},
		{status: 401, code: domain.CodeMax401, retryable: true},
		{status: 403, code: domain.CodeRecipientUnreachable, unreachable: true},
		{status: 404, code: domain.CodeRecipientUnreachable, unreachable: true},
		{status: 429, headers: map[string]string{"Retry-After": "30"}, code: domain.CodeMax429, retryable: true, retryAfter: 30 * time.Second},
		{status: 429, code: domain.CodeMax429, retryable: true},
		{status: 500, code: domain.CodeMax5xx, retryable: true},
		{status: 503, code: domain.CodeMax5xx, retryable: true},
		{status: 302, code: domain.CodeMax5xx, retryable: true},
	}
	for _, c := range cases {
		t.Run(http.StatusText(c.status), func(t *testing.T) {
			fake := &fakeMAX{t: t, status: c.status, headers: c.headers, response: `{"message":"error"}`}
			srv := httptest.NewTLSServer(fake)
			defer srv.Close()
			client := newClient(t, srv, maxapi.ButtonKindLink)

			_, err := client.SendMessage(context.Background(), reminder(), profile)
			var se *ports.SendError
			if !errors.As(err, &se) {
				t.Fatalf("ожидалась ошибка отправки, получено %v", err)
			}
			if se.Failure.Code != c.code || se.Failure.Retryable != c.retryable ||
				se.Failure.RecipientUnreachable != c.unreachable {
				t.Errorf("классификация %d: %+v", c.status, se.Failure)
			}
			if se.Failure.RetryAfter != c.retryAfter {
				t.Errorf("Retry-After: ожидалось %s, получено %s", c.retryAfter, se.Failure.RetryAfter)
			}
			if se.HTTPStatus != c.status {
				t.Errorf("код ответа в ошибке: %d", se.HTTPStatus)
			}
		})
	}
}

func TestSendMessageTimeoutAndTLS(t *testing.T) {
	t.Run("таймаут вызова", func(t *testing.T) {
		fake := &fakeMAX{t: t, delay: 300 * time.Millisecond, response: `{}`}
		srv := httptest.NewTLSServer(fake)
		defer srv.Close()
		client := newClient(t, srv, maxapi.ButtonKindLink)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := client.SendMessage(ctx, reminder(), profile)
		var se *ports.SendError
		if !errors.As(err, &se) || se.Failure.Code != domain.CodeTimeout || !se.Failure.Retryable {
			t.Fatalf("ожидался повторяемый таймаут, получено %v", err)
		}
	})

	t.Run("недоверенный сертификат", func(t *testing.T) {
		fake := &fakeMAX{t: t, response: `{}`}
		srv := httptest.NewTLSServer(fake)
		defer srv.Close()
		client, err := maxapi.New(maxapi.Config{
			BaseURL: srv.URL, Token: token, Timeout: time.Second, ButtonKind: maxapi.ButtonKindLink,
		}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
		if err != nil {
			t.Fatalf("создание клиента: %v", err)
		}
		_, err = client.SendMessage(context.Background(), reminder(), profile)
		var se *ports.SendError
		if !errors.As(err, &se) || se.Failure.Code != domain.CodeNetwork {
			t.Fatalf("сертификат вне пула доверия должен давать сетевую ошибку: %v", err)
		}
	})

	t.Run("обязательный файл сертификатов отсутствует", func(t *testing.T) {
		_, err := maxapi.New(maxapi.Config{
			BaseURL: "https://platform-api2.max.ru", Token: token, Timeout: time.Second,
			ButtonKind: maxapi.ButtonKindLink, ExtraCAFile: "/nonexistent/ca.pem", RequireExtraCA: true,
		}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
		if err == nil {
			t.Fatal("в режиме live отсутствие сертификатов Минцифры — ошибка запуска (F-40)")
		}
	})

	t.Run("файл сертификатов пуст", func(t *testing.T) {
		empty := filepath.Join(t.TempDir(), "empty.pem")
		if err := os.WriteFile(empty, []byte("не сертификат\n"), 0o600); err != nil {
			t.Fatalf("запись файла: %v", err)
		}
		_, err := maxapi.New(maxapi.Config{
			BaseURL: "https://platform-api2.max.ru", Token: token, Timeout: time.Second,
			ButtonKind: maxapi.ButtonKindLink, ExtraCAFile: empty, RequireExtraCA: true,
		}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
		if err == nil {
			t.Fatal("файл без сертификатов должен приводить к ошибке запуска")
		}
	})
}

func TestProfileSubscriptionsAndCommands(t *testing.T) {
	fake := &fakeMAX{t: t, response: `{"user_id":500100,"name":"Вовремя","username":"vovremya_bot","is_bot":true}`}
	srv := httptest.NewTLSServer(fake)
	defer srv.Close()
	client := newClient(t, srv, maxapi.ButtonKindLink)
	ctx := context.Background()

	p, err := client.GetMe(ctx)
	if err != nil {
		t.Fatalf("GET /me: %v", err)
	}
	if p.Username != "vovremya_bot" || p.DisplayName != "Вовремя" || p.UserID != 500100 {
		t.Errorf("профиль: %+v", p)
	}
	if fake.requests[0].path != "/me" || fake.requests[0].auth != token {
		t.Errorf("запрос профиля: %+v", fake.requests[0])
	}

	fake.response = `{"subscriptions":[{"url":"https://vovremya.example/max/webhook","update_types":["bot_started"]}]}`
	urls, err := client.ListSubscriptions(ctx)
	if err != nil || len(urls) != 1 || urls[0] != "https://vovremya.example/max/webhook" {
		t.Fatalf("подписки: %v %v", urls, err)
	}

	fake.response = `{"success":true}`
	if err := client.Subscribe(ctx, "https://vovremya.example/max/webhook", domain.SubscribedUpdateTypes, "secret-value"); err != nil {
		t.Fatalf("создание подписки: %v", err)
	}
	last := fake.requests[len(fake.requests)-1]
	if last.method != http.MethodPost || last.path != "/subscriptions" {
		t.Errorf("запрос подписки: %s %s", last.method, last.path)
	}
	if last.body["url"] != "https://vovremya.example/max/webhook" || last.body["secret"] != "secret-value" {
		t.Errorf("тело подписки: %v", last.body)
	}
	if types, _ := last.body["update_types"].([]any); len(types) != 6 {
		t.Errorf("типы событий подписки: %v", last.body["update_types"])
	}

	fake.response = `{"success":false,"message":"url is not reachable"}`
	if err := client.Subscribe(ctx, "https://vovremya.example/max/webhook", domain.SubscribedUpdateTypes, "secret-value"); err == nil {
		t.Error("ответ success=false должен приводить к ошибке")
	}

	fake.response = `{"success":true}`
	if err := client.SetCommands(ctx, []ports.BotCommand{{Name: "start", Description: "Открыть приложение"}}); err != nil {
		t.Fatalf("команды: %v", err)
	}
	last = fake.requests[len(fake.requests)-1]
	if last.method != http.MethodPatch || last.path != "/me/commands" {
		t.Errorf("запрос команд: %s %s", last.method, last.path)
	}
	if !contains(last.raw, `"name":"start"`) {
		t.Errorf("тело команд: %s", last.raw)
	}
}

func contains(s, sub string) bool { return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
