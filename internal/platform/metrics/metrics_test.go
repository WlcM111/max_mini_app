package metrics_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"vovremya/internal/platform/metrics"
)

// TestRegistryHasOnlyTechnicalMetrics закрепляет исправление B-1: общий каркас
// содержит только технические метрики, предметные объявляет сам сервис (ADR-026).
func TestRegistryHasOnlyTechnicalMetrics(t *testing.T) {
	m := metrics.New()
	m.DBPoolAcquired.Set(1)
	families, err := m.Gatherer().Gather()
	if err != nil {
		t.Fatalf("сбор метрик: %v", err)
	}
	for _, f := range families {
		name := f.GetName()
		for _, forbidden := range []string{"reminders", "ingest", "outbound", "webhook", "projection", "subscription"} {
			if strings.Contains(name, forbidden) {
				t.Errorf("каркас экспортирует предметную метрику %q", name)
			}
		}
	}
}

// TestMustRegisterAddsServiceMetrics — сервис добавляет свои метрики в общий реестр.
func TestMustRegisterAddsServiceMetrics(t *testing.T) {
	m := metrics.New()
	g := prometheus.NewGauge(prometheus.GaugeOpts{Name: "vovremya_test_service_metric", Help: "тест"})
	m.MustRegister(g)
	g.Set(3)
	families, err := m.Gatherer().Gather()
	if err != nil {
		t.Fatalf("сбор метрик: %v", err)
	}
	for _, f := range families {
		if f.GetName() == "vovremya_test_service_metric" {
			return
		}
	}
	t.Error("метрика сервиса не попала в общий реестр")
}
