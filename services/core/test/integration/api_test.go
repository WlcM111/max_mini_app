package integration_test

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"vovremya/services/core/test/testutil"
)

func TestSessionStartTargetAndMe(t *testing.T) {
	h := testutil.NewHarness(t)
	docID := "3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"
	initData := testutil.LaunchData(t, 5001, "Михаил", "doc_"+docID, h.Clock.Now())

	c := &client{t: t, h: h}
	var session struct {
		Token   string `json:"token"`
		Account struct {
			ID        string `json:"id"`
			FirstName string `json:"first_name"`
		} `json:"account"`
		Start struct {
			Kind       string `json:"kind"`
			DocumentID string `json:"document_id"`
		} `json:"start"`
	}
	if status := c.do(http.MethodPost, "/api/v1/sessions", map[string]any{"init_data": initData}, &session); status != http.StatusCreated {
		t.Fatalf("статус создания сессии: %d", status)
	}
	if !strings.HasPrefix(session.Token, "vvs_") || len(session.Token) != 47 {
		t.Fatalf("формат токена: %q", session.Token)
	}
	if session.Start.Kind != "document" || session.Start.DocumentID != docID {
		t.Fatalf("цель запуска: %+v", session.Start)
	}
	c.token = session.Token

	var me struct {
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
		Memberships      []any `json:"memberships"`
		RemindersChannel struct {
			State      string  `json:"state"`
			BotChatURL *string `json:"bot_chat_url"`
		} `json:"reminders_channel"`
		Limits struct {
			MaxOrganizations int `json:"max_organizations"`
		} `json:"limits"`
	}
	if status := c.do(http.MethodGet, "/api/v1/me", nil, &me); status != http.StatusOK {
		t.Fatalf("GET /me: %d", status)
	}
	if me.RemindersChannel.State != "active" {
		t.Fatalf("состояние канала: %s", me.RemindersChannel.State)
	}
	if me.Limits.MaxOrganizations != 20 {
		t.Fatalf("лимиты продукта: %+v", me.Limits)
	}

	// Повторное использование отозванной сессии невозможно.
	if status := c.do(http.MethodDelete, "/api/v1/sessions/current", nil, nil); status != http.StatusNoContent {
		t.Fatalf("удаление сессии: %d", status)
	}
	if status := c.do(http.MethodGet, "/api/v1/me", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("после выхода ожидается 401, получено %d", status)
	}
}

func TestUnauthenticatedAndMalformedRequests(t *testing.T) {
	h := testutil.NewHarness(t)
	anon := &client{t: t, h: h}
	if status := anon.do(http.MethodGet, "/api/v1/me", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("без токена ожидается 401, получено %d", status)
	}
	anon.token = "vvs_" + strings.Repeat("a", 43)
	if status := anon.do(http.MethodGet, "/api/v1/me", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("с выдуманным токеном ожидается 401, получено %d", status)
	}

	c := newClient(t, h, 5002, "Мария")
	var p problem
	status := c.do(http.MethodPost, "/api/v1/organizations", map[string]any{
		"id": uuidFor(1), "name": "Кафе", "business_category_code": "food_service",
		"region_code": "RU-SPE", "timezone": "Europe/Moscow", "feature_codes": []string{},
		"unknown_field": true,
	}, &p)
	if status != http.StatusBadRequest || p.Code != "VALIDATION_FAILED" {
		t.Fatalf("неизвестное поле должно отклоняться: %d %s", status, p.Code)
	}

	status = c.do(http.MethodPost, "/api/v1/sessions", map[string]any{"init_data": "короткая"}, &p)
	if status != http.StatusBadRequest {
		t.Fatalf("короткая строка init_data: %d", status)
	}
}

