// Package pgkit — доступ к PostgreSQL: пул соединений, явные транзакционные
// границы и запуск миграций. Транзакция передаётся через контекст, поэтому
// репозитории адаптеров работают и внутри транзакции, и вне её.
package pgkit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool — обёртка над pgxpool с транзакциями в контексте.
type Pool struct {
	pool *pgxpool.Pool
}

type txKey struct{}

// Open создаёт пул соединений и проверяет доступность БД.
func Open(ctx context.Context, dsn string, maxConns int32, appName string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.ConnConfig.RuntimeParams["application_name"] = appName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Pool{pool: pool}, nil
}

// Close закрывает пул.
func (p *Pool) Close() { p.pool.Close() }

// Ping проверяет доступность БД с заданным сроком.
func (p *Pool) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

// AcquiredConns возвращает число занятых соединений (для метрики).
func (p *Pool) AcquiredConns() int32 { return p.pool.Stat().AcquiredConns() }

// Raw возвращает исходный пул. Используется только в composition root и тестах.
func (p *Pool) Raw() *pgxpool.Pool { return p.pool }

// WithinTx выполняет fn в транзакции READ COMMITTED. Вложенный вызов
// присоединяется к существующей транзакции контекста и не открывает новую.
func (p *Pool) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {
		// Rollback после Commit возвращает ErrTxClosed и безопасен.
		_ = tx.Rollback(context.WithoutCancel(ctx))
	}()
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// DB возвращает исполнителя запросов: транзакцию из контекста либо пул.
func (p *Pool) DB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return p.pool
}

// InTx сообщает, выполняется ли контекст внутри транзакции.
func InTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(pgx.Tx)
	return ok
}
