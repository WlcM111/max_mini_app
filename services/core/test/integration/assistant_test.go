package integration_test

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"vovremya/services/core/internal/config"
	"vovremya/services/core/internal/ports"
	"vovremya/services/core/test/testutil"
)

type draftResponse struct {
	Title            string  `json:"title"`
	Number           *string `json:"number"`
	Issuer           *string `json:"issuer"`
	ValidFrom        *string `json:"valid_from"`
	ValidUntil       *string `json:"valid_until"`
	DocumentTypeCode *string `json:"document_type_code"`
	Offsets          []int   `json:"reminder_offsets_days"`
	Confidence       float64 `json:"confidence"`
}

type profileResponse struct {
	BusinessCategoryCode *string  `json:"business_category_code"`
	FeatureCodes         []string `json:"feature_codes"`
	Confidence           float64  `json:"confidence"`
}

// TestAssistantDraftDocument: FR-21 — черновик карточки из свободного текста.
func TestAssistantDraftDocument(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9101, "Владелец")
	orgID := owner.createOrganization(uuidFor(9101), "Кафе").ID

	h.Assistant.SetDraft(ports.DocumentDraft{
		Title:            "  Лицензия на алкоголь  ",
		Number:           "78РПА0012345",
		Issuer:           "Комитет по промышленной политике",
		ValidFrom:        "14.03.2024",
		ValidUntil:       "13.03.2029",
		DocumentTypeCode: "alcohol_retail_license",
		Confidence:       0.93,
	})

	var draft draftResponse
	status := owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
		map[string]any{"text": "Лицензия № 78РПА0012345 действует до 13.03.2029"}, &draft)
	if status != http.StatusOK {
		t.Fatalf("черновик: статус %d", status)
	}
	if draft.Title != "Лицензия на алкоголь" {
		t.Fatalf("название не очищено: %q", draft.Title)
	}
	if draft.ValidFrom == nil || *draft.ValidFrom != "2024-03-14" {
		t.Fatalf("дата начала не приведена к YYYY-MM-DD: %v", draft.ValidFrom)
	}
	if draft.ValidUntil == nil || *draft.ValidUntil != "2029-03-13" {
		t.Fatalf("дата окончания не приведена: %v", draft.ValidUntil)
	}
	if draft.DocumentTypeCode == nil || *draft.DocumentTypeCode != "alcohol_retail_license" {
		t.Fatalf("тип документа не распознан: %v", draft.DocumentTypeCode)
	}
	if len(draft.Offsets) == 0 {
		t.Fatalf("отступы напоминаний должны подставляться из справочника")
	}
	if draft.Confidence <= 0 {
		t.Fatalf("уверенность не передана: %v", draft.Confidence)
	}

	// Ассистент получил очищенный текст и закрытые списки кодов.
	if got := h.Assistant.LastText(); got != "Лицензия № 78РПА0012345 действует до 13.03.2029" {
		t.Fatalf("ассистенту передан не тот текст: %q", got)
	}
	if len(h.Assistant.LastCatalog().DocumentTypes) == 0 {
		t.Fatalf("ассистенту не переданы коды справочника")
	}

	// Черновик ничего не сохраняет: реестр остаётся пустым.
	var page struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if status := owner.do(http.MethodGet, "/api/v1/organizations/"+orgID+"/documents", nil, &page); status != http.StatusOK {
		t.Fatalf("реестр: статус %d", status)
	}
	if len(page.Items) != 0 {
		t.Fatalf("черновик не должен создавать документы, найдено %d", len(page.Items))
	}
}

// TestAssistantDraftSanitizesModelOutput: ответ модели не принимается на веру.
func TestAssistantDraftSanitizesModelOutput(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9102, "Владелец")
	orgID := owner.createOrganization(uuidFor(9102), "Кафе").ID

	h.Assistant.SetDraft(ports.DocumentDraft{
		Title:            strings.Repeat("я", 400),
		ValidFrom:        "2029-01-01",
		ValidUntil:       "2024-01-01",
		DocumentTypeCode: "выдуманный_код",
		Confidence:       7,
	})

	var draft draftResponse
	if status := owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
		map[string]any{"text": "любой текст"}, &draft); status != http.StatusOK {
		t.Fatalf("черновик: статус %d", status)
	}
	if len([]rune(draft.Title)) != 200 {
		t.Fatalf("название должно обрезаться до 200 символов, получено %d", len([]rune(draft.Title)))
	}
	if draft.ValidFrom != nil || draft.ValidUntil != nil {
		t.Fatalf("перевёрнутые даты должны обнуляться: %v, %v", draft.ValidFrom, draft.ValidUntil)
	}
	if draft.DocumentTypeCode != nil {
		t.Fatalf("код вне справочника должен отбрасываться: %v", draft.DocumentTypeCode)
	}
	if draft.Confidence != 1 {
		t.Fatalf("уверенность должна ограничиваться единицей: %v", draft.Confidence)
	}
}

