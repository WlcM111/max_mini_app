package gigachat_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"vovremya/services/core/internal/adapters/gigachat"
	"vovremya/services/core/internal/adapters/gigachat/gigachattest"
	"vovremya/services/core/internal/ports"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func testCatalog() ports.AssistantCatalog {
	return ports.AssistantCatalog{
		DocumentTypes: []ports.AssistantOption{{Code: "alcohol_license", Title: "Лицензия на алкоголь"}},
		Categories:    []ports.AssistantOption{{Code: "food_service", Title: "Общественное питание"}},
		Features:      []ports.AssistantOption{{Code: "sells_alcohol", Title: "Продаёте алкоголь?"}},
	}
}

func newClient(t *testing.T, fake *gigachattest.Server, opts ...func(*gigachat.Config)) *gigachat.Client {
	t.Helper()
	cfg := gigachat.Config{
		AuthKey:  base64.StdEncoding.EncodeToString([]byte("client-id:client-secret")),
		Scope:    "GIGACHAT_API_PERS",
		Model:    "GigaChat-Pro",
		BaseURL:  fake.BaseURL(),
		OAuthURL: fake.OAuthURL(),
		Timeout:  2 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	client, err := gigachat.New(cfg)
	if err != nil {
		t.Fatalf("создание клиента: %v", err)
	}
	return client
}

func TestDraftDocumentRequestShape(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetContent(`{"title":"Лицензия","number":"78РПА1","issuer":"Комитет","valid_from":"2024-03-14","valid_until":"2029-03-13","document_type_code":"alcohol_license","confidence":0.9}`)
	client := newClient(t, fake)

	draft, err := client.DraftDocument(context.Background(), "Лицензия № 78РПА1 до 13.03.2029", testCatalog())
	if err != nil {
		t.Fatalf("разбор документа: %v", err)
	}
	if draft.Title != "Лицензия" || draft.DocumentTypeCode != "alcohol_license" || draft.Confidence != 0.9 {
		t.Fatalf("ответ разобран неверно: %+v", draft)
	}

	req := fake.LastRequest()
	if req["model"] != "GigaChat-Pro" {
		t.Fatalf("модель не передана: %v", req["model"])
	}
	if req["function_call"] != "none" {
		t.Fatalf("встроенные функции должны быть отключены: %v", req["function_call"])
	}
	if req["temperature"] != float64(0) {
		t.Fatalf("температура должна быть нулевой: %v", req["temperature"])
	}
	format, ok := req["response_format"].(map[string]any)
	if !ok || format["type"] != "json_schema" || format["strict"] != true {
		t.Fatalf("ожидался строгий json_schema: %v", req["response_format"])
	}
	schema, _ := format["schema"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	typeField, _ := props["document_type_code"].(map[string]any)
	values, _ := typeField["enum"].([]any)
	if len(values) != 2 || values[0] != "alcohol_license" || values[1] != "" {
		t.Fatalf("перечень кодов должен быть закрытым: %v", values)
	}
	if !uuidV4.MatchString(fake.LastRqUID()) {
		t.Fatalf("RqUID должен быть uuid4: %q", fake.LastRqUID())
	}
	if !strings.HasPrefix(fake.LastAuthorization(), "Basic ") {
		t.Fatalf("ключ авторизации передаётся схемой Basic: %q", fake.LastAuthorization())
	}
}

func TestAccessTokenIsCached(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetContent(`{"title":"А","confidence":1}`)
	client := newClient(t, fake)

	for i := 0; i < 3; i++ {
		if _, err := client.DraftDocument(context.Background(), "текст", testCatalog()); err != nil {
			t.Fatalf("запрос %d: %v", i, err)
		}
	}
	if fake.OAuthCalls() != 1 {
		t.Fatalf("токен должен запрашиваться один раз, получено %d", fake.OAuthCalls())
	}
	if fake.ChatCalls() != 3 {
		t.Fatalf("ожидалось три обращения к генерации, получено %d", fake.ChatCalls())
	}
}

func TestAccessTokenRefreshedBeforeExpiry(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetContent(`{"title":"А","confidence":1}`)
	fake.SetTokenTTL(90 * time.Second)
	now := time.Now()
	client := newClient(t, fake, func(c *gigachat.Config) {
		c.Now = func() time.Time { return now }
	})

	if _, err := client.DraftDocument(context.Background(), "текст", testCatalog()); err != nil {
		t.Fatalf("первый запрос: %v", err)
	}
	now = now.Add(80 * time.Second) // до истечения меньше минуты — нужен новый токен
	if _, err := client.DraftDocument(context.Background(), "текст", testCatalog()); err != nil {
		t.Fatalf("второй запрос: %v", err)
	}
	if fake.OAuthCalls() != 2 {
		t.Fatalf("токен должен обновляться до истечения, обращений: %d", fake.OAuthCalls())
	}
}

func TestRetryOnServerError(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetContent(`{"title":"А","confidence":1}`)
	fake.FailFirst(1)
	client := newClient(t, fake)

	if _, err := client.DraftDocument(context.Background(), "текст", testCatalog()); err != nil {
		t.Fatalf("после одной неудачи запрос должен пройти: %v", err)
	}
	if fake.ChatCalls() != 2 {
		t.Fatalf("ожидалась ровно одна повторная попытка, обращений: %d", fake.ChatCalls())
	}
}

func TestNoRetryOnClientError(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetStatus(http.StatusBadRequest)
	client := newClient(t, fake)

	if _, err := client.DraftDocument(context.Background(), "текст", testCatalog()); err == nil {
		t.Fatal("ошибка 400 должна возвращаться вызывающему")
	}
	if fake.ChatCalls() != 1 {
		t.Fatalf("повтор при 400 не нужен, обращений: %d", fake.ChatCalls())
	}
}

func TestTimeoutIsRespected(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetDelay(300 * time.Millisecond)
	client := newClient(t, fake, func(c *gigachat.Config) { c.Timeout = 100 * time.Millisecond })

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := client.DraftDocument(ctx, "текст", testCatalog()); err == nil {
		t.Fatal("ожидалась ошибка по истечении времени ожидания")
	}
}

func TestDailyTokenBudget(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetContent(`{"title":"А","confidence":1}`)
	fake.SetTotalTokens(120)
	client := newClient(t, fake, func(c *gigachat.Config) { c.DailyTokenBudget = 200 })

	if _, err := client.DraftDocument(context.Background(), "текст", testCatalog()); err != nil {
		t.Fatalf("первый запрос: %v", err)
	}
	if client.SpentTokens() != 120 {
		t.Fatalf("расход токенов не учтён: %d", client.SpentTokens())
	}
	if _, err := client.DraftDocument(context.Background(), "текст", testCatalog()); err != nil {
		t.Fatalf("второй запрос в пределах бюджета: %v", err)
	}
	_, err := client.DraftDocument(context.Background(), "текст", testCatalog())
	if !errors.Is(err, gigachat.ErrBudgetExceeded) {
		t.Fatalf("после исчерпания бюджета ожидался отказ, получено: %v", err)
	}
}

func TestMalformedContentRejected(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetContent("это не json")
	client := newClient(t, fake)

	if _, err := client.DraftDocument(context.Background(), "текст", testCatalog()); !errors.Is(err, gigachat.ErrBadResponse) {
		t.Fatalf("мусор в ответе должен отклоняться: %v", err)
	}
}

func TestFencedJSONAccepted(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetContent("```json\n{\"title\":\"Договор\",\"confidence\":0.5}\n```")
	client := newClient(t, fake)

	draft, err := client.DraftDocument(context.Background(), "текст", testCatalog())
	if err != nil {
		t.Fatalf("обрамление ```json должно сниматься: %v", err)
	}
	if draft.Title != "Договор" {
		t.Fatalf("ответ разобран неверно: %+v", draft)
	}
}

func TestMatchProfile(t *testing.T) {
	fake := gigachattest.New(t)
	fake.SetContent(`{"business_category_code":"food_service","feature_codes":["sells_alcohol"],"confidence":0.7}`)
	client := newClient(t, fake)

	match, err := client.MatchProfile(context.Background(), "кофейня, продаём пиво", testCatalog())
	if err != nil {
		t.Fatalf("подбор профиля: %v", err)
	}
	if match.BusinessCategoryCode != "food_service" || len(match.FeatureCodes) != 1 {
		t.Fatalf("ответ разобран неверно: %+v", match)
	}
	req := fake.LastRequest()
	messages, _ := req["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("ожидались системное и пользовательское сообщения: %v", messages)
	}
	user, _ := messages[1].(map[string]any)
	content, _ := user["content"].(string)
	if !strings.Contains(content, "sells_alcohol") || !strings.Contains(content, "кофейня") {
		t.Fatalf("в запрос должны попасть коды справочника и описание: %q", content)
	}
}

func TestRequiresAuthKey(t *testing.T) {
	if _, err := gigachat.New(gigachat.Config{}); err == nil {
		t.Fatal("без ключа авторизации клиент создаваться не должен")
	}
}
