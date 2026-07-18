// Package db owns the SQLite connection and schema migrations for loxbak.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens (creating if necessary) the SQLite database at path, applies
// pragmas suited for a small single-writer server workload, and runs
// migrations.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", path, err)
	}

	// modernc.org/sqlite does not support concurrent writers; keep a single
	// connection so WAL + busy_timeout are enough to serialize access
	// without SQLITE_BUSY errors under the API's low concurrency.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db: enable foreign_keys: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db: enable WAL: %w", err)
	}

	if err := RunMigrations(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db: run migrations: %w", err)
	}

	return sqlDB, nil
}
