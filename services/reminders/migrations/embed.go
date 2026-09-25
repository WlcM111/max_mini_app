// Package migrations встраивает SQL-миграции схемы reminders в бинарник сервиса,
// чтобы команда `reminders migrate up` не зависела от файлов рядом с образом.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
