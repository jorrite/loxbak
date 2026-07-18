package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Session is a row in the sessions table.
type Session struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ErrSessionNotFound is returned by GetSession when no session with the
// given id exists (or it has already expired and been pruned).
var ErrSessionNotFound = errors.New("db: session not found")

// CreateSession inserts a new session row.
func CreateSession(sqlDB *sql.DB, id string, expiresAt time.Time) error {
	_, err := sqlDB.Exec(
		`INSERT INTO sessions (id, expires_at) VALUES (?, ?)`,
		id, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("db: create session: %w", err)
	}
	return nil
}

// GetSession looks up a session by id. It returns ErrSessionNotFound if the
// session does not exist or has expired.
func GetSession(sqlDB *sql.DB, id string) (*Session, error) {
	var s Session
	err := sqlDB.QueryRow(
		`SELECT id, created_at, expires_at FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: get session: %w", err)
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, ErrSessionNotFound
	}
	return &s, nil
}

// DeleteSession removes a session row (used on logout).
func DeleteSession(sqlDB *sql.DB, id string) error {
	if _, err := sqlDB.Exec(`DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("db: delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions prunes all sessions past their expiry.
func DeleteExpiredSessions(sqlDB *sql.DB) error {
	if _, err := sqlDB.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now()); err != nil {
		return fmt.Errorf("db: delete expired sessions: %w", err)
	}
	return nil
}
