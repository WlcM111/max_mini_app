package domain

import (
	"sort"
	"time"
)

// PlanKey — естественный ключ напоминания: период документа, получатель, отступ.
// Совпадает с ключом идемпотентности передачи в bot-service.
type PlanKey struct {
	PeriodID   string
	AccountID  string
	DaysBefore int
	// SnoozeDay — день нажатия «Напомнить через неделю» (сутки UTC от эпохи);
	// 0 — обычное напоминание плана, иначе — отложенный повтор (ADR-036).
	SnoozeDay int
}

// PlanItem — элемент желаемого плана напоминаний.
type PlanItem struct {
	Key   PlanKey
	DueAt time.Time // момент отправки в UTC
}

// PlanInput — исходные данные для построения плана по одному документу.
type PlanInput struct {
	Organization Organization
	Document     Document
	Members      []Member
	Now          time.Time
	// Grace — насколько в прошлом допустимо запланировать напоминание.
	// Более старые моменты не планируются (их отправка уже бессмысленна).
	Grace time.Duration
}

// BuildPlan вычисляет желаемое множество напоминаний документа.
// Чистая функция: результат зависит только от входных данных.
//
// Правила (исходное ТЗ, ADR-010, перенесены без изменений):
//   - напоминания строятся только для документа с текущим периодом и датой окончания;
//   - получатели — участники организации с включёнными уведомлениями и аккаунтом MAX;
//   - момент = (дата окончания − отступ) в локальном времени участника, приведённый к UTC;
//   - моменты старше now − grace не планируются;
//   - удалённые документ, организация или участие не дают напоминаний.
func BuildPlan(in PlanInput) ([]PlanItem, error) {
	if in.Organization.Deleted || !in.Document.Plannable() {
		return nil, nil
	}
	if in.Document.OrganizationID != in.Organization.ID {
		return nil, invalid("organization_id", "документ принадлежит другой организации")
	}
	loc, err := in.Organization.Location()
	if err != nil {
		return nil, err
	}
	until := *in.Document.Period.ValidUntil
	earliest := in.Now.Add(-in.Grace)

	offsets := append([]int(nil), in.Document.OffsetsDays...)
	sort.Sort(sort.Reverse(sort.IntSlice(offsets)))

	items := make([]PlanItem, 0, len(offsets)*len(in.Members))
	for _, m := range in.Members {
		if !m.Receives() || m.OrganizationID != in.Organization.ID {
			continue
		}
		for _, days := range offsets {
			dueAt := until.AddDays(-days).AtLocalMinutes(m.NotifyLocalMinutes, loc).UTC()
			if dueAt.Before(earliest) {
				continue
			}
			items = append(items, PlanItem{
				Key:   PlanKey{PeriodID: in.Document.Period.ID, AccountID: m.AccountID, DaysBefore: days},
				DueAt: dueAt,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].DueAt.Equal(items[j].DueAt) {
			return items[i].DueAt.Before(items[j].DueAt)
		}
		if items[i].Key.AccountID != items[j].Key.AccountID {
			return items[i].Key.AccountID < items[j].Key.AccountID
		}
		return items[i].Key.DaysBefore > items[j].Key.DaysBefore
	})
	return items, nil
}
