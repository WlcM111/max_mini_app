// Package metrics собирает метрики Prometheus, общие для сервисов проекта.
// Имена и метки — docs/architecture/observability.md §2.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Registry — набор метрик одного сервиса.
type Registry struct {
	reg *prometheus.Registry

	GRPCServerHandled  *prometheus.CounterVec
	GRPCClientRequests *prometheus.CounterVec
	GRPCDuration       *prometheus.HistogramVec
	DBQueryDuration    *prometheus.HistogramVec
	DBPoolAcquired     prometheus.Gauge
	AppErrors          *prometheus.CounterVec
}

// New создаёт набор метрик и регистрирует их в собственном реестре.
func New() *Registry {
	r := prometheus.NewRegistry()
	m := &Registry{
		reg: r,
		GRPCServerHandled: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_grpc_server_handled_total",
			Help: "Обработанные gRPC-запросы сервера по методу и коду.",
		}, []string{"method", "code"}),
		GRPCClientRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_grpc_client_requests_total",
			Help: "Исходящие gRPC-запросы по методу и коду.",
		}, []string{"method", "code"}),
		GRPCDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "vovremya_grpc_duration_seconds",
			Help:    "Длительность gRPC-вызовов.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 10},
		}, []string{"method", "side"}),
		DBQueryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "vovremya_db_query_duration_seconds",
			Help:    "Длительность запросов к PostgreSQL по имени запроса.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.5, 1, 5},
		}, []string{"query"}),
		DBPoolAcquired: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vovremya_db_pool_acquired_conns",
			Help: "Занятые соединения пула PostgreSQL.",
		}),
		AppErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_app_errors_total",
			Help: "Ошибки приложения по коду.",
		}, []string{"code"}),
	}
	r.MustRegister(m.GRPCServerHandled, m.GRPCClientRequests, m.GRPCDuration,
		m.DBQueryDuration, m.DBPoolAcquired, m.AppErrors)
	return m
}

// MustRegister регистрирует метрики конкретного сервиса в общем реестре.
// Предметные метрики объявляет сам сервис: каркас их не знает (ADR-026).
func (m *Registry) MustRegister(cs ...prometheus.Collector) { m.reg.MustRegister(cs...) }

// Gatherer возвращает реестр для admin-эндпоинта /metrics.
func (m *Registry) Gatherer() prometheus.Gatherer { return m.reg }

// ObserveDB фиксирует длительность запроса к БД.
func (m *Registry) ObserveDB(query string, started time.Time) {
	m.DBQueryDuration.WithLabelValues(query).Observe(time.Since(started).Seconds())
}