func TestOrganizationLifecycleAndIdempotency(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 5003, "Владелец")
	id := uuidFor(10)
	org := c.createOrganization(id, "Кафе «Пример»")
	if org.MyRole != "owner" || org.Version != 1 {
		t.Fatalf("создание организации: %+v", org)
	}

	// Повтор того же идентификатора тем же пользователем — 200 без дубля.
	var repeat organization
	status := c.do(http.MethodPost, "/api/v1/organizations", map[string]any{
		"id": id, "name": "Кафе «Пример»", "business_category_code": "food_service",
		"region_code": "RU-SPE", "timezone": "Europe/Moscow", "feature_codes": []string{"has_premises", "sells_alcohol"},
	}, &repeat)
	if status != http.StatusOK || repeat.ID != id {
		t.Fatalf("повтор создания: %d %+v", status, repeat)
	}

	// Тот же идентификатор у другого пользователя — конфликт.
	other := newClient(t, h, 5004, "Чужой")
	var p problem
	status = other.do(http.MethodPost, "/api/v1/organizations", map[string]any{
		"id": id, "name": "Другое", "business_category_code": "food_service",
		"region_code": "RU-SPE", "timezone": "Europe/Moscow", "feature_codes": []string{},
	}, &p)
	if status != http.StatusConflict || p.Code != "CONFLICT_ID_REUSED" {
		t.Fatalf("повторное использование id: %d %s", status, p.Code)
	}

	// Чужая организация недоступна и неотличима от отсутствующей.
	if status := other.do(http.MethodGet, "/api/v1/organizations/"+id, nil, nil); status != http.StatusNotFound {
		t.Fatalf("чужая организация: %d", status)
	}

	var updated organization
	status = c.do(http.MethodPatch, "/api/v1/organizations/"+id, map[string]any{
		"expected_version": 1, "name": "Кафе «Пример» на Невском",
	}, &updated)
	if status != http.StatusOK || updated.Version != 2 || updated.Name != "Кафе «Пример» на Невском" {
		t.Fatalf("изменение организации: %d %+v", status, updated)
	}

	status = c.do(http.MethodPatch, "/api/v1/organizations/"+id, map[string]any{
		"expected_version": 1, "name": "Устаревшая версия",
	}, &p)
	if status != http.StatusConflict || p.Code != "CONFLICT_VERSION" {
		t.Fatalf("оптимистичная блокировка: %d %s", status, p.Code)
	}

	status = c.do(http.MethodPatch, "/api/v1/organizations/"+id, map[string]any{
		"expected_version": 2, "region_code": "XX-UNKNOWN",
	}, &p)
	if status != http.StatusBadRequest {
		t.Fatalf("неизвестный регион: %d", status)
	}

	if status := c.do(http.MethodDelete, "/api/v1/organizations/"+id, nil, nil); status != http.StatusNoContent {
		t.Fatalf("удаление организации: %d", status)
	}
	if status := c.do(http.MethodGet, "/api/v1/organizations/"+id, nil, nil); status != http.StatusNotFound {
		t.Fatalf("после удаления ожидается 404, получено %d", status)
	}
}

func TestOrganizationQuota(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 5005, "Много организаций")
	for i := 0; i < 20; i++ {
		c.createOrganization(uuidFor(100+i), fmt.Sprintf("Организация %d", i))
	}
	var p problem
	status := c.do(http.MethodPost, "/api/v1/organizations", map[string]any{
		"id": uuidFor(200), "name": "Лишняя", "business_category_code": "food_service",
		"region_code": "RU-SPE", "timezone": "Europe/Moscow", "feature_codes": []string{},
	}, &p)
	if status != http.StatusConflict || p.Code != "QUOTA_EXCEEDED" {
		t.Fatalf("квота организаций: %d %s", status, p.Code)
	}
}

