// Сценарии сквозной проверки с участием core-service: публичный HTTP API →
// outbox → IngestService reminders-service → MessagingService bot-service.
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	remindersv1 "vovremya/gen/go/vovremya/reminders/v1"
)

// coreEnv — доступ к публичному API core-service.
type coreEnv struct {
	base   string
	admin  string
	token  string
	secret string
	client *http.Client
}

func newCoreEnv() *coreEnv {
	return &coreEnv{
		base:   envOr("E2E_CORE_HTTP", "http://127.0.0.1:18070"),
		admin:  envOr("E2E_CORE_ADMIN", "http://127.0.0.1:18071"),
		secret: envOr("CORE_MAX_WEBAPP_SECRET_HEX", "e45f53166d1da778700f29abbf89f6ac247864cb97356f927707e56f0066d663"),
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// request выполняет запрос к публичному API и возвращает статус и тело.
func (c *coreEnv) request(method, path string, body any) (int, []byte) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			log.Fatalf("e2e: сериализация тела: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		log.Fatalf("e2e: запрос %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		log.Fatalf("e2e: выполнение %s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

func (c *coreEnv) mustJSON(method, path string, body any, want int, out any) {
	status, raw := c.request(method, path, body)
	if status != want {
		log.Fatalf("e2e: %s %s вернул %d, ожидалось %d: %s", method, path, status, want, raw)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			log.Fatalf("e2e: разбор ответа %s %s: %v: %s", method, path, err, raw)
		}
	}
}

// signInitData собирает подписанные данные запуска мини-приложения MAX.
func (c *coreEnv) signInitData(userID int64, firstName, startParam string) string {
	secret, err := hex.DecodeString(c.secret)
	if err != nil {
		log.Fatalf("e2e: ключ мини-приложения: %v", err)
	}
	params := map[string]string{
		"auth_date": fmt.Sprintf("%d", time.Now().Unix()),
		"query_id":  fmt.Sprintf("e2e-%d", userID),
		"user": fmt.Sprintf(`{"id":%d,"first_name":%q,"last_name":null,"username":null,"language_code":"ru"}`,
			userID, firstName),
	}
	if startParam != "" {
		params["start_param"] = startParam
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	parts := make([]string, 0, len(keys)+1)
	for _, k := range keys {
		lines = append(lines, k+"="+params[k])
		parts = append(parts, k+"="+url.PathEscape(params[k]))
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.Join(lines, "\n")))
	parts = append(parts, "hash="+hex.EncodeToString(mac.Sum(nil)))
	return strings.Join(parts, "&")
}

// login создаёт сессию в core-service по подписанным данным запуска.
func (c *coreEnv) login(userID int64, firstName string) string {
	var session struct {
		Token   string `json:"token"`
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
	}
	c.mustJSON(http.MethodPost, "/api/v1/sessions",
		map[string]any{"init_data": c.signInitData(userID, firstName, ""), "platform": "web"},
		http.StatusCreated, &session)
	c.token = session.Token
	return session.Account.ID
}

type coreDocument struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Status         string  `json:"status"`
	RemindersState string  `json:"reminders_state"`
	NextReminderAt *string `json:"next_reminder_at"`
	Version        int     `json:"version"`
	CurrentPeriod  struct {
		ID string `json:"id"`
	} `json:"current_period"`
}

// caseCore — основной сквозной сценарий трёх сервисов: мини-приложение
// создаёт организацию и документ в core, core доставляет события в
// reminders-service, тот передаёт напоминание в bot-service.
func (e *env) caseCore(ctx context.Context) {
	c := newCoreEnv()

	step("1. Сессия мини-приложения по подписанным данным запуска MAX")
	accountID := c.login(maxUID, "Пользователь e2e")
	fmt.Printf("   аккаунт: %s\n", accountID)

	step("2. Организация создаётся через публичный API core")
	orgUUID := "7c1e5a90-2b3d-4f6a-8e91-5d4c7b2a1f03"
	var org struct {
		ID      string `json:"id"`
		MyRole  string `json:"my_role"`
		Version int    `json:"version"`
	}
	c.mustJSON(http.MethodPost, "/api/v1/organizations", map[string]any{
		"id": orgUUID, "name": "Кафе сквозной проверки", "business_category_code": "food_service",
		"region_code": "RU-SPE", "timezone": "Europe/Moscow",
		"feature_codes": []string{"has_premises", "sells_alcohol"},
	}, http.StatusCreated, &org)
	if org.MyRole != "owner" {
		log.Fatalf("e2e: роль создателя организации: %s", org.MyRole)
	}
	fmt.Printf("   организация %s, версия %d\n", org.ID, org.Version)

	step("3. Напоминания приходят в этот чат: настройки участия")
	// Время напоминаний выбирается чуть раньше текущего момента в поясе
	// организации: план должен сработать сразу, в пределах допустимого опоздания.
	nowLocal := time.Now().UTC().Add(3 * time.Hour) // Europe/Moscow
	localTime := "00:00"
	if shifted := nowLocal.Add(-5 * time.Minute); shifted.Day() == nowLocal.Day() {
		localTime = shifted.Format("15:04")
	}
	c.mustJSON(http.MethodPut, "/api/v1/organizations/"+org.ID+"/notification-settings",
		map[string]any{"enabled": true, "local_time": localTime}, http.StatusOK, nil)
	fmt.Printf("   время напоминаний: %s (Europe/Moscow)\n", localTime)

	step("4. Документ со сроком создаётся через API; событие уходит в reminders")
	docUUID := "2f8b6d41-9c3e-4a75-b1d8-6e5f4a3c2b10"
	// Срок через три дня и отступ в три дня дают напоминание на сегодня.
	until := nowLocal.AddDate(0, 0, 3).Format("2006-01-02")
	var doc coreDocument
	c.mustJSON(http.MethodPost, "/api/v1/organizations/"+org.ID+"/documents", map[string]any{
		"id": docUUID, "title": "Лицензия сквозной проверки", "valid_until": until,
		"reminder_offsets_days": []int{3},
	}, http.StatusCreated, &doc)
	fmt.Printf("   документ %s, состояние срока %s, состояние плана %s\n", doc.ID, doc.Status, doc.RemindersState)
	if doc.RemindersState == "unavailable" {
		log.Fatal("e2e: reminders-service недоступен при создании документа")
	}

	step("5. reminders-service построил план по событию core")
	planned := waitPlanned(ctx, e.query, docUUID, 20*time.Second)
	fmt.Printf("   запланировано напоминание: получатель %s, за %d дн., срок %s\n",
		planned.GetAccountId(), planned.GetDaysBefore(), planned.GetDueAt().AsTime().Format(time.RFC3339))
	if planned.GetAccountId() != accountID {
		log.Fatalf("e2e: получатель плана %s, ожидался %s", planned.GetAccountId(), accountID)
	}

	step("6. core показывает ближайшее напоминание в карточке документа")
	var card coreDocument
	c.mustJSON(http.MethodGet, "/api/v1/documents/"+docUUID, nil, http.StatusOK, &card)
	if card.NextReminderAt == nil {
		log.Fatal("e2e: core не получил план напоминаний из reminders-service")
	}
	if card.RemindersState != "actual" {
		log.Fatalf("e2e: состояние плана %s, ожидалось actual", card.RemindersState)
	}
	fmt.Printf("   next_reminder_at = %s, reminders_state = %s\n", *card.NextReminderAt, card.RemindersState)

	step("7. Напоминание передано в bot-service и доставлено в MAX (режим stub)")
	handed := waitHandedOff(ctx, e.query, docUUID, 40*time.Second)
	fmt.Printf("   передано в bot: %s\n", handed.GetHandedOffAt().AsTime().Format(time.RFC3339))
	msg := waitMessage(e, maxUID, "Лицензия сквозной проверки", 30*time.Second)
	fmt.Printf("   сообщение получателю %d: %s\n", msg.Recipient, firstLine(msg.Text))

	step("8. Изменение документа обновляет план (новая версия агрегата)")
	newUntil := nowLocal.AddDate(0, 0, 40).Format("2006-01-02")
	var updated coreDocument
	c.mustJSON(http.MethodPatch, "/api/v1/documents/"+docUUID, map[string]any{
		"expected_version": card.Version, "valid_until": newUntil, "reminder_offsets_days": []int{30},
	}, http.StatusOK, &updated)
	if updated.Version != card.Version+1 {
		log.Fatalf("e2e: версия документа после изменения: %d", updated.Version)
	}
	planned2 := waitPlannedDays(ctx, e.query, docUUID, 30, 20*time.Second)
	fmt.Printf("   план пересчитан: за %d дн. до %s\n", planned2.GetDaysBefore(),
		planned2.GetDueAt().AsTime().Format("2006-01-02"))

	step("9. Экспорт календаря доступен по одноразовой ссылке")
	var export struct {
		DownloadURL string `json:"download_url"`
		FileName    string `json:"file_name"`
	}
	c.mustJSON(http.MethodPost, "/api/v1/organizations/"+org.ID+"/exports/calendar", nil, http.StatusCreated, &export)
	token := export.DownloadURL[strings.LastIndex(export.DownloadURL, "/")+1:]
	status, raw := c.request(http.MethodGet, "/api/v1/downloads/"+token, nil)
	if status != http.StatusOK || !bytes.HasPrefix(raw, []byte("BEGIN:VCALENDAR")) {
		log.Fatalf("e2e: скачивание ICS: %d %.80s", status, raw)
	}
	fmt.Printf("   файл %s, %d байт\n", export.FileName, len(raw))

	step("10. Удаление документа снимает план напоминаний")
	if status, raw := c.request(http.MethodDelete, "/api/v1/documents/"+docUUID, nil); status != http.StatusNoContent {
		log.Fatalf("e2e: удаление документа: %d %s", status, raw)
	}
	waitPlanEmpty(ctx, e.query, docUUID, 20*time.Second)
	fmt.Println("   план документа очищен в reminders-service")
}

// caseCorePending — сценарий T-CONS-01: при недоступном reminders-service
// изменение сохраняется, ответ помечается как pending, после восстановления
// событие доставляется и состояние становится actual.
func (e *env) caseCorePending(_ context.Context) {
	c := newCoreEnv()
	c.login(maxUID+1, "Пользователь отказа")

	step("1. Организация и документ создаются при недоступном reminders-service")
	orgUUID := "5a2c9e31-7b4d-4f80-9c15-3e6a8d2b4f05"
	var org struct {
		ID string `json:"id"`
	}
	c.mustJSON(http.MethodPost, "/api/v1/organizations", map[string]any{
		"id": orgUUID, "name": "Кафе без напоминаний", "business_category_code": "food_service",
		"region_code": "RU-SPE", "timezone": "Europe/Moscow", "feature_codes": []string{"has_premises"},
	}, http.StatusCreated, &org)

	docUUID := "9d4f2a68-1c7b-4e39-85a0-7f2b6c4d1e08"
	var doc coreDocument
	c.mustJSON(http.MethodPost, "/api/v1/organizations/"+org.ID+"/documents", map[string]any{
		"id": docUUID, "title": "Документ при отказе reminders",
		"valid_until": time.Now().AddDate(0, 0, 20).Format("2006-01-02"),
	}, http.StatusCreated, &doc)
	if doc.RemindersState != "unavailable" && doc.RemindersState != "pending" {
		log.Fatalf("e2e: ожидалось unavailable или pending, получено %s", doc.RemindersState)
	}
	fmt.Printf("   документ создан, состояние плана: %s\n", doc.RemindersState)

	step("2. Реестр документов продолжает работать")
	var page struct {
		Items []coreDocument `json:"items"`
	}
	c.mustJSON(http.MethodGet, "/api/v1/organizations/"+org.ID+"/documents", nil, http.StatusOK, &page)
	if len(page.Items) != 1 {
		log.Fatalf("e2e: в реестре %d документов", len(page.Items))
	}
	fmt.Printf("   реестр доступен, состояние плана в списке: %s\n", page.Items[0].RemindersState)
}

// caseCoreRecovered проверяет доставку накопленных событий после восстановления.
func (e *env) caseCoreRecovered(ctx context.Context) {
	c := newCoreEnv()
	c.login(maxUID+1, "Пользователь отказа")
	docUUID := "9d4f2a68-1c7b-4e39-85a0-7f2b6c4d1e08"

	step("1. Relay core доставляет накопленные события в reminders-service")
	planned := waitPlanned(ctx, e.query, docUUID, 60*time.Second)
	fmt.Printf("   план построен: за %d дн., срок %s\n", planned.GetDaysBefore(),
		planned.GetDueAt().AsTime().Format(time.RFC3339))

	step("2. core показывает состояние плана actual")
	deadline := time.Now().Add(30 * time.Second)
	for {
		var card coreDocument
		c.mustJSON(http.MethodGet, "/api/v1/documents/"+docUUID, nil, http.StatusOK, &card)
		if card.RemindersState == "actual" {
			fmt.Printf("   reminders_state = %s\n", card.RemindersState)
			return
		}
		if time.Now().After(deadline) {
			log.Fatalf("e2e: состояние плана осталось %s", card.RemindersState)
		}
		time.Sleep(time.Second)
	}
}

// caseCoreBotDown проверяет поведение core при недоступном bot-service.
func (e *env) caseCoreBotDown(_ context.Context) {
	c := newCoreEnv()
	c.login(maxUID+2, "Пользователь без бота")

	step("1. Организация и документ создаются без bot-service")
	orgUUID := "3b7d1f52-8a6c-4e09-b374-2d5f9c1a8e06"
	var org struct {
		ID string `json:"id"`
	}
	c.mustJSON(http.MethodPost, "/api/v1/organizations", map[string]any{
		"id": orgUUID, "name": "Кафе без бота", "business_category_code": "food_service",
		"region_code": "RU-SPE", "timezone": "Europe/Moscow", "feature_codes": []string{},
	}, http.StatusCreated, &org)
	c.mustJSON(http.MethodPost, "/api/v1/organizations/"+org.ID+"/documents", map[string]any{
		"id": "4e9a7c30-5d1b-4f28-9e61-8c3a2f5d7b04", "title": "Документ без бота",
		"valid_until": time.Now().AddDate(0, 0, 45).Format("2006-01-02"),
	}, http.StatusCreated, nil)
	fmt.Println("   документ создан, CRUD не зависит от bot-service")

	step("2. Состояние канала напоминаний — unavailable")
	var me struct {
		RemindersChannel struct {
			State string `json:"state"`
		} `json:"reminders_channel"`
	}
	c.mustJSON(http.MethodGet, "/api/v1/me", nil, http.StatusOK, &me)
	if me.RemindersChannel.State != "unavailable" {
		log.Fatalf("e2e: состояние канала %s, ожидалось unavailable", me.RemindersChannel.State)
	}
	fmt.Printf("   reminders_channel.state = %s\n", me.RemindersChannel.State)

	step("3. Создание приглашения отвечает 503 DEPENDENCY_UNAVAILABLE")
	status, raw := c.request(http.MethodPost, "/api/v1/organizations/"+org.ID+"/invites",
		map[string]any{"id": "6c2b8e14-3f7a-4d95-8b20-1e4c6a9d3f07", "role": "viewer"})
	if status != http.StatusServiceUnavailable {
		log.Fatalf("e2e: приглашение без bot: статус %d, тело %s", status, raw)
	}
	if !strings.Contains(string(raw), "DEPENDENCY_UNAVAILABLE") {
		log.Fatalf("e2e: код ошибки приглашения: %s", raw)
	}
	fmt.Println("   ссылка приглашения не выдаётся без профиля бота: 503 DEPENDENCY_UNAVAILABLE")
}

// waitPlanned ждёт появления запланированного напоминания по документу.
func waitPlanned(ctx context.Context, q remindersv1.ReminderQueryServiceClient,
	documentID string, limit time.Duration) *remindersv1.PlannedReminder {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		resp, err := q.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{DocumentId: documentID})
		if err == nil {
			for _, item := range resp.GetItems() {
				if item.GetStatus() == remindersv1.ReminderStatus_REMINDER_STATUS_PLANNED ||
					item.GetStatus() == remindersv1.ReminderStatus_REMINDER_STATUS_HANDED_OFF {
					return item
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Fatalf("e2e: план документа %s не построен за %s", documentID, limit)
	return nil
}

// waitPlannedDays ждёт появления напоминания с заданным отступом.
func waitPlannedDays(ctx context.Context, q remindersv1.ReminderQueryServiceClient,
	documentID string, days uint32, limit time.Duration) *remindersv1.PlannedReminder {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		resp, err := q.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{DocumentId: documentID})
		if err == nil {
			for _, item := range resp.GetItems() {
				if item.GetDaysBefore() == days &&
					item.GetStatus() != remindersv1.ReminderStatus_REMINDER_STATUS_CANCELLED {
					return item
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Fatalf("e2e: напоминание за %d дн. по документу %s не найдено", days, documentID)
	return nil
}

// waitPlanEmpty ждёт, пока в плане не останется действующих напоминаний.
func waitPlanEmpty(ctx context.Context, q remindersv1.ReminderQueryServiceClient,
	documentID string, limit time.Duration) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		resp, err := q.GetDocumentPlan(ctx, &remindersv1.GetDocumentPlanRequest{DocumentId: documentID})
		if err == nil {
			active := 0
			for _, item := range resp.GetItems() {
				if item.GetStatus() == remindersv1.ReminderStatus_REMINDER_STATUS_PLANNED {
					active++
				}
			}
			if active == 0 {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Fatalf("e2e: план документа %s не очищен за %s", documentID, limit)
}
