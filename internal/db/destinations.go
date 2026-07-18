package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Destination is a row in the destinations table.
type Destination struct {
	ID              int64
	Name            string
	Type            string
	ConfigJSON      string
	EncryptedSecret []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ErrDestinationNotFound is returned when a destination id has no matching row.
var ErrDestinationNotFound = errors.New("db: destination not found")

// ErrDestinationInUse is returned by DeleteDestination when one or more
// schedules still reference the destination. schedule_destinations has
// ON DELETE CASCADE on destination_id, so without this guard a delete would
// silently drop the destination out of any schedule using it rather than
// refusing the delete.
var ErrDestinationInUse = errors.New("db: destination is still in use by a schedule")

// ListDestinations returns all configured destinations, newest first.
func ListDestinations(sqlDB *sql.DB) ([]Destination, error) {
	rows, err := sqlDB.Query(
		`SELECT id, name, type, config_json, encrypted_secret, created_at, updated_at
		   FROM destinations ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list destinations: %w", err)
	}
	defer rows.Close()

	var out []Destination
	for rows.Next() {
		var d Destination
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.ConfigJSON, &d.EncryptedSecret, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("db: scan destination: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list destinations: %w", err)
	}
	return out, nil
}

// GetDestination returns a single destination by id.
func GetDestination(sqlDB *sql.DB, id int64) (*Destination, error) {
	var d Destination
	err := sqlDB.QueryRow(
		`SELECT id, name, type, config_json, encrypted_secret, created_at, updated_at
		   FROM destinations WHERE id = ?`, id,
	).Scan(&d.ID, &d.Name, &d.Type, &d.ConfigJSON, &d.EncryptedSecret, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDestinationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: get destination: %w", err)
	}
	return &d, nil
}

// CreateDestination inserts a new destination and returns its id.
func CreateDestination(sqlDB *sql.DB, name, typ, configJSON string, encryptedSecret []byte) (int64, error) {
	res, err := sqlDB.Exec(
		`INSERT INTO destinations (name, type, config_json, encrypted_secret)
		 VALUES (?, ?, ?, ?)`,
		name, typ, configJSON, encryptedSecret,
	)
	if err != nil {
		return 0, fmt.Errorf("db: create destination: %w", err)
	}
	return res.LastInsertId()
}

// UpdateDestination updates an existing destination's fields.
func UpdateDestination(sqlDB *sql.DB, id int64, name, typ, configJSON string, encryptedSecret []byte) error {
	res, err := sqlDB.Exec(
		`UPDATE destinations
		    SET name = ?, type = ?, config_json = ?, encrypted_secret = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE id = ?`,
		name, typ, configJSON, encryptedSecret, id,
	)
	if err != nil {
		return fmt.Errorf("db: update destination: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: update destination: %w", err)
	}
	if n == 0 {
		return ErrDestinationNotFound
	}
	return nil
}

// DeleteDestination removes a destination by id, refusing (ErrDestinationInUse)
// if any schedule still references it.
func DeleteDestination(sqlDB *sql.DB, id int64) error {
	var inUse int
	err := sqlDB.QueryRow(
		`SELECT COUNT(*) FROM schedule_destinations WHERE destination_id = ?`, id,
	).Scan(&inUse)
	if err != nil {
		return fmt.Errorf("db: check destination in use: %w", err)
	}
	if inUse > 0 {
		return ErrDestinationInUse
	}

	res, err := sqlDB.Exec(`DELETE FROM destinations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("db: delete destination: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: delete destination: %w", err)
	}
	if n == 0 {
		return ErrDestinationNotFound
	}
	return nil
}
