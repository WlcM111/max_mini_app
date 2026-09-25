// Package ics формирует файл календаря сроков документов (RFC 5545).
package ics

import (
	"fmt"
	"strings"
	"time"

	"vovremya/services/core/internal/app"
	"vovremya/services/core/internal/domain"
)

// Render формирует содержимое файла .ics по данным организации.
func Render(data app.CalendarData) string {
	var b strings.Builder
	write := func(line string) { b.WriteString(line + "\r\n") }
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

// escape экранирует служебные символы текста (RFC 5545 §3.3.11).
func escape(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, "\n", `\n`)
	return replacer.Replace(s)
}

var _ = domain.Document{}
var _ = time.Time{}
