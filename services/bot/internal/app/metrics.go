// Package app содержит сценарии bot-service.
package app

import (
	"github.com/prometheus/client_golang/prometheus"

	"vovremya/internal/platform/metrics"
)

// Metrics — предметные метрики bot-service поверх общего реестра каркаса.
type Metrics struct {
	*metrics.Registry

	QueueDepth     prometheus.Gauge
	OutboundSend   *prometheus.CounterVec
	WebhookUpdates *prometheus.CounterVec
	SubscriptionOK prometheus.Gauge
}

// NewMetrics создаёт метрики сервиса и регистрирует их в общем реестре.
func NewMetrics(reg *metrics.Registry) *Metrics {
	m := &Metrics{
		Registry: reg,
		QueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vovremya_outbound_queue_depth",
			Help: "Сообщения бота в незавершённых состояниях (queued, sending, retry_wait).",
		}),
		OutboundSend: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_outbound_send_total",
			Help: "Результаты обработки сообщений очереди доставки.",
		}, []string{"result"}),
		WebhookUpdates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_max_webhook_updates_total",
			Help: "События webhook MAX по типу и исходу обработки.",
		}, []string{"type", "outcome"}),
		SubscriptionOK: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vovremya_max_subscription_ok",
			Help: "1 — подписка webhook подтверждена, 0 — нет.",
		}),
	}
	reg.MustRegister(m.QueueDepth, m.OutboundSend, m.WebhookUpdates, m.SubscriptionOK)
	return m
}
