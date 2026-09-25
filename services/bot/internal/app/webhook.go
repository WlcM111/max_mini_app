package app

import (
	"context"
	"log/slog"
	"unicode/utf8"

	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// InboundUpdate — событие webhook после разбора адаптером.
type InboundUpdate struct {
	Key     domain.DedupeKey
	RawType string // update_type как получен; пусто — поле отсутствует
	Parsed  bool   // тип и время события распознаны
	FromBot bool   // отправитель сообщения — бот
	Update  domain.Update
}

// WebhookResult — итог обработки события.
type WebhookResult struct {
	Duplicate bool
	Outcome   domain.Outcome
	Reply     domain.Reply
}

// WebhookService обрабатывает события MAX: дедупликация, состояние получателя,
// ответы бота — одной транзакцией (spec §7).
type WebhookService struct {
	tx         ports.TxManager
	inbound    ports.InboundRepo
	recipients ports.RecipientRepo
	messages   ports.MessageRepo
	profile    *ProfileStore
	clock      ports.Clock
	log        *slog.Logger
	metrics    *Metrics
}

// NewWebhookService создаёт обработчик событий.
func NewWebhookService(tx ports.TxManager, inbound ports.InboundRepo, recipients ports.RecipientRepo,
	messages ports.MessageRepo, profile *ProfileStore, clock ports.Clock, log *slog.Logger, m *Metrics) *WebhookService {
	return &WebhookService{tx: tx, inbound: inbound, recipients: recipients, messages: messages,
		profile: profile, clock: clock, log: log, metrics: m}
}

// Handle обрабатывает событие. Повтор того же тела распознаётся по ключу
// дедупликации и не даёт повторных эффектов.
func (s *WebhookService) Handle(ctx context.Context, in InboundUpdate) (WebhookResult, error) {
	now := s.clock.Now()
	u := in.Update
	outcome := domain.OutcomeIgnored
	if in.Parsed && !in.FromBot && u.Type.AffectsRecipient() && u.MaxUserID > 0 && !s.isOwnID(u.MaxUserID) {
		outcome = domain.OutcomeApplied
	}
	eventTime := u.EventTime
	if !in.Parsed || eventTime.IsZero() {
		eventTime = now
	}
	rec := domain.InboundRecord{Key: in.Key, UpdateType: storageType(in.RawType), EventTime: eventTime,
		MaxUserID: u.MaxUserID, Outcome: outcome, ReceivedAt: now}

	var res WebhookResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		inserted, err := s.inbound.Insert(ctx, rec)
		if err != nil {
			return err
		}
		if !inserted {
			res.Duplicate = true
			return nil
		}
		res.Outcome = outcome
		if outcome != domain.OutcomeApplied {
			return nil
		}
		r, err := s.recipients.LockOrCreate(ctx, u.MaxUserID, now)
		if err != nil {
			return err
		}
		if next, accepted := r.ApplyEvent(u.Type, eventTime); accepted {
			if err := s.recipients.Save(ctx, next, now); err != nil {
				return err
			}
		}
		reply := u.Reply()
		if reply == domain.ReplyHint {
			// Строка получателя заблокирована: параллельные события того же
			// пользователя проверяют интервал подсказки последовательно.
			recent, err := s.messages.HasRecentKind(ctx, u.MaxUserID, domain.KindHelp, now.Add(-domain.HintCooldown))
			if err != nil {
				return err
			}
			if recent {
				reply = domain.ReplyNone
			}
		}
		req, ok := domain.ReplyEnqueueRequest(reply, u.MaxUserID, in.Key, now)
		if !ok {
			return nil
		}
		if _, err := enqueueTx(ctx, s.messages, req, domain.ContentHash(req), now); err != nil {
			return err
		}
		res.Reply = reply
		return nil
	})
	if err != nil {
		return WebhookResult{}, err
	}
	label := string(res.Outcome)
	if res.Duplicate {
		label = "duplicate"
	}
	s.metrics.WebhookUpdates.WithLabelValues(metricType(u.Type, in.Parsed), label).Inc()
	return res, nil
}

// isOwnID защищает от реакции бота на собственные сообщения.
func (s *WebhookService) isOwnID(id int64) bool {
	p, ok := s.profile.Get()
	return ok && p.UserID != 0 && p.UserID == id
}

// storageType приводит тип события к ограничению столбца (1..64 символа).
func storageType(raw string) string {
	if raw == "" || !utf8.ValidString(raw) {
		return "unknown"
	}
	if utf8.RuneCountInString(raw) > 64 {
		return string([]rune(raw)[:64])
	}
	return raw
}

// metricType ограничивает кардинальность метки типа события.
func metricType(t domain.UpdateType, parsed bool) string {
	if parsed && t.AffectsRecipient() {
		return string(t)
	}
	return "other"
}
