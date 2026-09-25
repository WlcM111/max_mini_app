package pgkit

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// MigrateUp применяет миграции goose из встроенной файловой системы.
// Выполняется ролью-владельцем схемы; таблица версий — <schema>.goose_db_version.
func MigrateUp(ctx context.Context, dsn string, files fs.FS, schema string) error {
	db, err := openSQL(dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	goose.SetBaseFS(files)
	goose.SetTableName(schema + ".goose_db_version")
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// MigrateDownTo откатывает миграции до указанной версии включительно.
func MigrateDownTo(ctx context.Context, dsn string, files fs.FS, schema string, version int64) error {
	db, err := openSQL(dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	goose.SetBaseFS(files)
	goose.SetTableName(schema + ".goose_db_version")
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("dialect: %w", err)
	}
	if err := goose.DownToContext(ctx, db, ".", version); err != nil {
		return fmt.Errorf("goose down-to: %w", err)
	}
	return nil
}

func openSQL(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx/v5", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

var _ = stdlib.GetDefaultDriver