func TestCatalogAndSuggestions(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 5006, "Каталог")
	var catalog struct {
		Version       string `json:"version"`
		DocumentTypes []struct {
			Code string `json:"code"`
		} `json:"document_types"`
	}
	status, headers, _ := c.raw(http.MethodGet, "/api/v1/catalog", nil)
	if status != http.StatusOK {
		t.Fatalf("справочник: %d", status)
	}
	etag := headers.Get("ETag")
	if etag == "" {
		t.Fatal("ожидался заголовок ETag")
	}
	if status, _, _ := c.raw(http.MethodGet, "/api/v1/catalog", map[string]string{"If-None-Match": etag}); status != http.StatusNotModified {
		t.Fatalf("повторный запрос справочника: %d", status)
	}
	if status := c.do(http.MethodGet, "/api/v1/catalog", nil, &catalog); status != http.StatusOK {
		t.Fatalf("разбор справочника: %d", status)
	}
	if len(catalog.DocumentTypes) == 0 {
		t.Fatal("справочник пуст: миграция 00002_catalog_seed не применена")
	}

	org := c.createOrganization(uuidFor(11), "Кафе с подсказками")
	var suggestions struct {
		Items []struct {
			DocumentTypeCode string `json:"document_type_code"`
		} `json:"items"`
	}
	if status := c.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/suggestions", nil, &suggestions); status != http.StatusOK {
		t.Fatalf("подсказки: %d", status)
	}
	if len(suggestions.Items) == 0 {
		t.Fatal("для общепита с алкоголем ожидались подсказки")
	}
	first := suggestions.Items[0].DocumentTypeCode

	// Добавленный документ исчезает из подсказок.
	var doc document
	until := h.Clock.Now().AddDate(0, 0, 100).Format("2006-01-02")
	if status := c.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/documents", map[string]any{
		"id": uuidFor(12), "title": "Документ типа " + first, "document_type_code": first, "valid_until": until,
	}, &doc); status != http.StatusCreated {
		t.Fatalf("создание документа с типом: %d", status)
	}
	if len(doc.RenewalSteps) == 0 && first != "" {
		t.Log("у типа документа нет шагов продления — допустимо")
	}
	var after struct {
		Items []struct {
			DocumentTypeCode string `json:"document_type_code"`
		} `json:"items"`
	}
	c.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/suggestions", nil, &after)
	for _, item := range after.Items {
		if item.DocumentTypeCode == first {
			t.Fatalf("тип %s должен исчезнуть из подсказок", first)
		}
	}
}

