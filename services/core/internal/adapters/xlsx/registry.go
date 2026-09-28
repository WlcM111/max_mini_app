// Package xlsx собирает реестр документов в формате Office Open XML без сторонних библиотек.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"vovremya/services/core/internal/app"
	"vovremya/services/core/internal/domain"
)

const (
	styleText   = 0
	styleHeader = 1
	styleDate   = 2
)

var statusTitles = map[domain.DeadlineStatus]string{
	domain.StatusExpired:  "Просрочен",
	domain.StatusExpiring: "Скоро истекает",
	domain.StatusValid:    "В порядке",
	domain.StatusNoExpiry: "Бессрочный",
}

var columns = []struct {
	title string
	width int
}{
	{"Документ", 40}, {"Номер", 18}, {"Кем выдан", 30}, {"Ответственный", 20}, {"Действует с", 13},
	{"Действует до", 13}, {"Статус", 16}, {"Дней до окончания", 12}, {"Напоминания, дней", 16}, {"Заметки", 40},
}

// Серийные даты Excel отсчитываются от 30.12.1899.
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// Registry возвращает книгу .xlsx с листом «Реестр»: строка на каждый документ,
// статус считается на сегодняшнюю дату в часовом поясе организации.
func Registry(data app.CalendarData) ([]byte, error) {
	loc, err := time.LoadLocation(data.Organization.Timezone)
	if err != nil {
		loc = time.UTC
	}
	today := domain.TodayIn(data.GeneratedAt, loc)

	var rows strings.Builder
	rows.WriteString(`<row r="1">`)
	for i, c := range columns {
		rows.WriteString(cellString(i, 1, c.title, styleHeader))
	}
	rows.WriteString(`</row>`)
	for n, doc := range data.Documents {
		r := n + 2
		until := doc.CurrentPeriod.ValidUntil
		fmt.Fprintf(&rows, `<row r="%d">`, r)
		rows.WriteString(cellString(0, r, doc.Title, styleText))
		rows.WriteString(cellString(1, r, doc.Number, styleText))
		rows.WriteString(cellString(2, r, doc.Issuer, styleText))
		rows.WriteString(cellString(3, r, doc.ResponsibleLabel, styleText))
		rows.WriteString(cellDate(4, r, doc.CurrentPeriod.ValidFrom))
		rows.WriteString(cellDate(5, r, until))
		rows.WriteString(cellString(6, r, statusTitles[domain.StatusOf(until, today)], styleText))
		if days := domain.DaysLeft(until, today); days != nil {
			fmt.Fprintf(&rows, `<c r="%s"><v>%d</v></c>`, ref(7, r), *days)
		}
		rows.WriteString(cellString(8, r, joinInts(data.Offsets[doc.PublicID]), styleText))
		rows.WriteString(cellString(9, r, doc.Notes, styleText))
		rows.WriteString(`</row>`)
	}

	var cols strings.Builder
	for i, c := range columns {
		fmt.Fprintf(&cols, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, c.width)
	}
	last := ref(len(columns)-1, len(data.Documents)+1)
	sheet := xmlHeader + `<worksheet xmlns="` + nsMain + `">` +
		`<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>` +
		`<cols>` + cols.String() + `</cols>` +
		`<sheetData>` + rows.String() + `</sheetData>` +
		`<autoFilter ref="A1:` + last + `"/>` +
		`</worksheet>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := []struct{ name, body string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rootRels},
		{"xl/workbook.xml", workbook},
		{"xl/_rels/workbook.xml.rels", workbookRels},
		{"xl/styles.xml", styles},
		{"xl/worksheets/sheet1.xml", sheet},
	}
	for _, p := range parts {
		f, err := zw.Create(p.name)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte(p.body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ref — адрес ячейки; в реестре меньше 26 столбцов, поэтому буква одна.
func ref(col, row int) string { return string(rune('A'+col)) + strconv.Itoa(row) }

func cellString(col, row int, value string, style int) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf(`<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref(col, row), style, escape(value))
}

func cellDate(col, row int, t *time.Time) string {
	if t == nil {
		return ""
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	serial := int(day.Sub(excelEpoch).Hours() / 24)
	return fmt.Sprintf(`<c r="%s" s="%d"><v>%d</v></c>`, ref(col, row), styleDate, serial)
}

func joinInts(values []int) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ", ")
}

func escape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

const (
	xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
	nsMain    = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"

	contentTypes = xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
		`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` +
		`</Types>`

	rootRels = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`</Relationships>`

	workbook = xmlHeader + `<workbook xmlns="` + nsMain + `" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="Реестр" sheetId="1" r:id="rId1"/></sheets></workbook>`

	workbookRels = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
		`</Relationships>`

	// Стили: 0 — обычный текст, 1 — шапка, 2 — дата дд.мм.гггг.
	styles = xmlHeader + `<styleSheet xmlns="` + nsMain + `">` +
		`<numFmts count="1"><numFmt numFmtId="164" formatCode="dd.mm.yyyy"/></numFmts>` +
		`<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>` +
		`<fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>` +
		`<fill><patternFill patternType="solid"><fgColor rgb="FFE8ECFF"/><bgColor indexed="64"/></patternFill></fill></fills>` +
		`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
		`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
		`<cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
		`<xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1"/>` +
		`<xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/></cellXfs>` +
		`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
		`</styleSheet>`
)
