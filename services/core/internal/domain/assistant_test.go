package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"vovremya/services/core/internal/domain"
)

func TestValidateAssistantText(t *testing.T) {
	if _, err := domain.ValidateAssistantText("text", "   ", 100); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("пустой текст должен отклоняться: %v", err)
	}
	if _, err := domain.ValidateAssistantText("text", strings.Repeat("я", 101), 100); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("длина считается в символах, а не байтах: %v", err)
	}
	clean, err := domain.ValidateAssistantText("text", "  Лицензия  ", 100)
	if err != nil || clean != "Лицензия" {
		t.Fatalf("ожидалась обрезка пробелов, получено %q, %v", clean, err)
	}
}

func TestParseAssistantDate(t *testing.T) {
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"2029-03-13": "2029-03-13",
		"13.03.2029": "2029-03-13",
		"13/03/2029": "2029-03-13",
		"":           "",
		"не дата":    "",
		"2029-13-45": "",
		"1899-01-01": "",
		"2099-01-01": "",
	}
	for in, want := range cases {
		if got := domain.ParseAssistantDate(in, now); got != want {
			t.Fatalf("ParseAssistantDate(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestSanitizeAssistantDraft(t *testing.T) {
	catalog := testCatalog()
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)

	draft := domain.SanitizeAssistantDraft(catalog, "  Лицензия на алкоголь  ", "78РПА0012345",
		"Комитет по промышленной политике", "14.03.2024", "13.03.2029", "alcohol_license", 0.9, now)
	if draft.Title != "Лицензия на алкоголь" || draft.Number != "78РПА0012345" {
		t.Fatalf("реквизиты не приведены: %+v", draft)
	}
	if draft.ValidFrom != "2024-03-14" || draft.ValidUntil != "2029-03-13" {
		t.Fatalf("даты не приведены: %+v", draft)
	}
	if draft.DocumentTypeCode != "alcohol_license" {
		t.Fatalf("тип документа потерян: %+v", draft)
	}
	if len(draft.Offsets) != 2 || draft.Offsets[0] != 60 {
		t.Fatalf("отступы напоминаний не подставлены: %+v", draft.Offsets)
	}

	unknown := domain.SanitizeAssistantDraft(catalog, "Договор", "", "", "", "", "не_существует", 2, now)
	if unknown.DocumentTypeCode != "" || len(unknown.Offsets) != 0 {
		t.Fatalf("код вне справочника должен отбрасываться: %+v", unknown)
	}
	if unknown.Confidence != 1 {
		t.Fatalf("уверенность должна ограничиваться единицей: %v", unknown.Confidence)
	}

	reversed := domain.SanitizeAssistantDraft(catalog, "Договор", "", "", "2029-01-01", "2024-01-01", "", 0.5, now)
	if reversed.ValidFrom != "" || reversed.ValidUntil != "" {
		t.Fatalf("перевёрнутые даты должны обнуляться: %+v", reversed)
	}

	long := domain.SanitizeAssistantDraft(catalog, strings.Repeat("я", 250), strings.Repeat("н", 150),
		strings.Repeat("и", 250), "", "", "", 0.1, now)
	if len([]rune(long.Title)) != domain.AssistantMaxTitleLen ||
		len([]rune(long.Number)) != domain.AssistantMaxNumberLen ||
		len([]rune(long.Issuer)) != domain.AssistantMaxIssuerLen {
		t.Fatalf("длины не обрезаны по границам OpenAPI: %d/%d/%d",
			len([]rune(long.Title)), len([]rune(long.Number)), len([]rune(long.Issuer)))
	}

	empty := domain.SanitizeAssistantDraft(catalog, "", "", "", "", "", "alcohol_license", 0.3, now)
	if empty.Title != "Лицензия на алкоголь" {
		t.Fatalf("название должно подставляться из справочника: %+v", empty)
	}
}

func TestSanitizeAssistantProfile(t *testing.T) {
	catalog := testCatalog()
	profile := domain.SanitizeAssistantProfile(catalog, "food_service",
		[]string{"sells_alcohol", "sells_alcohol", "чужой_код", "", "has_premises"}, 0.8)
	if profile.BusinessCategoryCode != "food_service" {
		t.Fatalf("вид деятельности потерян: %+v", profile)
	}
	if len(profile.FeatureCodes) != 2 || profile.FeatureCodes[0] != "sells_alcohol" ||
		profile.FeatureCodes[1] != "has_premises" {
		t.Fatalf("признаки не очищены от повторов и чужих кодов: %+v", profile.FeatureCodes)
	}

	unknown := domain.SanitizeAssistantProfile(catalog, "чужая_категория", nil, -1)
	if unknown.BusinessCategoryCode != "" || len(unknown.FeatureCodes) != 0 || unknown.Confidence != 0 {
		t.Fatalf("неизвестные значения должны отбрасываться: %+v", unknown)
	}
}

func TestParseAssistantDateLayouts(t *testing.T) {
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"2029-3-13":            "2029-03-13",
		"3.9.2027":             "2027-09-03",
		"2029-03-13T00:00:00Z": "2029-03-13",
		"2029-03-13 00:00":     "2029-03-13",
	}
	for in, want := range cases {
		if got := domain.ParseAssistantDate(in, now); got != want {
			t.Fatalf("ParseAssistantDate(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestExtractTextDates(t *testing.T) {
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		text        string
		from, until string
	}{
		{"Лицензия на Алкоголь от 12.03.2022", "2022-03-12", ""},
		{"Лицензия № 78РПА0012345, выдана 14.03.2024, действует до 13.03.2029", "2024-03-14", "2029-03-13"},
		{"Договор аренды с 01.01.2025 по 31.12.2025", "2025-01-01", "2025-12-31"},
		{"Срок действия: до 13 марта 2029 г.", "", "2029-03-13"},
		{"Дата начала действия 01.02.2026, дата окончания 31.01.2027", "2026-02-01", "2027-01-31"},
		{"Сертификат 2027-05-20", "", ""},
		{"Удостоверение до 31.02.2027", "", ""},
		{"Номер 112.03.2022 до 1.4.28", "", "2028-04-01"},
		{"Продлена до 01.06.2030, прежний срок до 01.06.2025", "", "2030-06-01"},
	}
	for _, c := range cases {
		got := domain.ExtractTextDates(c.text, now)
		if got.From != c.from || got.Until != c.until {
			t.Fatalf("ExtractTextDates(%q) = %+v, ожидалось from=%q until=%q", c.text, got, c.from, c.until)
		}
	}
}

func TestMergeTextDates(t *testing.T) {
	found := domain.TextDates{From: "2022-03-12"}
	// Модель поставила дату выдачи в окончание срока — исправляется.
	fixed := domain.MergeTextDates(domain.AssistantDraft{Title: "Лицензия", ValidUntil: "2022-03-12"}, found)
	if fixed.ValidFrom != "2022-03-12" || fixed.ValidUntil != "" {
		t.Fatalf("дата выдачи должна перейти в начало действия: %+v", fixed)
	}
	// Значения модели сохраняются, пустые поля дополняются.
	kept := domain.MergeTextDates(domain.AssistantDraft{ValidUntil: "2029-03-13"},
		domain.TextDates{From: "2024-03-14", Until: "2029-01-01"})
	if kept.ValidFrom != "2024-03-14" || kept.ValidUntil != "2029-03-13" {
		t.Fatalf("значения модели не должны заменяться: %+v", kept)
	}
	// Перевёрнутая пара теряет дату начала, срок окончания остаётся.
	reversed := domain.MergeTextDates(domain.AssistantDraft{ValidFrom: "2030-01-01"}, domain.TextDates{Until: "2029-03-13"})
	if reversed.ValidFrom != "" || reversed.ValidUntil != "2029-03-13" {
		t.Fatalf("срок окончания важнее даты начала: %+v", reversed)
	}
}

func TestAssistantDraftEmpty(t *testing.T) {
	if !(domain.AssistantDraft{Confidence: 0.4}).Empty() {
		t.Fatal("черновик без реквизитов должен считаться пустым")
	}
	if (domain.AssistantDraft{ValidUntil: "2029-03-13"}).Empty() {
		t.Fatal("черновик со сроком не пуст")
	}
}
