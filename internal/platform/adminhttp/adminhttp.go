// Package adminhttp — служебный HTTP-сервер сервиса: liveness, readiness и метрики.
// Порт не публикуется наружу и доступен только во внутренней сети (ADR-029).
package adminhttp

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ReadyFunc проверяет готовность зависимостей (например, ping БД).
type ReadyFunc func(ctx context.Context) error

// Server — admin-сервер с /healthz, /readyz и /metrics.
type Server struct {
	srv          *http.Server
	mux          *http.ServeMux
	shuttingDown atomic.Bool
}

// New создаёт admin-сервер.
func New(addr string, gatherer prometheus.Gatherer, ready ReadyFunc, readyTimeout time.Duration) *Server {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		if s.shuttingDown.Load() {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if s.shuttingDown.Load() {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
		defer cancel()
		if err := ready(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	s.mux = mux
	s.srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return s
}

// Handle добавляет служебный маршрут (например, буфер локального режима).
// Вызывается до Start.
func (s *Server) Handle(pattern string, h http.Handler) { s.mux.Handle(pattern, h) }

// Start запускает сервер и блокируется до его остановки.
func (s *Server) Start() error {
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown переводит readiness в 503 и останавливает сервер.
func (s *Server) Shutdown(ctx context.Context) error {
	s.shuttingDown.Store(true)
	return s.srv.Shutdown(ctx)
}
