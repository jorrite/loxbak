package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// LoxoneCredential is a row in the loxone_credential table. The schema
// allows multiple rows for future multi-Miniserver support, but v1 app
// logic only ever reads/writes the single row with the lowest id.
type LoxoneCredential struct {
	ID                int64
	Host              string
	Port              int
	Username          string
	EncryptedPassword []byte
	UpdatedAt         time.Time
}

// ErrCredentialNotFound is returned when no Loxone credential has been
// stored yet (i.e. nobody has logged in successfully).
var ErrCredentialNotFound = errors.New("db: loxone credential not found")

// GetCredential returns the stored Loxone credential, if any.
func GetCredential(sqlDB *sql.DB) (*LoxoneCredential, error) {
	var c LoxoneCredential
	err := sqlDB.QueryRow(
		`SELECT id, host, port, username, encrypted_password, updated_at
		   FROM loxone_credential ORDER BY id LIMIT 1`,
	).Scan(&c.ID, &c.Host, &c.Port, &c.Username, &c.EncryptedPassword, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCredentialNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: get credential: %w", err)
	}
	return &c, nil
}

// UpsertCredential stores the Loxone credential used by the scheduler,
// replacing whatever was stored before. v1 keeps a single credential row.
func UpsertCredential(sqlDB *sql.DB, host string, port int, username string, encryptedPassword []byte) error {
	existing, err := GetCredential(sqlDB)
	if err != nil && !errors.Is(err, ErrCredentialNotFound) {
		return err
	}

	if existing == nil {
		_, err := sqlDB.Exec(
			`INSERT INTO loxone_credential (host, port, username, encrypted_password)
			 VALUES (?, ?, ?, ?)`,
			host, port, username, encryptedPassword,
		)
		if err != nil {
			return fmt.Errorf("db: insert credential: %w", err)
		}
		return nil
	}

	_, err = sqlDB.Exec(
		`UPDATE loxone_credential
		    SET host = ?, port = ?, username = ?, encrypted_password = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE id = ?`,
		host, port, username, encryptedPassword, existing.ID,
	)
	if err != nil {
		return fmt.Errorf("db: update credential: %w", err)
	}
	return nil
}
