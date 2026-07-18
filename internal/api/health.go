package api

import "net/http"

// handleHealth handles GET /api/health. No auth required — used for
// container healthchecks and the done-criteria smoke test.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
