package maxapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vovremya/services/bot/internal/adapters/maxapi"
)

// TestAnswerCallback: ответ на нажатие callback-кнопки уходит в POST /answers
// с идентификатором нажатия в запросе и текстом уведомления в теле (ADR-036).
func TestAnswerCallback(t *testing.T) {
	var got struct {
		method, path, callbackID, auth string
		body                           map[string]string
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		got.callbackID = r.URL.Query().Get("callback_id")
		got.auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got.body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()
	client := newClient(t, srv, maxapi.ButtonKindLink)

	if err := client.AnswerCallback(context.Background(), "cb-42", "Напомню через неделю"); err != nil {
		t.Fatalf("ответ на нажатие: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/answers" || got.callbackID != "cb-42" {
		t.Fatalf("запрос: %s %s callback_id=%q", got.method, got.path, got.callbackID)
	}
	if got.auth == "" {
		t.Fatal("запрос без токена бота")
	}
	if got.body["notification"] != "Напомню через неделю" {
		t.Fatalf("тело запроса: %+v", got.body)
	}
}

// TestAnswerCallbackReportsFailure: ошибка платформы возвращается вызывающему.
func TestAnswerCallbackReportsFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"bad.request","message":"invalid callback"}`))
	}))
	defer srv.Close()
	client := newClient(t, srv, maxapi.ButtonKindLink)
	if err := client.AnswerCallback(context.Background(), "cb-1", "текст"); err == nil {
		t.Fatal("ожидалась ошибка при ответе 400")
	}
}
