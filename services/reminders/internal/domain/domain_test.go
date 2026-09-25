package domain_test

import (
	"errors"
	"testing"
	"time"

	"vovremya/services/reminders/internal/domain"
)

func TestPluralDays(t *testing.T) {
	cases := map[int]string{0: "дней", 1: "день", 2: "дня", 3: "дня", 4: "дня", 5: "дней",
		11: "дней", 12: "дней", 14: "дней", 21: "день", 22: "дня", 25: "дней", 101: "день", 114: "дней"}
	for n, want := range cases {
		if got := domain.PluralDays(n); got != want {
			t.Errorf("PluralDays(%d): ожидалось %q, получено %q", n, want, got)
		}
	}
}

func TestReminderText(t *testing.T) {
	until, _ := domain.ParseDate("2026-12-31")
	got := domain.ReminderText("Фискальный накопитель", "Кафе на Неве", 3, until)
	want := "Через 3 дня заканчивается срок: «Фискальный накопитель». Организация: Кафе на Неве. Срок до 31.12.2026."
	if got != want {
		t.Errorf("текст напоминания:\nожидалось %q\nполучено  %q", want, got)
	}
	today := domain.ReminderText("Лицензия", "ООО Ромашка", 0, until)
	if today != "Сегодня заканчивается срок: «Лицензия». Организация: ООО Ромашка. Срок до 31.12.2026." {
		t.Errorf("текст на сегодня неверен: %q", today)
	}
}

func TestReminderTextRespectsMaxLength(t *testing.T) {
	until, _ := domain.ParseDate("2026-12-31")
	long := make([]rune, 500)
	for i := range long {
		long[i] = 'я'
	}
	text := domain.ReminderText(string(long), string(long), 5, until)
	if len([]rune(text)) > domain.MaxMessageLen {
		t.Errorf("длина текста %d превышает предел %d", len([]rune(text)), domain.MaxMessageLen)
	}
}

func TestBackoffBounds(t *testing.T) {
	p := domain.DefaultBackoff
	for attempts := 0; attempts < 12; attempts++ {
		d := p.Backoff(attempts)
		if d <= 0 {
			t.Fatalf("attempts=%d: задержка должна быть положительной, получено %s", attempts, d)
		}
		if d > p.MaxDelay() {
			t.Fatalf("attempts=%d: задержка %s превышает предел %s", attempts, d, p.MaxDelay())
		}
	}
	if min := p.Backoff(0); min < p.Base {
		t.Errorf("первая задержка %s меньше базовой %s", min, p.Base)
	}
}

func TestStatusTransitions(t *testing.T) {
	cases := []struct {
		from, to domain.Status
		allowed  bool
	}{
		{domain.StatusPlanned, domain.StatusHandedOff, true},
		{domain.StatusPlanned, domain.StatusCancelled, true},
		{domain.StatusPlanned, domain.StatusSkipped, true},
		{domain.StatusCancelled, domain.StatusPlanned, true},
		{domain.StatusHandedOff, domain.StatusPlanned, false},
		{domain.StatusHandedOff, domain.StatusCancelled, false},
		{domain.StatusSkipped, domain.StatusPlanned, false},
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.allowed {
			t.Errorf("переход %s -> %s: ожидалось %t", c.from, c.to, c.allowed)
		}
	}
}

func TestReminderExpiry(t *testing.T) {
	due := time.Date(2026, 9, 20, 6, 0, 0, 0, time.UTC)
	r := domain.Reminder{DueAt: due}
	if r.Expired(due.Add(23*time.Hour), 24*time.Hour) {
		t.Error("напоминание не должно считаться просроченным внутри grace")
	}
	if !r.Expired(due.Add(25*time.Hour), 24*time.Hour) {
		t.Error("напоминание должно считаться просроченным за пределами grace")
	}
	if got := r.NotAfter(24 * time.Hour); !got.Equal(due.Add(24 * time.Hour)) {
		t.Errorf("NotAfter: ожидалось %s, получено %s", due.Add(24*time.Hour), got)
	}
}

