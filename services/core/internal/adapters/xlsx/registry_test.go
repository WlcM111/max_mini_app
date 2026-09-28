package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"vovremya/services/core/internal/app"
	"vovremya/services/core/internal/domain"
)

func TestRegistry(t *testing.T) {
	until := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	data := app.CalendarData{
		Organization: domain.Organization{PublicID: "3f2a9c1e-5b7d-4e8a-9c21-7d4e5f6a8b90", Name: "Кафе «Дружба»", Timezone: "Europe/Moscow"},
		Documents: []domain.Document{{PublicID: "d1", Title: "Лицензия <алкоголь> & пиво",
			CurrentPeriod: domain.Period{ValidUntil: &until}}},
		Offsets:     map[string][]int{"d1": {30, 7}},
		GeneratedAt: time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC),
	}
	body, err := Registry(data)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	var sheet string
	for _, f := range zr.File {
		if f.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(rc)
		_ = rc.Close()
		sheet = string(raw)
	}
	for _, want := range []string{"Лицензия &lt;алкоголь&gt; &amp; пиво", "Скоро истекает", "<v>5</v>", "30, 7", `autoFilter ref="A1:J2"`} {
		if !strings.Contains(sheet, want) {
			t.Errorf("в листе нет %q", want)
		}
	}
}
