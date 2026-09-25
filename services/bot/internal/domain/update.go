package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// UpdateType — тип события MAX (F-48).
type UpdateType string

const (
	UpdateBotStarted     UpdateType = "bot_started"
	UpdateBotStopped     UpdateType = "bot_stopped"
	UpdateDialogRemoved  UpdateType = "dialog_removed"
	UpdateDialogMuted    UpdateType = "dialog_muted"
	UpdateDialogUnmuted  UpdateType = "dialog_unmuted"
	UpdateMessageCreated UpdateType = "message_created"
)

// SubscribedUpdateTypes — события, на которые оформляется подписка (spec §7).
var SubscribedUpdateTypes = []string{
	string(UpdateBotStarted), string(UpdateBotStopped), string(UpdateDialogRemoved),
	string(UpdateDialogMuted), string(UpdateDialogUnmuted), string(UpdateMessageCreated),
}

// Update — событие MAX в терминах бота. Текст сообщения используется только
// для распознавания команды и не сохраняется.
type Update struct {
	Type      UpdateType
	EventTime time.Time
	MaxUserID int64
	Text      string
}

// DedupeKey — ключ дедупликации события: SHA-256 тела запроса webhook.
type DedupeKey [32]byte

// NewDedupeKey вычисляет ключ по телу запроса.
func NewDedupeKey(body []byte) DedupeKey { return DedupeKey(sha256.Sum256(body)) }

// Hex возвращает ключ в шестнадцатеричном виде.
func (k DedupeKey) Hex() string { return hex.EncodeToString(k[:]) }

// Outcome — результат обработки события (столбец inbound_updates.outcome).
type Outcome string

const (
	OutcomeApplied Outcome = "applied"
	OutcomeIgnored Outcome = "ignored"
)

// Reply — ответ бота на событие.
type Reply int

const (
	ReplyNone Reply = iota
	ReplyWelcome
	ReplyHelp
	ReplyHint
)

// AffectsRecipient сообщает, влияет ли тип события на состояние получателя.
func (t UpdateType) AffectsRecipient() bool {
	_, affects := StateAfterEvent(RecipientUnknown, t)
	return affects
}

// Reply определяет ответ бота (spec §7): приветствие на bot_started и /start,
// справка на /help, подсказка на прочие сообщения.
func (u Update) Reply() Reply {
	switch u.Type {
	case UpdateBotStarted:
		return ReplyWelcome
	case UpdateMessageCreated:
		switch command(u.Text) {
		case "/start":
			return ReplyWelcome
		case "/help":
			return ReplyHelp
		}
		return ReplyHint
	}
	return ReplyNone
}

// command выделяет команду: первое слово текста, начинающееся с «/».
func command(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return ""
	}
	return strings.ToLower(fields[0])
}

// InboundRecord — запись журнала дедупликации webhook (без текста сообщения).
type InboundRecord struct {
	Key        DedupeKey
	UpdateType string
	EventTime  time.Time
	MaxUserID  int64 // 0 — идентификатор отсутствует
	Outcome    Outcome
	ReceivedAt time.Time
}
