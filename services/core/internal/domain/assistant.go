package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Границы полей черновика повторяют ограничения OpenAPI для документа.
const (
	AssistantMaxTitleLen  = 200
	AssistantMaxNumberLen = 100
	AssistantMaxIssuerLen = 200
	AssistantMinYear      = 1990
	AssistantMaxYearAhead = 30
)

// AssistantDraft — проверенный черновик карточки документа.
type AssistantDraft struct {
	Title            string
	Number           string
	Issuer           string
	ValidFrom        string
	ValidUntil       string
	DocumentTypeCode string
	Offsets          []int
	Confidence       float64
}

// Empty сообщает, что ассистент не нашёл ни одного реквизита: пользователю нечего
// проверять, и сценарий отвечает DOCUMENT_NOT_RECOGNIZED.
func (d AssistantDraft) Empty() bool {
	return d.Title == "" && d.Number == "" && d.Issuer == "" && d.ValidFrom == "" && d.ValidUntil == "" &&
		d.DocumentTypeCode == ""
}

// AssistantProfile — проверенный профиль организации по описанию.
type AssistantProfile struct {
	BusinessCategoryCode string
	FeatureCodes         []string
	Confidence           float64
}

// ValidateAssistantText проверяет текст, который пользователь отдаёт ассистенту.
func ValidateAssistantText(field, text string, maxChars int) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", ValidationFor(field, CodeRequired, "поле обязательно")
	}
	if utf8.RuneCountInString(trimmed) > maxChars {
		return "", ValidationFor(field, CodeTooLong, "текст длиннее допустимого")
	}
	return trimmed, nil
}

// clampRunes обрезает строку по числу символов, а не байт.
func clampRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:max]))
}

// ParseAssistantDate приводит дату ответа модели к YYYY-MM-DD. Допускаются форматы
// YYYY-MM-DD и DD.MM.YYYY (день и месяц — одной или двумя цифрами, разделитель — точка
// или косая черта), в том числе с временем после даты; значения вне разумного
// диапазона отбрасываются.
func ParseAssistantDate(value string, now time.Time) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 10 && (trimmed[10] == 'T' || trimmed[10] == ' ') {
		trimmed = trimmed[:10] // 2029-03-13T00:00:00Z
	}
	if trimmed == "" {
		return ""
	}
	var parsed time.Time
	var err error
	for _, layout := range []string{"2006-1-2", "2.1.2006", "2/1/2006"} {
		parsed, err = time.Parse(layout, trimmed)
		if err == nil {
			break
		}
	}
	if err != nil || !assistantYearInRange(parsed.Year(), now) {
		return ""
	}
	return parsed.Format("2006-01-02")
}

// assistantYearInRange отсекает явные ошибки распознавания: даты до 1990 года и дальше 30 лет вперёд.
func assistantYearInRange(year int, now time.Time) bool {
	return year >= AssistantMinYear && year <= now.UTC().Year()+AssistantMaxYearAhead
}

// SanitizeAssistantDraft приводит ответ модели к безопасному черновику: режет длины,
// проверяет даты и отбрасывает коды, которых нет в справочнике.
func SanitizeAssistantDraft(catalog *Catalog, title, number, issuer, validFrom, validUntil, typeCode string,
	confidence float64, now time.Time) AssistantDraft {
	draft := AssistantDraft{
		Title:      clampRunes(title, AssistantMaxTitleLen),
		Number:     clampRunes(number, AssistantMaxNumberLen),
		Issuer:     clampRunes(issuer, AssistantMaxIssuerLen),
		ValidFrom:  ParseAssistantDate(validFrom, now),
		ValidUntil: ParseAssistantDate(validUntil, now),
		Confidence: clampConfidence(confidence),
	}
	if draft.ValidFrom != "" && draft.ValidUntil != "" && draft.ValidUntil < draft.ValidFrom {
		draft.ValidFrom, draft.ValidUntil = "", ""
	}
	code := strings.TrimSpace(typeCode)
	if code != "" && catalog != nil {
		if t, ok := catalog.DocumentType(code); ok {
			draft.DocumentTypeCode = t.Code
			draft.Offsets = append([]int(nil), t.DefaultOffsets...)
			if draft.Title == "" {
				draft.Title = clampRunes(t.Title, AssistantMaxTitleLen)
			}
		}
	}
	return draft
}

// SanitizeAssistantProfile оставляет только существующие коды справочника.
func SanitizeAssistantProfile(catalog *Catalog, categoryCode string, featureCodes []string,
	confidence float64) AssistantProfile {
	profile := AssistantProfile{Confidence: clampConfidence(confidence), FeatureCodes: []string{}}
	if catalog == nil {
		return profile
	}
	if code := strings.TrimSpace(categoryCode); code != "" && catalog.HasCategory(code) {
		profile.BusinessCategoryCode = code
	}
	seen := make(map[string]struct{}, len(featureCodes))
	for _, raw := range featureCodes {
		code := strings.TrimSpace(raw)
		if code == "" || !catalog.HasFeature(code) {
			continue
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		profile.FeatureCodes = append(profile.FeatureCodes, code)
	}
	return profile
}

func clampConfidence(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
