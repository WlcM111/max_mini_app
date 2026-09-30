package integration_test

import (
	"context"
	"testing"
	"time"

	"vovremya/services/reminders/internal/app"
)

// TestDigestRecipientFilters: сводку получают только участники с включёнными уведомлениями
// из неудалённых организаций; документы удалённые в неё не попадают (BUG-007).
func TestDigestRecipientFilters(t *testing.T) {
	e := newEnv(t)
	e.seedPlan(t)
	e.apply(t, e.memberEvent(1, accID2, false, 10*60)) // уведомления выключены
	ctx := context.Background()

	recipients, err := e.proj.ListDigestRecipients(ctx)
	if err != nil {
		t.Fatalf("получатели: %v", err)
	}
	if len(recipients) != 1 || recipients[0].AccountID != accID || recipients[0].MaxUserID != accMaxUser ||
		recipients[0].Timezone != "Europe/Moscow" || recipients[0].NotifyLocalMinutes != 9*60 {
		t.Fatalf("получатели сводки: %+v", recipients)
	}
	docs, err := e.proj.ListDigestDocuments(ctx, orgID, time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC))
	if err != nil || len(docs) != 1 {
		t.Fatalf("документы сводки: %+v, %v", docs, err)
	}

	e.apply(t, app.Event{ID: e.eventID(), Type: app.EventDocumentDeleted, SchemaVersion: app.SchemaVersion,
		SourceService: "core", AggregateType: app.AggregateDocument, AggregateID: docID,
		AggregateVersion: 2, OccurredAt: e.clock.Now()})
	if docs, _ := e.proj.ListDigestDocuments(ctx, orgID, time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)); len(docs) != 0 {
		t.Fatalf("удалённый документ попал в сводку: %+v", docs)
	}

	e.apply(t, app.Event{ID: e.eventID(), Type: app.EventOrganizationDeleted, SchemaVersion: app.SchemaVersion,
		SourceService: "core", AggregateType: app.AggregateOrganization, AggregateID: orgID,
		AggregateVersion: 2, OccurredAt: e.clock.Now()})
	if recipients, _ := e.proj.ListDigestRecipients(ctx); len(recipients) != 0 {
		t.Fatalf("после удаления организации получателей быть не должно: %+v", recipients)
	}
}
