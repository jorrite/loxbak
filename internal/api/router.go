package api

import (
	"encoding/json"
	"net/http"

	"loxbak/internal/web"
)

// Router builds the full route table using Go 1.22+ net/http pattern
// routing. Static frontend files are served as the catch-all.
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)

	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.requireSession(s.handleMe))

	mux.HandleFunc("GET /api/schedules", s.requireSession(s.handleListSchedules))
	mux.HandleFunc("POST /api/schedules", s.requireSession(s.handleCreateSchedule))
	mux.HandleFunc("GET /api/schedules/{id}", s.requireSession(s.handleGetSchedule))
	mux.HandleFunc("PUT /api/schedules/{id}", s.requireSession(s.handleUpdateSchedule))
	mux.HandleFunc("DELETE /api/schedules/{id}", s.requireSession(s.handleDeleteSchedule))
	mux.HandleFunc("POST /api/schedules/{id}/run", s.requireSession(s.handleRunSchedule))

	mux.HandleFunc("GET /api/destinations", s.requireSession(s.handleListDestinations))
	mux.HandleFunc("POST /api/destinations", s.requireSession(s.handleCreateDestination))
	mux.HandleFunc("GET /api/destinations/{id}", s.requireSession(s.handleGetDestination))
	mux.HandleFunc("PUT /api/destinations/{id}", s.requireSession(s.handleUpdateDestination))
	mux.HandleFunc("DELETE /api/destinations/{id}", s.requireSession(s.handleDeleteDestination))
	mux.HandleFunc("POST /api/destinations/{id}/test", s.requireSession(s.handleTestDestination))

	mux.HandleFunc("GET /api/runs", s.requireSession(s.handleListRuns))
	mux.HandleFunc("GET /api/runs/{id}", s.requireSession(s.handleGetRun))

	mux.HandleFunc("GET /api/backups", s.requireSession(s.handleListBackups))
	mux.HandleFunc("GET /api/backups/{destinationId}/{filename}/download", s.requireSession(s.handleDownloadBackup))
	mux.HandleFunc("DELETE /api/backups/{destinationId}/{filename}", s.requireSession(s.handleDeleteBackup))

	// Catch-all: serve the embedded, statically-exported frontend.
	mux.Handle("/", web.Handler())

	return mux
}

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// apiError is the JSON shape of an error response.
type apiError struct {
	Error string `json:"error"`
}

// writeError writes a JSON error response of the form {"error": message}.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}
