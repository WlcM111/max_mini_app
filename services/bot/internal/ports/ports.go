// Package ports объявляет интерфейсы, которые нужны сценариям bot-service:
// хранилище, клиент MAX Bot API, ограничитель скорости, часы.
package ports

import (
	"context"
	"time"

	"vovremya/services/bot/internal/domain"
)

// Clock — источник текущего времени.
type Clock interface{ Now() time.Time }

// Random — источник случайных чисел [0; 1) для выдержки повторов.
type Random interface{ Float64() float64 }

// TxManager выполняет функцию в транзакции; вложенные вызовы присоединяются.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// MessageRepo — очередь исходящих сообщений (bot.outbound_messages, outbound_buttons).
type MessageRepo interface {
	// Insert ставит сообщение в очередь. inserted=false — ключ уже существует.
	Insert(ctx context.Context, n domain.Notification) (stored domain.Notification, inserted bool, err error)
	// GetByKey читает сообщение по ключу идемпотентности (domain.ErrNotFound).
	GetByKey(ctx context.Context, key string) (domain.Notification, error)
	// CountActive — число сообщений в незавершённых состояниях.
	CountActive(ctx context.Context) (int64, error)
	// ClaimReady захватывает готовые к отправке сообщения с арендой
	// (FOR UPDATE SKIP LOCKED); увеличивает счётчик попыток.
	ClaimReady(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Notification, error)
	// MarkSent фиксирует успешную отправку при действующей аренде.
	MarkSent(ctx context.Context, id int64, lease time.Time, maxMessageID string, now time.Time) (bool, error)
	// Finish переводит сообщение в failed, expired или retry_wait при действующей аренде.
	Finish(ctx context.Context, id int64, lease time.Time, tr domain.Transition, now time.Time) (bool, error)
	// Release возвращает сообщение в очередь без учёта попытки.
	Release(ctx context.Context, id int64, lease time.Time, nextAttemptAt, now time.Time) (bool, error)
	// ReapExpiredLeases возвращает в retry_wait сообщения с истёкшей арендой.
	ReapExpiredLeases(ctx context.Context, now time.Time) (int64, error)
	// HasRecentKind сообщает, ставилось ли получателю сообщение вида kind после since.
	HasRecentKind(ctx context.Context, recipient int64, kind domain.Kind, since time.Time) (bool, error)
	// DeleteFinalizedBefore удаляет sent/failed/expired старше before.
	DeleteFinalizedBefore(ctx context.Context, before time.Time) (int64, error)
}

// RecipientRepo — состояния диалогов (bot.recipients).
type RecipientRepo interface {
	// Get читает получателя; found=false — строки нет (состояние UNKNOWN).
	Get(ctx context.Context, maxUserID int64) (r domain.Recipient, found bool, err error)
	// LockOrCreate создаёт строку в состоянии unknown при отсутствии и блокирует её
	// до конца транзакции. Вызывается только внутри транзакции.
	LockOrCreate(ctx context.Context, maxUserID int64, now time.Time) (domain.Recipient, error)
	// Save записывает состояние заблокированного получателя.
	Save(ctx context.Context, r domain.Recipient, now time.Time) error
	// MarkDelivered фиксирует момент успешной доставки.
	MarkDelivered(ctx context.Context, maxUserID int64, at time.Time) error
	// DeleteStoppedInactive удаляет остановленные диалоги без событий после before.
	DeleteStoppedInactive(ctx context.Context, before time.Time) (int64, error)
}

// InboundRepo — журнал дедупликации событий webhook (bot.inbound_updates).
type InboundRepo interface {
	// Insert записывает событие; inserted=false — событие уже обрабатывалось.
	Insert(ctx context.Context, rec domain.InboundRecord) (inserted bool, err error)
	// DeleteBefore удаляет записи, полученные раньше before.
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

// SendResult — результат успешной отправки.
type SendResult struct {
	MessageID string // body.mid из ответа MAX, если есть
}

// SendError — неуспешная отправка, уже классифицированная адаптером (spec §8).
type SendError struct {
	Failure    domain.SendFailure
	HTTPStatus int
	Err        error
}

func (e *SendError) Error() string {
	if e.Err != nil {
		return e.Failure.Code + ": " + e.Err.Error()
	}
	return e.Failure.Code
}

// Unwrap возвращает исходную ошибку транспорта.
func (e *SendError) Unwrap() error { return e.Err }

// BotCommand — команда меню бота.
type BotCommand struct {
	Name        string
	Description string
}

// MaxClient — вызовы MAX Bot API. Формат JSON и HTTP — забота адаптера.
type MaxClient interface {
	// SendMessage отправляет сообщение; ошибка — *SendError.
	// Профиль нужен для построения диплинков кнопок open_app.
	SendMessage(ctx context.Context, msg domain.Message, profile domain.Profile) (SendResult, error)
	// GetMe возвращает профиль бота.
	GetMe(ctx context.Context) (domain.Profile, error)
	// ListSubscriptions возвращает URL активных подписок webhook.
	ListSubscriptions(ctx context.Context) ([]string, error)
	// Subscribe создаёт подписку webhook.
	Subscribe(ctx context.Context, url string, updateTypes []string, secret string) error
	// SetCommands задаёт команды меню бота.
	SetCommands(ctx context.Context, cmds []BotCommand) error
}

// RateLimiter ограничивает скорость вызовов MAX: глобально и на получателя.
type RateLimiter interface {
	// Wait ожидает разрешения на отправку получателю.
	Wait(ctx context.Context, recipient int64) error
	// Penalize снижает глобальную скорость после ответа 429.
	Penalize()
}