func TestDocumentsLifecycleAndRegistry(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 5007, "Документы")
	org := c.createOrganization(uuidFor(20), "Кафе с документами")

	expired := c.createDocument(org.ID, uuidFor(21), "Просроченный", -5)
	if expired.Status != "expired" || expired.DaysLeft == nil || *expired.DaysLeft != -5 {
		t.Fatalf("состояние просроченного: %+v", expired)
	}
	expiring := c.createDocument(org.ID, uuidFor(22), "Скоро истекает", 10)
	if expiring.Status != "expiring" {
		t.Fatalf("состояние истекающего: %s", expiring.Status)
	}
	valid := c.createDocument(org.ID, uuidFor(23), "Действующий", 200)
	if valid.Status != "valid" {
		t.Fatalf("состояние действующего: %s", valid.Status)
	}
	var noExpiry document
	if status := c.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/documents", map[string]any{
		"id": uuidFor(24), "title": "Бессрочный",
	}, &noExpiry); status != http.StatusCreated {
		t.Fatalf("бессрочный документ: %d", status)
	}
	if noExpiry.Status != "no_expiry" || noExpiry.DaysLeft != nil {
		t.Fatalf("бессрочный документ: %+v", noExpiry)
	}
	if len(noExpiry.Offsets) != 3 || noExpiry.Offsets[0] != 30 {
		t.Fatalf("отступы по умолчанию: %v", noExpiry.Offsets)
	}

	// Реестр отсортирован по сроку, бессрочные в конце.
	var page struct {
		Items []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
		NextCursor *string `json:"next_cursor"`
	}
	if status := c.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/documents", nil, &page); status != http.StatusOK {
		t.Fatalf("реестр: %d", status)
	}
	if len(page.Items) != 4 || page.Items[0].ID != expired.ID || page.Items[3].ID != noExpiry.ID {
		t.Fatalf("порядок реестра: %+v", page.Items)
	}

	// Фильтр по состоянию и поиск по названию.
	var filtered struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	c.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/documents?status=expiring", nil, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].ID != expiring.ID {
		t.Fatalf("фильтр expiring: %+v", filtered.Items)
	}
	c.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/documents?q=%D0%B1%D0%B5%D1%81%D1%81%D1%80%D0%BE%D1%87", nil, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].ID != noExpiry.ID {
		t.Fatalf("поиск по названию: %+v", filtered.Items)
	}

	// Постраничный обход курсором обходит все документы без повторов.
	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 5; i++ {
		path := "/api/v1/organizations/" + org.ID + "/documents?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		var p2 struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
			NextCursor *string `json:"next_cursor"`
		}
		if status := c.do(http.MethodGet, path, nil, &p2); status != http.StatusOK {
			t.Fatalf("страница реестра: %d", status)
		}
		for _, item := range p2.Items {
			if seen[item.ID] {
				t.Fatalf("документ %s выдан дважды", item.ID)
			}
			seen[item.ID] = true
		}
		if p2.NextCursor == nil {
			break
		}
		cursor = *p2.NextCursor
	}
	if len(seen) != 4 {
		t.Fatalf("обход курсором выдал %d документов", len(seen))
	}

	// Изменение реквизитов и дат.
	var updated document
	newUntil := h.Clock.Now().AddDate(0, 0, 15).Format("2006-01-02")
	status := c.do(http.MethodPatch, "/api/v1/documents/"+valid.ID, map[string]any{
		"expected_version": valid.Version, "title": "Действующий (исправлен)",
		"valid_until": newUntil, "reminder_offsets_days": []int{14, 3},
	}, &updated)
	if status != http.StatusOK || updated.Version != valid.Version+1 || updated.Status != "expiring" {
		t.Fatalf("изменение документа: %d %+v", status, updated)
	}
	if len(updated.Offsets) != 2 || updated.Offsets[0] != 14 {
		t.Fatalf("отступы после изменения: %v", updated.Offsets)
	}

	var p problem
	status = c.do(http.MethodPatch, "/api/v1/documents/"+valid.ID, map[string]any{
		"expected_version": valid.Version, "title": "Опять",
	}, &p)
	if status != http.StatusConflict || p.Code != "CONFLICT_VERSION" {
		t.Fatalf("версия документа: %d %s", status, p.Code)
	}

	// Продление создаёт новый текущий период; повтор идемпотентен.
	renewalID := uuidFor(25)
	var renewed document
	status = c.do(http.MethodPost, "/api/v1/documents/"+valid.ID+"/renewals", map[string]any{
		"id": renewalID, "valid_from": h.Clock.Now().Format("2006-01-02"),
		"valid_until": h.Clock.Now().AddDate(1, 0, 0).Format("2006-01-02"),
	}, &renewed)
	if status != http.StatusCreated || renewed.CurrentPeriod.ID != renewalID || len(renewed.Periods) != 2 {
		t.Fatalf("продление: %d %+v", status, renewed)
	}
	status = c.do(http.MethodPost, "/api/v1/documents/"+valid.ID+"/renewals", map[string]any{
		"id": renewalID, "valid_from": h.Clock.Now().Format("2006-01-02"),
		"valid_until": h.Clock.Now().AddDate(1, 0, 0).Format("2006-01-02"),
	}, &renewed)
	if status != http.StatusOK {
		t.Fatalf("повтор продления: %d", status)
	}

	if status := c.do(http.MethodDelete, "/api/v1/documents/"+expired.ID, nil, nil); status != http.StatusNoContent {
		t.Fatalf("удаление документа: %d", status)
	}
	if status := c.do(http.MethodGet, "/api/v1/documents/"+expired.ID, nil, nil); status != http.StatusNotFound {
		t.Fatalf("после удаления: %d", status)
	}

	var org2 organization
	c.do(http.MethodGet, "/api/v1/organizations/"+org.ID, nil, &org2)
	if org2.Stats.Total != 3 || org2.Stats.NoExpiry != 1 {
		t.Fatalf("сводка статусов: %+v", org2.Stats)
	}
}

