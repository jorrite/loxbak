package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var migrationVersionRe = regexp.MustCompile(`^(\d+)_`)

// RunMigrations executes every .sql file in internal/db/migrations, in
// order, skipping any whose numeric filename prefix is <= the
// schema_version already recorded in the settings table.
//
// Early migrations (0001) are written as CREATE TABLE IF NOT EXISTS and
// safe to re-run regardless — this version gate exists for later
// migrations that aren't naturally idempotent (e.g. rebuilding a table to
// change a CHECK constraint, which SQLite has no ALTER for): no migration
// framework for purely-additive changes, but real version tracking once a
// change stops being idempotent.
func RunMigrations(sqlDB *sql.DB) error {
	// Bootstrap settings itself before it can be queried for schema_version
	// — safe to run before/alongside 0001_init.sql's own (also
	// IF-NOT-EXISTS) creation of the same table.
	if _, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("bootstrap settings table: %w", err)
	}

	current := 0
	if v, ok, err := GetSetting(sqlDB, "schema_version"); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	} else if ok {
		current, err = strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("parse schema_version %q: %w", v, err)
		}
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		m := migrationVersionRe.FindStringSubmatch(name)
		if m == nil {
			return fmt.Errorf("migration %s: filename must start with a numeric prefix (e.g. 0002_)", name)
		}
		version, err := strconv.Atoi(m[1])
		if err != nil {
			return fmt.Errorf("migration %s: parse version prefix: %w", name, err)
		}
		if version <= current {
			continue
		}

		contents, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := sqlDB.Exec(string(contents)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if err := SetSetting(sqlDB, "schema_version", strconv.Itoa(version)); err != nil {
			return fmt.Errorf("record schema_version after %s: %w", name, err)
		}
		current = version
	}

	return nil
}
