package maxapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// stubBufferSize — размер кольцевого буфера сообщений режима stub (spec §12).
const stubBufferSize = 500

// StubMessage — запись буфера локального режима.
type StubMessage struct {
	Recipient int64            `json:"recipient"`
	Text      string           `json:"text"`
	Buttons   []keyboardButton `json:"buttons"`
	Notify    bool             `json:"notify"`
	MessageID string           `json:"mid"`
	CreatedAt time.Time        `json:"created_at"`
}

// Stub — локальная реализация MaxClient: сообщения не уходят в MAX, а
// складываются в кольцевой буфер и доступны на GET /debug/stub/messages.
// Режим запрещён при APP_ENV=prod (ADR-031).
type Stub struct {
	username string
	kind     ButtonKind
	log      *slog.Logger
	now      func() time.Time

	mu       sync.Mutex
	messages []StubMessage
	seq      int64
}

var _ ports.MaxClient = (*Stub)(nil)

// NewStub создаёт локальную реализацию клиента.
func NewStub(username string, kind ButtonKind, log *slog.Logger, now func() time.Time) *Stub {
	return &Stub{username: username, kind: kind, log: log, now: now}
}

// SendMessage сохраняет сообщение в буфер и возвращает идентификатор stub-<n>.
func (s *Stub) SendMessage(_ context.Context, msg domain.Message, profile domain.Profile) (ports.SendResult, error) {
	if profile.Username == "" {
		profile = domain.Profile{Username: s.username, DisplayName: s.username}
	}
	body, err := renderSendBody(msg, profile, s.kind)
	if err != nil {
		return ports.SendResult{}, &ports.SendError{Failure: domain.SendFailure{Code: domain.CodeMax4xx}, Err: err}
	}
	var buttons []keyboardButton
	for _, a := range body.Attachments {
		for _, row := range a.Payload.Buttons {
			buttons = append(buttons, row...)
		}
	}
	s.mu.Lock()
	s.seq++
	mid := fmt.Sprintf("stub-%d", s.seq)
	s.messages = append(s.messages, StubMessage{
		Recipient: msg.RecipientMaxUserID,
		Text:      body.Text,
		Buttons:   buttons,
		Notify:    body.Notify,
		MessageID: mid,
		CreatedAt: s.now().UTC(),
	})
	if len(s.messages) > stubBufferSize {
		s.messages = append([]StubMessage(nil), s.messages[len(s.messages)-stubBufferSize:]...)
	}
	s.mu.Unlock()
	// Текст сообщения не пишется в журнал: он содержит данные пользователя.
	s.log.Info("stub message stored", slog.Int64("recipient", msg.RecipientMaxUserID),
		slog.String("kind", string(msg.Kind)), slog.Int("buttons", len(buttons)), slog.String("mid", mid))
	return ports.SendResult{MessageID: mid}, nil
}

// GetMe возвращает профиль локального бота (BOT_STUB_USERNAME).
func (s *Stub) GetMe(context.Context) (domain.Profile, error) {
	return domain.Profile{Username: s.username, DisplayName: s.username}, nil
}

// ListSubscriptions в режиме stub не поддерживается: подписка не оформляется.
func (s *Stub) ListSubscriptions(context.Context) ([]string, error) {
	return nil, errors.New("подписка webhook не используется в режиме stub")
}

// Subscribe в режиме stub не поддерживается.
func (s *Stub) Subscribe(context.Context, string, []string, string) error {
	return errors.New("подписка webhook не используется в режиме stub")
}

// SetCommands в режиме stub не обращается к MAX.
func (s *Stub) SetCommands(context.Context, []ports.BotCommand) error {
	s.log.Info("stub mode: bot commands are not sent to MAX")
	return nil
}

// Messages возвращает копию буфера (старые записи — первыми).
func (s *Stub) Messages() []StubMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]StubMessage(nil), s.messages...)
}

// Handler отдаёт буфер сообщений: GET /debug/stub/messages.
func (s *Stub) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		msgs := s.Messages()
		if msgs == nil {
			msgs = []StubMessage{}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(msgs); err != nil {
			s.log.Warn("stub buffer encode failed", slog.Any("error", err))
		}
	})
}
