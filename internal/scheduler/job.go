package scheduler

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"loxbak/internal/crypto"
	"loxbak/internal/db"
	"loxbak/internal/destinations"
	"loxbak/internal/ftp"
)

// runTimeout bounds a single scheduled backup run end-to-end (mirror +
// archive + store to every destination).
const runTimeout = 2 * time.Hour

// mirrorMu serializes the sync-then-archive section of doRun across
// concurrent runs (a cron-triggered backup overlapping a manual "run now",
// say). All runs share one persistent mirror directory now, so two runs
// syncing/archiving it at once would race; the rest of doRun (destination
// uploads) doesn't touch that shared state and stays unserialized.
var mirrorMu sync.Mutex

// RunSchedule executes one full backup run for the given schedule id: it
// fetches the Miniserver's files via FTP, archives them into a single zip,
// hands that archive to every destination configured for the schedule, and
// records the outcome in the runs / run_destination_results tables. Used by
// the cron trigger, which waits for the result to log it.
//
// This is the "plumbing" version: each step is real, but error handling is
// simple by design at this stage — later passes can add retries, partial
// resume, etc.
func RunSchedule(ctx context.Context, sqlDB *sql.DB, masterKey string, scheduleID int64) error {
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	startedAt := time.Now().UTC()
	runID, err := db.CreateRun(sqlDB, &scheduleID, startedAt)
	if err != nil {
		return fmt.Errorf("scheduler: create run: %w", err)
	}

	runErr := executeAndFinish(ctx, sqlDB, masterKey, scheduleID, runID)
	if runErr != nil {
		slog.Error("scheduled backup run failed", "run_id", runID, "schedule_id", scheduleID, "error", runErr)
	}
	return runErr
}

