package domain_test

import (
	"errors"
	"testing"
	"time"

	"vovremya/services/core/internal/domain"
)

func date(t *testing.T, value string) time.Time {
	t.Helper()
	d, err := domain.ParseLocalDate(value)
	if err != nil {
		t.Fatalf("разбор даты %q: %v", value, err)
	}
	return d
}

func TestStatusOfBoundaries(t *testing.T) {
	today := date(t, "2026-09-24")
	cases := []struct {
		name       string
		validUntil *time.Time
		want       domain.DeadlineStatus
		daysLeft   *int
	}{
		{"бессрочный", nil, domain.StatusNoExpiry, nil},
		{"вчера", ptr(date(t, "2026-09-23")), domain.StatusExpired, ptr(-1)},
		{"сегодня", ptr(date(t, "2026-09-24")), domain.StatusExpiring, ptr(0)},
		{"через 30 дней", ptr(date(t, "2026-10-24")), domain.StatusExpiring, ptr(30)},
		{"через 31 день", ptr(date(t, "2026-10-25")), domain.StatusValid, ptr(31)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.StatusOf(tc.validUntil, today); got != tc.want {
				t.Fatalf("StatusOf = %s, ожидалось %s", got, tc.want)
			}
			got := domain.DaysLeft(tc.validUntil, today)
			switch {
			case tc.daysLeft == nil && got != nil:
				t.Fatalf("DaysLeft = %d, ожидалось nil", *got)
			case tc.daysLeft != nil && (got == nil || *got != *tc.daysLeft):
				t.Fatalf("DaysLeft = %v, ожидалось %d", got, *tc.daysLeft)
			}
		})
	}
}

func TestTodayInTimezone(t *testing.T) {
	moment := time.Date(2026, 9, 24, 21, 30, 0, 0, time.UTC)
	vladivostok, err := time.LoadLocation("Asia/Vladivostok")
	if err != nil {
		t.Skipf("база часовых поясов недоступна: %v", err)
	}
	if got := domain.FormatLocalDate(domain.TodayIn(moment, vladivostok)); got != "2026-09-25" {
		t.Fatalf("во Владивостоке уже следующий день, получено %s", got)
	}
	if got := domain.FormatLocalDate(domain.TodayIn(moment, time.UTC)); got != "2026-09-24" {
		t.Fatalf("в UTC дата не должна меняться, получено %s", got)
	}
}

func TestParseStartTargetGrammar(t *testing.T) {
	docID := "3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"
	token := "abcdefghijklmnopqrstuvwxyz0123456789_-ABCDE"
	cases := []struct {
		param string
		want  domain.StartKind
	}{
		{"", domain.StartNone},
		{"doc_" + docID, domain.StartDocument},
		{"org_" + docID, domain.StartOrganization},
		{"inv_" + token, domain.StartInvite},
		{"doc_не-uuid", domain.StartNone},
		{"inv_short", domain.StartNone},
		{"хулиганство", domain.StartNone},
		{"DOC_" + docID, domain.StartNone},
	}
	for _, tc := range cases {
		if got := domain.ParseStartTarget(tc.param); got.Kind != tc.want {
			t.Fatalf("ParseStartTarget(%q).Kind = %s, ожидалось %s", tc.param, got.Kind, tc.want)
		}
	}
	if got := domain.ParseStartTarget("inv_" + token); got.InviteToken != token {
		t.Fatalf("токен приглашения не извлечён: %q", got.InviteToken)
	}
}

func TestRoleAllows(t *testing.T) {
	if !domain.RoleOwner.Allows(domain.RoleEditor) || !domain.RoleEditor.Allows(domain.RoleViewer) {
		t.Fatal("порядок ролей нарушен")
	}
	if domain.RoleViewer.Allows(domain.RoleEditor) {
		t.Fatal("наблюдатель не может редактировать")
	}
	if domain.Role("stranger").Allows(domain.RoleViewer) {
		t.Fatal("неизвестная роль не даёт прав")
	}
	if domain.RoleOwner.Invitable() {
		t.Fatal("роль владельца не выдаётся приглашением")
	}
}

func TestCatalogApplicability(t *testing.T) {
	catalog := testCatalog()
	cases := []struct {
		name     string
		category string
		features []string
		want     []string
	}{
		{"общепит с алкоголем", "food_service", []string{"sells_alcohol", "has_premises"},
			[]string{"alcohol_license", "lease_contract", "common_doc"}},
		{"общепит без признаков", "food_service", nil, []string{"common_doc"}},
		{"розница с помещением", "retail", []string{"has_premises"}, []string{"lease_contract", "common_doc"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := catalog.Applicable(tc.category, tc.features)
			if len(got) != len(tc.want) {
				t.Fatalf("получено %d типов, ожидалось %d: %+v", len(got), len(tc.want), got)
			}
			for i, code := range tc.want {
				if got[i].Code != code {
					t.Fatalf("позиция %d: %s, ожидалось %s", i, got[i].Code, code)
				}
			}
		})
	}
}