// TestAssistantDraftRequiresEditor: наблюдателю разбор недоступен.
func TestAssistantDraftRequiresEditor(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9103, "Владелец")
	orgID := owner.createOrganization(uuidFor(9103), "Кафе").ID
	var invite struct {
		LinkURL string `json:"link_url"`
	}
	if status := owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/invites",
		map[string]any{"id": uuidFor(9204), "role": "viewer"}, &invite); status != http.StatusCreated {
		t.Fatalf("создание приглашения: статус %d", status)
	}
	token := invite.LinkURL[strings.Index(invite.LinkURL, "startapp=inv_")+len("startapp=inv_"):]
	viewer := newClient(t, h, 9104, "Наблюдатель")
	if status := viewer.do(http.MethodPost, "/api/v1/invites/accept",
		map[string]any{"token": token}, nil); status != http.StatusOK {
		t.Fatalf("принятие приглашения: статус %d", status)
	}

	status := viewer.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
		map[string]any{"text": "любой текст"}, nil)
	if status != http.StatusForbidden {
		t.Fatalf("ожидался 403 для наблюдателя, получено %d", status)
	}
}

// TestAssistantValidatesInput: пустой и слишком длинный текст отклоняются.
func TestAssistantValidatesInput(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9105, "Владелец")
	orgID := owner.createOrganization(uuidFor(9104), "Кафе").ID

	var problem struct {
		Code   string `json:"code"`
		Errors []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	status := owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
		map[string]any{"text": "   "}, &problem)
	if status != http.StatusBadRequest || problem.Code != "VALIDATION_FAILED" {
		t.Fatalf("пустой текст: статус %d, код %q", status, problem.Code)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Field != "text" {
		t.Fatalf("ожидалась ошибка поля text: %+v", problem.Errors)
	}

	long := strings.Repeat("я", 2001)
	status = owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
		map[string]any{"text": long}, &problem)
	if status != http.StatusBadRequest {
		t.Fatalf("слишком длинный текст: статус %d", status)
	}

	status = owner.do(http.MethodPost, "/api/v1/profile-match", map[string]any{"description": ""}, &problem)
	if status != http.StatusBadRequest || problem.Code != "VALIDATION_FAILED" {
		t.Fatalf("пустое описание: статус %d, код %q", status, problem.Code)
	}
}

// TestAssistantUnavailable: отказ внешнего сервиса не ломает сценарий.
func TestAssistantUnavailable(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9106, "Владелец")
	orgID := owner.createOrganization(uuidFor(9105), "Кафе").ID

	h.Assistant.SetError(errAssistantDown{})
	var problem struct {
		Code string `json:"code"`
	}
	status := owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
		map[string]any{"text": "текст"}, &problem)
	if status != http.StatusServiceUnavailable || problem.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("ожидался 503 DEPENDENCY_UNAVAILABLE, получено %d %q", status, problem.Code)
	}

	// Ручное создание документа продолжает работать при отключённом ассистенте.
	doc := owner.createDocument(orgID, uuidFor(9306), "Договор аренды", 120)
	if doc.ID == "" {
		t.Fatal("создание документа должно работать без ассистента")
	}
}

// TestAssistantDisabled: без ключа функция выключена и объявлена в GET /me.
func TestAssistantDisabled(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9107, "Владелец")
	orgID := owner.createOrganization(uuidFor(9106), "Кафе").ID

	var me struct {
		AssistantEnabled bool `json:"assistant_enabled"`
	}
	if status := owner.do(http.MethodGet, "/api/v1/me", nil, &me); status != http.StatusOK {
		t.Fatalf("профиль: статус %d", status)
	}
	if !me.AssistantEnabled {
		t.Fatal("при подключённом ассистенте ожидалось assistant_enabled = true")
	}

	h.App.Assistant = nil
	if status := owner.do(http.MethodGet, "/api/v1/me", nil, &me); status != http.StatusOK {
		t.Fatalf("профиль: статус %d", status)
	}
	if me.AssistantEnabled {
		t.Fatal("после отключения ожидалось assistant_enabled = false")
	}
	status := owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
		map[string]any{"text": "текст"}, nil)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("при выключенном ассистенте ожидался 503, получено %d", status)
	}
}

