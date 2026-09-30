package domain_test

import (
	"errors"
	"testing"
	"time"

	"vovremya/services/core/internal/domain"
)

func TestHasControlChars(t *testing.T) {
	cases := []struct {
		in        string
		multiline bool
		want      bool
	}{
		{"Лицензия на алкоголь", false, false},
		{"Лицензия\x00", false, true},
		{"Договор\rBEGIN:VALARM", false, true},
		{"строка\nстрока", false, true},
		{"строка\nстрока\tс табуляцией", true, false},
		{"строка\rстрока", true, true},
		{"escape\x1b[31m", true, true},
		{"удаление\u007f", false, true},
		{"C1\u0085", false, true},
	}
	for _, tc := range cases {
		if got := domain.HasControlChars(tc.in, tc.multiline); got != tc.want {
			t.Errorf("HasControlChars(%q, %v) = %v, ожидалось %v", tc.in, tc.multiline, got, tc.want)
		}
	}
}

func TestNormalizeMultilineAndSingleLine(t *testing.T) {
	if got := domain.NormalizeMultiline("а\r\nб\rв\nг"); got != "а\nб\nв\nг" {
		t.Fatalf("NormalizeMultiline: %q", got)
	}
	if got := domain.SingleLine("  Лицензия\nна\t алкоголь\x00  "); got != "Лицензия на алкоголь" {
		t.Fatalf("SingleLine: %q", got)
	}
}

func fieldCodes(err error) map[string]string {
	var ve *domain.ValidationError
	out := map[string]string{}
	if errors.As(err, &ve) {
		for _, f := range ve.Fields {
			out[f.Field] = f.Code
		}
	}
	return out
}

// TestValidateDocumentRejectsControlCharacters: NUL, CR и прочие управляющие символы
// отклоняются как invalid_format, в заметках допустимы перевод строки и табуляция (BUG-003, BUG-004).
func TestValidateDocumentRejectsControlCharacters(t *testing.T) {
	catalog := testCatalog()
	v := &domain.Validator{}
	domain.ValidateDocument(domain.DocumentInput{
		Title: "Лицензия\x00", Number: "№\r1", Issuer: "Комитет\x1b", ResponsibleLabel: "Иван\n",
		Notes: "первая строка\nвторая\tс табуляцией", ReferenceURL: "https://example.ru/\x00",
	}, catalog, v)
	codes := fieldCodes(v.Err())
	for _, field := range []string{"title", "number", "issuer", "responsible_label", "reference_url"} {
		if codes[field] != domain.CodeInvalidFormat {
			t.Errorf("%s: код %q, ожидался invalid_format", field, codes[field])
		}
	}
	if _, bad := codes["notes"]; bad {
		t.Errorf("перевод строки и табуляция в заметках допустимы: %v", codes)
	}

	v = &domain.Validator{}
	domain.ValidateDocument(domain.DocumentInput{Title: "Лицензия", Notes: "звонок\x07"}, catalog, v)
	if fieldCodes(v.Err())["notes"] != domain.CodeInvalidFormat {
		t.Error("управляющий символ в заметках должен отклоняться")
	}
}

func TestValidateOrganizationRejectsControlCharacters(t *testing.T) {
	v := &domain.Validator{}
	domain.ValidateOrganization(domain.OrganizationInput{Name: "Кафе\x00"}, testCatalog(), v)
	if fieldCodes(v.Err())["name"] != domain.CodeInvalidFormat {
		t.Fatalf("название организации с NUL должно отклоняться: %v", v.Err())
	}
}

// TestAssistantDraftIsSingleLine: ответ модели с переводами строк не должен приводить
// к отказу при сохранении формы.
func TestAssistantDraftIsSingleLine(t *testing.T) {
	d := domain.SanitizeAssistantDraft(testCatalog(), "Лицензия\nна алкоголь", "78\r\nРПА", "Комитет\t по\x00 политике",
		"", "", "", 0.5, time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC))
	if d.Title != "Лицензия на алкоголь" || d.Number != "78 РПА" || d.Issuer != "Комитет по политике" {
		t.Fatalf("черновик: %+v", d)
	}
}
