package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/test/testutil"
)

// outboxRow — строка исходящих событий для проверки состояния доставки.
type outboxRow struct {
	EventType     string
	AggregateType string
	AggregateID   string
	Version       int64
	Status        string
	Attempts      int
	NextAttemptAt time.Time
	Payload       []byte
}

func readOutbox(t *testing.T, h *testutil.Harness) []outboxRow {
	t.Helper()
	ctx := context.Background()
	rows, err := h.Pool.DB(ctx).Query(ctx, `SELECT event_type, aggregate_type, aggregate_id,
		aggregate_version, status, attempts, next_attempt_at, payload
		FROM core.outbox_events ORDER BY id`)
	if err != nil {
		t.Fatalf("чтение outbox: %v", err)
	}
	defer rows.Close()
	var out []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.EventType, &r.AggregateType, &r.AggregateID, &r.Version,
			&r.Status, &r.Attempts, &r.NextAttemptAt, &r.Payload); err != nil {
			t.Fatalf("разбор строки outbox: %v", err)
		}
		out = append(out, r)
	}
	return out
}

func TestOutboxWrittenWithBusinessChangeAndDelivered(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 6001, "События")
	org := c.createOrganization(uuidFor(300), "Кафе событий")
	doc := c.createDocument(org.ID, uuidFor(301), "Лицензия", 30)

	rows := readOutbox(t, h)
	if len(rows) != 3 {
		t.Fatalf("ожидались события организации, участия и документа, получено %d: %+v", len(rows), rows)
	}
	if rows[0].EventType != domain.EventOrganizationState || rows[0].AggregateID != org.ID {
		t.Fatalf("первое событие: %+v", rows[0])
	}
	if rows[1].EventType != domain.EventMembershipState ||
		rows[1].AggregateID != domain.MembershipAggregateID(org.ID, membershipAccountID(t, h, org.ID)) {
		t.Fatalf("событие участия: %+v", rows[1])
	}
	if rows[2].EventType != domain.EventDocumentState || rows[2].AggregateID != doc.ID {
		t.Fatalf("событие документа: %+v", rows[2])
	}
	for _, r := range rows {
		if r.Status != domain.OutboxSent {
			t.Fatalf("синхронная попытка доставки должна перевести событие в sent: %+v", r)
		}
	}

	// Содержимое события документа соответствует контракту ingest.proto.
	var payload domain.DocumentStatePayload
	if err := json.Unmarshal(rows[2].Payload, &payload); err != nil {
		t.Fatalf("payload документа: %v", err)
	}
	if payload.DocumentID != doc.ID || payload.OrganizationID != org.ID || payload.Title != "Лицензия" {
		t.Fatalf("payload документа: %+v", payload)
	}
	if payload.CurrentPeriod.PeriodID != doc.CurrentPeriod.ID || payload.CurrentPeriod.ValidUntil == "" {
		t.Fatalf("период в payload: %+v", payload.CurrentPeriod)
	}
	if len(payload.Offsets) != 3 {
		t.Fatalf("отступы в payload: %v", payload.Offsets)
	}

	applied := h.Reminders.AppliedEvents()
	if len(applied) != 3 {
		t.Fatalf("reminders получил %d событий", len(applied))
	}
	if applied[2].AggregateVersion != 1 || applied[2].AggregateType != domain.AggregateDocument {
		t.Fatalf("версия агрегата документа: %+v", applied[2])
	}
}

func membershipAccountID(t *testing.T, h *testutil.Harness, orgPublicID string) string {
	t.Helper()
	ctx := context.Background()
	var accountID string
	err := h.Pool.DB(ctx).QueryRow(ctx, `SELECT a.public_id::text FROM core.memberships m
		JOIN core.accounts a ON a.id = m.account_id
		JOIN core.organizations o ON o.id = m.organization_id
		WHERE o.public_id = $1 AND m.role = 'owner'`, orgPublicID).Scan(&accountID)
	if err != nil {
		t.Fatalf("идентификатор владельца: %v", err)
	}
	return accountID
}

