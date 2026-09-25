package domain

import "time"

// DeadlineStatus — состояние срока документа.
type DeadlineStatus string

// Значения состояния срока.
const (
	StatusExpired  DeadlineStatus = "expired"
	StatusExpiring DeadlineStatus = "expiring"
	StatusValid    DeadlineStatus = "valid"
	StatusNoExpiry DeadlineStatus = "no_expiry"
)

// ExpiringWindowDays — граница состояния expiring в днях.
const ExpiringWindowDays = 30

// Valid сообщает, входит ли значение в словарь состояний.
func (s DeadlineStatus) Valid() bool {
	switch s {
	case StatusExpired, StatusExpiring, StatusValid, StatusNoExpiry:
		return true
	default:
		return false
	}
}

// StatusOf вычисляет состояние срока: nil → no_expiry; раньше сегодня → expired;
// не позже сегодня+30 → expiring; иначе valid.
func StatusOf(validUntil *time.Time, today time.Time) DeadlineStatus {
	if validUntil == nil {
		return StatusNoExpiry
	}
	d := DaysBetween(today, *validUntil)
	switch {
	case d < 0:
		return StatusExpired
	case d <= ExpiringWindowDays:
		return StatusExpiring
	default:
		return StatusValid
	}
}

// DaysLeft возвращает число дней до окончания срока; nil для бессрочного.
func DaysLeft(validUntil *time.Time, today time.Time) *int {
	if validUntil == nil {
		return nil
	}
	d := DaysBetween(today, *validUntil)
	return &d
}

// DaysBetween — разница календарных дат в днях (to − from).
func DaysBetween(from, to time.Time) int {
	a := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}

// ParseLocalDate разбирает дату формата YYYY-MM-DD.
func ParseLocalDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, time.UTC)
}

// FormatLocalDate печатает дату в формате YYYY-MM-DD.
func FormatLocalDate(t time.Time) string { return t.Format("2006-01-02") }

// TodayIn возвращает текущую календарную дату в указанном поясе.
func TodayIn(now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}