func TestDocumentsBatchIsAllOrNothing(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 5008, "Онбординг")
	org := c.createOrganization(uuidFor(30), "Кафе онбординга")

	var result struct {
		Items []document `json:"items"`
	}
	status := c.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/documents/batch", map[string]any{
		"items": []map[string]any{
			{"id": uuidFor(31), "title": "Первый", "valid_until": h.Clock.Now().AddDate(0, 0, 5).Format("2006-01-02")},
			{"id": uuidFor(32), "title": "Второй"},
		},
	}, &result)
	if status != http.StatusCreated || len(result.Items) != 2 {
		t.Fatalf("пакетное создание: %d %+v", status, result.Items)
	}

	// Ошибка в одном элементе отменяет весь пакет.
	var p problem
	status = c.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/documents/batch", map[string]any{
		"items": []map[string]any{
			{"id": uuidFor(33), "title": "Корректный"},
			{"id": uuidFor(34), "title": ""},
		},
	}, &p)
	if status != http.StatusBadRequest {
		t.Fatalf("пакет с ошибкой: %d", status)
	}
	var page struct {
		Items []document `json:"items"`
	}
	c.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/documents", nil, &page)
	if len(page.Items) != 2 {
		t.Fatalf("после отката пакета в реестре %d документов", len(page.Items))
	}
}

func TestRolesAndPermissions(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 5009, "Владелец")
	org := owner.createOrganization(uuidFor(40), "Кафе с ролями")
	doc := owner.createDocument(org.ID, uuidFor(41), "Документ", 20)

	viewer := newClient(t, h, 5010, "Наблюдатель")
	inviteID := uuidFor(42)
	var invite struct {
		LinkURL string `json:"link_url"`
	}
	if status := owner.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/invites", map[string]any{
		"id": inviteID, "role": "viewer",
	}, &invite); status != http.StatusCreated {
		t.Fatalf("создание приглашения: %d", status)
	}
	token := invite.LinkURL[strings.Index(invite.LinkURL, "inv_")+4:]
	if status := viewer.do(http.MethodPost, "/api/v1/invites/accept", map[string]any{"token": token}, nil); status != http.StatusOK {
		t.Fatalf("принятие приглашения: %d", status)
	}

	// Наблюдатель видит, но не изменяет.
	var got document
	if status := viewer.do(http.MethodGet, "/api/v1/documents/"+doc.ID, nil, &got); status != http.StatusOK {
		t.Fatalf("чтение наблюдателем: %d", status)
	}
	if got.CanEdit {
		t.Fatal("can_edit для наблюдателя должен быть false")
	}
	if status := viewer.do(http.MethodPatch, "/api/v1/documents/"+doc.ID, map[string]any{
		"expected_version": got.Version, "title": "Попытка",
	}, nil); status != http.StatusForbidden {
		t.Fatalf("изменение наблюдателем: %d", status)
	}
	if status := viewer.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/invites", map[string]any{
		"id": uuidFor(43), "role": "viewer",
	}, nil); status != http.StatusForbidden {
		t.Fatalf("приглашение от наблюдателя: %d", status)
	}

	// Владелец повышает роль, участник получает право редактирования.
	var members struct {
		Items []struct {
			AccountID string `json:"account_id"`
			Role      string `json:"role"`
			IsMe      bool   `json:"is_me"`
		} `json:"items"`
	}
	owner.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/members", nil, &members)
	if len(members.Items) != 2 {
		t.Fatalf("участников: %d", len(members.Items))
	}
	var viewerID string
	for _, m := range members.Items {
		if m.Role == "viewer" {
			viewerID = m.AccountID
		}
	}
	if status := owner.do(http.MethodPatch, "/api/v1/organizations/"+org.ID+"/members/"+viewerID,
		map[string]any{"role": "editor"}, nil); status != http.StatusOK {
		t.Fatalf("смена роли: %d", status)
	}
	if status := viewer.do(http.MethodPatch, "/api/v1/documents/"+doc.ID, map[string]any{
		"expected_version": got.Version, "title": "Теперь можно",
	}, nil); status != http.StatusOK {
		t.Fatalf("редактор не может изменить документ: %d", status)
	}

	// Владелец не может выйти из организации.
	var ownerMembers struct {
		Items []struct {
			AccountID string `json:"account_id"`
			Role      string `json:"role"`
		} `json:"items"`
	}
	owner.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/members", nil, &ownerMembers)
	var ownerID string
	for _, m := range ownerMembers.Items {
		if m.Role == "owner" {
			ownerID = m.AccountID
		}
	}
	if status := owner.do(http.MethodDelete, "/api/v1/organizations/"+org.ID+"/members/"+ownerID, nil, nil); status != http.StatusForbidden {
		t.Fatalf("выход владельца: %d", status)
	}
	if status := owner.do(http.MethodDelete, "/api/v1/organizations/"+org.ID+"/members/"+viewerID, nil, nil); status != http.StatusNoContent {
		t.Fatalf("исключение участника: %d", status)
	}
	if status := viewer.do(http.MethodGet, "/api/v1/documents/"+doc.ID, nil, nil); status != http.StatusNotFound {
		t.Fatalf("после исключения документ недоступен: %d", status)
	}
}

