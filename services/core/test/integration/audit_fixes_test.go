package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/test/testutil"
)

type fieldProblem struct {
	Code   string `json:"code"`
	Errors []struct {
		Field string `json:"field"`
		Code  string `json:"code"`
	} `json:"errors"`
}

func (p fieldProblem) field(name string) string {
	for _, e := range p.Errors {
		if e.Field == name {
			return e.Code
		}
	}
	return ""
}

// TestTextFieldsRejectControlCharacters: NUL и CR в текстовых полях дают 400 с кодом поля
// invalid_format, а не 500 (BUG-003, BUG-004).
func TestTextFieldsRejectControlCharacters(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9701, "Аудит")
	org := owner.createOrganization(uuidFor(9701), "Кафе")
	for i, title := range []string{"Лицензия\u0000", "Договор\rBEGIN:VALARM", "Знак\u001b"} {
		var p fieldProblem
		status := owner.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/documents", map[string]any{
			"id": uuidFor(9710 + i), "title": title, "valid_until": "2030-01-01", "reminder_offsets_days": []int{30},
		}, &p)
		if status != http.StatusBadRequest || p.Code != "VALIDATION_FAILED" || p.field("title") != "invalid_format" {
			t.Errorf("название %q: статус %d, ответ %+v", title, status, p)
		}
	}
	var p fieldProblem
	status := owner.do(http.MethodPost, "/api/v1/organizations", map[string]any{
		"id": uuidFor(9720), "name": "Кафе\u0000", "business_category_code": "food_service",
		"region_code": "RU-SPE", "timezone": "Europe/Moscow", "feature_codes": []string{},
	}, &p)
	if status != http.StatusBadRequest || p.field("name") != "invalid_format" {
		t.Errorf("организация с NUL: статус %d, ответ %+v", status, p)
	}
}

// TestNotesLineBreaksNormalized: CRLF в заметках из таблиц и форм принимается и хранится как \n.
func TestNotesLineBreaksNormalized(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9702, "Аудит")
	org := owner.createOrganization(uuidFor(9702), "Кафе")
	var doc struct {
		ID    string `json:"id"`
		Notes string `json:"notes"`
	}
	status := owner.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/documents", map[string]any{
		"id": uuidFor(9730), "title": "Договор", "notes": "первая\r\nвторая\rтретья",
		"valid_until": "2030-01-01", "reminder_offsets_days": []int{30},
	}, &doc)
	if status != http.StatusCreated || doc.Notes != "первая\nвторая\nтретья" {
		t.Fatalf("статус %d, заметки %q", status, doc.Notes)
	}
}

// TestCalendarExportFoldsLongLines: строки .ics не длиннее 75 октетов, свёртка не разрывает
// UTF-8 и после развёртки даёт исходное название (BUG-005).
func TestCalendarExportFoldsLongLines(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9703, "Аудит")
	org := owner.createOrganization(uuidFor(9703), "Кафе")
	// 159 символов кириллицы — около 300 октетов в строке SUMMARY.
	title := strings.TrimSpace(strings.Repeat("Лицензия на розничную продажу алкогольной продукции ", 3))
	owner.createDocumentWithTitle(t, org.ID, uuidFor(9740), title)

	var exp struct {
		DownloadURL string `json:"download_url"`
	}
	if status := owner.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/exports/calendar", map[string]any{}, &exp); status != http.StatusCreated {
		t.Fatalf("экспорт: статус %d", status)
	}
	resp, err := http.Get(h.Server.URL + exp.DownloadURL[strings.Index(exp.DownloadURL, "/api/v1/"):])
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	text := string(body)
	for _, line := range strings.Split(strings.TrimSuffix(text, "\r\n"), "\r\n") {
		if len(line) > 75 || !utf8.ValidString(line) {
			t.Fatalf("строка %d октетов или с разорванным UTF-8: %q", len(line), line)
		}
	}
	if strings.Count(strings.ReplaceAll(text, "\r\n", ""), "\r") != 0 {
		t.Fatal("в файле одиночный CR")
	}
	unfolded := strings.ReplaceAll(text, "\r\n ", "")
	if !strings.Contains(unfolded, "SUMMARY:Срок: "+title+"\r\n") {
		t.Fatalf("после развёртки название не восстановилось:\n%s", unfolded)
	}
}

