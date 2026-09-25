package domain

import (
	"fmt"
	"time"
)

// Date — календарная дата без времени и пояса (валидность документа задаётся датой).
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// ParseDate разбирает дату формата YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, invalid("date", "ожидается формат YYYY-MM-DD")
	}
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}, nil
}

// DateOf возвращает календарную дату момента в заданном поясе.
func DateOf(t time.Time, loc *time.Location) Date {
	t = t.In(loc)
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// String возвращает дату в формате YYYY-MM-DD.
func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day) }

// Format возвращает дату в пользовательском формате DD.MM.YYYY.
func (d Date) Format() string { return fmt.Sprintf("%02d.%02d.%04d", d.Day, int(d.Month), d.Year) }

// AddDays возвращает дату, сдвинутую на n дней.
func (d Date) AddDays(n int) Date {
	t := time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, n)
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// Before сообщает, что дата раньше другой.
func (d Date) Before(o Date) bool {
	if d.Year != o.Year {
		return d.Year < o.Year
	}
	if d.Month != o.Month {
		return d.Month < o.Month
	}
	return d.Day < o.Day
}

// AtLocalMinutes возвращает момент времени в поясе loc: дата + minutes от полуночи.
// Для несуществующего локального времени (переход на летнее время) time.Date
// нормализует значение к ближайшему существующему моменту.
func (d Date) AtLocalMinutes(minutes int, loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, minutes/60, minutes%60, 0, 0, loc)
}

// ToTime возвращает полночь даты в поясе loc.
func (d Date) ToTime(loc *time.Location) time.Time { return d.AtLocalMinutes(0, loc) }
