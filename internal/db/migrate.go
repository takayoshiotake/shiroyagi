package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

const migrationsDir = "migrations"

func Migrate(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		if err := applyMigration(ctx, database, name); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, database *sql.DB, name string) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var applied bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", name, err)
	}
	if applied {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit skipped migration %s: %w", name, err)
		}
		return nil
	}

	content, err := fs.ReadFile(migrationFS, migrationsDir+"/"+name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}
	return nil
}
