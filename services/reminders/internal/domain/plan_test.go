package domain_test

import (
	"testing"
	"time"

	"vovremya/services/reminders/internal/domain"
)

const (
	orgID  = "0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01"
	docID  = "3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"
	perID  = "6d1f0a73-2b8c-4e5f-9a10-3c7b5e2d8f04"
	accID  = "9f3c7b21-4d6e-4a8b-b5c0-1e2f3a4b5c06"
	accID2 = "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c07"
)

func date(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func baseInput(t *testing.T) domain.PlanInput {
	t.Helper()
	until := date(t, "2026-12-31")
	return domain.PlanInput{
		Organization: domain.Organization{ID: orgID, Name: "Кафе на Неве", Timezone: "Europe/Moscow", Version: 1},
		Document: domain.Document{
			ID: docID, OrganizationID: orgID, Title: "Фискальный накопитель",
			Period:      domain.Period{ID: perID, ValidUntil: &until},
			OffsetsDays: []int{30, 14, 3},
			Version:     1,
		},
		Members: []domain.Member{{
			OrganizationID: orgID, AccountID: accID, Kind: domain.AccountKindMax, MaxUserID: 1001,
			NotifyEnabled: true, NotifyLocalMinutes: 9 * 60, Version: 1,
		}},
		Now:   time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
		Grace: 24 * time.Hour,
	}
}

func TestBuildPlanUsesOrganizationTimezoneAndMemberTime(t *testing.T) {
	in := baseInput(t)
	items, err := domain.BuildPlan(in)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("ожидалось 3 напоминания, получено %d", len(items))
	}
	// 2026-12-31 минус 30 дней = 2026-12-01, 09:00 по Москве = 06:00 UTC.
	want := time.Date(2026, 12, 1, 6, 0, 0, 0, time.UTC)
	if !items[0].DueAt.Equal(want) {
		t.Errorf("первое напоминание: ожидалось %s, получено %s", want, items[0].DueAt)
	}
	if items[0].Key.DaysBefore != 30 || items[0].Key.PeriodID != perID || items[0].Key.AccountID != accID {
		t.Errorf("ключ первого напоминания неверен: %+v", items[0].Key)
	}
	// Элементы отсортированы по возрастанию момента отправки.
	for i := 1; i < len(items); i++ {
		if items[i].DueAt.Before(items[i-1].DueAt) {
			t.Fatalf("план не отсортирован по due_at: %v", items)
		}
	}
}

func TestBuildPlanDifferentTimezones(t *testing.T) {
	cases := map[string]time.Time{
		"Europe/Kaliningrad": time.Date(2026, 12, 1, 7, 0, 0, 0, time.UTC),
		"Europe/Moscow":      time.Date(2026, 12, 1, 6, 0, 0, 0, time.UTC),
		"Asia/Yekaterinburg": time.Date(2026, 12, 1, 4, 0, 0, 0, time.UTC),
		"Asia/Vladivostok":   time.Date(2026, 11, 30, 23, 0, 0, 0, time.UTC),
	}
	for tz, want := range cases {
		t.Run(tz, func(t *testing.T) {
			in := baseInput(t)
			in.Organization.Timezone = tz
			in.Document.OffsetsDays = []int{30}
			items, err := domain.BuildPlan(in)
			if err != nil {
				t.Fatalf("BuildPlan: %v", err)
			}
			if len(items) != 1 {
				t.Fatalf("ожидалось 1 напоминание, получено %d", len(items))
			}
			if !items[0].DueAt.Equal(want) {
				t.Errorf("%s: ожидалось %s, получено %s", tz, want, items[0].DueAt)
			}
		})
	}
}

func TestBuildPlanSkipsPastBeyondGrace(t *testing.T) {
	in := baseInput(t)
	until := date(t, "2026-09-21")
	in.Document.Period.ValidUntil = &until
	in.Document.OffsetsDays = []int{30, 1} // 2026-08-22 (давно прошло) и 2026-09-20
	items, err := domain.BuildPlan(in)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ожидалось 1 напоминание в пределах grace, получено %d", len(items))
	}
	if items[0].Key.DaysBefore != 1 {
		t.Errorf("ожидался отступ 1 день, получен %d", items[0].Key.DaysBefore)
	}
}

func TestBuildPlanExcludesNonReceivingMembers(t *testing.T) {
	in := baseInput(t)
	in.Document.OffsetsDays = []int{30}
	in.Members = []domain.Member{
		{OrganizationID: orgID, AccountID: accID, Kind: domain.AccountKindMax, MaxUserID: 1001, NotifyEnabled: false},
		{OrganizationID: orgID, AccountID: accID2, Kind: domain.AccountKindReview, NotifyEnabled: true},
		{OrganizationID: orgID, AccountID: accID2, Kind: domain.AccountKindMax, MaxUserID: 2002, NotifyEnabled: true, Removed: true},
	}
	items, err := domain.BuildPlan(in)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("ожидался пустой план, получено %d элементов", len(items))
	}
}

func TestBuildPlanEmptyWithoutExpiryOrDeleted(t *testing.T) {
	t.Run("без даты окончания", func(t *testing.T) {
		in := baseInput(t)
		in.Document.Period.ValidUntil = nil
		items, err := domain.BuildPlan(in)
		if err != nil || len(items) != 0 {
			t.Fatalf("ожидался пустой план, получено %d, err=%v", len(items), err)
		}
	})
	t.Run("документ удалён", func(t *testing.T) {
		in := baseInput(t)
		in.Document.Deleted = true
		items, _ := domain.BuildPlan(in)
		if len(items) != 0 {
			t.Fatalf("ожидался пустой план для удалённого документа, получено %d", len(items))
		}
	})
	t.Run("организация удалена", func(t *testing.T) {
		in := baseInput(t)
		in.Organization.Deleted = true
		items, _ := domain.BuildPlan(in)
		if len(items) != 0 {
			t.Fatalf("ожидался пустой план для удалённой организации, получено %d", len(items))
		}
	})
}

func TestBuildPlanRejectsForeignOrganization(t *testing.T) {
	in := baseInput(t)
	in.Document.OrganizationID = accID
	if _, err := domain.BuildPlan(in); err == nil {
		t.Fatal("ожидалась ошибка для документа чужой организации")
	}
}

func TestBuildPlanUnknownTimezone(t *testing.T) {
	in := baseInput(t)
	in.Organization.Timezone = "Mars/Olympus"
	if _, err := domain.BuildPlan(in); err == nil {
		t.Fatal("ожидалась ошибка для неизвестного часового пояса")
	}
}

func TestPlanKeyIdempotencyKey(t *testing.T) {
	key := domain.PlanKey{PeriodID: perID, AccountID: accID, DaysBefore: 3}
	want := "rem:" + perID + ":" + accID + ":3"
	if got := key.IdempotencyKey(); got != want {
		t.Errorf("ключ идемпотентности: ожидалось %q, получено %q", want, got)
	}
}