func TestProjectionValidation(t *testing.T) {
	t.Run("организация", func(t *testing.T) {
		org := domain.Organization{ID: orgID, Name: "ООО", Timezone: "Europe/Moscow"}
		if err := org.Validate(); err != nil {
			t.Fatalf("корректная организация отвергнута: %v", err)
		}
		bad := org
		bad.Timezone = "Nowhere/Nothing"
		if err := bad.Validate(); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("ожидалась ошибка валидации пояса, получено %v", err)
		}
		bad = org
		bad.Name = "  "
		if err := bad.Validate(); err == nil {
			t.Error("пустое название должно отвергаться")
		}
	})
	t.Run("участие", func(t *testing.T) {
		m := domain.Member{OrganizationID: orgID, AccountID: accID, Kind: domain.AccountKindMax,
			MaxUserID: 1001, NotifyEnabled: true, NotifyLocalMinutes: 540}
		if err := m.Validate(); err != nil {
			t.Fatalf("корректное участие отвергнуто: %v", err)
		}
		bad := m
		bad.NotifyLocalMinutes = 1440
		if err := bad.Validate(); err == nil {
			t.Error("время 1440 минут должно отвергаться")
		}
		bad = m
		bad.Kind = domain.AccountKindReview
		if err := bad.Validate(); err == nil {
			t.Error("служебный аккаунт с max_user_id должен отвергаться")
		}
		bad = m
		bad.MaxUserID = 0
		if err := bad.Validate(); err == nil {
			t.Error("аккаунт MAX без идентификатора должен отвергаться")
		}
	})
	t.Run("документ", func(t *testing.T) {
		until, _ := domain.ParseDate("2026-12-31")
		from, _ := domain.ParseDate("2026-01-01")
		d := domain.Document{ID: docID, OrganizationID: orgID, Title: "Документ",
			Period: domain.Period{ID: perID, ValidFrom: &from, ValidUntil: &until}, OffsetsDays: []int{30, 7}}
		if err := d.Validate(); err != nil {
			t.Fatalf("корректный документ отвергнут: %v", err)
		}
		bad := d
		bad.OffsetsDays = []int{7, 7}
		if err := bad.Validate(); err == nil {
			t.Error("повторяющиеся отступы должны отвергаться")
		}
		bad = d
		bad.OffsetsDays = []int{1, 2, 3, 4, 5, 6}
		if err := bad.Validate(); err == nil {
			t.Error("более пяти отступов должны отвергаться")
		}
		bad = d
		bad.OffsetsDays = []int{400}
		if err := bad.Validate(); err == nil {
			t.Error("отступ больше 365 должен отвергаться")
		}
		bad = d
		swapped := domain.Period{ID: perID, ValidFrom: &until, ValidUntil: &from}
		bad.Period = swapped
		if err := bad.Validate(); err == nil {
			t.Error("дата окончания раньше начала должна отвергаться")
		}
	})
}

func TestDateArithmetic(t *testing.T) {
	d, err := domain.ParseDate("2026-03-01")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	if got := d.AddDays(-1).String(); got != "2026-02-28" {
		t.Errorf("AddDays(-1): ожидалось 2026-02-28, получено %s", got)
	}
	if got := d.Format(); got != "01.03.2026" {
		t.Errorf("Format: ожидалось 01.03.2026, получено %s", got)
	}
	if _, err := domain.ParseDate("01.03.2026"); err == nil {
		t.Error("ожидалась ошибка для неверного формата даты")
	}
}

func TestIsUUID(t *testing.T) {
	if !domain.IsUUID(orgID) {
		t.Error("корректный UUID v4 не распознан")
	}
	for _, bad := range []string{"", "not-a-uuid", "0B6F2C1E-5A3D-4C8E-9F21-7D4A6B8C9E01",
		"0b6f2c1e-5a3d-1c8e-9f21-7d4a6b8c9e01"} {
		if domain.IsUUID(bad) {
			t.Errorf("значение %q не должно считаться UUID v4", bad)
		}
	}
}
