package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"loxbak/internal/db"
	"loxbak/internal/scheduler"
)

type scheduleDestinationInput struct {
	DestinationID  int64 `json:"destination_id"`
	RetentionCount int   `json:"retention_count"`
}

type scheduleRequest struct {
	Name         string                     `json:"name"`
	CronExpr     string                     `json:"cron_expr"`
	Enabled      bool                       `json:"enabled"`
	Destinations []scheduleDestinationInput `json:"destinations"`
}

type scheduleResponse struct {
	ID           int64                      `json:"id"`
	Name         string                     `json:"name"`
	CronExpr     string                     `json:"cron_expr"`
	Enabled      bool                       `json:"enabled"`
	Destinations []scheduleDestinationInput `json:"destinations"`
}

func toScheduleResponse(sched db.Schedule, links []db.ScheduleDestination) scheduleResponse {
	dests := make([]scheduleDestinationInput, 0, len(links))
	for _, l := range links {
		dests = append(dests, scheduleDestinationInput{
			DestinationID:  l.DestinationID,
			RetentionCount: l.RetentionCount,
		})
	}
	return scheduleResponse{
		ID:           sched.ID,
		Name:         sched.Name,
		CronExpr:     sched.CronExpr,
		Enabled:      sched.Enabled,
		Destinations: dests,
	}
}

// handleListSchedules handles GET /api/schedules.
func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := db.ListSchedules(s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list schedules")
		return
	}

	out := make([]scheduleResponse, 0, len(schedules))
	for _, sched := range schedules {
		links, err := db.ListScheduleDestinations(s.DB, sched.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list schedule destinations")
			return
		}
		out = append(out, toScheduleResponse(sched, links))
	}

	writeJSON(w, http.StatusOK, out)
}

// handleCreateSchedule handles POST /api/schedules.
func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.CronExpr == "" {
		writeError(w, http.StatusBadRequest, "name and cron_expr are required")
		return
	}

	id, err := db.CreateSchedule(s.DB, req.Name, req.CronExpr, req.Enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create schedule")
		return
	}

	if err := s.setScheduleDestinations(id, req.Destinations); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set schedule destinations")
		return
	}

	s.reloadScheduler()

	sched, err := db.GetSchedule(s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load created schedule")
		return
	}
	links, _ := db.ListScheduleDestinations(s.DB, id)
	writeJSON(w, http.StatusCreated, toScheduleResponse(*sched, links))
}

// handleGetSchedule handles GET /api/schedules/{id}.
func (s *Server) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	sched, err := db.GetSchedule(s.DB, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}
	links, err := db.ListScheduleDestinations(s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list schedule destinations")
		return
	}

	writeJSON(w, http.StatusOK, toScheduleResponse(*sched, links))
}

// handleUpdateSchedule handles PUT /api/schedules/{id}.
func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	var req scheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.CronExpr == "" {
		writeError(w, http.StatusBadRequest, "name and cron_expr are required")
		return
	}

	if err := db.UpdateSchedule(s.DB, id, req.Name, req.CronExpr, req.Enabled); err != nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}
	if err := s.setScheduleDestinations(id, req.Destinations); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set schedule destinations")
		return
	}

	s.reloadScheduler()

	sched, err := db.GetSchedule(s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load updated schedule")
		return
	}
	links, _ := db.ListScheduleDestinations(s.DB, id)
	writeJSON(w, http.StatusOK, toScheduleResponse(*sched, links))
}

// handleDeleteSchedule handles DELETE /api/schedules/{id}.
func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	if err := db.DeleteSchedule(s.DB, id); err != nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	s.reloadScheduler()

	w.WriteHeader(http.StatusNoContent)
}

// handleRunSchedule handles POST /api/schedules/{id}/run: triggers an
// immediate backup for this schedule, outside its cron schedule. The
// backup runs in the background; the response carries the new run's id
// (status "running") so the client can poll GET /api/runs/{id}.
func (s *Server) handleRunSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	if _, err := db.GetSchedule(s.DB, id); err != nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}

	runID, err := scheduler.TriggerRun(s.DB, s.MasterKey, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start backup run")
		return
	}

	run, err := db.GetRun(s.DB, runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load created run")
		return
	}

	writeJSON(w, http.StatusAccepted, toRunResponse(*run, nil))
}

func (s *Server) setScheduleDestinations(scheduleID int64, inputs []scheduleDestinationInput) error {
	links := make([]db.ScheduleDestination, 0, len(inputs))
	for _, in := range inputs {
		links = append(links, db.ScheduleDestination{
			ScheduleID:     scheduleID,
			DestinationID:  in.DestinationID,
			RetentionCount: in.RetentionCount,
		})
	}
	return db.SetScheduleDestinations(s.DB, scheduleID, links)
}

// reloadScheduler re-syncs the running scheduler after a CRUD change to
// schedules. Logged rather than surfaced to the client — the CRUD change
// itself already succeeded.
func (s *Server) reloadScheduler() {
	if s.Scheduler == nil {
		return
	}
	if err := s.Scheduler.Reload(); err != nil {
		writeErrorLog("failed to reload scheduler", err)
	}
}

// parseIDParam extracts and parses the {id} path value present on this
// route.
func parseIDParam(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
