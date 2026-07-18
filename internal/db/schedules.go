package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Schedule is a row in the schedules table.
type Schedule struct {
	ID        int64
	Name      string
	CronExpr  string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ScheduleDestination links a schedule to a destination with its own
// retention policy (schedule_destinations row).
type ScheduleDestination struct {
	ID             int64
	ScheduleID     int64
	DestinationID  int64
	RetentionCount int
}

// ErrScheduleNotFound is returned when a schedule id has no matching row.
var ErrScheduleNotFound = errors.New("db: schedule not found")

// ListSchedules returns all schedules, ordered by id.
func ListSchedules(sqlDB *sql.DB) ([]Schedule, error) {
	rows, err := sqlDB.Query(
		`SELECT id, name, cron_expr, enabled, created_at, updated_at
		   FROM schedules ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list schedules: %w", err)
	}
	defer rows.Close()

	var out []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(&s.ID, &s.Name, &s.CronExpr, &s.Enabled, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("db: scan schedule: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list schedules: %w", err)
	}
	return out, nil
}

// ListEnabledSchedules returns only schedules with enabled = 1, used by the
// scheduler to (re)build its cron entries.
func ListEnabledSchedules(sqlDB *sql.DB) ([]Schedule, error) {
	rows, err := sqlDB.Query(
		`SELECT id, name, cron_expr, enabled, created_at, updated_at
		   FROM schedules WHERE enabled = 1 ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list enabled schedules: %w", err)
	}
	defer rows.Close()

	var out []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(&s.ID, &s.Name, &s.CronExpr, &s.Enabled, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("db: scan schedule: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list enabled schedules: %w", err)
	}
	return out, nil
}

// GetSchedule returns a single schedule by id.
func GetSchedule(sqlDB *sql.DB, id int64) (*Schedule, error) {
	var s Schedule
	err := sqlDB.QueryRow(
		`SELECT id, name, cron_expr, enabled, created_at, updated_at
		   FROM schedules WHERE id = ?`, id,
	).Scan(&s.ID, &s.Name, &s.CronExpr, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrScheduleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: get schedule: %w", err)
	}
	return &s, nil
}

// CreateSchedule inserts a new schedule and returns its id.
func CreateSchedule(sqlDB *sql.DB, name, cronExpr string, enabled bool) (int64, error) {
	res, err := sqlDB.Exec(
		`INSERT INTO schedules (name, cron_expr, enabled) VALUES (?, ?, ?)`,
		name, cronExpr, enabled,
	)
	if err != nil {
		return 0, fmt.Errorf("db: create schedule: %w", err)
	}
	return res.LastInsertId()
}

// UpdateSchedule updates an existing schedule's fields.
func UpdateSchedule(sqlDB *sql.DB, id int64, name, cronExpr string, enabled bool) error {
	res, err := sqlDB.Exec(
		`UPDATE schedules
		    SET name = ?, cron_expr = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE id = ?`,
		name, cronExpr, enabled, id,
	)
	if err != nil {
		return fmt.Errorf("db: update schedule: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: update schedule: %w", err)
	}
	if n == 0 {
		return ErrScheduleNotFound
	}
	return nil
}

// DeleteSchedule removes a schedule by id (cascades to schedule_destinations).
func DeleteSchedule(sqlDB *sql.DB, id int64) error {
	res, err := sqlDB.Exec(`DELETE FROM schedules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("db: delete schedule: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: delete schedule: %w", err)
	}
	if n == 0 {
		return ErrScheduleNotFound
	}
	return nil
}

// ListScheduleDestinations returns the destination links (with retention
// policy) for a given schedule.
func ListScheduleDestinations(sqlDB *sql.DB, scheduleID int64) ([]ScheduleDestination, error) {
	rows, err := sqlDB.Query(
		`SELECT id, schedule_id, destination_id, retention_count
		   FROM schedule_destinations WHERE schedule_id = ? ORDER BY id`,
		scheduleID,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list schedule destinations: %w", err)
	}
	defer rows.Close()

	var out []ScheduleDestination
	for rows.Next() {
		var sd ScheduleDestination
		if err := rows.Scan(&sd.ID, &sd.ScheduleID, &sd.DestinationID, &sd.RetentionCount); err != nil {
			return nil, fmt.Errorf("db: scan schedule destination: %w", err)
		}
		out = append(out, sd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list schedule destinations: %w", err)
	}
	return out, nil
}

// SetScheduleDestinations replaces the full set of destination links for a
// schedule with the given list, inside a transaction.
func SetScheduleDestinations(sqlDB *sql.DB, scheduleID int64, links []ScheduleDestination) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("db: set schedule destinations: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM schedule_destinations WHERE schedule_id = ?`, scheduleID); err != nil {
		return fmt.Errorf("db: clear schedule destinations: %w", err)
	}
	for _, l := range links {
		if _, err := tx.Exec(
			`INSERT INTO schedule_destinations (schedule_id, destination_id, retention_count)
			 VALUES (?, ?, ?)`,
			scheduleID, l.DestinationID, l.RetentionCount,
		); err != nil {
			return fmt.Errorf("db: insert schedule destination: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: set schedule destinations: %w", err)
	}
	return nil
}
