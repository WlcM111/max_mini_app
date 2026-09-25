package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// OrganizationInput — проверяемые поля организации.
type OrganizationInput struct {
	Name                 string
	BusinessCategoryCode string
	RegionCode           string
	Timezone             string
	FeatureCodes         []string
}

// ValidateOrganization проверяет поля организации по справочнику.
func ValidateOrganization(in OrganizationInput, catalog *Catalog, v *Validator) {
	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		v.Add("name", CodeRequired, "название обязательно")
	case utf8.RuneCountInString(name) > 100:
		v.Add("name", CodeTooLong, "не более 100 символов")
	}
	if !catalog.HasCategory(in.BusinessCategoryCode) {
		v.Add("business_category_code", CodeUnknownValue, "вид деятельности не найден в справочнике")
	}
	if !catalog.HasRegion(in.RegionCode) {
		v.Add("region_code", CodeUnknownValue, "регион не найден в справочнике")
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		v.Add("timezone", CodeInvalidFormat, "часовой пояс IANA не распознан")
	}
	seen := make(map[string]struct{}, len(in.FeatureCodes))
	for _, code := range in.FeatureCodes {
		if _, dup := seen[code]; dup {
			v.Add("feature_codes", CodeNotUnique, "признаки не должны повторяться")
			continue
		}
		seen[code] = struct{}{}
		if !catalog.HasFeature(code) {
			v.Add("feature_codes", CodeUnknownValue, "признак не найден в справочнике: "+code)
		}
	}
	if len(in.FeatureCodes) > 20 {
		v.Add("feature_codes", CodeOutOfRange, "не более 20 признаков")
	}
}

// DocumentInput — проверяемые поля документа.
type DocumentInput struct {
	PublicID         string
	DocumentTypeCode string
	Title            string
	Number           string
	Issuer           string
	ResponsibleLabel string
	Notes            string
	ReferenceURL     string
	ValidFrom        *time.Time
	ValidUntil       *time.Time
	Offsets          []int
}

// ValidateDocument проверяет поля документа (handoff §7).
func ValidateDocument(in DocumentInput, catalog *Catalog, v *Validator) {
	if in.PublicID != "" && !IsUUIDv4(in.PublicID) {
		v.Add("id", CodeInvalidFormat, "ожидается UUID версии 4")
	}
	title := strings.TrimSpace(in.Title)
	switch {
	case title == "":
		v.Add("title", CodeRequired, "название обязательно")
	case utf8.RuneCountInString(title) > 200:
		v.Add("title", CodeTooLong, "не более 200 символов")
	}
	checkLen := func(field, value string, max int) {
		if utf8.RuneCountInString(value) > max {
			v.Add(field, CodeTooLong, "превышена допустимая длина")
		}
	}
	checkLen("number", in.Number, 100)
	checkLen("issuer", in.Issuer, 200)
	checkLen("responsible_label", in.ResponsibleLabel, 100)
	checkLen("notes", in.Notes, 2000)
	if in.ReferenceURL != "" {
		if !strings.HasPrefix(in.ReferenceURL, "https://") || strings.ContainsAny(in.ReferenceURL, " \t\n") ||
			utf8.RuneCountInString(in.ReferenceURL) > 1024 {
			v.Add("reference_url", CodeInvalidFormat, "ожидается ссылка https:// длиной до 1024 символов")
		}
	}
	if in.DocumentTypeCode != "" {
		if _, ok := catalog.DocumentType(in.DocumentTypeCode); !ok {
			v.Add("document_type_code", CodeUnknownValue, "тип документа не найден в справочнике")
		}
	}
	ValidatePeriodDates(in.ValidFrom, in.ValidUntil, v)
	ValidateOffsets(in.Offsets, v)
}

// ValidatePeriodDates проверяет даты периода.
func ValidatePeriodDates(from, until *time.Time, v *Validator) {
	if from != nil && until != nil && until.Before(*from) {
		v.Add("valid_until", CodeInvalidCombination, "дата окончания раньше даты начала")
	}
}

// ValidateOffsets проверяет отступы напоминаний.
func ValidateOffsets(offsets []int, v *Validator) {
	if offsets == nil {
		return
	}
	if len(offsets) > MaxReminderOffsets {
		v.Add("reminder_offsets_days", CodeOutOfRange, "не более 5 значений")
	}
	seen := make(map[int]struct{}, len(offsets))
	for _, d := range offsets {
		if d < 0 || d > 365 {
			v.Add("reminder_offsets_days", CodeOutOfRange, "значение вне диапазона 0…365")
		}
		if _, dup := seen[d]; dup {
			v.Add("reminder_offsets_days", CodeNotUnique, "значения не должны повторяться")
		}
		seen[d] = struct{}{}
	}
}

// NormalizeOffsets сортирует отступы по убыванию и убирает повторы.
func NormalizeOffsets(offsets []int) []int {
	seen := make(map[int]struct{}, len(offsets))
	out := make([]int, 0, len(offsets))
	for _, d := range offsets {
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] > out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// ValidateNotifyLocalTime проверяет время напоминаний формата HH:MM.
func ValidateNotifyLocalTime(value string, v *Validator) {
	if _, _, ok := parseHHMM(value); !ok || len(value) != 5 {
		v.Add("local_time", CodeInvalidFormat, "ожидается время в формате HH:MM")
	}
}
