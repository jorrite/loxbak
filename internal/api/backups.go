package api

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"loxbak/internal/db"
	"loxbak/internal/destinations"
	"loxbak/internal/scheduler"
)

// backupEntryResponse is one archive actually present at a destination —
// the Backups page's unit of display, one row per (destination, filename)
// rather than one row per run. A schedule with two destinations produces
// two of these per run.
type backupEntryResponse struct {
	DestinationID int64      `json:"destination_id"`
	Filename      string     `json:"filename"`
	SizeBytes     int64      `json:"size_bytes"`
	ScheduleID    *int64     `json:"schedule_id,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Downloadable  bool       `json:"downloadable"`
}

// handleListBackups handles GET /api/backups: for every destination that
// supports listing (destinations.Lister), lists what's actually stored
// there — not loxbak's run history — so it surfaces backups that predate
// run tracking or whose history row was since pruned, and reflects
// deletions/uploads made outside loxbak too. Each entry is enriched with
// its producing run's schedule/timing when a match still exists in
// history, falling back to filename-derived best-effort info otherwise.
func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	destRows, err := db.ListDestinations(s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list destinations")
		return
	}
	schedules, err := db.ListSchedules(s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list schedules")
		return
	}

	var out []backupEntryResponse
	for _, destRow := range destRows {
		dest, err := s.buildDestination(destRow)
		if err != nil {
			writeErrorLog("failed to build destination for backups list", err)
			continue
		}
		lister, ok := dest.(destinations.Lister)
		if !ok {
			continue
		}
		_, downloadable := dest.(destinations.Reader)

		entries, err := lister.List(r.Context())
		if err != nil {
			writeErrorLog("failed to list backups for destination", err)
			continue
		}

		for _, e := range entries {
			resp := backupEntryResponse{
				DestinationID: destRow.ID,
				Filename:      e.Filename,
				SizeBytes:     e.Size,
				StartedAt:     e.ModTime,
				Downloadable:  downloadable,
			}

			if info, err := db.FindRunInfoByDestinationAndFilename(s.DB, destRow.ID, e.Filename); err == nil && info != nil {
				if info.ScheduleID.Valid {
					v := info.ScheduleID.Int64
					resp.ScheduleID = &v
				}
				resp.StartedAt = info.StartedAt
				if info.FinishedAt.Valid {
					v := info.FinishedAt.Time
					resp.FinishedAt = &v
				}
			} else {
				// No run history for this file (predates tracking, or was
				// pruned) — best-effort schedule match by filename prefix,
				// and a truer "started" than mtime if the timestamp this
				// app itself embeds in the filename parses out.
				for _, sched := range schedules {
					if hasArchivePrefix(e.Filename, sched.Name) {
						id := sched.ID
						resp.ScheduleID = &id
						break
					}
				}
				if t, ok := scheduler.ParseArchiveTimestamp(e.Filename); ok {
					resp.StartedAt = t
				}
			}

			out = append(out, resp)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })

	writeJSON(w, http.StatusOK, out)
}

func hasArchivePrefix(filename, scheduleName string) bool {
	return strings.HasPrefix(filename, scheduler.SanitizeName(scheduleName)+"-")
}

// handleDownloadBackup handles GET /api/backups/{destinationId}/{filename}/download,
// streaming the archive back from whichever destination it lives on — any
// destination implementing destinations.Reader, not just local.
func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	destID, err := strconv.ParseInt(r.PathValue("destinationId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination id")
		return
	}
	filename := r.PathValue("filename")
	if filename == "" {
		writeError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	destRow, err := db.GetDestination(s.DB, destID)
	if err != nil {
		writeError(w, http.StatusNotFound, "destination not found")
		return
	}
	dest, err := s.buildDestination(*destRow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build destination")
		return
	}
	reader, ok := dest.(destinations.Reader)
	if !ok {
		writeError(w, http.StatusNotFound, "this destination doesn't support downloading backups")
		return
	}

	rc, err := reader.Open(r.Context(), filename)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/zip")
	if _, err := io.Copy(w, rc); err != nil {
		writeErrorLog("failed to stream backup download", err)
	}
}

// handleDeleteBackup handles DELETE /api/backups/{destinationId}/{filename}:
// removes one backup archive from one destination directly, via
// destinations.Lister — works for any destination configured, including
// backups that predate loxbak tracking runs at all, since it doesn't go
// through run history to find what to delete.
func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	destID, err := strconv.ParseInt(r.PathValue("destinationId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination id")
		return
	}
	filename := r.PathValue("filename")
	if filename == "" {
		writeError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	destRow, err := db.GetDestination(s.DB, destID)
	if err != nil {
		writeError(w, http.StatusNotFound, "destination not found")
		return
	}
	dest, err := s.buildDestination(*destRow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build destination")
		return
	}
	lister, ok := dest.(destinations.Lister)
	if !ok {
		writeError(w, http.StatusNotFound, "this destination doesn't support removing backups")
		return
	}

	if err := lister.Delete(r.Context(), filename); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete backup")
		return
	}

	if err := db.DeleteRunDestinationResultByFilename(s.DB, destID, filename); err != nil {
		writeErrorLog("failed to clean up run history for deleted backup", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
