package domain

import "time"

// Собственные тексты бота (spec §15). Тексты напоминаний и сообщений
// о присоединении участника формируют вызывающие сервисы.
const (
	TextWelcome   = "Здравствуйте! Я «Вовремя» — напоминаю о сроках лицензий, договоров и других документов бизнеса. Откройте приложение, чтобы добавить документы. Напоминания придут в этот чат."
	TextHelp      = "Что я умею: веду список документов с датами окончания; заранее напоминаю о сроках — по умолчанию за 30, 7 и 1 день; помогаю вести сроки вместе с сотрудниками. Настройки напоминаний — в приложении, раздел «Настройки»."
	TextHint      = "Я не читаю сообщения — все действия доступны в приложении."
	ButtonOpenApp = "Открыть приложение"

	// ReplyTTL — срок, после которого ответ бота теряет смысл (handoff §8).
	ReplyTTL = time.Hour
	// HintCooldown — подсказка на произвольный текст не чаще раза в 10 минут.
	HintCooldown = 10 * time.Minute
)

// ReplyEnqueueRequest формирует ответ бота на событие с ключом reply:<hex>.
func ReplyEnqueueRequest(r Reply, recipient int64, key DedupeKey, now time.Time) (EnqueueRequest, bool) {
	var kind Kind
	var text string
	switch r {
	case ReplyWelcome:
		kind, text = KindWelcome, TextWelcome
	case ReplyHelp:
		kind, text = KindHelp, TextHelp
	case ReplyHint:
		// Подсказка хранится с видом help (bot-service-v2 §1).
		kind, text = KindHelp, TextHint
	default:
		return EnqueueRequest{}, false
	}
	return EnqueueRequest{
		IdempotencyKey: "reply:" + key.Hex(),
		Message: Message{
			Kind:               kind,
			RecipientMaxUserID: recipient,
			Text:               text,
			Buttons:            []Button{{Text: ButtonOpenApp, Action: ActionOpenApp, Payload: ""}},
		},
		NotAfter: now.Add(ReplyTTL),
	}, true
}
