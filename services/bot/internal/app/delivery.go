package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// DeliveryConfig — параметры воркеров доставки.
type DeliveryConfig struct {
	Workers        int
	Batch          int
	PollInterval   time.Duration
	Lease          time.Duration
	RequestTimeout time.Duration // таймаут вызова MAX (10 с)
	ProfileWait    time.Duration // отсрочка сообщений с open_app без профиля (30 с)
	Policy         domain.RetryPolicy
}

// persistTimeout ограничивает запись результата после отправки: запись
// выполняется и во время остановки, чтобы не потерять результат попытки.
const persistTimeout = 5 * time.Second

// Delivery — воркеры отправки сообщений очереди в MAX.
type Delivery struct {
	tx         ports.TxManager
	messages   ports.MessageRepo
	recipients ports.RecipientRepo
	client     ports.MaxClient
	limiter    ports.RateLimiter
	profile    *ProfileStore
	clock      ports.Clock
	rnd        ports.Random
	cfg        DeliveryConfig
	log        *slog.Logger
	metrics    *Metrics
	authLog    throttle
}

// NewDelivery создаёт воркеры доставки.
func NewDelivery(tx ports.TxManager, messages ports.MessageRepo, recipients ports.RecipientRepo,
	client ports.MaxClient, limiter ports.RateLimiter, profile *ProfileStore, clock ports.Clock,
	rnd ports.Random, cfg DeliveryConfig, log *slog.Logger, m *Metrics) *Delivery {
	return &Delivery{tx: tx, messages: messages, recipients: recipients, client: client, limiter: limiter,
		profile: profile, clock: clock, rnd: rnd, cfg: cfg, log: log, metrics: m,
		authLog: throttle{every: time.Minute}}
}

// Run запускает cfg.Workers воркеров и ждёт их завершения после отмены ctx.
// Каждый воркер завершает текущее сообщение и возвращает в очередь
// захваченные, но не начатые сообщения.
func (d *Delivery) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for i := 0; i < d.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.worker(ctx)
		}()
	}
	wg.Wait()
	return nil
}

