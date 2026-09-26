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
