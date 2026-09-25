package domain

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Kind — вид сообщения бота (столбец outbound_messages.kind).
type Kind string

const (
	KindReminder     Kind = "reminder"
	KindMemberJoined Kind = "member_joined"
	KindWelcome      Kind = "welcome"
	KindHelp         Kind = "help"
)

func (k Kind) valid() bool {
	switch k {
	case KindReminder, KindMemberJoined, KindWelcome, KindHelp:
		return true
	}
	return false
}

// Status — состояние сообщения в очереди доставки (D8-2).
type Status string

const (
	StatusQueued    Status = "queued"
	StatusSending   Status = "sending"
	StatusSent      Status = "sent"
	StatusRetryWait Status = "retry_wait"
	StatusFailed    Status = "failed"
	StatusExpired   Status = "expired"
)

// Final сообщает, что состояние терминально.
func (s Status) Final() bool { return s == StatusSent || s == StatusFailed || s == StatusExpired }

// ButtonAction — действие кнопки под сообщением.
type ButtonAction string

const (
	ActionOpenApp ButtonAction = "open_app"
	ActionURL     ButtonAction = "url"
)

// Button — кнопка под сообщением; каждая кнопка занимает отдельный ряд.
type Button struct {
	Text    string
	Action  ButtonAction
	Payload string // для ActionOpenApp: payload диплинка мини-приложения
	URL     string // для ActionURL: внешняя ссылка https://
}

// Ограничения контракта MessagingService и MAX (F-05, F-49, F-51).
const (
	MaxTextRunes       = 4000
	MaxButtons         = 3
	MaxButtonTextRunes = 64
	MaxURLRunes        = 2048
	MaxNotAfterHorizon = 7 * 24 * time.Hour
)

var (
	idempotencyKeyRe = regexp.MustCompile(`^[a-z0-9:_-]{8,200}$`)
	payloadRe        = regexp.MustCompile(`^[A-Za-z0-9_-]{0,512}$`)
)

// ValidateIdempotencyKey проверяет формат ключа идемпотентности.
func ValidateIdempotencyKey(key string) error {
	if !idempotencyKeyRe.MatchString(key) {
		return invalid("idempotency_key", "ожидается ^[a-z0-9:_-]{8,200}$")
	}
	return nil
}

func validText(field, s string, minRunes, maxRunes int) error {
	if !utf8.ValidString(s) {
		return invalid(field, "недопустимая кодировка UTF-8")
	}
	if strings.ContainsRune(s, 0) {
		return invalid(field, "недопустимый символ NUL")
	}
	if n := utf8.RuneCountInString(s); n < minRunes || n > maxRunes {
		return invalid(field, fmt.Sprintf("длина %d..%d символов", minRunes, maxRunes))
	}
	return nil
}

func (b Button) validate(field string) error {
	if err := validText(field+".text", b.Text, 1, MaxButtonTextRunes); err != nil {
		return err
	}
	switch b.Action {
	case ActionOpenApp:
		if b.URL != "" {
			return invalid(field, "допустимо только одно действие: open_app_payload или url")
		}
		if !payloadRe.MatchString(b.Payload) {
			return invalid(field+".open_app_payload", "ожидается ^[A-Za-z0-9_-]{0,512}$")
		}
	case ActionURL:
		if b.Payload != "" {
			return invalid(field, "допустимо только одно действие: open_app_payload или url")
		}
		if !strings.HasPrefix(b.URL, "https://") || utf8.RuneCountInString(b.URL) > MaxURLRunes {
			return invalid(field+".url", "ожидается https:// не длиннее 2048 символов")
		}
		if err := validText(field+".url", b.URL, 9, MaxURLRunes); err != nil {
			return err
		}
	default:
		return invalid(field, "требуется open_app_payload или url")
	}
	return nil
}

// Message — содержимое сообщения бота.
type Message struct {
	Kind               Kind
	RecipientMaxUserID int64
	Text               string
	Silent             bool
	Buttons            []Button
}

// HasOpenAppButtons сообщает, что для отправки нужен загруженный профиль бота:
// диплинк мини-приложения строится из ника бота (spec §8, §10).
func (m Message) HasOpenAppButtons() bool {
	for _, b := range m.Buttons {
		if b.Action == ActionOpenApp {
			return true
		}
	}
	return false
}

// EnqueueRequest — запрос постановки сообщения в очередь.
type EnqueueRequest struct {
	IdempotencyKey string
	Message
	NotAfter time.Time
}

// Validate проверяет запрос по правилам grpc-contract §1.
func (r EnqueueRequest) Validate(now time.Time) error {
	if err := ValidateIdempotencyKey(r.IdempotencyKey); err != nil {
		return err
	}
	if !r.Kind.valid() {
		return invalid("kind", "обязательное поле")
	}
	if r.RecipientMaxUserID <= 0 {
		return invalid("recipient_max_user_id", "ожидается положительное число")
	}
	if err := validText("text", r.Text, 1, MaxTextRunes); err != nil {
		return err
	}
	if len(r.Buttons) > MaxButtons {
		return invalid("buttons", "не более трёх кнопок")
	}
	for i, b := range r.Buttons {
		if err := b.validate(fmt.Sprintf("buttons[%d]", i)); err != nil {
			return err
		}
	}
	if r.NotAfter.IsZero() {
		return invalid("not_after", "обязательное поле")
	}
	if !r.NotAfter.After(now) || r.NotAfter.After(now.Add(MaxNotAfterHorizon)) {
		return invalid("not_after", "ожидается момент в интервале (now; now + 7 суток]")
	}
	return nil
}

// Notification — сообщение в очереди доставки.
type Notification struct {
	ID             int64
	PublicID       string
	IdempotencyKey string
	RequestHash    []byte
	Message
	NotAfter      time.Time
	Status        Status
	Attempts      int
	NextAttemptAt time.Time
	LockedUntil   *time.Time
	MaxMessageID  string
	LastErrorCode string
	CreatedAt     time.Time
	SentAt        *time.Time
	UpdatedAt     time.Time
}

// ContentHash — SHA-256 канонического представления содержимого запроса без
// ключа идемпотентности. Используется для собственных ответов бота; для
// запросов gRPC хеш вычисляется по контракту из protobuf (grpc-contract §1).
func ContentHash(r EnqueueRequest) []byte {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x1f%d\x1f%t\x1f%d\x1f%s", r.Kind, r.RecipientMaxUserID, r.Silent,
		r.NotAfter.UTC().UnixMicro(), r.Text)
	for _, b := range r.Buttons {
		fmt.Fprintf(h, "\x1e%s\x1f%s\x1f%s\x1f%s", b.Text, b.Action, b.Payload, b.URL)
	}
	return h.Sum(nil)
}