func (d *Delivery) worker(ctx context.Context) {
	t := time.NewTicker(d.cfg.PollInterval)
	defer t.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		n, err := d.ProcessBatch(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			d.log.Warn("delivery batch failed", slog.Any("error", err))
			d.metrics.AppErrors.WithLabelValues("delivery_batch").Inc()
		}
		if err == nil && n == d.cfg.Batch {
			continue // пакет заполнен целиком: готовые сообщения, вероятно, ещё есть
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// ProcessBatch захватывает и обрабатывает один пакет сообщений.
func (d *Delivery) ProcessBatch(ctx context.Context) (int, error) {
	batch, err := d.messages.ClaimReady(ctx, d.clock.Now(), d.cfg.Lease, d.cfg.Batch)
	if err != nil {
		return 0, err
	}
	for i, n := range batch {
		if ctx.Err() != nil {
			for _, rest := range batch[i:] {
				d.release(ctx, rest, d.clock.Now(), "released_shutdown")
			}
			return i, ctx.Err()
		}
		d.process(ctx, n)
	}
	return len(batch), nil
}

func (d *Delivery) process(ctx context.Context, n domain.Notification) {
	log := d.log.With(slog.Int64("message_id", n.ID), slog.String("kind", string(n.Kind)),
		slog.Int("attempt", n.Attempts))
	now := d.clock.Now()

	state := domain.RecipientUnknown
	rec, found, err := d.recipients.Get(ctx, n.RecipientMaxUserID)
	if err != nil {
		// Результат неизвестен: сообщение вернёт в очередь истечение аренды.
		log.Warn("recipient state read failed", slog.Any("error", err))
		return
	}
	if found {
		state = rec.State
	}
	if tr, stop := domain.PreSendCheck(n.NotAfter, state, now); stop {
		d.finish(ctx, n, tr, now, nil, log)
		return
	}

	profile, loaded := d.profile.Get()
	if n.HasOpenAppButtons() && !loaded {
		// Диплинк строится из ника бота: ждём загрузки профиля (spec §10).
		d.release(ctx, n, now.Add(d.cfg.ProfileWait), "profile_wait")
		return
	}

	// Ожидание лимитеров не должно съесть время, нужное на сам вызов MAX
	// в пределах аренды: иначе сообщение отдаётся обратно в очередь.
	// Бюджет считается как длительность по часам сервиса и переносится в
	// контекст: это не смешивает доменное время с системным.
	budget := n.LockedUntil.Sub(now) - d.cfg.RequestTimeout
	if budget <= 0 {
		d.release(ctx, n, d.clock.Now(), "released_lease")
		return
	}
	waitCtx, cancelWait := context.WithTimeout(ctx, budget)
	err = d.limiter.Wait(waitCtx, n.RecipientMaxUserID)
	cancelWait()
	if err != nil {
		d.release(ctx, n, d.clock.Now(), "released_limiter")
		return
	}

	// Начатая отправка завершается и при остановке сервиса.
	sendCtx, cancelSend := context.WithTimeout(context.WithoutCancel(ctx), d.cfg.RequestTimeout)
	res, err := d.client.SendMessage(sendCtx, n.Message, profile)
	cancelSend()
	now = d.clock.Now()
	if err == nil {
		d.markSent(ctx, n, res.MessageID, now, log)
		return
	}

	var se *ports.SendError
	if !errors.As(err, &se) {
		se = &ports.SendError{Failure: domain.SendFailure{Code: domain.CodeNetwork, Retryable: true}, Err: err}
	}
	switch se.Failure.Code {
	case domain.CodeMax429:
		d.limiter.Penalize()
		log.Warn("max rate limited", slog.Duration("retry_after", se.Failure.RetryAfter))
	case domain.CodeMax401:
		if d.authLog.allow(now) {
			log.Error("max rejected bot token (401)", slog.Int("http_status", se.HTTPStatus))
		}
	case domain.CodeMax4xx:
		log.Error("max rejected request", slog.Int("http_status", se.HTTPStatus), slog.Any("error", se.Err))
	default:
		log.Warn("message send failed", slog.String("code", se.Failure.Code), slog.Any("error", se.Err))
	}
	tr := d.cfg.Policy.AfterFailure(se.Failure, n.Attempts, now, d.rnd.Float64())
	d.finish(ctx, n, tr, now, &se.Failure, log)
}

func (d *Delivery) markSent(ctx context.Context, n domain.Notification, messageID string, now time.Time, log *slog.Logger) {
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()
	var ok bool
	err := d.tx.WithinTx(pctx, func(ctx context.Context) error {
		var err error
		ok, err = d.messages.MarkSent(ctx, n.ID, *n.LockedUntil, messageID, now)
		if err != nil || !ok {
			return err
		}
		return d.recipients.MarkDelivered(ctx, n.RecipientMaxUserID, now)
	})
	switch {
	case err != nil:
		log.Error("sent message not recorded; lease expiry may cause a duplicate", slog.Any("error", err))
		d.metrics.AppErrors.WithLabelValues("delivery_persist").Inc()
	case !ok:
		log.Warn("lease lost before recording send result")
		d.metrics.OutboundSend.WithLabelValues("lease_lost").Inc()
	default:
		d.metrics.OutboundSend.WithLabelValues("sent").Inc()
	}
}

func (d *Delivery) finish(ctx context.Context, n domain.Notification, tr domain.Transition, now time.Time,
	failure *domain.SendFailure, log *slog.Logger) {
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()
	var ok bool
	err := d.tx.WithinTx(pctx, func(ctx context.Context) error {
		var err error
		ok, err = d.messages.Finish(ctx, n.ID, *n.LockedUntil, tr, now)
		if err != nil || !ok || failure == nil || !failure.RecipientUnreachable {
			return err
		}
		r, err := d.recipients.LockOrCreate(ctx, n.RecipientMaxUserID, now)
		if err != nil {
			return err
		}
		if next, changed := r.MarkUnreachable(now); changed {
			return d.recipients.Save(ctx, next, now)
		}
		return nil
	})
	switch {
	case err != nil:
		log.Error("delivery result not recorded", slog.Any("error", err))
		d.metrics.AppErrors.WithLabelValues("delivery_persist").Inc()
	case !ok:
		log.Warn("lease lost before recording delivery result")
		d.metrics.OutboundSend.WithLabelValues("lease_lost").Inc()
	default:
		d.metrics.OutboundSend.WithLabelValues(resultLabel(tr)).Inc()
	}
}

func resultLabel(tr domain.Transition) string {
	switch {
	case tr.Status == domain.StatusRetryWait:
		return "retry"
	case tr.Status == domain.StatusExpired:
		return "expired"
	case tr.ErrorCode == domain.CodeRecipientStopped:
		return "recipient_stopped"
	case tr.ErrorCode == domain.CodeRecipientUnreachable:
		return "recipient_unreachable"
	default:
		return "failed"
	}
}

// release возвращает сообщение в очередь без учёта попытки.
func (d *Delivery) release(ctx context.Context, n domain.Notification, next time.Time, label string) {
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()
	if _, err := d.messages.Release(pctx, n.ID, *n.LockedUntil, next, d.clock.Now()); err != nil {
		d.log.Warn("message release failed; lease expiry will return it", slog.Int64("message_id", n.ID), slog.Any("error", err))
		return
	}
	d.metrics.OutboundSend.WithLabelValues(label).Inc()
}

// throttle пропускает событие не чаще одного раза за every.
type throttle struct {
	mu    sync.Mutex
	every time.Duration
	last  time.Time
}

func (t *throttle) allow(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.last.IsZero() && now.Sub(t.last) < t.every {
		return false
	}
	t.last = now
	return true
}

// LeaseReaper возвращает в очередь сообщения с истёкшей арендой (после сбоя процесса).
type LeaseReaper struct {
	messages ports.MessageRepo
	clock    ports.Clock
	log      *slog.Logger
	metrics  *Metrics
	Interval time.Duration
}

// NewLeaseReaper создаёт задачу возврата аренды.
func NewLeaseReaper(messages ports.MessageRepo, clock ports.Clock, log *slog.Logger, m *Metrics) *LeaseReaper {
	return &LeaseReaper{messages: messages, clock: clock, log: log, metrics: m, Interval: 30 * time.Second}
}

// RunOnce выполняет один проход.
func (r *LeaseReaper) RunOnce(ctx context.Context) (int64, error) {
	n, err := r.messages.ReapExpiredLeases(ctx, r.clock.Now())
	if err != nil {
		return 0, err
	}
	if n > 0 {
		r.log.Warn("expired delivery leases returned to queue", slog.Int64("count", n))
		r.metrics.OutboundSend.WithLabelValues("lease_expired").Add(float64(n))
	}
	return n, nil
}

// Run выполняет проходы до остановки.
func (r *LeaseReaper) Run(ctx context.Context) error {
	return every(ctx, r.Interval, func(ctx context.Context) {
		if _, err := r.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.log.Warn("lease reaper failed", slog.Any("error", err))
		}
	})
}

// every вызывает fn сразу и затем с периодом interval до отмены ctx.
func every(ctx context.Context, interval time.Duration, fn func(ctx context.Context)) error {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		fn(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}
