// Package store provides SQLite database connection management with WAL mode.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB with SQLite-specific initialization.
type DB struct {
	Writer *sql.DB // single writer connection (WAL mode)
	Reader *sql.DB // multiple reader connections
	path   string
}

// Open creates the SQLite database, enables WAL, sets PRAGMAs, and returns a DB instance.
// It creates the parent directory if it does not exist.
func Open(path string) (*DB, error) {
	// For in-memory databases, use shared cache so Writer and Reader share the same DB
	if path == ":memory:" {
		return openMemory()
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: create db directory: %w", err)
	}

	// Writer connection (single, serialized writes)
	writerDSN := fmt.Sprintf("file:%s?_txlock=immediate&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON", path)
	writer, err := sql.Open("sqlite", writerDSN)
	if err != nil {
		return nil, fmt.Errorf("store: open writer: %w", err)
	}
	writer.SetMaxOpenConns(1)

	// Reader connections (multiple concurrent readers)
	readerDSN := fmt.Sprintf("file:%s?mode=ro&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON", path)
	reader, err := sql.Open("sqlite", readerDSN)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("store: open reader: %w", err)
	}
	reader.SetMaxOpenConns(4)

	db := &DB{
		Writer: writer,
		Reader: reader,
		path:   path,
	}

	// Apply PRAGMAs on writer
	if err := db.applyPragmas(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: apply pragmas: %w", err)
	}

	slog.Info("database opened", "path", path, "mode", "WAL")
	return db, nil
}

// Close gracefully closes both writer and reader pools.
func (db *DB) Close() error {
	var errs []error
	if err := db.Writer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close writer: %w", err))
	}
	if err := db.Reader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close reader: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("store: close: %v", errs)
	}
	slog.Info("database closed", "path", db.path)
	return nil
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// Backup performs an online backup to the specified destination path.
// Uses SQLite's VACUUM INTO for a consistent snapshot.
func (db *DB) Backup(ctx context.Context, destPath string) error {
	// Ensure destination directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store: backup mkdir: %w", err)
	}

	_, err := db.Writer.ExecContext(ctx, "VACUUM INTO ?", destPath)
	if err != nil {
		return fmt.Errorf("store: backup: %w", err)
	}

	slog.Info("database backup completed", "dest", destPath)
	return nil
}

// openMemory creates an in-memory database where Writer and Reader share the same instance.
func openMemory() (*DB, error) {
	// Use shared cache so both connections see the same in-memory DB
	dsn := "file::memory:?cache=shared&_foreign_keys=ON"
	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open memory writer: %w", err)
	}
	writer.SetMaxOpenConns(1)

	reader, err := sql.Open("sqlite", dsn)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("store: open memory reader: %w", err)
	}
	reader.SetMaxOpenConns(4)

	db := &DB{Writer: writer, Reader: reader, path: ":memory:"}
	if err := db.applyPragmas(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: apply pragmas: %w", err)
	}
	slog.Info("database opened", "path", ":memory:", "mode", "shared-cache")
	return db, nil
}

func (db *DB) applyPragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA cache_size=-64000", // 64MB cache
		"PRAGMA temp_store=MEMORY",
	}
	for _, p := range pragmas {
		if _, err := db.Writer.Exec(p); err != nil {
			return fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return nil
}
