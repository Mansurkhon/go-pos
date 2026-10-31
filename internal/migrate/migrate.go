// Package migrate — минимальный раннер миграций: применяет *.up.sql по порядку
// и запоминает применённые версии в таблице schema_migrations.
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Up(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := fs.Glob(fsys, "*.up.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	for _, name := range names {
		version := strings.TrimSuffix(name, ".up.sql")

		var applied bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version,
		).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check %s: %w", version, err)
		}
		if applied {
			continue
		}

		if err := apply(ctx, pool, fsys, name, version); err != nil {
			return err
		}
		log.Printf("migration applied: %s", version)
	}
	return nil
}

// apply выполняет одну миграцию в транзакции: либо файл и запись о версии
// применяются вместе, либо не применяется ничего.
func apply(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, name, version string) error {
	sql, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // после Commit ничего не делает

	// Exec без аргументов идёт простым протоколом, он разрешает несколько команд в одной строке.
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("apply %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return fmt.Errorf("record %s: %w", version, err)
	}
	return tx.Commit(ctx)
}
