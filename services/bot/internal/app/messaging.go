package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// QueueDepth — кэш глубины очереди для backpressure (обновляется периодически).
type QueueDepth struct{ value atomic.Int64 }

// Value возвращает последнее известное значение.
func (q *QueueDepth) Value() int64 { return q.value.Load() }

// Set сохраняет значение.
func (q *QueueDepth) Set(v int64) { q.value.Store(v) }

// MessagingService реализует постановку в очередь и чтение состояний.
type MessagingService struct {
	tx         ports.TxManager
	messages   ports.MessageRepo
	recipients ports.RecipientRepo
	profile    *ProfileStore
	depth      *QueueDepth
	clock      ports.Clock
	queueLimit int64
}

// NewMessagingService создаёт сервис постановки и чтения.
func NewMessagingService(tx ports.TxManager, messages ports.MessageRepo, recipients ports.RecipientRepo,
	profile *ProfileStore, depth *QueueDepth, clock ports.Clock, queueLimit int64) *MessagingService {
	return &MessagingService{tx: tx, messages: messages, recipients: recipients, profile: profile,
		depth: depth, clock: clock, queueLimit: queueLimit}
}

// EnqueueResult — результат постановки в очередь.
type EnqueueResult struct {
	Notification domain.Notification
	Duplicate    bool
}

// Enqueue ставит сообщение в очередь. Повтор с тем же ключом и тем же
// содержимым возвращает прежнее сообщение с признаком Duplicate; тот же ключ
// с другим содержимым — domain.ErrIdempotencyConflict.
func (s *MessagingService) Enqueue(ctx context.Context, req domain.EnqueueRequest, requestHash []byte) (EnqueueResult, error) {
	now := s.clock.Now()
	if err := req.Validate(now); err != nil {
		return EnqueueResult{}, err
	}
	if len(requestHash) != 32 {
		return EnqueueResult{}, fmt.Errorf("request hash must be 32 bytes")
	}
	if s.queueLimit > 0 && s.depth.Value() > s.queueLimit {
		return EnqueueResult{}, domain.ErrQueueFull
	}
	var res EnqueueResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		r, err := enqueueTx(ctx, s.messages, req, requestHash, now)
		res = r
		return err
	})
	return res, err
}

// enqueueTx выполняет постановку внутри уже открытой транзакции.
func enqueueTx(ctx context.Context, messages ports.MessageRepo, req domain.EnqueueRequest,
	requestHash []byte, now time.Time) (EnqueueResult, error) {
	n := domain.Notification{
		IdempotencyKey: req.IdempotencyKey,
		RequestHash:    requestHash,
		Message:        req.Message,
		NotAfter:       req.NotAfter,
		Status:         domain.StatusQueued,
		NextAttemptAt:  now,
		CreatedAt:      now,
	}
	stored, inserted, err := messages.Insert(ctx, n)
	if err != nil {
		return EnqueueResult{}, err
	}
	if inserted {
		return EnqueueResult{Notification: stored}, nil
	}
	existing, err := messages.GetByKey(ctx, req.IdempotencyKey)
	if err != nil {
		return EnqueueResult{}, err
	}
	if !bytes.Equal(existing.RequestHash, requestHash) {
		return EnqueueResult{}, domain.ErrIdempotencyConflict
	}
	return EnqueueResult{Notification: existing, Duplicate: true}, nil
}

// NotificationStatus возвращает состояние доставки по ключу.
func (s *MessagingService) NotificationStatus(ctx context.Context, key string) (domain.Notification, error) {
	if err := domain.ValidateIdempotencyKey(key); err != nil {
		return domain.Notification{}, err
	}
	return s.messages.GetByKey(ctx, key)
}

// RecipientStatus — состояние канала получателя.
type RecipientStatus struct {
	Recipient domain.Recipient
	Known     bool
	ChatURL   string
}

// RecipientStatus возвращает состояние получателя; отсутствие строки — UNKNOWN.
func (s *MessagingService) RecipientStatus(ctx context.Context, maxUserID int64) (RecipientStatus, error) {
	if maxUserID <= 0 {
		return RecipientStatus{}, &domain.ValidationError{Field: "max_user_id", Reason: "ожидается положительное число"}
	}
	r, found, err := s.recipients.Get(ctx, maxUserID)
	if err != nil {
		return RecipientStatus{}, err
	}
	out := RecipientStatus{Recipient: r, Known: found && r.State != domain.RecipientUnknown}
	if !found {
		out.Recipient = domain.Recipient{MaxUserID: maxUserID, State: domain.RecipientUnknown}
	}
	if p, ok := s.profile.Get(); ok {
		out.ChatURL = p.ChatURL()
	}
	return out, nil
}

// BotProfile возвращает профиль бота или domain.ErrProfileUnavailable.
func (s *MessagingService) BotProfile() (domain.Profile, domain.Mode, error) {
	p, ok := s.profile.Get()
	if !ok {
		return domain.Profile{}, s.profile.Mode(), domain.ErrProfileUnavailable
	}
	return p, s.profile.Mode(), nil
}

// QueueMonitor периодически обновляет глубину очереди (кэш 5 с, handoff §8).
type QueueMonitor struct {
	messages ports.MessageRepo
	depth    *QueueDepth
	metrics  *Metrics
	log      *slog.Logger
	Interval time.Duration
}

// NewQueueMonitor создаёт монитор глубины очереди.
func NewQueueMonitor(messages ports.MessageRepo, depth *QueueDepth, m *Metrics, log *slog.Logger) *QueueMonitor {
	return &QueueMonitor{messages: messages, depth: depth, metrics: m, log: log, Interval: 5 * time.Second}
}

// Refresh обновляет значение один раз.
func (q *QueueMonitor) Refresh(ctx context.Context) error {
	n, err := q.messages.CountActive(ctx)
	if err != nil {
		return err
	}
	q.depth.Set(n)
	q.metrics.QueueDepth.Set(float64(n))
	return nil
}

// Run обновляет глубину очереди до остановки.
func (q *QueueMonitor) Run(ctx context.Context) error {
	t := time.NewTicker(q.Interval)
	defer t.Stop()
	for {
		if err := q.Refresh(ctx); err != nil && !errors.Is(err, context.Canceled) {
			q.log.Warn("queue depth refresh failed", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}
