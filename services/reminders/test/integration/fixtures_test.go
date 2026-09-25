package integration_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/reminders/internal/adapters/postgres"
	"vovremya/services/reminders/internal/app"
	"vovremya/services/reminders/internal/domain"
	"vovremya/services/reminders/test/testutil"
)

const (
	orgID   = "0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01"
	orgID2  = "2c7f3d1a-6b4e-4f9c-8a21-5d3e7b9c1f08"
	docID   = "3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"
	docID2  = "4d0f8b63-2a5c-4e7b-9f13-6c8d0a2e4f09"
	periodA = "6d1f0a73-2b8c-4e5f-9a10-3c7b5e2d8f04"
	periodB = "7e2a1b84-3c9d-4f60-8b21-4d8c6f3e9a05"
	accID   = "9f3c7b21-4d6e-4a8b-b5c0-1e2f3a4b5c06"
	accID2  = "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c07"
)

// env — собранный для теста экземпляр сервиса с реальной PostgreSQL.
type env struct {
	pool      *pgkit.Pool
	clock     *testutil.Clock
	bot       *testutil.FakeBot
	ingest    *app.IngestService
	query     *app.QueryService
	scheduler *app.Scheduler
	retention *app.RetentionJob
	reminders *postgres.ReminderRepo
	proj      *postgres.ProjectionRepo
	inbox     *postgres.InboxRepo
	eventSeq  int
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := testutil.Pool(t)
	m := metrics.New()
	am := app.NewMetrics(m)
	log := slog.New(slog.DiscardHandler)
	clk := testutil.NewClock(time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC))
	bot := testutil.NewFakeBot()

	inbox := postgres.NewInboxRepo(pool, m)
	proj := postgres.NewProjectionRepo(pool, m)
	remRepo := postgres.NewReminderRepo(pool, m)
	replanner := app.NewReplanner(proj, remRepo, clk, 24*time.Hour)

	return &env{
		pool:   pool,
		clock:  clk,
		bot:    bot,
		ingest: app.NewIngestService(pool, inbox, proj, replanner, clk, log, am),
		query:  app.NewQueryService(remRepo, proj),
		scheduler: app.NewScheduler(remRepo, proj, bot, clk, app.SchedulerConfig{
			Interval: time.Second, Batch: 100, Lease: 2 * time.Minute, Grace: 24 * time.Hour,
			Concurrency: 4, Backoff: domain.DefaultBackoff,
		}, log, am),
		retention: app.NewRetentionJob(inbox, remRepo, clk, app.RetentionConfig{
			Interval: time.Hour, InboxTTL: 168 * time.Hour, FinalizedTTL: 720 * time.Hour,
		}, log, am),
		reminders: remRepo,
		proj:      proj,
		inbox:     inbox,
	}
}

// eventID формирует детерминированный уникальный UUID события.
func (e *env) eventID() string {
	e.eventSeq++
	return uuidSeq(e.eventSeq)
}

func uuidSeq(n int) string {
	const hex = "0123456789abcdef"
	tail := []byte("000000000000")
	for i := len(tail) - 1; i >= 0 && n > 0; i-- {
		tail[i] = hex[n%16]
		n /= 16
	}
	return "5e000000-0000-4000-8000-" + string(tail)
}

func (e *env) apply(t *testing.T, ev app.Event) app.Result {
	t.Helper()
	res, err := e.ingest.Apply(context.Background(), ev)
	if err != nil {
		t.Fatalf("Apply(%s): %v", ev.Type, err)
	}
	return res
}

func (e *env) organizationEvent(version uint64, tz string) app.Event {
	return app.Event{
		ID: e.eventID(), Type: app.EventOrganizationState, SchemaVersion: app.SchemaVersion,
		SourceService: "core", AggregateType: app.AggregateOrganization, AggregateID: orgID,
		AggregateVersion: version, OccurredAt: e.clock.Now(),
		Organization: &domain.Organization{ID: orgID, Name: "Кафе на Неве", Timezone: tz},
	}
}

func (e *env) memberEvent(version uint64, accountID string, enabled bool, minutes int) app.Event {
	return app.Event{
		ID: e.eventID(), Type: app.EventMembershipState, SchemaVersion: app.SchemaVersion,
		SourceService: "core", AggregateType: app.AggregateMembership,
		AggregateID:      app.MembershipAggregateID(orgID, accountID),
		AggregateVersion: version, OccurredAt: e.clock.Now(),
		Member: &domain.Member{
			OrganizationID: orgID, AccountID: accountID, Kind: domain.AccountKindMax,
			MaxUserID: 1000 + int64(len(accountID)), NotifyEnabled: enabled, NotifyLocalMinutes: minutes,
		},
	}
}

func (e *env) documentEvent(t *testing.T, version uint64, periodID, validUntil string, offsets []int) app.Event {
	t.Helper()
	doc := &domain.Document{
		ID: docID, OrganizationID: orgID, Title: "Фискальный накопитель",
		Period: domain.Period{ID: periodID}, OffsetsDays: offsets,
	}
	if validUntil != "" {
		d, err := domain.ParseDate(validUntil)
		if err != nil {
			t.Fatalf("ParseDate: %v", err)
		}
		doc.Period.ValidUntil = &d
	}
	return app.Event{
		ID: e.eventID(), Type: app.EventDocumentState, SchemaVersion: app.SchemaVersion,
		SourceService: "core", AggregateType: app.AggregateDocument, AggregateID: docID,
		AggregateVersion: version, OccurredAt: e.clock.Now(), Document: doc,
	}
}

// seedPlan создаёт организацию, участника и документ с планом напоминаний.
func (e *env) seedPlan(t *testing.T) {
	t.Helper()
	e.apply(t, e.organizationEvent(1, "Europe/Moscow"))
	e.apply(t, e.memberEvent(1, accID, true, 9*60))
	e.apply(t, e.documentEvent(t, 1, periodA, "2026-12-31", []int{30, 14, 3}))
}

func (e *env) planFor(t *testing.T, documentID string) []domain.Reminder {
	t.Helper()
	items, err := e.reminders.ListByDocument(context.Background(), documentID)
	if err != nil {
		t.Fatalf("ListByDocument: %v", err)
	}
	return items
}

func countByStatus(items []domain.Reminder, status domain.Status) int {
	n := 0
	for _, it := range items {
		if it.Status == status {
			n++
		}
	}
	return n
}
