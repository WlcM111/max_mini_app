// Package migrations встраивает SQL-миграции схемы bot в бинарник сервиса,
// чтобы команда `bot migrate up` не зависела от файлов рядом с образом.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
