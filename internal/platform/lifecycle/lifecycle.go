// Package lifecycle запускает компоненты сервиса и останавливает их по сигналу
// в предсказуемом порядке: приём трафика прекращается раньше фоновых задач.
package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Component — запускаемая часть сервиса.
type Component struct {
	Name  string
	Start func(ctx context.Context) error
	// Stop вызывается при остановке; ctx ограничен общим сроком остановки.
	Stop func(ctx context.Context) error
}

// Runner запускает компоненты и обеспечивает graceful shutdown.
type Runner struct {
	log        *slog.Logger
	timeout    time.Duration
	components []Component
}

// New создаёт runner с общим сроком остановки.
func New(log *slog.Logger, shutdownTimeout time.Duration) *Runner {
	return &Runner{log: log, timeout: shutdownTimeout}
}

// Add добавляет компонент. Порядок добавления определяет порядок остановки.
func (r *Runner) Add(c Component) { r.components = append(r.components, c) }

// Run запускает все компоненты и блокируется до сигнала или первой ошибки.
func (r *Runner) Run(ctx context.Context) error {
	ctx, stopSignals := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	errCh := make(chan error, len(r.components))
	var wg sync.WaitGroup
	for _, c := range r.components {
		if c.Start == nil {
			continue
		}
		wg.Add(1)
		go func(c Component) {
			defer wg.Done()
			if err := c.Start(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				r.log.Error("component failed", slog.String("component", c.Name), slog.Any("error", err))
				errCh <- err
			}
		}(c)
	}

	var runErr error
	select {
	case <-ctx.Done():
		r.log.Info("shutdown signal received")
	case err := <-errCh:
		runErr = err
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.timeout)
	defer cancel()
	for _, c := range r.components {
		if c.Stop == nil {
			continue
		}
		if err := c.Stop(shutdownCtx); err != nil {
			r.log.Error("component stop failed", slog.String("component", c.Name), slog.Any("error", err))
		} else {
			r.log.Info("component stopped", slog.String("component", c.Name))
		}
	}
	cancelRun()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		r.log.Warn("shutdown timeout: some components did not stop in time")
	}
	return runErr
}