func TestInvitesLifecycle(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 5011, "Владелец")
	org := owner.createOrganization(uuidFor(50), "Кафе приглашений")

	var created struct {
		ID        string `json:"id"`
		Role      string `json:"role"`
		LinkURL   string `json:"link_url"`
		ShareText string `json:"share_text"`
	}
	if status := owner.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/invites",
		map[string]any{"id": uuidFor(51), "role": "editor"}, &created); status != http.StatusCreated {
		t.Fatalf("создание приглашения: %d", status)
	}
	if !strings.Contains(created.LinkURL, "startapp=inv_") {
		t.Fatalf("ссылка приглашения: %s", created.LinkURL)
	}
	if !strings.Contains(created.ShareText, "Кафе приглашений") {
		t.Fatalf("текст приглашения: %s", created.ShareText)
	}
	token := created.LinkURL[strings.Index(created.LinkURL, "inv_")+4:]

	guest := newClient(t, h, 5012, "Гость")
	var preview struct {
		OrganizationName string `json:"organization_name"`
		Role             string `json:"role"`
	}
	if status := guest.do(http.MethodPost, "/api/v1/invites/preview", map[string]any{"token": token}, &preview); status != http.StatusOK {
		t.Fatalf("предпросмотр: %d", status)
	}
	if preview.OrganizationName != "Кафе приглашений" || preview.Role != "editor" {
		t.Fatalf("предпросмотр: %+v", preview)
	}

	var accepted struct {
		OrganizationID string `json:"organization_id"`
		Role           string `json:"role"`
	}
	if status := guest.do(http.MethodPost, "/api/v1/invites/accept", map[string]any{"token": token}, &accepted); status != http.StatusOK {
		t.Fatalf("принятие: %d", status)
	}
	if accepted.OrganizationID != org.ID || accepted.Role != "editor" {
		t.Fatalf("результат принятия: %+v", accepted)
	}

	// Владелец получает сообщение о новом участнике (spec §15).
	queued := h.Bot.Queued()
	if len(queued) != 1 || queued[0].Kind != "member_joined" {
		t.Fatalf("сообщение владельцу не поставлено: %+v", queued)
	}
	if !strings.Contains(queued[0].Text, "Гость") || !strings.Contains(queued[0].Text, "редактор") {
		t.Fatalf("текст сообщения: %q", queued[0].Text)
	}
	if queued[0].IdempotencyKey != "mj:"+uuidFor(51) {
		t.Fatalf("ключ идемпотентности: %s", queued[0].IdempotencyKey)
	}
	if len(queued[0].Buttons) != 1 || queued[0].Buttons[0].OpenAppPayload != "org_"+org.ID {
		t.Fatalf("кнопка сообщения: %+v", queued[0].Buttons)
	}

	// Повторное принятие невозможно, а участник уже в организации.
	var p problem
	if status := guest.do(http.MethodPost, "/api/v1/invites/accept", map[string]any{"token": token}, &p); status != http.StatusNotFound {
		t.Fatalf("повторное принятие: %d %s", status, p.Code)
	}
	// Отозвать принятое приглашение нельзя (OpenAPI 1.1.0: 409).
	if status := owner.do(http.MethodDelete, "/api/v1/invites/"+uuidFor(51), nil, &p); status != http.StatusConflict {
		t.Fatalf("отзыв принятого приглашения: %d", status)
	}

	// Отзыв активного приглашения делает его недействительным.
	var second struct {
		LinkURL string `json:"link_url"`
	}
	owner.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/invites",
		map[string]any{"id": uuidFor(52), "role": "viewer"}, &second)
	if status := owner.do(http.MethodDelete, "/api/v1/invites/"+uuidFor(52), nil, nil); status != http.StatusNoContent {
		t.Fatalf("отзыв приглашения: %d", status)
	}
	if status := owner.do(http.MethodDelete, "/api/v1/invites/"+uuidFor(52), nil, nil); status != http.StatusNoContent {
		t.Fatalf("повторный отзыв должен быть идемпотентным: %d", status)
	}
	secondToken := second.LinkURL[strings.Index(second.LinkURL, "inv_")+4:]
	newGuest := newClient(t, h, 5013, "Второй гость")
	if status := newGuest.do(http.MethodPost, "/api/v1/invites/accept", map[string]any{"token": secondToken}, nil); status != http.StatusNotFound {
		t.Fatalf("отозванное приглашение: %d", status)
	}

	// Истёкшее приглашение.
	var third struct {
		LinkURL string `json:"link_url"`
	}
	owner.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/invites",
		map[string]any{"id": uuidFor(53), "role": "viewer"}, &third)
	thirdToken := third.LinkURL[strings.Index(third.LinkURL, "inv_")+4:]
	// Срок приглашения — 72 часа; сессия живёт 12 часов, поэтому после сдвига
	// часов гость открывает приложение заново.
	h.Clock.Advance(73 * time.Hour)
	lateGuest := newClient(t, h, 5019, "Опоздавший")
	if status := lateGuest.do(http.MethodPost, "/api/v1/invites/preview", map[string]any{"token": thirdToken}, &p); status != http.StatusConflict || p.Code != "INVITE_EXPIRED" {
		t.Fatalf("истёкшее приглашение: %d %s", status, p.Code)
	}
}