func TestCatalogDefaultOffsetsAndETag(t *testing.T) {
	catalog := testCatalog()
	if got := catalog.DefaultOffsets("alcohol_license"); len(got) != 2 || got[0] != 60 {
		t.Fatalf("отступы типа документа не применены: %v", got)
	}
	if got := catalog.DefaultOffsets("unknown_type"); len(got) != 3 || got[0] != 30 {
		t.Fatalf("ожидались отступы по умолчанию 30/7/1, получено %v", got)
	}
	if catalog.ETag() != testCatalog().ETag() {
		t.Fatal("ETag справочника должен быть детерминированным")
	}
}

func TestValidateDocument(t *testing.T) {
	catalog := testCatalog()
	v := &domain.Validator{}
	until := date(t, "2026-01-01")
	from := date(t, "2026-02-01")
	domain.ValidateDocument(domain.DocumentInput{
		PublicID: "не-uuid", Title: "  ", ReferenceURL: "http://insecure",
		DocumentTypeCode: "missing_type", ValidFrom: &from, ValidUntil: &until,
		Offsets: []int{1, 1, 400, 2, 3, 4},
	}, catalog, v)
	err := v.Err()
	if err == nil {
		t.Fatal("ожидались ошибки валидации")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatal("ошибка должна разворачиваться в ErrValidation")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatal("ожидался ValidationError")
	}
	fields := map[string]bool{}
	for _, f := range ve.Fields {
		fields[f.Field] = true
	}
	for _, want := range []string{"id", "title", "reference_url", "document_type_code", "valid_until", "reminder_offsets_days"} {
		if !fields[want] {
			t.Fatalf("не найдена ошибка поля %s: %+v", want, ve.Fields)
		}
	}
}

func TestNormalizeOffsets(t *testing.T) {
	got := domain.NormalizeOffsets([]int{7, 30, 7, 1})
	want := []int{30, 7, 1}
	if len(got) != len(want) {
		t.Fatalf("получено %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("получено %v, ожидалось %v", got, want)
		}
	}
}

func TestOutboxBackoffGrowsAndIsBounded(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	first := domain.OutboxBackoff(now, 1, 0).Sub(now)
	second := domain.OutboxBackoff(now, 2, 0).Sub(now)
	if second <= first {
		t.Fatalf("выдержка должна расти: %s → %s", first, second)
	}
	if got := domain.OutboxBackoff(now, 20, 0).Sub(now); got != 5*time.Minute {
		t.Fatalf("предел выдержки 5 минут, получено %s", got)
	}
	withJitter := domain.OutboxBackoff(now, 1, 1).Sub(now)
	if withJitter <= first || withJitter > first+5*time.Second {
		t.Fatalf("случайная добавка вне диапазона: %s", withJitter)
	}
}

func TestMemberJoinedText(t *testing.T) {
	got := domain.MemberJoinedText("Кафе «Пример»", "Мария", domain.RoleEditor)
	want := "В организацию «Кафе «Пример»» присоединился участник Мария с ролью «редактор»."
	if got != want {
		t.Fatalf("текст сообщения: %q", got)
	}
}

func TestMembershipNotifyMinutes(t *testing.T) {
	m := domain.Membership{NotifyLocalTime: "18:45"}
	if got := m.NotifyLocalMinutes(); got != 18*60+45 {
		t.Fatalf("минуты от полуночи: %d", got)
	}
	broken := domain.Membership{NotifyLocalTime: "oops"}
	if got := broken.NotifyLocalMinutes(); got != 9*60 {
		t.Fatalf("при некорректном значении ожидается 09:00, получено %d", got)
	}
}

func testCatalog() *domain.Catalog {
	categories := []domain.BusinessCategory{{Code: "food_service", Title: "Общепит"}, {Code: "retail", Title: "Розница"}}
	regions := []domain.Region{{Code: "RU-SPE", Title: "Санкт-Петербург", DefaultTimezone: "Europe/Moscow"}}
	features := []domain.Feature{
		{Code: "sells_alcohol", Question: "Продаёте алкоголь?"},
		{Code: "has_premises", Question: "Есть помещение?"},
	}
	types := []domain.DocumentType{
		{Code: "alcohol_license", Title: "Лицензия на алкоголь", Description: "…", DataStatus: "model",
			DefaultOffsets: []int{60, 30}, SortOrder: 1},
		{Code: "lease_contract", Title: "Договор аренды", Description: "…", DataStatus: "model", SortOrder: 2},
		{Code: "common_doc", Title: "Общий документ", Description: "…", DataStatus: "verified", SortOrder: 3},
	}
	rules := []domain.ApplicabilityRule{
		{DocumentTypeCode: "alcohol_license", BusinessCategoryCode: "food_service", FeatureCodes: []string{"sells_alcohol"}},
		{DocumentTypeCode: "lease_contract", FeatureCodes: []string{"has_premises"}},
		{DocumentTypeCode: "common_doc"},
	}
	return domain.NewCatalog(categories, regions, features, types, rules)
}

func ptr[T any](v T) *T { return &v }
