// Package postgres реализует порты хранения reminders-service поверх pgx.
// Все запросы параметризованы; сервис обращается только к схеме reminders
// и не читает схемы core и bot (ADR-020).
package postgres

import (
	"context"
	"time"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
)

// base — общая часть репозиториев: доступ к пулу или транзакции из контекста.
type base struct {
	pool    *pgkit.Pool
	metrics *metrics.Registry
}

func (b base) db(ctx context.Context) pgkit.DBTX { return b.pool.DB(ctx) }

func (b base) observe(query string, started time.Time) {
	if b.metrics != nil {
		b.metrics.ObserveDB(query, started)
	}
}
