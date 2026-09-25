// Package postgres реализует порты хранения bot-service поверх pgx.
// Все запросы параметризованы; сервис обращается только к схеме bot
// и не читает схемы core и reminders (ADR-019, ADR-020).
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/bot/internal/domain"
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

// errNoTx возвращается, если запрос требует открытой транзакции.
var errNoTx = errors.New("операция требует транзакции")

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nilIfZero(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// notFound переводит pgx.ErrNoRows в доменную ошибку.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}
