package integration_test

import (
	"context"
	"testing"
	"time"

	"vovremya/services/reminders/internal/app"
	"vovremya/services/reminders/internal/domain"
)

func TestIngestBuildsPlanFromCoreEvents(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)

	items := e.planFor(t, docID)
	if got := countByStatus(items, domain.StatusPlanned); got != 3 {
		t.Fatalf("ожидалось 3 запланированных напоминания, получено %d (всего %d)", got, len(items))
	}
	// Ближайшее напоминание: 2026-12-31 минус 30 дней, 09:00 Europe/Moscow = 06:00 UTC.
	want := time.Date(2026, 12, 1, 6, 0, 0, 0, time.UTC)
	if !items[0].DueAt.Equal(want) {
		t.Errorf("due_at: ожидалось %s, получено %s", want, items[0].DueAt)
	}
	if items[0].Key.PeriodID != periodA || items[0].Key.AccountID != accID {
		t.Errorf("ключ напоминания неверен: %+v", items[0].Key)
	}
}

func TestIngestIsIdempotentByEventID(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	ev := e.documentEvent(t, 2, periodA, "2026-12-31", []int{30, 14, 3})

	first := e.apply(t, ev)
	if first.Outcome != app.OutcomeApplied {
		t.Fatalf("первая доставка: ожидалось applied, получено %s", first.Outcome)
	}
	second := e.apply(t, ev) // повторная доставка того же события
	if second.Outcome != app.OutcomeDuplicate {
		t.Fatalf("повторная доставка: ожидалось duplicate, получено %s", second.Outcome)
	}
	if got := countByStatus(e.planFor(t, docID), domain.StatusPlanned); got != 3 {
		t.Errorf("повтор не должен менять план: запланировано %d", got)
	}
}

func TestIngestRejectsStaleVersion(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	// Версия 3 применяется, версия 2 приходит позже и должна быть отброшена.
	e.apply(t, e.documentEvent(t, 3, periodA, "2027-01-31", []int{30}))
	stale := e.apply(t, e.documentEvent(t, 2, periodA, "2026-12-31", []int{30, 14, 3}))
	if stale.Outcome != app.OutcomeStale {
		t.Fatalf("ожидалось stale, получено %s", stale.Outcome)
	}
	items := e.planFor(t, docID)
	if got := countByStatus(items, domain.StatusPlanned); got != 1 {
		t.Fatalf("после применения версии 3 должно остаться 1 напоминание, получено %d", got)
	}
	want := time.Date(2027, 1, 1, 6, 0, 0, 0, time.UTC)
	for _, it := range items {
		if it.Status == domain.StatusPlanned && !it.DueAt.Equal(want) {
			t.Errorf("план отражает устаревшую версию: due_at=%s", it.DueAt)
		}
	}
}

func TestIngestRejectsInvalidEvent(t *testing.T) {
	e := newEnv(t)
	bad := e.documentEvent(t, 1, periodA, "2026-12-31", []int{30})
	bad.Document.Title = "   "
	res := e.apply(t, bad)
	if res.Outcome != app.OutcomeRejected {
		t.Fatalf("ожидалось rejected, получено %s (%s)", res.Outcome, res.Message)
	}
	if len(e.planFor(t, docID)) != 0 {
		t.Error("отклонённое событие не должно создавать план")
	}
}

func TestRenewalCancelsOldPeriodAndPlansNew(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	// Продление: новый период с новой датой окончания (AC-05).
	e.apply(t, e.documentEvent(t, 2, periodB, "2027-06-30", []int{30, 14, 3}))

	items := e.planFor(t, docID)
	var oldPlanned, newPlanned, cancelled int
	for _, it := range items {
		switch {
		case it.Key.PeriodID == periodA && it.Status == domain.StatusPlanned:
			oldPlanned++
		case it.Key.PeriodID == periodA && it.Status == domain.StatusCancelled:
			cancelled++
		case it.Key.PeriodID == periodB && it.Status == domain.StatusPlanned:
			newPlanned++
		}
	}
	if oldPlanned != 0 || cancelled != 3 || newPlanned != 3 {
		t.Fatalf("после продления: planned(old)=%d cancelled(old)=%d planned(new)=%d", oldPlanned, cancelled, newPlanned)
	}
}

func TestTimezoneChangeReplansOrganization(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	e.apply(t, e.organizationEvent(2, "Asia/Vladivostok"))

	items := e.planFor(t, docID)
	want := time.Date(2026, 11, 30, 23, 0, 0, 0, time.UTC) // 09:00 во Владивостоке
	var found bool
	for _, it := range items {
		if it.Status == domain.StatusPlanned && it.Key.DaysBefore == 30 {
			found = true
			if !it.DueAt.Equal(want) {
				t.Errorf("после смены пояса due_at=%s, ожидалось %s", it.DueAt, want)
			}
		}
	}
	if !found {
		t.Fatal("напоминание за 30 дней не найдено после смены часового пояса")
	}
}