// TestAssistantMatchProfile: FR-22 — подбор профиля по описанию бизнеса.
func TestAssistantMatchProfile(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9108, "Владелец")

	h.Assistant.SetProfile(ports.ProfileMatch{
		BusinessCategoryCode: "food_service",
		FeatureCodes:         []string{"sells_alcohol", "sells_alcohol", "несуществующий_признак"},
		Confidence:           0.8,
	})

	var match profileResponse
	status := owner.do(http.MethodPost, "/api/v1/profile-match",
		map[string]any{"description": "Кофейня на 30 мест, продаём пиво"}, &match)
	if status != http.StatusOK {
		t.Fatalf("подбор профиля: статус %d", status)
	}
	if match.BusinessCategoryCode == nil || *match.BusinessCategoryCode != "food_service" {
		t.Fatalf("вид деятельности не распознан: %v", match.BusinessCategoryCode)
	}
	if len(match.FeatureCodes) != 1 || match.FeatureCodes[0] != "sells_alcohol" {
		t.Fatalf("признаки должны очищаться от повторов и чужих кодов: %v", match.FeatureCodes)
	}

	h.Assistant.SetProfile(ports.ProfileMatch{BusinessCategoryCode: "чужая_категория"})
	if status := owner.do(http.MethodPost, "/api/v1/profile-match",
		map[string]any{"description": "неизвестный бизнес"}, &match); status != http.StatusOK {
		t.Fatalf("подбор профиля: статус %d", status)
	}
	if match.BusinessCategoryCode != nil {
		t.Fatalf("неизвестный код должен отбрасываться: %v", match.BusinessCategoryCode)
	}
	if match.FeatureCodes == nil {
		t.Fatal("в ответе ожидается пустой массив признаков, а не null")
	}
}

// TestAssistantRateLimited: частота обращений ограничена на аккаунт.
func TestAssistantRateLimited(t *testing.T) {
	h := testutil.NewHarness(t, func(c *config.Config) { c.RateAssistantPerMin = 1 })
	owner := newClient(t, h, 9109, "Владелец")
	orgID := owner.createOrganization(uuidFor(9107), "Кафе").ID
	h.Assistant.SetDraft(ports.DocumentDraft{Title: "Договор"})

	if status := owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
		map[string]any{"text": "текст"}, nil); status != http.StatusOK {
		t.Fatalf("первый запрос: статус %d", status)
	}
	if status := owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
		map[string]any{"text": "текст"}, nil); status != http.StatusTooManyRequests {
		t.Fatalf("второй запрос должен упереться в предел, получено %d", status)
	}
}

type errAssistantDown struct{}

func (errAssistantDown) Error() string { return "ассистент недоступен" }

// problemBody — поля problem+json, которые проверяют тесты ассистента.
type problemBody struct {
	Code   string `json:"code"`
	Status int    `json:"status"`
	Errors []struct {
		Field string `json:"field"`
		Code  string `json:"code"`
	} `json:"errors"`
}

// TestAssistantDraftMergesTextDates: дата с маркером «от» из текста попадает в начало действия,
// даже если модель её пропустила или поставила в окончание срока.
func TestAssistantDraftMergesTextDates(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9111, "Владелец")
	orgID := owner.createOrganization(uuidFor(9111), "Кафе").ID
	const text = "Лицензия на Алкоголь от 12.03.2022"

	for _, model := range []ports.DocumentDraft{
		{Title: "Лицензия на Алкоголь", Confidence: 0.6},
		{Title: "Лицензия на Алкоголь", ValidUntil: "2022-03-12", Confidence: 0.6},
	} {
		h.Assistant.SetDraft(model)
		var draft draftResponse
		if status := owner.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents/draft",
			map[string]any{"text": text}, &draft); status != http.StatusOK {
			t.Fatalf("черновик: статус %d", status)
		}
		if draft.ValidFrom == nil || *draft.ValidFrom != "2022-03-12" {
			t.Fatalf("дата выдачи должна стать началом действия: %v (модель: %+v)", draft.ValidFrom, model)
		}
		if draft.ValidUntil != nil {
			t.Fatalf("срок окончания в тексте не указан и не вычисляется: %v", *draft.ValidUntil)
		}
	}
}

