package domain

import "time"

// RecipientState — состояние диалога пользователя с ботом (D8-4).
type RecipientState string

const (
	RecipientUnknown     RecipientState = "unknown"
	RecipientActive      RecipientState = "active"
	RecipientMuted       RecipientState = "muted"
	RecipientStopped     RecipientState = "stopped"
	RecipientUnreachable RecipientState = "unreachable"
)

// Recipient — состояние получателя.
type Recipient struct {
	MaxUserID      int64
	State          RecipientState
	StateChangedAt time.Time
	LastEventTime  *time.Time
	LastDeliveryAt *time.Time
}

// StateAfterEvent возвращает состояние получателя после события MAX.
// Переходы D8-4 нормативны; для сочетаний, которых диаграмма не описывает,
// применяется эффект события из таблицы spec §7 с двумя уточнениями:
// остановленный диалог возвращается только событием bot_started, а сообщение
// пользователя не снимает отключение звука диалога.
func StateAfterEvent(current RecipientState, ev UpdateType) (RecipientState, bool) {
	switch ev {
	case UpdateBotStarted:
		return RecipientActive, true
	case UpdateBotStopped, UpdateDialogRemoved:
		return RecipientStopped, true
	case UpdateDialogMuted:
		if current == RecipientStopped {
			return current, true
		}
		return RecipientMuted, true
	case UpdateDialogUnmuted:
		if current == RecipientStopped {
			return current, true
		}
		return RecipientActive, true
	case UpdateMessageCreated:
		if current == RecipientStopped || current == RecipientMuted {
			return current, true
		}
		return RecipientActive, true
	}
	return current, false
}

// ApplyEvent применяет событие с учётом порядка: состояние меняется только
// событием не старее последнего учтённого (spec §7). Возвращает новое
// состояние получателя и признак того, что событие учтено.
func (r Recipient) ApplyEvent(ev UpdateType, eventTime time.Time) (Recipient, bool) {
	next, affects := StateAfterEvent(r.State, ev)
	if !affects {
		return r, false
	}
	if r.LastEventTime != nil && eventTime.Before(*r.LastEventTime) {
		return r, false
	}
	out := r
	t := eventTime
	out.LastEventTime = &t
	if next != r.State || r.StateChangedAt.IsZero() {
		out.State = next
		out.StateChangedAt = eventTime
	}
	return out, true
}

// MarkUnreachable фиксирует отказ MAX 403/404: получатель недоступен.
// Остановленный пользователем диалог остаётся остановленным.
func (r Recipient) MarkUnreachable(now time.Time) (Recipient, bool) {
	if r.State == RecipientStopped || r.State == RecipientUnreachable {
		return r, false
	}
	out := r
	out.State = RecipientUnreachable
	out.StateChangedAt = now
	return out, true
}