func TestNotificationSettingsChangeCancelsPlan(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	e.apply(t, e.memberEvent(2, accID, false, 9*60)) // уведомления выключены

	items := e.planFor(t, docID)
	if got := countByStatus(items, domain.StatusPlanned); got != 0 {
		t.Fatalf("после отключения уведомлений не должно остаться запланированных, получено %d", got)
	}
	if got := countByStatus(items, domain.StatusCancelled); got != 3 {
		t.Fatalf("ожидалось 3 отменённых напоминания, получено %d", got)
	}

	// Включение обратно восстанавливает план с новым локальным временем.
	e.apply(t, e.memberEvent(3, accID, true, 21*60))
	items = e.planFor(t, docID)
	if got := countByStatus(items, domain.StatusPlanned); got != 3 {
		t.Fatalf("после включения уведомлений ожидалось 3 напоминания, получено %d", got)
	}
	want := time.Date(2026, 12, 1, 18, 0, 0, 0, time.UTC) // 21:00 по Москве
	for _, it := range items {
		if it.Status == domain.StatusPlanned && it.Key.DaysBefore == 30 && !it.DueAt.Equal(want) {
			t.Errorf("due_at после смены времени: %s, ожидалось %s", it.DueAt, want)
		}
	}
}

func TestMemberRemovalAndAccountDeletionCancelReminders(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	e.apply(t, e.memberEvent(1, accID2, true, 10*60))
	if got := countByStatus(e.planFor(t, docID), domain.StatusPlanned); got != 6 {
		t.Fatalf("ожидалось 6 напоминаний на двух получателей, получено %d", got)
	}

	e.apply(t, app.Event{
		ID: e.eventID(), Type: app.EventMembershipRemoved, SchemaVersion: app.SchemaVersion,
		SourceService: "core", AggregateType: app.AggregateMembership,
		AggregateID:      app.MembershipAggregateID(orgID, accID2),
		AggregateVersion: 2, OccurredAt: e.clock.Now(),
	})
	if got := countByStatus(e.planFor(t, docID), domain.StatusPlanned); got != 3 {
		t.Fatalf("после исключения участника ожидалось 3 напоминания, получено %d", got)
	}

	e.apply(t, app.Event{
		ID: e.eventID(), Type: app.EventAccountDeleted, SchemaVersion: app.SchemaVersion,
		SourceService: "core", AggregateType: app.AggregateAccount, AggregateID: accID,
		AggregateVersion: 1, OccurredAt: e.clock.Now(),
	})
	if got := countByStatus(e.planFor(t, docID), domain.StatusPlanned); got != 0 {
		t.Fatalf("после удаления аккаунта не должно остаться напоминаний, получено %d", got)
	}
	if _, err := e.proj.GetMember(context.Background(), orgID, accID); err == nil {
		t.Error("проекция участия удалённого аккаунта должна быть удалена")
	}
}

func TestDocumentAndOrganizationDeletionCancelReminders(t *testing.T) {
	t.Run("документ", func(t *testing.T) {
		e := newEnv(t)
		e.seedPlan(t)
		e.apply(t, app.Event{
			ID: e.eventID(), Type: app.EventDocumentDeleted, SchemaVersion: app.SchemaVersion,
			SourceService: "core", AggregateType: app.AggregateDocument, AggregateID: docID,
			AggregateVersion: 2, OccurredAt: e.clock.Now(),
		})
		if got := countByStatus(e.planFor(t, docID), domain.StatusPlanned); got != 0 {
			t.Fatalf("после удаления документа ожидалось 0 напоминаний, получено %d", got)
		}
	})
	t.Run("организация", func(t *testing.T) {
		e := newEnv(t)
		e.seedPlan(t)
		e.apply(t, app.Event{
			ID: e.eventID(), Type: app.EventOrganizationDeleted, SchemaVersion: app.SchemaVersion,
			SourceService: "core", AggregateType: app.AggregateOrganization, AggregateID: orgID,
			AggregateVersion: 2, OccurredAt: e.clock.Now(),
		})
		if got := countByStatus(e.planFor(t, docID), domain.StatusPlanned); got != 0 {
			t.Fatalf("после удаления организации ожидалось 0 напоминаний, получено %d", got)
		}
	})
}

func TestIngestStateReportsAppliedVersions(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	states, err := e.ingest.GetIngestState(context.Background(), []app.IngestState{
		{AggregateType: app.AggregateDocument, AggregateID: docID},
		{AggregateType: app.AggregateDocument, AggregateID: docID2},
	})
	if err != nil {
		t.Fatalf("GetIngestState: %v", err)
	}
	if !states[0].Exists || states[0].Version != 1 {
		t.Errorf("состояние известного документа: %+v", states[0])
	}
	if states[1].Exists || states[1].Version != 0 {
		t.Errorf("неизвестный документ должен иметь версию 0: %+v", states[1])
	}
}