// TriggerRun starts a backup for scheduleID immediately, outside its cron
// schedule (the API's "run now" action). Unlike RunSchedule, it creates the
// run row and returns its id right away, then executes the actual backup
// in the background — the caller doesn't block for however long the backup
// takes, and can poll GET /api/runs/{id} for progress.
func TriggerRun(sqlDB *sql.DB, masterKey string, scheduleID int64) (int64, error) {
	startedAt := time.Now().UTC()
	runID, err := db.CreateRun(sqlDB, &scheduleID, startedAt)
	if err != nil {
		return 0, fmt.Errorf("scheduler: create run: %w", err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		defer cancel()
		if err := executeAndFinish(ctx, sqlDB, masterKey, scheduleID, runID); err != nil {
			slog.Error("manual backup run failed", "run_id", runID, "schedule_id", scheduleID, "error", err)
		}
	}()

	return runID, nil
}

// executeAndFinish runs the backup for an already-created run row and
// records its outcome. Shared by the synchronous (cron) and asynchronous
// (manual trigger) paths above.
func executeAndFinish(ctx context.Context, sqlDB *sql.DB, masterKey string, scheduleID, runID int64) error {
	sizeBytes, filename, runErr := doRun(ctx, sqlDB, masterKey, scheduleID, runID)

	finishedAt := time.Now().UTC()
	status := "success"
	errMsg := ""
	if runErr != nil {
		status = "failed"
		errMsg = runErr.Error()
	}

	var sizePtr *int64
	if sizeBytes > 0 {
		sizePtr = &sizeBytes
	}
	if err := db.FinishRun(sqlDB, runID, finishedAt, status, sizePtr, filename, errMsg); err != nil {
		return fmt.Errorf("scheduler: finish run: %w", err)
	}

	return runErr
}

// doRun performs the actual work and returns the archive size, the archive
// filename (empty if no archive was produced), and any error. Per-
// destination outcomes are always recorded, even if some destinations fail
// while others succeed (a "partial" run) — see the status computation
// below.
func doRun(ctx context.Context, sqlDB *sql.DB, masterKey string, scheduleID, runID int64) (int64, string, error) {
	cred, err := db.GetCredential(sqlDB)
	if err != nil {
		return 0, "", fmt.Errorf("no Loxone credential stored: %w", err)
	}
	password, err := crypto.Decrypt(masterKey, cred.EncryptedPassword)
	if err != nil {
		return 0, "", fmt.Errorf("decrypt stored Loxone credential: %w", err)
	}

	links, err := db.ListScheduleDestinations(sqlDB, scheduleID)
	if err != nil {
		return 0, "", fmt.Errorf("list schedule destinations: %w", err)
	}
	if len(links) == 0 {
		return 0, "", fmt.Errorf("schedule has no destinations configured")
	}

	sched, err := db.GetSchedule(sqlDB, scheduleID)
	if err != nil {
		return 0, "", fmt.Errorf("get schedule: %w", err)
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}
	// Persistent, not per-run: Mirror only re-downloads what changed since
	// the last sync (rsync-style), so keeping this directory around across
	// runs is what makes that speedup possible. It's never deleted here.
	mirrorDir := filepath.Join(dataDir, "mirror")

	var archivePath string
	var size int64
	err = func() error {
		mirrorMu.Lock()
		defer mirrorMu.Unlock()

		if err := ftp.Mirror(ctx, cred.Host, cred.Port, cred.Username, string(password), mirrorDir); err != nil {
			return fmt.Errorf("mirror Miniserver files: %w", err)
		}

		var archErr error
		archivePath, size, archErr = archiveDir(mirrorDir, sched.Name)
		if archErr != nil {
			return fmt.Errorf("archive backup: %w", archErr)
		}
		return nil
	}()
	if err != nil {
		return 0, "", err
	}
	defer os.Remove(archivePath)

	filename := filepath.Base(archivePath)
	meta := destinations.BackupMeta{
		ScheduleName: sched.Name,
		Filename:     filename,
		Size:         size,
	}

	var anySuccess, anyFailure bool
	for _, link := range links {
		destErr := storeToDestination(ctx, sqlDB, masterKey, link.DestinationID, archivePath, meta, link.RetentionCount)

		status := "success"
		errMsg := ""
		if destErr != nil {
			status = "failed"
			errMsg = destErr.Error()
			anyFailure = true
		} else {
			anySuccess = true
		}

		destID := link.DestinationID
		if _, err := db.CreateRunDestinationResult(sqlDB, runID, &destID, status, &size, errMsg); err != nil {
			slog.Error("failed to record run destination result", "run_id", runID, "destination_id", destID, "error", err)
		}
	}

	if anyFailure && anySuccess {
		return size, filename, fmt.Errorf("backup partially failed: some destinations succeeded, others failed")
	}
	if anyFailure {
		return size, filename, fmt.Errorf("backup failed for all destinations")
	}
	return size, filename, nil
}

func storeToDestination(ctx context.Context, sqlDB *sql.DB, masterKey string, destinationID int64, archivePath string, meta destinations.BackupMeta, retentionCount int) error {
	destRow, err := db.GetDestination(sqlDB, destinationID)
	if err != nil {
		return fmt.Errorf("get destination: %w", err)
	}

	var secret []byte
	if len(destRow.EncryptedSecret) > 0 {
		secret, err = crypto.Decrypt(masterKey, destRow.EncryptedSecret)
		if err != nil {
			return fmt.Errorf("decrypt destination secret: %w", err)
		}
	}

	dest, err := destinations.Build(destRow.Type, destRow.Name, destRow.ConfigJSON, secret)
	if err != nil {
		return fmt.Errorf("build destination: %w", err)
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	if err := dest.Store(ctx, meta, f); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	pruneOldBackups(ctx, sqlDB, dest, destinationID, meta.ScheduleName, retentionCount)

	return nil
}

// pruneOldBackups enforces a schedule-destination's retention_count, if any
// and if the destination supports it (see destinations.Lister) — a
// destination type that doesn't implement Lister just means retention_count
// is stored but has no effect for it, not an error. Pruning failures are
// logged rather than propagated: the backup itself already succeeded, and a
// retention hiccup shouldn't be reported as a run failure.
func pruneOldBackups(ctx context.Context, sqlDB *sql.DB, dest destinations.Destination, destinationID int64, scheduleName string, retentionCount int) {
	if retentionCount <= 0 {
		return
	}
	lister, ok := dest.(destinations.Lister)
	if !ok {
		slog.Debug("retention set but destination doesn't support pruning", "destination_id", destinationID, "keep", retentionCount)
		return
	}

	prefix := SanitizeName(scheduleName) + "-"
	deleted, err := destinations.Prune(ctx, lister, prefix, retentionCount)
	if err != nil {
		slog.Warn("failed to prune old backups", "destination_id", destinationID, "filename_prefix", prefix, "keep", retentionCount, "error", err)
	}
	slog.Info("retention prune finished", "destination_id", destinationID, "filename_prefix", prefix, "keep", retentionCount, "deleted_count", len(deleted), "deleted", deleted)
	for _, filename := range deleted {
		if err := db.DeleteRunDestinationResultByFilename(sqlDB, destinationID, filename); err != nil {
			slog.Warn("failed to clean up run history for pruned backup", "destination_id", destinationID, "filename", filename, "error", err)
		}
	}
}

// ArchiveTimestampLayout is the time.Parse/Format layout embedded in every
// archive filename (see archiveDir) — exported so the API layer can parse
// it back out with ParseArchiveTimestamp when deriving a "Started" time for
// a backup that has no matching run history row.
const ArchiveTimestampLayout = "20060102T150405Z"

var archiveFilenameRe = regexp.MustCompile(`-(\d{8}T\d{6}Z)\.zip$`)

// ParseArchiveTimestamp extracts the timestamp encoded in an archive
// filename produced by archiveDir
// (<SanitizeName(scheduleName)>-<timestamp>.zip). Used by the Backups page
// to derive a "Started" time for entries with no matching run history row
// — pruned, or predating loxbak tracking runs at all.
func ParseArchiveTimestamp(filename string) (time.Time, bool) {
	m := archiveFilenameRe.FindStringSubmatch(filename)
	if m == nil {
		return time.Time{}, false
	}
	t, err := time.Parse(ArchiveTimestampLayout, m[1])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// archiveDir zips the contents of dir into a new temp file named after
// scheduleName and the current time, and returns its path and size.
func archiveDir(dir, scheduleName string) (string, int64, error) {
	filename := fmt.Sprintf("%s-%s.zip", SanitizeName(scheduleName), time.Now().UTC().Format(ArchiveTimestampLayout))
	archivePath := filepath.Join(os.TempDir(), filename)

	f, err := os.Create(archivePath)
	if err != nil {
		return "", 0, fmt.Errorf("create archive file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		w, err := zw.Create(filepath.ToSlash(relPath))
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		if _, err := io.Copy(w, src); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		zw.Close()
		return "", 0, fmt.Errorf("walk mirrored files: %w", err)
	}

	if err := zw.Close(); err != nil {
		return "", 0, fmt.Errorf("close archive: %w", err)
	}

	info, err := os.Stat(archivePath)
	if err != nil {
		return "", 0, fmt.Errorf("stat archive: %w", err)
	}

	return archivePath, info.Size(), nil
}

// SanitizeName strips a human-provided name (a schedule's name) down to the
// characters safe to use in an archive filename, exported so the API layer
// can reverse-match a stored archive's filename prefix back to a schedule.
func SanitizeName(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "backup"
	}
	return string(out)
}
