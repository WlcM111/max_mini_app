package maxapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vovremya/services/bot/internal/adapters/maxapi"
	"vovremya/services/bot/internal/domain"
)

func newStub(t *testing.T) *maxapi.Stub {
	t.Helper()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	return maxapi.NewStub("vovremya_local_bot", maxapi.ButtonKindLink,
		slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })
}

func TestStubStoresRenderedMessages(t *testing.T) {
	stub := newStub(t)
	ctx := context.Background()

	profile, err := stub.GetMe(ctx)
	if err != nil || profile.Username != "vovremya_local_bot" {
		t.Fatalf("профиль режима stub: %+v %v", profile, err)
	}
	res, err := stub.SendMessage(ctx, reminder(), profile)
	if err != nil || res.MessageID != "stub-1" {
		t.Fatalf("отправка: %+v %v", res, err)
	}
	msgs := stub.Messages()
	if len(msgs) != 1 || msgs[0].Recipient != 1001 || msgs[0].Text != reminder().Text {
		t.Fatalf("буфер: %+v", msgs)
	}
	if len(msgs[0].Buttons) != 1 {
		t.Fatalf("кнопки: %+v", msgs[0].Buttons)
	}

	srv := httptest.NewServer(stub.Handler())
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL + "/debug/stub/messages")
	if err != nil {
		t.Fatalf("запрос буфера: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("код ответа: %d", resp.StatusCode)
	}
	var decoded []struct {
		Recipient int64  `json:"recipient"`
		Text      string `json:"text"`
		MID       string `json:"mid"`
		CreatedAt string `json:"created_at"`
		Buttons   []struct {
			Type string `json:"type"`
			Text string `json:"text"`
			URL  string `json:"url"`
		} `json:"buttons"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Recipient != 1001 || decoded[0].MID != "stub-1" {
		t.Fatalf("ответ буфера: %+v", decoded)
	}
	if decoded[0].Buttons[0].URL != "https://max.ru/vovremya_local_bot?startapp=doc_42" {
		t.Errorf("кнопка в буфере: %+v", decoded[0].Buttons[0])
	}
}

func TestStubRingBuffer(t *testing.T) {
	stub := newStub(t)
	ctx := context.Background()
	profile, _ := stub.GetMe(ctx)
	msg := domain.Message{Kind: domain.KindReminder, RecipientMaxUserID: 1, Text: "текст"}
	for i := 0; i < 600; i++ {
		if _, err := stub.SendMessage(ctx, msg, profile); err != nil {
			t.Fatalf("отправка %d: %v", i, err)
		}
	}
	msgs := stub.Messages()
	if len(msgs) != 500 {
		t.Fatalf("размер кольцевого буфера: %d, ожидалось 500", len(msgs))
	}
	if msgs[0].MessageID != "stub-101" || msgs[499].MessageID != "stub-600" {
		t.Errorf("границы буфера: %s..%s", msgs[0].MessageID, msgs[499].MessageID)
	}
}

func TestStubDoesNotPretendSubscription(t *testing.T) {
	stub := newStub(t)
	ctx := context.Background()
	if _, err := stub.ListSubscriptions(ctx); err == nil {
		t.Error("в режиме stub подписка не оформляется: ожидалась ошибка")
	}
	if err := stub.Subscribe(ctx, "https://example.org/max/webhook", domain.SubscribedUpdateTypes, "secret"); err == nil {
		t.Error("в режиме stub подписка не оформляется: ожидалась ошибка")
	}
}
