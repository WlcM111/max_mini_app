// Package migrations встраивает SQL-миграции схемы core в бинарник сервиса.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