func TestCalendarExport(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 5014, "Экспорт")
	org := c.createOrganization(uuidFor(60), "Кафе экспорта")
	c.createDocument(org.ID, uuidFor(61), "Лицензия; с запятой, и точкой", 30)

	var export struct {
		DownloadURL string `json:"download_url"`
		FileName    string `json:"file_name"`
	}
	if status := c.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/exports/calendar", nil, &export); status != http.StatusCreated {
		t.Fatalf("создание экспорта: %d", status)
	}
	if !strings.HasSuffix(export.FileName, ".ics") {
		t.Fatalf("имя файла: %s", export.FileName)
	}
	token := export.DownloadURL[strings.LastIndex(export.DownloadURL, "/")+1:]

	anon := &client{t: t, h: h}
	status, headers, body := anon.raw(http.MethodGet, "/api/v1/downloads/"+token, nil)
	if status != http.StatusOK {
		t.Fatalf("скачивание без Bearer: %d", status)
	}
	if !strings.Contains(headers.Get("Content-Type"), "text/calendar") {
		t.Fatalf("Content-Type: %s", headers.Get("Content-Type"))
	}
	if headers.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control: %s", headers.Get("Cache-Control"))
	}
	if !strings.HasPrefix(body, "BEGIN:VCALENDAR\r\n") || !strings.Contains(body, "BEGIN:VALARM") {
		t.Fatalf("содержимое ICS: %.120q", body)
	}
	if !strings.Contains(body, `SUMMARY:Срок: Лицензия\; с запятой\, и точкой`) {
		t.Fatalf("экранирование в ICS: %.200q", body)
	}

	// Не более трёх скачиваний по одной ссылке.
	for i := 0; i < 2; i++ {
		if status, _, _ := anon.raw(http.MethodGet, "/api/v1/downloads/"+token, nil); status != http.StatusOK {
			t.Fatalf("скачивание %d: %d", i+2, status)
		}
	}
	if status, _, _ := anon.raw(http.MethodGet, "/api/v1/downloads/"+token, nil); status != http.StatusGone {
		t.Fatalf("четвёртое скачивание: %d", status)
	}
	if status, _, _ := anon.raw(http.MethodGet, "/api/v1/downloads/"+strings.Repeat("a", 43), nil); status != http.StatusNotFound {
		t.Fatalf("неизвестная ссылка: %d", status)
	}
}