// TestAssistantDraftNotRecognized: вход без реквизитов — 422 DOCUMENT_NOT_RECOGNIZED, а не 503.
func TestAssistantDraftNotRecognized(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9112, "Владелец")
	orgID := owner.createOrganization(uuidFor(9112), "Кафе").ID
	path := "/api/v1/organizations/" + orgID + "/documents/draft"

	var problem problemBody
	h.Assistant.SetDraft(ports.DocumentDraft{})
	if status := owner.do(http.MethodPost, path, map[string]any{"text": "привет"}, &problem); status != http.StatusUnprocessableEntity ||
		problem.Code != "DOCUMENT_NOT_RECOGNIZED" {
		t.Fatalf("пустой черновик: статус %d, код %q", status, problem.Code)
	}

	h.Assistant.SetError(ports.ErrAssistantNoResult)
	if status := owner.do(http.MethodPost, path, map[string]any{"text": "привет"}, &problem); status != http.StatusUnprocessableEntity ||
		problem.Code != "DOCUMENT_NOT_RECOGNIZED" {
		t.Fatalf("отказ модели по содержанию: статус %d, код %q", status, problem.Code)
	}

	// Даже без ответа модели срок с явным маркером берётся из текста.
	var draft draftResponse
	if status := owner.do(http.MethodPost, path, map[string]any{"text": "Действует до 13.03.2029"}, &draft); status != http.StatusOK {
		t.Fatalf("текст со сроком: статус %d", status)
	}
	if draft.ValidUntil == nil || *draft.ValidUntil != "2029-03-13" {
		t.Fatalf("срок из текста не подставлен: %v", draft.ValidUntil)
	}
}

// jpegBytes — байты с сигнатурой JPEG: сценарий проверяет формат по сигнатуре, не декодируя файл.
var jpegBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xFF, 0xD9}

// TestAssistantDraftImage: FR-23 — черновик по фотографии и понятные отказы.
func TestAssistantDraftImage(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9113, "Владелец")
	orgID := owner.createOrganization(uuidFor(9113), "Кафе").ID
	path := "/api/v1/organizations/" + orgID + "/documents/draft-image"
	photo := map[string]any{"image": base64.StdEncoding.EncodeToString(jpegBytes), "mime_type": "image/jpeg"}

	h.Assistant.SetDraft(ports.DocumentDraft{Title: "Лицензия", Number: "78РПА1", ValidUntil: "13.03.2029", Confidence: 0.8})
	var draft draftResponse
	if status := owner.do(http.MethodPost, path, photo, &draft); status != http.StatusOK {
		t.Fatalf("распознавание фото: статус %d", status)
	}
	if draft.Title != "Лицензия" || draft.ValidUntil == nil || *draft.ValidUntil != "2029-03-13" {
		t.Fatalf("черновик по фото разобран неверно: %+v", draft)
	}
	if !bytes.Equal(h.Assistant.LastImage(), jpegBytes) {
		t.Fatal("ассистенту должны передаваться декодированные байты фотографии")
	}

	var problem problemBody
	wrongType := map[string]any{"image": photo["image"], "mime_type": "image/png"}
	if status := owner.do(http.MethodPost, path, wrongType, &problem); status != http.StatusBadRequest ||
		len(problem.Errors) == 0 || problem.Errors[0].Field != "image" {
		t.Fatalf("несовпадение формата: статус %d, %+v", status, problem)
	}

	h.Assistant.SetDraft(ports.DocumentDraft{})
	if status := owner.do(http.MethodPost, path, photo, &problem); status != http.StatusUnprocessableEntity ||
		problem.Code != "DOCUMENT_NOT_RECOGNIZED" {
		t.Fatalf("фото без реквизитов: статус %d, код %q", status, problem.Code)
	}

	h.Assistant.SetError(ports.ErrAssistantNoResult)
	if status := owner.do(http.MethodPost, path, photo, &problem); status != http.StatusUnprocessableEntity {
		t.Fatalf("отказ модели читать фото: статус %d", status)
	}

	h.Assistant.SetError(errAssistantDown{})
	if status := owner.do(http.MethodPost, path, photo, &problem); status != http.StatusServiceUnavailable ||
		problem.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("сбой сервиса: статус %d, код %q", status, problem.Code)
	}
}
