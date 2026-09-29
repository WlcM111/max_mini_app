package domain

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// TextDates — даты с явной ролью, найденные в тексте пользователя без языковой модели.
type TextDates struct {
	From  string // YYYY-MM-DD: «от», «с», «выдана», «дата выдачи»
	Until string // YYYY-MM-DD: «до», «по», «истекает», «дата окончания»
}

var (
	numericDatePattern = regexp.MustCompile(`(\d{1,2})[./](\d{1,2})[./](\d{4}|\d{2})`)
	isoDatePattern     = regexp.MustCompile(`(\d{4})-(\d{1,2})-(\d{1,2})`)
	wordDatePattern    = regexp.MustCompile(`(\d{1,2})[\s\x{00a0}]+(январ[яь]|феврал[яь]|марта?|апрел[яь]|ма[яй]|июн[яь]|июл[яь]|августа?|сентябр[яь]|октябр[яь]|ноябр[яь]|декабр[яь])[\s\x{00a0}]+(\d{4})`)

	// Основы названий месяцев: «март» проверяется раньше «ма» (май, мая).
	monthStems = []string{"январ", "феврал", "март", "апрел", "ма", "июн", "июл", "август", "сентябр", "октябр", "ноябр", "декабр"}

	untilMarkers = map[string]bool{"до": true, "по": true, "истекает": true, "истечения": true, "окончания": true,
		"окончание": true, "заканчивается": true, "годен": true, "годна": true}
	fromMarkers = map[string]bool{"от": true, "с": true, "со": true, "выдан": true, "выдана": true, "выдано": true,
		"выдачи": true, "начала": true, "заключен": true, "заключён": true, "заключения": true}
)

type dateRole int

const (
	roleUnknown dateRole = iota
	roleFrom
	roleUntil
)

type textDate struct {
	start int
	value string
}

// ExtractTextDates находит в тексте даты с явным маркером перед ними: «от 12.03.2022»,
// «выдана 14.03.2024» — начало действия; «до 13.03.2029», «по 31.12.2026» — окончание.
// Даты без маркера не используются: роль даты не угадывается (ADR-032, ADR-033).
func ExtractTextDates(text string, now time.Time) TextDates {
	lower := strings.ToLower(text)
	var out TextDates
	for _, d := range findTextDates(lower, now) {
		switch roleBefore(lower[:d.start]) {
		case roleFrom:
			if out.From == "" {
				out.From = d.value
			}
		case roleUntil:
			if d.value > out.Until {
				out.Until = d.value // «продлена до …» — берётся самый поздний срок
			}
		}
	}
	return out
}

// MergeTextDates дополняет черновик датами с явными маркерами из текста. Значения модели
// сохраняются; исправляется только известная ошибка — дата выдачи в поле окончания срока.
func MergeTextDates(draft AssistantDraft, found TextDates) AssistantDraft {
	if found.From != "" && draft.ValidUntil == found.From && found.Until != found.From {
		draft.ValidUntil = found.Until
		if draft.ValidFrom == "" {
			draft.ValidFrom = found.From
		}
	}
	if draft.ValidFrom == "" {
		draft.ValidFrom = found.From
	}
	if draft.ValidUntil == "" {
		draft.ValidUntil = found.Until
	}
	if draft.ValidFrom != "" && draft.ValidUntil != "" && draft.ValidUntil < draft.ValidFrom {
		draft.ValidFrom = "" // срок окончания важнее: напоминания строятся по нему
	}
	return draft
}

// findTextDates возвращает корректные даты текста в порядке появления.
func findTextDates(lower string, now time.Time) []textDate {
	var out []textDate
	add := func(start int, year, month, day int) {
		if value, ok := calendarDate(year, month, day, now); ok {
			out = append(out, textDate{start: start, value: value})
		}
	}
	for _, m := range numericDatePattern.FindAllStringSubmatchIndex(lower, -1) {
		if partOfNumber(lower, m[0], m[1]) {
			continue
		}
		year := atoi(lower[m[6]:m[7]])
		if m[7]-m[6] == 2 {
			year += 2000
		}
		add(m[0], year, atoi(lower[m[4]:m[5]]), atoi(lower[m[2]:m[3]]))
	}
	for _, m := range isoDatePattern.FindAllStringSubmatchIndex(lower, -1) {
		if partOfNumber(lower, m[0], m[1]) {
			continue
		}
		add(m[0], atoi(lower[m[2]:m[3]]), atoi(lower[m[4]:m[5]]), atoi(lower[m[6]:m[7]]))
	}
	for _, m := range wordDatePattern.FindAllStringSubmatchIndex(lower, -1) {
		if m[0] > 0 && isDigitByte(lower[m[0]-1]) {
			continue
		}
		add(m[0], atoi(lower[m[6]:m[7]]), monthByName(lower[m[4]:m[5]]), atoi(lower[m[2]:m[3]]))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

// roleBefore определяет роль даты по последнему слову перед ней.
func roleBefore(prefix string) dateRole {
	const window = 80
	if len(prefix) > window {
		cut := len(prefix) - window
		for cut < len(prefix) && !utf8.RuneStart(prefix[cut]) {
			cut++
		}
		prefix = prefix[cut:]
	}
	words := strings.FieldsFunc(prefix, func(r rune) bool { return !unicode.IsLetter(r) })
	if len(words) == 0 {
		return roleUnknown
	}
	last := words[len(words)-1]
	switch {
	case untilMarkers[last]:
		return roleUntil
	case fromMarkers[last]:
		return roleFrom
	case last == "действия" || last == "действует" || last == "действительна" || last == "действителен":
		// «Дата начала действия 01.01.2024» — начало; «срок действия 31.12.2026» — окончание.
		if len(words) > 1 && words[len(words)-2] == "начала" {
			return roleFrom
		}
		return roleUntil
	}
	return roleUnknown
}

// calendarDate проверяет существование даты и разумный диапазон лет.
func calendarDate(year, month, day int, now time.Time) (string, bool) {
	if month < 1 || month > 12 || day < 1 || day > 31 || !assistantYearInRange(year, now) {
		return "", false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Day() != day || int(t.Month()) != month {
		return "", false // 31.02 и подобные
	}
	return t.Format("2006-01-02"), true
}

// partOfNumber отсекает совпадения внутри длинных чисел и номеров вида 12.03.2022.1.
func partOfNumber(s string, start, end int) bool {
	if start > 0 && (isDigitByte(s[start-1]) || s[start-1] == '.' || s[start-1] == '/' || s[start-1] == '-') {
		return true
	}
	return end < len(s) && isDigitByte(s[end])
}

func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func monthByName(name string) int {
	for i, stem := range monthStems {
		if strings.HasPrefix(name, stem) {
			return i + 1
		}
	}
	return 0
}
