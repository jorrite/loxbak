// Package scheduler wraps github.com/robfig/cron/v3 to run backups on the
// cron expressions stored per-schedule in the database.
package scheduler

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"

	"loxbak/internal/db"
)

// Scheduler loads enabled schedules from the database into cron entries and
// keeps them in sync with the schedules table.
type Scheduler struct {
	cron      *cron.Cron
	sqlDB     *sql.DB
	masterKey string

	mu      sync.Mutex
	entries map[int64]cron.EntryID // scheduleID -> cron entry id
}

// New constructs a Scheduler. Call Reload to populate it from the DB, then
// Start to begin running jobs.
func New(sqlDB *sql.DB, masterKey string) *Scheduler {
	return &Scheduler{
		cron:      cron.New(),
		sqlDB:     sqlDB,
		masterKey: masterKey,
		entries:   make(map[int64]cron.EntryID),
	}
}

// Start begins running the underlying cron scheduler in its own goroutine.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop halts the scheduler. It does not interrupt any run already in
// progress.
func (s *Scheduler) Stop(ctx context.Context) {
	stopCtx := s.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-ctx.Done():
	}
}

// Reload re-syncs cron entries from the schedules table: every existing
// entry is removed and enabled schedules are re-added with their current
// cron expression. Call this after any CRUD change to schedules.
func (s *Scheduler) Reload() error {
	schedules, err := db.ListEnabledSchedules(s.sqlDB)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, entryID := range s.entries {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}

	for _, sched := range schedules {
		scheduleID := sched.ID
		entryID, err := s.cron.AddFunc(sched.CronExpr, func() {
			if err := RunSchedule(context.Background(), s.sqlDB, s.masterKey, scheduleID); err != nil {
				slog.Error("scheduled backup run failed", "schedule_id", scheduleID, "error", err)
			}
		})
		if err != nil {
			slog.Error("failed to schedule cron entry", "schedule_id", scheduleID, "cron_expr", sched.CronExpr, "error", err)
			continue
		}
		s.entries[scheduleID] = entryID
	}

	return nil
}