func TestRemindersUnavailableDoesNotBlockDocuments(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 6002, "Отказ reminders")
	org := c.createOrganization(uuidFor(310), "Кафе отказа")
	h.Reminders.SetFail(true)

	doc := c.createDocument(org.ID, uuidFor(311), "Документ при отказе", 20)
	if doc.RemindersState != string(domain.RemindersUnavailable) {
		t.Fatalf("состояние плана при отказе: %s", doc.RemindersState)
	}
	if doc.NextReminderAt != nil {
		t.Fatalf("при отказе план не выдаётся: %v", *doc.NextReminderAt)
	}

	var page struct {
		Items []struct {
			ID             string `json:"id"`
			RemindersState string `json:"reminders_state"`
		} `json:"items"`
	}
	if status := c.do(http.MethodGet, "/api/v1/organizations/"+org.ID+"/documents", nil, &page); status != http.StatusOK {
		t.Fatalf("реестр при отказе reminders: %d", status)
	}
	if len(page.Items) != 1 || page.Items[0].RemindersState != "unavailable" {
		t.Fatalf("реестр: %+v", page.Items)
	}

	// Событие осталось неотправленным и будет доставлено после восстановления.
	rows := readOutbox(t, h)
	last := rows[len(rows)-1]
	if last.Status == domain.OutboxSent {
		t.Fatalf("событие не должно считаться доставленным: %+v", last)
	}
	if last.Attempts < 1 || !last.NextAttemptAt.After(h.Clock.Now()) {
		t.Fatalf("ожидалась отложенная повторная попытка: %+v", last)
	}

	h.Reminders.SetFail(false)
	h.Clock.Advance(time.Minute)
	if _, err := h.App.DeliverOnce(context.Background()); err != nil {
		t.Fatalf("повторная доставка: %v", err)
	}
	rows = readOutbox(t, h)
	for _, r := range rows {
		if r.Status != domain.OutboxSent {
			t.Fatalf("после восстановления все события должны быть доставлены: %+v", r)
		}
	}

	var restored document
	if status := c.do(http.MethodGet, "/api/v1/documents/"+doc.ID, nil, &restored); status != http.StatusOK {
		t.Fatalf("карточка документа: %d", status)
	}
	if restored.RemindersState != string(domain.RemindersActual) {
		t.Fatalf("после восстановления ожидается actual, получено %s", restored.RemindersState)
	}
}

func TestPendingStateWhenDeliveryLags(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 6003, "Отложенная доставка")
	org := c.createOrganization(uuidFor(320), "Кафе задержки")
	// Чтение плана работает, приём событий — нет: изменение ещё не учтено.
	h.Reminders.SetFailApply(true)
	doc := c.createDocument(org.ID, uuidFor(321), "Документ в очереди", 25)
	if doc.RemindersState != string(domain.RemindersPending) {
		t.Fatalf("ожидалось pending, получено %s", doc.RemindersState)
	}

	h.Reminders.SetFailApply(false)
	h.Clock.Advance(2 * time.Minute)
	if _, err := h.App.DeliverOnce(context.Background()); err != nil {
		t.Fatalf("доставка: %v", err)
	}
	due := h.Clock.Now().Add(48 * time.Hour)
	h.Reminders.SetNext(doc.ID, due)
	var refreshed document
	c.do(http.MethodGet, "/api/v1/documents/"+doc.ID, nil, &refreshed)
	if refreshed.RemindersState != string(domain.RemindersActual) {
		t.Fatalf("после доставки ожидается actual, получено %s", refreshed.RemindersState)
	}
	if refreshed.NextReminderAt == nil {
		t.Fatal("ожидалось ближайшее напоминание из reminders-service")
	}
}

func TestRejectedEventIsNotRetriedImmediately(t *testing.T) {
	h := testutil.NewHarness(t)
	h.Reminders.Rejected[domain.EventDocumentState] = true
	c := newClient(t, h, 6004, "Отвергнутое событие")
	org := c.createOrganization(uuidFor(330), "Кафе отказа приёма")
	c.createDocument(org.ID, uuidFor(331), "Неприемлемый документ", 15)

	rows := readOutbox(t, h)
	last := rows[len(rows)-1]
	if last.EventType != domain.EventDocumentState {
		t.Fatalf("последнее событие: %+v", last)
	}
	if last.Status != domain.OutboxFailed {
		t.Fatalf("отвергнутое событие должно быть failed: %+v", last)
	}
	if !last.NextAttemptAt.After(h.Clock.Now().Add(23 * time.Hour)) {
		t.Fatalf("повтор отвергнутого события должен быть разрежен: %v", last.NextAttemptAt)
	}
}