func (c *client) createDocumentWithTitle(t *testing.T, orgID, docID, title string) {
	t.Helper()
	status := c.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents", map[string]any{
		"id": docID, "title": title, "valid_until": "2030-01-01", "reminder_offsets_days": []int{30},
	}, nil)
	if status != http.StatusCreated {
		t.Fatalf("документ: статус %d", status)
	}
}

func bearer(t *testing.T, h *testutil.Harness, token, method, path string, body any) int {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, h.Server.URL+path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// TestReviewerAccessPath: демо-данные, токены проверяющих двух ролей, границы срока
// и отзыв — путь, по которому жюри проверяет API (BUG-008).
func TestReviewerAccessPath(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "demo", "demo-data.json"))
	if err != nil {
		t.Fatalf("демо-данные: %v", err)
	}
	orgID, err := h.App.SeedDemo(ctx, raw)
	if err != nil || orgID == "" {
		t.Fatalf("загрузка демо-данных: %q, %v", orgID, err)
	}
	editor, expires, err := h.App.IssueReviewToken(ctx, "reviewer_editor", domain.RoleEditor, 720*time.Hour, orgID)
	if err != nil || !strings.HasPrefix(editor, "vvs_") || !expires.After(h.Clock.Now().Add(719*time.Hour)) {
		t.Fatalf("токен редактора: %q до %s, %v", editor, expires, err)
	}
	viewer, _, err := h.App.IssueReviewToken(ctx, "reviewer_viewer", domain.RoleViewer, 24*time.Hour, orgID)
	if err != nil {
		t.Fatalf("токен наблюдателя: %v", err)
	}

	docs := "/api/v1/organizations/" + orgID + "/documents"
	if st := bearer(t, h, viewer, http.MethodGet, docs, nil); st != http.StatusOK {
		t.Fatalf("наблюдатель читает реестр демо-организации: статус %d", st)
	}
	newDoc := map[string]any{"id": uuidFor(9750), "title": "Проверка жюри", "valid_until": "2030-01-01",
		"reminder_offsets_days": []int{30}}
	if st := bearer(t, h, viewer, http.MethodPost, docs, newDoc); st != http.StatusForbidden {
		t.Fatalf("наблюдатель не должен создавать документы: статус %d", st)
	}
	if st := bearer(t, h, editor, http.MethodPost, docs, newDoc); st != http.StatusCreated {
		t.Fatalf("редактор создаёт документ: статус %d", st)
	}

	for name, call := range map[string]func() error{
		"неверный логин": func() error {
			_, _, e := h.App.IssueReviewToken(ctx, "Reviewer Editor!", domain.RoleEditor, time.Hour, orgID)
			return e
		},
		"роль владельца": func() error {
			_, _, e := h.App.IssueReviewToken(ctx, "reviewer_owner", domain.RoleOwner, time.Hour, orgID)
			return e
		},
		"нулевой срок": func() error {
			_, _, e := h.App.IssueReviewToken(ctx, "reviewer_editor", domain.RoleEditor, 0, orgID)
			return e
		},
		"больше 720 часов": func() error {
			_, _, e := h.App.IssueReviewToken(ctx, "reviewer_editor", domain.RoleEditor, 721*time.Hour, orgID)
			return e
		},
	} {
		if err := call(); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: ожидалась ошибка проверки, получено %v", name, err)
		}
	}

	if err := h.App.RevokeReviewTokens(ctx, "reviewer_editor"); err != nil {
		t.Fatalf("отзыв: %v", err)
	}
	if st := bearer(t, h, editor, http.MethodGet, docs, nil); st != http.StatusUnauthorized {
		t.Fatalf("отозванный токен должен давать 401: статус %d", st)
	}
	if st := bearer(t, h, viewer, http.MethodGet, docs, nil); st != http.StatusOK {
		t.Fatalf("токен другого логина отзываться не должен: статус %d", st)
	}
	again, _, err := h.App.IssueReviewToken(ctx, "reviewer_editor", domain.RoleEditor, time.Hour, orgID)
	if err != nil || bearer(t, h, again, http.MethodGet, docs, nil) != http.StatusOK {
		t.Fatalf("повторный выпуск после отзыва: %v", err)
	}
}

type inviteLink struct {
	LinkURL string `json:"link_url"`
}

