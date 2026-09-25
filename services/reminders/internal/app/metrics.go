package app

import (
	"github.com/prometheus/client_golang/prometheus"

	"vovremya/internal/platform/metrics"
)

// Metrics — предметные метрики reminders-service поверх общего реестра каркаса.
type Metrics struct {
	*metrics.Registry

	IngestEvents        *prometheus.CounterVec
	RemindersBacklog    prometheus.Gauge
	RemindersHandoff    *prometheus.CounterVec
	RemindersPlanned    prometheus.Gauge
	ProjectionLagSecond prometheus.Gauge
}

// NewMetrics создаёт метрики сервиса и регистрирует их в общем реестре.
func NewMetrics(reg *metrics.Registry) *Metrics {
	m := &Metrics{
		Registry: reg,
		IngestEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_ingest_events_total",
			Help: "Принятые события core по типу и исходу.",
		}, []string{"type", "outcome"}),
		RemindersBacklog: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vovremya_reminders_due_backlog",
			Help: "Запланированные напоминания с наступившим next_attempt_at.",
		}),
		RemindersHandoff: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_reminders_handoff_total",
			Help: "Передачи напоминаний в bot-service по результату.",
		}, []string{"result"}),
		RemindersPlanned: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vovremya_reminders_planned",
			Help: "Общее число напоминаний в статусе planned.",
		}),
		ProjectionLagSecond: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vovremya_projection_lag_seconds",
			Help: "Возраст последнего применённого события core.",
		}),
	}
	reg.MustRegister(m.IngestEvents, m.RemindersBacklog, m.RemindersHandoff,
		m.RemindersPlanned, m.ProjectionLagSecond)
	return m
}