func TestBotUnavailableDegradesGracefully(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 6005, "Отказ bot")
	org := c.createOrganization(uuidFor(340), "Кафе без бота")
	h.Bot.Fail = true

	// Документы продолжают работать.
	doc := c.createDocument(org.ID, uuidFor(341), "Документ без бота", 12)
	if doc.ID == "" {
		t.Fatal("документ должен создаваться при недоступном bot-service")
	}
	// Канал напоминаний помечается недоступным.
	var me struct {
		RemindersChannel struct {
			State string `json:"state"`
		} `json:"reminders_channel"`
	}
	if status := c.do(http.MethodGet, "/api/v1/me", nil, &me); status != http.StatusOK {
		t.Fatalf("GET /me при отказе bot: %d", status)
	}
	if me.RemindersChannel.State != "unavailable" {
		t.Fatalf("состояние канала: %s", me.RemindersChannel.State)
	}
	// Приглашение невозможно: ссылка строится по профилю бота.
	var p problem
	status := c.do(http.MethodPost, "/api/v1/organizations/"+org.ID+"/invites",
		map[string]any{"id": uuidFor(342), "role": "viewer"}, &p)
	if status != http.StatusServiceUnavailable || p.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("создание приглашения при отказе bot: %d %s", status, p.Code)
	}
}

func TestDeletionAndSettingsEvents(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 6006, "Удаления")
	org := c.createOrganization(uuidFor(350), "Кафе удалений")
	doc := c.createDocument(org.ID, uuidFor(351), "Документ на удаление", 30)

	if status := c.do(http.MethodPut, "/api/v1/organizations/"+org.ID+"/notification-settings",
		map[string]any{"enabled": true, "local_time": "20:15"}, nil); status != http.StatusOK {
		t.Fatalf("настройки: %d", status)
	}
	if status := c.do(http.MethodDelete, "/api/v1/documents/"+doc.ID, nil, nil); status != http.StatusNoContent {
		t.Fatalf("удаление документа: %d", status)
	}
	if status := c.do(http.MethodDelete, "/api/v1/organizations/"+org.ID, nil, nil); status != http.StatusNoContent {
		t.Fatalf("удаление организации: %d", status)
	}

	types := map[string]int{}
	var settings domain.MembershipStatePayload
	for _, r := range readOutbox(t, h) {
		types[r.EventType]++
		if r.EventType == domain.EventMembershipState {
			if err := json.Unmarshal(r.Payload, &settings); err != nil {
				t.Fatalf("payload участия: %v", err)
			}
		}
	}
	if types[domain.EventDocumentDeleted] != 1 || types[domain.EventOrganizationDeleted] != 1 {
		t.Fatalf("события удаления: %+v", types)
	}
	if types[domain.EventMembershipState] != 2 {
		t.Fatalf("событие изменения настроек не записано: %+v", types)
	}
	if settings.NotifyLocalMinutes != 20*60+15 || !settings.NotifyEnabled {
		t.Fatalf("настройки в событии: %+v", settings)
	}
}

func TestOutboxRowRequiresTransaction(t *testing.T) {
	h := testutil.NewHarness(t)
	// Прямая запись события вне транзакции запрещена: событие всегда
	// сохраняется вместе с изменением предметных данных.
	err := h.App.Outbox.Append(context.Background(), domain.OutboxEvent{
		EventID: "00000000-0000-4000-8000-000000000001", EventType: domain.EventDocumentState,
		AggregateType: domain.AggregateDocument, AggregateID: uuidFor(360), AggregateVersion: 1,
		Payload: []byte(`{}`), NextAttemptAt: h.Clock.Now(), CreatedAt: h.Clock.Now(),
	})
	if err == nil {
		t.Fatal("ожидался отказ записи события вне транзакции")
	}
}

func TestRetentionRemovesDeliveredEvents(t *testing.T) {
	h := testutil.NewHarness(t)
	c := newClient(t, h, 6007, "Очистка")
	org := c.createOrganization(uuidFor(370), "Кафе очистки")
	c.createDocument(org.ID, uuidFor(371), "Документ", 30)
	if len(readOutbox(t, h)) == 0 {
		t.Fatal("события не записаны")
	}
	h.Clock.Advance(8 * 24 * time.Hour)
	if err := h.App.RetentionOnce(context.Background()); err != nil {
		t.Fatalf("очистка: %v", err)
	}
	if rows := readOutbox(t, h); len(rows) != 0 {
		t.Fatalf("доставленные события старше срока хранения должны удаляться: %+v", rows)
	}
}
