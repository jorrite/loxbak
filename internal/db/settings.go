package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// GetSetting returns the value stored under key, and false if no such
// setting exists.
func GetSetting(sqlDB *sql.DB, key string) (string, bool, error) {
	var value string
	err := sqlDB.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("db: get setting %s: %w", key, err)
	}
	return value, true, nil
}

// SetSetting upserts a key/value pair into the settings table.
func SetSetting(sqlDB *sql.DB, key, value string) error {
	_, err := sqlDB.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("db: set setting %s: %w", key, err)
	}
	return nil
}