func TestNotificationSettingsAndClientEvents(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 5015, "Настройки")
	org := c.createOrganization(uuidFor(70), "Кафе настроек")

	var settings struct {
		Enabled   bool   `json:"enabled"`
		LocalTime string `json:"local_time"`
	}
	if status := c.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/notification-settings", nil, &settings); status != http.StatusOK {
		t.Fatalf("настройки: %d", status)
	}
	if !settings.Enabled || settings.LocalTime != "09:00" {
		t.Fatalf("настройки по умолчанию: %+v", settings)
	}
	if status := c.do(http.MethodPut, "/api/v1/organizations/"+org.ID+"/notification-settings",
		map[string]any{"enabled": false, "local_time": "19:30"}, &settings); status != http.StatusOK {
		t.Fatalf("изменение настроек: %d", status)
	}
	if settings.Enabled || settings.LocalTime != "19:30" {
		t.Fatalf("настройки после изменения: %+v", settings)
	}
	if status := c.do(http.MethodPut, "/api/v1/organizations/"+org.ID+"/notification-settings",
		map[string]any{"enabled": true, "local_time": "25:00"}, nil); status != http.StatusBadRequest {
		t.Fatalf("некорректное время: %d", status)
	}

	anon := &client{t: t, h: h}
	if status := anon.do(http.MethodPost, "/api/v1/client-events", map[string]any{
		"events": []map[string]any{{"name": "bootstrap_completed", "duration_ms": 350, "platform": "web"}},
	}, nil); status != http.StatusAccepted {
		t.Fatalf("события клиента: %d", status)
	}
	if status := anon.do(http.MethodPost, "/api/v1/client-events", map[string]any{
		"events": []map[string]any{{"name": "unknown_event"}},
	}, nil); status != http.StatusBadRequest {
		t.Fatalf("неизвестное событие: %d", status)
	}
}

func TestConcurrentUpdatesProduceSingleConflict(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 5016, "Конкуренция")
	org := c.createOrganization(uuidFor(80), "Кафе конкуренции")
	doc := c.createDocument(org.ID, uuidFor(81), "Спорный документ", 40)

	for attempt := 0; attempt < 20; attempt++ {
		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			ok      int
			conflic int
		)
		version := doc.Version + attempt
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				sub := &client{t: t, h: h, token: c.token}
				status := sub.do(http.MethodPatch, "/api/v1/documents/"+doc.ID, map[string]any{
					"expected_version": version, "title": fmt.Sprintf("Попытка %d-%d", attempt, i),
				}, nil)
				mu.Lock()
				defer mu.Unlock()
				switch status {
				case http.StatusOK:
					ok++
				case http.StatusConflict:
					conflic++
				default:
					t.Errorf("неожиданный статус %d", status)
				}
			}(i)
		}
		wg.Wait()
		if ok != 1 || conflic != 1 {
			t.Fatalf("прогон %d: успешных %d, конфликтов %d", attempt, ok, conflic)
		}
	}
}

func TestAccountDeletionRemovesOwnedData(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 5017, "Удаляемый")
	org := owner.createOrganization(uuidFor(90), "Кафе на удаление")
	owner.createDocument(org.ID, uuidFor(91), "Документ", 10)

	if status := owner.do(http.MethodDelete, "/api/v1/me", nil, nil); status != http.StatusNoContent {
		t.Fatalf("удаление аккаунта: %d", status)
	}
	if status := owner.do(http.MethodGet, "/api/v1/me", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("после удаления сессия недействительна: %d", status)
	}
	next := newClient(t, h, 5018, "Следующий")
	if status := next.do(http.MethodGet, "/api/v1/organizations/"+org.ID, nil, nil); status != http.StatusNotFound {
		t.Fatalf("организация удалена вместе с владельцем: %d", status)
	}
}
