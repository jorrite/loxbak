package api

import (
	"net/http"
	"strconv"
	"time"

	"loxbak/internal/db"
)

type runDestinationResultResponse struct {
	ID            int64  `json:"id"`
	DestinationID *int64 `json:"destination_id,omitempty"`
	Status        string `json:"status"`
	SizeBytes     *int64 `json:"size_bytes,omitempty"`
	Error         string `json:"error,omitempty"`
}

type runResponse struct {
	ID           int64                          `json:"id"`
	ScheduleID   *int64                         `json:"schedule_id,omitempty"`
	StartedAt    time.Time                      `json:"started_at"`
	FinishedAt   *time.Time                     `json:"finished_at,omitempty"`
	Status       string                         `json:"status"`
	SizeBytes    *int64                         `json:"size_bytes,omitempty"`
	Error        string                         `json:"error,omitempty"`
	Destinations []runDestinationResultResponse `json:"destinations,omitempty"`
}

func toRunResponse(run db.Run, results []db.RunDestinationResult) runResponse {
	resp := runResponse{
		ID:        run.ID,
		StartedAt: run.StartedAt,
		Status:    run.Status,
	}
	if run.ScheduleID.Valid {
		v := run.ScheduleID.Int64
		resp.ScheduleID = &v
	}
	if run.FinishedAt.Valid {
		v := run.FinishedAt.Time
		resp.FinishedAt = &v
	}
	if run.SizeBytes.Valid {
		v := run.SizeBytes.Int64
		resp.SizeBytes = &v
	}
	if run.Error.Valid {
		resp.Error = run.Error.String
	}

	for _, res := range results {
		rr := runDestinationResultResponse{
			ID:     res.ID,
			Status: res.Status,
		}
		if res.DestinationID.Valid {
			v := res.DestinationID.Int64
			rr.DestinationID = &v
		}
		if res.SizeBytes.Valid {
			v := res.SizeBytes.Int64
			rr.SizeBytes = &v
		}
		if res.Error.Valid {
			rr.Error = res.Error.String
		}
		resp.Destinations = append(resp.Destinations, rr)
	}

	return resp
}

// handleListRuns handles GET /api/runs?schedule_id=&limit=&offset=. This is
// run history (one row per execution, with its overall status) — for the
// backups actually stored and downloadable/removable, see
// GET /api/backups instead, which lists destinations' own contents rather
// than this table.
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var scheduleID *int64
	if v := q.Get("schedule_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid schedule_id")
			return
		}
		scheduleID = &id
	}

	limit := 50
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}

	offset := 0
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		offset = n
	}

	runs, err := db.ListRuns(s.DB, scheduleID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runs")
		return
	}

	out := make([]runResponse, 0, len(runs))
	for _, run := range runs {
		results, err := db.ListRunDestinationResults(s.DB, run.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list run destination results")
			return
		}
		out = append(out, toRunResponse(run, results))
	}

	writeJSON(w, http.StatusOK, out)
}

// handleGetRun handles GET /api/runs/{id}.
func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	run, err := db.GetRun(s.DB, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	results, err := db.ListRunDestinationResults(s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list run destination results")
		return
	}

	writeJSON(w, http.StatusOK, toRunResponse(*run, results))
}
