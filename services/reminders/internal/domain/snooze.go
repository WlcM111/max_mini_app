package domain

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// Отложенный повтор напоминания («Напомнить через неделю», ADR-036).
const (
	// SnoozeDays — на сколько дней откладывается напоминание.
	SnoozeDays = 7
	// SnoozeTextPrefix предваряет текст повтора: получатель видит, что сам просил напомнить.
	SnoozeTextPrefix = "Напоминаю ещё раз. "
	snoozeKeyMarker  = ":snz:"
	maxDaysBefore    = 365
)

// SnoozeOutcome — исход запроса отложить напоминание.
type SnoozeOutcome int

// Исходы запроса отложить напоминание.
const (
	SnoozeSnoozed SnoozeOutcome = iota + 1
	SnoozeAlreadySnoozed
	SnoozeTooLate
	SnoozeStale
	SnoozeNotFound
	SnoozeForbidden
)

// ErrInvalidKey — ключ напоминания не соответствует формату.
var ErrInvalidKey = errors.New("reminders: неверный ключ напоминания")

// ParseIdempotencyKey разбирает ключ rem:<период>:<аккаунт>:<дни>[:snz:<день>].
func ParseIdempotencyKey(key string) (PlanKey, error) {
	base, snooze, hasSnooze := strings.Cut(key, snoozeKeyMarker)
	parts := strings.Split(base, ":")
	if len(parts) != 4 || parts[0] != "rem" || !uuidV4.MatchString(parts[1]) || !uuidV4.MatchString(parts[2]) {
		return PlanKey{}, ErrInvalidKey
	}
	days, err := strconv.Atoi(parts[3])
	if err != nil || days < 0 || days > maxDaysBefore {
		return PlanKey{}, ErrInvalidKey
	}
	k := PlanKey{PeriodID: parts[1], AccountID: parts[2], DaysBefore: days}
	if hasSnooze {
		day, err := strconv.Atoi(snooze)
		if err != nil || day <= 0 {
			return PlanKey{}, ErrInvalidKey
		}
		k.SnoozeDay = day
	}
	return k, nil
}

// SnoozeDay возвращает номер дня нажатия (сутки UTC от эпохи): повторное нажатие
// в тот же день не создаёт второго повтора.
func SnoozeDay(now time.Time) int { return int(now.UTC().Unix() / 86400) }

// SnoozeDue рассчитывает повтор: через SnoozeDays дней во время напоминаний участника
// в поясе организации. daysLeft — сколько дней останется до окончания срока в день повтора.
// ok = false, если повтор пришёл бы в день окончания срока или позже.
func SnoozeDue(now time.Time, until Date, notifyMinutes int, loc *time.Location) (due time.Time, daysLeft int, ok bool) {
	dueDate := DateOf(now, loc).AddDays(SnoozeDays)
	daysLeft = int(until.ToTime(time.UTC).Sub(dueDate.ToTime(time.UTC)) / (24 * time.Hour))
	if daysLeft < 1 || daysLeft > maxDaysBefore {
		return time.Time{}, daysLeft, false
	}
	return dueDate.AtLocalMinutes(notifyMinutes, loc).UTC(), daysLeft, true
}
