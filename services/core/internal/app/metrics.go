package app

import (
	"github.com/prometheus/client_golang/prometheus"

	"vovremya/internal/platform/metrics"
)

// Metrics — предметные метрики core-service (ADR-026: объявляются в сервисе).
type Metrics struct {
	SessionCreate   *prometheus.CounterVec
	ClientEvents    *prometheus.CounterVec
	OutboxPending   prometheus.Gauge
	OutboxDelivery  *prometheus.CounterVec
	RemindersRead   *prometheus.CounterVec
	Assistant       *prometheus.CounterVec
	AssistantTokens *prometheus.CounterVec
	AppErrors       *prometheus.CounterVec
}

// NewMetrics регистрирует метрики сервиса в общем реестре.
func NewMetrics(reg *metrics.Registry) *Metrics {
	m := &Metrics{
		SessionCreate: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_session_create_total",
			Help: "Создание сессий по результату проверки данных запуска MAX.",
		}, []string{"result"}),
		ClientEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_client_events_total",
			Help: "Технические события мини-приложения.",
		}, []string{"name", "code"}),
		OutboxPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vovremya_outbox_pending",
			Help: "Число неотправленных событий в outbox.",
		}),
		OutboxDelivery: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_outbox_delivery_total",
			Help: "Исходы доставки событий в reminders-service.",
		}, []string{"outcome"}),
		RemindersRead: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_reminders_read_total",
			Help: "Чтение плана напоминаний из reminders-service.",
		}, []string{"result"}),
		Assistant: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_assistant_requests_total",
			Help: "Обращения к языковому ассистенту по операции и исходу.",
		}, []string{"operation", "result"}),
		AssistantTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_assistant_tokens_total",
			Help: "Израсходованные токены языкового ассистента.",
		}, []string{"operation"}),
	}
	if reg != nil {
		// vovremya_app_errors_total объявлена в общем каркасе (observability §2):
		// сервис переиспользует её, а не регистрирует повторно.
		m.AppErrors = reg.AppErrors
		reg.MustRegister(m.SessionCreate, m.ClientEvents, m.OutboxPending, m.OutboxDelivery, m.RemindersRead,
			m.Assistant, m.AssistantTokens)
	} else {
		m.AppErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_app_errors_total",
			Help: "Ошибки сценариев по коду.",
		}, []string{"code"})
	}
	return m
}
