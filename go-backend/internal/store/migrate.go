package store

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate reads all embedded SQL migration files and executes them in order.
// It uses a migrations tracking table to avoid re-running already applied migrations.
func (db *DB) Migrate(ctx context.Context) error {
	// Create migrations tracking table
	_, err := db.Writer.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return fmt.Errorf("store: create migrations table: %w", err)
	}

	// Read all migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("store: read migrations dir: %w", err)
	}

	// Sort by filename to ensure execution order
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		// Check if already applied
		var count int
		err := db.Writer.QueryRowContext(ctx, "SELECT COUNT(*) FROM _migrations WHERE name = ?", name).Scan(&count)
		if err != nil {
			return fmt.Errorf("store: check migration %s: %w", name, err)
		}
		if count > 0 {
			continue
		}

		// Read and execute
		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", name, err)
		}

		if _, err := db.Writer.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("store: execute migration %s: %w", name, err)
		}

		// Record as applied
		if _, err := db.Writer.ExecContext(ctx, "INSERT INTO _migrations (name) VALUES (?)", name); err != nil {
			return fmt.Errorf("store: record migration %s: %w", name, err)
		}

		slog.Info("migration applied", "name", name)
	}

	return nil
}
