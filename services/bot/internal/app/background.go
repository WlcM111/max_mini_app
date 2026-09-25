package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"vovremya/services/bot/internal/ports"
)

// SubscriptionKeeper поддерживает подписку webhook (spec §9, только режим live):
// при старте и каждые Interval проверяет наличие подписки на публичный URL и
// создаёт её при отсутствии; при ошибке повторяет через RetryInterval.
type SubscriptionKeeper struct {
	client        ports.MaxClient
	url           string
	updateTypes   []string
	secret        string
	log           *slog.Logger
	metrics       *Metrics
	Interval      time.Duration
	RetryInterval time.Duration
}

// NewSubscriptionKeeper создаёт задачу поддержки подписки.
func NewSubscriptionKeeper(client ports.MaxClient, url string, updateTypes []string, secret string,
	interval time.Duration, log *slog.Logger, m *Metrics) *SubscriptionKeeper {
	return &SubscriptionKeeper{client: client, url: url, updateTypes: updateTypes, secret: secret,
		log: log, metrics: m, Interval: interval, RetryInterval: 30 * time.Second}
}

// EnsureOnce проверяет и при необходимости создаёт подписку.
func (k *SubscriptionKeeper) EnsureOnce(ctx context.Context) error {
	urls, err := k.client.ListSubscriptions(ctx)
	if err != nil {
		k.metrics.SubscriptionOK.Set(0)
		return err
	}
	found := false
	for _, u := range urls {
		if u == k.url {
			found = true
			continue
		}
		// Лишние подписки не удаляются: параметры DELETE /subscriptions не проверены (spec §9).
		k.log.Warn("foreign webhook subscription left untouched", slog.String("url", u))
	}
	if !found {
		if err := k.client.Subscribe(ctx, k.url, k.updateTypes, k.secret); err != nil {
			k.metrics.SubscriptionOK.Set(0)
			return err
		}
		k.log.Info("webhook subscription created", slog.String("url", k.url))
	}
	k.metrics.SubscriptionOK.Set(1)
	k.log.Info("webhook subscription ensured")
	return nil
}

// Run поддерживает подписку до остановки.
func (k *SubscriptionKeeper) Run(ctx context.Context) error {
	for {
		wait := k.Interval
		if err := k.EnsureOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			k.log.Error("webhook subscription check failed", slog.Any("error", err))
			wait = k.RetryInterval
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
	}
}

// RetentionConfig — сроки хранения (handoff §8).
type RetentionConfig struct {
	Interval   time.Duration // 1 ч
	InboundTTL time.Duration // 7 суток
	FinalTTL   time.Duration // 30 суток
	StoppedTTL time.Duration // 30 суток
}

// RetentionJob удаляет данные с истёкшим сроком хранения.
type RetentionJob struct {
	inbound    ports.InboundRepo
	messages   ports.MessageRepo
	recipients ports.RecipientRepo
	clock      ports.Clock
	cfg        RetentionConfig
	log        *slog.Logger
	metrics    *Metrics
}

// NewRetentionJob создаёт задачу очистки.
func NewRetentionJob(inbound ports.InboundRepo, messages ports.MessageRepo, recipients ports.RecipientRepo,
	clock ports.Clock, cfg RetentionConfig, log *slog.Logger, m *Metrics) *RetentionJob {
	return &RetentionJob{inbound: inbound, messages: messages, recipients: recipients, clock: clock,
		cfg: cfg, log: log, metrics: m}
}

// RunOnce выполняет одну очистку.
func (j *RetentionJob) RunOnce(ctx context.Context) error {
	now := j.clock.Now()
	inb, err := j.inbound.DeleteBefore(ctx, now.Add(-j.cfg.InboundTTL))
	if err != nil {
		return err
	}
	msgs, err := j.messages.DeleteFinalizedBefore(ctx, now.Add(-j.cfg.FinalTTL))
	if err != nil {
		return err
	}
	recs, err := j.recipients.DeleteStoppedInactive(ctx, now.Add(-j.cfg.StoppedTTL))
	if err != nil {
		return err
	}
	if inb+msgs+recs > 0 {
		j.log.Info("retention cleanup", slog.Int64("inbound_updates", inb),
			slog.Int64("outbound_messages", msgs), slog.Int64("recipients", recs))
	}
	return nil
}

// Run выполняет очистку до остановки.
func (j *RetentionJob) Run(ctx context.Context) error {
	return every(ctx, j.cfg.Interval, func(ctx context.Context) {
		if err := j.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			j.log.Warn("retention failed", slog.Any("error", err))
			j.metrics.AppErrors.WithLabelValues("retention").Inc()
		}
	})
}