func (l inviteLink) token() string {
	return l.LinkURL[strings.Index(l.LinkURL, "startapp=inv_")+len("startapp=inv_"):]
}

// TestListInvitesShowsOnlyActive: список приглашений видит только владелец; отозванные
// и принятые приглашения в нём не показываются (BUG-009).
func TestListInvitesShowsOnlyActive(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9704, "Владелец")
	org := owner.createOrganization(uuidFor(9704), "Кафе")
	path := "/api/v1/organizations/" + org.ID + "/invites"
	var a, b, c inviteLink
	for i, dst := range []*inviteLink{&a, &b, &c} {
		role := []string{"viewer", "editor", "editor"}[i]
		if st := owner.do(http.MethodPost, path, map[string]any{"id": uuidFor(9760 + i), "role": role}, dst); st != http.StatusCreated {
			t.Fatalf("приглашение %d: статус %d", i, st)
		}
	}
	if st := owner.do(http.MethodDelete, "/api/v1/invites/"+uuidFor(9761), nil, nil); st != http.StatusNoContent {
		t.Fatalf("отзыв: статус %d", st)
	}
	editor := newClient(t, h, 9705, "Редактор")
	if st := editor.do(http.MethodPost, "/api/v1/invites/accept", map[string]any{"token": c.token()}, nil); st != http.StatusOK {
		t.Fatalf("принятие: статус %d", st)
	}

	var page struct {
		Items []struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"items"`
	}
	if st := owner.do(http.MethodGet, path, nil, &page); st != http.StatusOK {
		t.Fatalf("список: статус %d", st)
	}
	if len(page.Items) != 1 || page.Items[0].ID != uuidFor(9760) || page.Items[0].Role != "viewer" {
		t.Fatalf("в списке должно остаться одно действующее приглашение: %+v", page.Items)
	}
	if st := editor.do(http.MethodGet, path, nil, nil); st != http.StatusForbidden {
		t.Fatalf("редактор не видит приглашения: статус %d", st)
	}
}

// TestResponsibleMustBeMember: ответственным может быть только участник организации (BUG-010).
func TestResponsibleMustBeMember(t *testing.T) {
	h := testutil.NewHarness(t)
	owner := newClient(t, h, 9706, "Владелец")
	org := owner.createOrganization(uuidFor(9706), "Кафе")
	var inv inviteLink
	if st := owner.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/invites",
		map[string]any{"id": uuidFor(9770), "role": "viewer"}, &inv); st != http.StatusCreated {
		t.Fatalf("приглашение: статус %d", st)
	}
	member := newClient(t, h, 9707, "Участник")
	if st := member.do(http.MethodPost, "/api/v1/invites/accept", map[string]any{"token": inv.token()}, nil); st != http.StatusOK {
		t.Fatalf("принятие: статус %d", st)
	}
	outsider := newClient(t, h, 9708, "Посторонний")
	accountID := func(c *client) string {
		var me struct {
			Account struct {
				ID string `json:"id"`
			} `json:"account"`
		}
		c.do(http.MethodGet, "/api/v1/me", nil, &me)
		return me.Account.ID
	}
	docs := "/api/v1/organizations/" + org.ID + "/documents"
	create := func(n int, responsible string) (int, fieldProblem) {
		var p fieldProblem
		st := owner.do(http.MethodPost, docs, map[string]any{"id": uuidFor(n), "title": "Медкнижка",
			"valid_until": "2030-01-01", "reminder_offsets_days": []int{30}, "responsible_account_id": responsible}, &p)
		return st, p
	}
	if st, _ := create(9771, accountID(member)); st != http.StatusCreated {
		t.Fatalf("ответственный-участник: статус %d", st)
	}
	if st, p := create(9772, accountID(outsider)); st != http.StatusBadRequest || p.field("responsible_account_id") != "unknown_value" {
		t.Fatalf("посторонний аккаунт: статус %d, %+v", st, p)
	}
	if st, p := create(9773, uuidFor(9999)); st != http.StatusBadRequest || p.field("responsible_account_id") != "unknown_value" {
		t.Fatalf("несуществующий аккаунт: статус %d, %+v", st, p)
	}
	if st, p := create(9774, "не-идентификатор"); st != http.StatusBadRequest || p.field("responsible_account_id") != "invalid_format" {
		t.Fatalf("неверный формат: статус %d, %+v", st, p)
	}
}
