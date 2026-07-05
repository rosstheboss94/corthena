// Package sqlitewal exercises the approved SQLite driver and ownership model.
package sqlitewal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

const readerCount = 4

type migration struct {
	version int
	sql     string
}

var migrations = [...]migration{
	{
		version: 1,
		sql: `CREATE TABLE compatibility_rows (
			id INTEGER PRIMARY KEY,
			value TEXT NOT NULL
		) STRICT`,
	},
}

// Verify enables WAL, applies numbered migrations through PRAGMA user_version,
// and proves that concurrent readers proceed while the sole writer owns a
// transaction.
func Verify(ctx context.Context, directory string) error {
	path := filepath.Join(directory, "compatibility.sqlite")

	writer, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open SQLite writer: %w", err)
	}
	defer writer.Close()
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)

	if err := writer.PingContext(ctx); err != nil {
		return fmt.Errorf("ping SQLite writer: %w", err)
	}
	if err := configureWriter(ctx, writer); err != nil {
		return err
	}
	if err := applyMigrations(ctx, writer); err != nil {
		return err
	}
	if _, err := writer.ExecContext(
		ctx,
		"INSERT INTO compatibility_rows(id, value) VALUES(1, 'committed')",
	); err != nil {
		return fmt.Errorf("seed SQLite row: %w", err)
	}

	readers, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open SQLite readers: %w", err)
	}
	defer readers.Close()
	readers.SetMaxOpenConns(readerCount)
	readers.SetMaxIdleConns(readerCount)
	if _, err := readers.ExecContext(ctx, "PRAGMA query_only=ON"); err != nil {
		return fmt.Errorf("configure SQLite readers: %w", err)
	}

	transaction, err := writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQLite writer transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = transaction.Rollback()
		}
	}()
	if _, err := transaction.ExecContext(
		ctx,
		"INSERT INTO compatibility_rows(id, value) VALUES(2, 'pending')",
	); err != nil {
		return fmt.Errorf("insert pending SQLite row: %w", err)
	}

	// Each reader sends exactly once. This function owns and closes the channel
	// only after all reader goroutines terminate.
	results := make(chan error, readerCount)
	var readersDone sync.WaitGroup
	readersDone.Add(readerCount)
	for range readerCount {
		go func() {
			defer readersDone.Done()
			results <- readCount(ctx, readers, 1)
		}()
	}
	readersDone.Wait()
	close(results)
	for readErr := range results {
		if readErr != nil {
			return readErr
		}
	}

	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit SQLite writer transaction: %w", err)
	}
	committed = true
	if err := readCount(ctx, readers, 2); err != nil {
		return err
	}
	return nil
}

func configureWriter(ctx context.Context, database *sql.DB) error {
	var mode string
	if err := database.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return fmt.Errorf("enable SQLite WAL: %w", err)
	}
	if mode != "wal" {
		return fmt.Errorf("enable SQLite WAL: got journal mode %q", mode)
	}
	if _, err := database.ExecContext(ctx, "PRAGMA busy_timeout=2000"); err != nil {
		return fmt.Errorf("set SQLite busy timeout: %w", err)
	}
	if _, err := database.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("enable SQLite foreign keys: %w", err)
	}
	return nil
}

func applyMigrations(ctx context.Context, database *sql.DB) error {
	var current int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read SQLite user_version: %w", err)
	}
	for _, next := range migrations {
		if next.version <= current {
			continue
		}
		if next.version != current+1 {
			return fmt.Errorf("apply SQLite migration: version gap from %d to %d", current, next.version)
		}
		transaction, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin SQLite migration %d: %w", next.version, err)
		}
		if _, err := transaction.ExecContext(ctx, next.sql); err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("apply SQLite migration %d: %w", next.version, err)
		}
		if _, err := transaction.ExecContext(
			ctx,
			fmt.Sprintf("PRAGMA user_version=%d", next.version),
		); err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("set SQLite user_version %d: %w", next.version, err)
		}
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit SQLite migration %d: %w", next.version, err)
		}
		current = next.version
	}
	if current != migrations[len(migrations)-1].version {
		return errors.New("apply SQLite migrations: final version mismatch")
	}
	return nil
}

func readCount(ctx context.Context, database *sql.DB, want int) error {
	var count int
	if err := database.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM compatibility_rows",
	).Scan(&count); err != nil {
		return fmt.Errorf("read SQLite row count: %w", err)
	}
	if count != want {
		return fmt.Errorf("read SQLite row count: got %d, want %d", count, want)
	}
	return nil
}
