package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Run is a row in the runs table: one execution of a schedule.
type Run struct {
	ID         int64
	ScheduleID sql.NullInt64
	StartedAt  time.Time
	FinishedAt sql.NullTime
	Status     string
	SizeBytes  sql.NullInt64
	Error      sql.NullString
	Filename   sql.NullString
}

// RunDestinationResult is a row in run_destination_results: the outcome of
// storing a single run's archive to one configured destination.
type RunDestinationResult struct {
	ID            int64
	RunID         int64
	DestinationID sql.NullInt64
	Status        string
	SizeBytes     sql.NullInt64
	Error         sql.NullString
}

// ErrRunNotFound is returned when a run id has no matching row.
var ErrRunNotFound = errors.New("db: run not found")

// CreateRun inserts a new run row with status "running" and returns its id.
func CreateRun(sqlDB *sql.DB, scheduleID *int64, startedAt time.Time) (int64, error) {
	var sid sql.NullInt64
	if scheduleID != nil {
		sid = sql.NullInt64{Int64: *scheduleID, Valid: true}
	}
	res, err := sqlDB.Exec(
		`INSERT INTO runs (schedule_id, started_at, status) VALUES (?, ?, 'running')`,
		sid, startedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("db: create run: %w", err)
	}
	return res.LastInsertId()
}

// FinishRun marks a run as finished with the given status, size, archive
// filename (empty if no archive was produced, e.g. the mirror step itself
// failed), and optional error message.
func FinishRun(sqlDB *sql.DB, id int64, finishedAt time.Time, status string, sizeBytes *int64, filename, runErr string) error {
	var size sql.NullInt64
	if sizeBytes != nil {
		size = sql.NullInt64{Int64: *sizeBytes, Valid: true}
	}
	var filenameStr sql.NullString
	if filename != "" {
		filenameStr = sql.NullString{String: filename, Valid: true}
	}
	var errStr sql.NullString
	if runErr != "" {
		errStr = sql.NullString{String: runErr, Valid: true}
	}
	res, err := sqlDB.Exec(
		`UPDATE runs SET finished_at = ?, status = ?, size_bytes = ?, filename = ?, error = ? WHERE id = ?`,
		finishedAt, status, size, filenameStr, errStr, id,
	)
	if err != nil {
		return fmt.Errorf("db: finish run: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: finish run: %w", err)
	}
	if n == 0 {
		return ErrRunNotFound
	}
	return nil
}

// GetRun returns a single run by id.
func GetRun(sqlDB *sql.DB, id int64) (*Run, error) {
	var r Run
	err := sqlDB.QueryRow(
		`SELECT id, schedule_id, started_at, finished_at, status, size_bytes, error, filename
		   FROM runs WHERE id = ?`, id,
	).Scan(&r.ID, &r.ScheduleID, &r.StartedAt, &r.FinishedAt, &r.Status, &r.SizeBytes, &r.Error, &r.Filename)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: get run: %w", err)
	}
	return &r, nil
}

// ListRuns returns runs ordered newest-first, optionally filtered by
// scheduleID, with limit/offset pagination.
func ListRuns(sqlDB *sql.DB, scheduleID *int64, limit, offset int) ([]Run, error) {
	var rows *sql.Rows
	var err error
	if scheduleID != nil {
		rows, err = sqlDB.Query(
			`SELECT id, schedule_id, started_at, finished_at, status, size_bytes, error, filename
			   FROM runs WHERE schedule_id = ? ORDER BY started_at DESC LIMIT ? OFFSET ?`,
			*scheduleID, limit, offset,
		)
	} else {
		rows, err = sqlDB.Query(
			`SELECT id, schedule_id, started_at, finished_at, status, size_bytes, error, filename
			   FROM runs ORDER BY started_at DESC LIMIT ? OFFSET ?`,
			limit, offset,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("db: list runs: %w", err)
	}
	defer rows.Close()

	var out []Run
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.ID, &r.ScheduleID, &r.StartedAt, &r.FinishedAt, &r.Status, &r.SizeBytes, &r.Error, &r.Filename); err != nil {
			return nil, fmt.Errorf("db: scan run: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list runs: %w", err)
	}
	return out, nil
}

