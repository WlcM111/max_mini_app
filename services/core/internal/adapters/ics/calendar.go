// Package ics формирует файл календаря сроков документов (RFC 5545).
package ics

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"vovremya/services/core/internal/app"
)

// Render формирует содержимое файла .ics по данным организации.
func Render(data app.CalendarData) string {
	var b strings.Builder
	write := func(line string) { b.WriteString(foldLine(line) + "\r\n") }
	write("BEGIN:VCALENDAR")
	write("VERSION:2.0")
	write("PRODID:-//Vovremya//RU")
	write("CALSCALE:GREGORIAN")
	stamp := data.GeneratedAt.UTC().Format("20060102T150405Z")
	for _, doc := range data.Documents {
		if doc.CurrentPeriod.ValidUntil == nil {
			continue
		}
		write("BEGIN:VEVENT")
		write("UID:" + doc.CurrentPeriod.PublicID + "@vovremya")
		write("DTSTAMP:" + stamp)
		write("DTSTART;VALUE=DATE:" + doc.CurrentPeriod.ValidUntil.Format("20060102"))
		write("SUMMARY:" + escape("Срок: "+doc.Title))
		if doc.ResponsibleLabel != "" {
			write("DESCRIPTION:" + escape("Ответственный: "+doc.ResponsibleLabel))
		}
		for _, days := range data.Offsets[doc.PublicID] {
			write("BEGIN:VALARM")
			write("ACTION:DISPLAY")
			write(fmt.Sprintf("TRIGGER:-P%dD", days))
			write("DESCRIPTION:" + escape("Скоро заканчивается срок: "+doc.Title))
			write("END:VALARM")
		}
		write("END:VEVENT")
	}
	write("END:VCALENDAR")
	return b.String()
}

// escape экранирует служебные символы текста (RFC 5545 §3.3.11). Переводы строк
// любого вида становятся \n, прочие управляющие символы отбрасываются: иначе одиночный
// CR разорвал бы строку свойства и исказил файл (BUG-004).
func escape(s string) string {
	s = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(s)
	s = strings.Map(func(r rune) rune {
		if r != '\n' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	replacer := strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, "\n", `\n`)
	return replacer.Replace(s)
}

// foldLine сворачивает строку длиннее 75 октетов (RFC 5545 §3.1): продолжение начинается
// с пробела и занимает не более 74 октетов содержимого; многобайтовые символы UTF-8
// не разрываются (BUG-005).
func foldLine(line string) string {
	const limit = 75
	if len(line) <= limit {
		return line
	}
	var b strings.Builder
	max := limit
	for len(line) > max {
		cut := max
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		b.WriteString(line[:cut])
		b.WriteString("\r\n ")
		line = line[cut:]
		max = limit - 1
	}
	b.WriteString(line)
	return b.String()
}
