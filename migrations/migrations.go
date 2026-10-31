// Package migrations встраивает SQL-файлы в бинарник через go:embed,
// поэтому для запуска не нужно таскать папку с миграциями рядом.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