// CreateRunDestinationResult inserts a per-destination result row for a run.
func CreateRunDestinationResult(sqlDB *sql.DB, runID int64, destinationID *int64, status string, sizeBytes *int64, resultErr string) (int64, error) {
	var did sql.NullInt64
	if destinationID != nil {
		did = sql.NullInt64{Int64: *destinationID, Valid: true}
	}
	var size sql.NullInt64
	if sizeBytes != nil {
		size = sql.NullInt64{Int64: *sizeBytes, Valid: true}
	}
	var errStr sql.NullString
	if resultErr != "" {
		errStr = sql.NullString{String: resultErr, Valid: true}
	}
	res, err := sqlDB.Exec(
		`INSERT INTO run_destination_results (run_id, destination_id, status, size_bytes, error)
		 VALUES (?, ?, ?, ?, ?)`,
		runID, did, status, size, errStr,
	)
	if err != nil {
		return 0, fmt.Errorf("db: create run destination result: %w", err)
	}
	return res.LastInsertId()
}

// ListRunDestinationResults returns all per-destination results for a run.
func ListRunDestinationResults(sqlDB *sql.DB, runID int64) ([]RunDestinationResult, error) {
	rows, err := sqlDB.Query(
		`SELECT id, run_id, destination_id, status, size_bytes, error
		   FROM run_destination_results WHERE run_id = ? ORDER BY id`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("db: list run destination results: %w", err)
	}
	defer rows.Close()

	var out []RunDestinationResult
	for rows.Next() {
		var r RunDestinationResult
		if err := rows.Scan(&r.ID, &r.RunID, &r.DestinationID, &r.Status, &r.SizeBytes, &r.Error); err != nil {
			return nil, fmt.Errorf("db: scan run destination result: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list run destination results: %w", err)
	}
	return out, nil
}

// DeleteRunDestinationResultByFilename removes the run_destination_results
// row for (destinationID, filename) — used after a destination prunes one
// of its own older backups during retention enforcement, so the Backups
// page stops offering a download link for a file that no longer exists.
// The run's own history row (started_at, status, size, ...) is left
// alone; only this one destination's now-gone copy is forgotten.
func DeleteRunDestinationResultByFilename(sqlDB *sql.DB, destinationID int64, filename string) error {
	_, err := sqlDB.Exec(
		`DELETE FROM run_destination_results
		   WHERE destination_id = ?
		     AND run_id IN (SELECT id FROM runs WHERE filename = ?)`,
		destinationID, filename,
	)
	if err != nil {
		return fmt.Errorf("db: delete run destination result by filename: %w", err)
	}
	return nil
}

// RunInfo is the subset of a run's fields relevant to enriching a
// destination-listed backup entry with schedule/timing info.
type RunInfo struct {
	ScheduleID sql.NullInt64
	StartedAt  time.Time
	FinishedAt sql.NullTime
}

// FindRunInfoByDestinationAndFilename looks up the run that produced the
// archive at (destinationID, filename), if loxbak's own history still has
// a record of it. Returns (nil, nil) on no match — used by the Backups
// page to show accurate schedule/started/duration for a destination-listed
// backup, falling back to filename-derived best-effort info when no run
// row matches (a backup that predates run tracking, or whose
// run_destination_results row was since pruned).
func FindRunInfoByDestinationAndFilename(sqlDB *sql.DB, destinationID int64, filename string) (*RunInfo, error) {
	var info RunInfo
	err := sqlDB.QueryRow(
		`SELECT r.schedule_id, r.started_at, r.finished_at
		   FROM run_destination_results rdr
		   JOIN runs r ON r.id = rdr.run_id
		  WHERE rdr.destination_id = ? AND r.filename = ?
		  LIMIT 1`,
		destinationID, filename,
	).Scan(&info.ScheduleID, &info.StartedAt, &info.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: find run info by destination and filename: %w", err)
	}
	return &info, nil
}
