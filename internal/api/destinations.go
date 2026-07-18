package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"loxbak/internal/crypto"
	"loxbak/internal/db"
	"loxbak/internal/destinations"
)

type destinationRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	ConfigJSON string `json:"config_json"`
	// Secret is an opaque credential associated with this destination
	// (e.g. a WebDAV password). Never returned in responses.
	Secret string `json:"secret,omitempty"`
}

type destinationResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	ConfigJSON string `json:"config_json"`
	HasSecret  bool   `json:"has_secret"`
}

func toDestinationResponse(d db.Destination) destinationResponse {
	return destinationResponse{
		ID:         d.ID,
		Name:       d.Name,
		Type:       d.Type,
		ConfigJSON: d.ConfigJSON,
		HasSecret:  len(d.EncryptedSecret) > 0,
	}
}

// handleListDestinations handles GET /api/destinations.
func (s *Server) handleListDestinations(w http.ResponseWriter, r *http.Request) {
	dests, err := db.ListDestinations(s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list destinations")
		return
	}

	out := make([]destinationResponse, 0, len(dests))
	for _, d := range dests {
		out = append(out, toDestinationResponse(d))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCreateDestination handles POST /api/destinations.
func (s *Server) handleCreateDestination(w http.ResponseWriter, r *http.Request) {
	var req destinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}

	encryptedSecret, err := s.encryptDestinationSecret(req.Secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt secret")
		return
	}

	id, err := db.CreateDestination(s.DB, req.Name, req.Type, req.ConfigJSON, encryptedSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create destination")
		return
	}

	d, err := db.GetDestination(s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load created destination")
		return
	}
	writeJSON(w, http.StatusCreated, toDestinationResponse(*d))
}

// handleGetDestination handles GET /api/destinations/{id}.
func (s *Server) handleGetDestination(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination id")
		return
	}

	d, err := db.GetDestination(s.DB, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "destination not found")
		return
	}
	writeJSON(w, http.StatusOK, toDestinationResponse(*d))
}

// handleUpdateDestination handles PUT /api/destinations/{id}.
func (s *Server) handleUpdateDestination(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination id")
		return
	}

	var req destinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}

	// Preserve the existing secret unless a new one was provided.
	existing, err := db.GetDestination(s.DB, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "destination not found")
		return
	}
	encryptedSecret := existing.EncryptedSecret
	if req.Secret != "" {
		encryptedSecret, err = s.encryptDestinationSecret(req.Secret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt secret")
			return
		}
	}

	if err := db.UpdateDestination(s.DB, id, req.Name, req.Type, req.ConfigJSON, encryptedSecret); err != nil {
		writeError(w, http.StatusNotFound, "destination not found")
		return
	}

	d, err := db.GetDestination(s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load updated destination")
		return
	}
	writeJSON(w, http.StatusOK, toDestinationResponse(*d))
}

// handleDeleteDestination handles DELETE /api/destinations/{id}, or
// DELETE /api/destinations/{id}?purge=true to also delete every backup
// currently stored there (via destinations.Lister) rather than just
// removing loxbak's own record of the destination and leaving its
// contents behind.
func (s *Server) handleDeleteDestination(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination id")
		return
	}
	purge := r.URL.Query().Get("purge") == "true"

	// Build the destination (if purging) before the DB row disappears —
	// buildDestination needs the row's config/secret.
	var dest destinations.Destination
	if purge {
		if destRow, err := db.GetDestination(s.DB, id); err == nil {
			dest, err = s.buildDestination(*destRow)
			if err != nil {
				dest = nil
				writeErrorLog("failed to build destination for purge", err)
			}
		}
	}

	if err := db.DeleteDestination(s.DB, id); err != nil {
		if errors.Is(err, db.ErrDestinationInUse) {
			writeError(w, http.StatusConflict, "destination is used by one or more schedules — remove it from those schedules first")
			return
		}
		writeError(w, http.StatusNotFound, "destination not found")
		return
	}

	if purge && dest != nil {
		if lister, ok := dest.(destinations.Lister); ok {
			entries, err := lister.List(r.Context())
			if err != nil {
				writeErrorLog("failed to list backups for purge", err)
			}
			for _, e := range entries {
				if err := lister.Delete(r.Context(), e.Filename); err != nil {
					writeErrorLog("failed to purge backup", err)
				}
			}
		}
	}

	s.reloadScheduler()

	w.WriteHeader(http.StatusNoContent)
}

// handleTestDestination handles POST /api/destinations/{id}/test. Stubbed
// for this scaffolding pass — a real connectivity check comes later.
func (s *Server) handleTestDestination(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "destination connectivity test not yet implemented")
}

func (s *Server) encryptDestinationSecret(secret string) ([]byte, error) {
	if secret == "" {
		return nil, nil
	}
	return crypto.Encrypt(s.MasterKey, []byte(secret))
}

// buildDestination decrypts d's stored secret and constructs the live
// destinations.Destination for it — shared by the Backups page's
// list/download/delete handlers and destination purge-on-delete, all of
// which need to actually talk to the destination rather than just read its
// DB row.
func (s *Server) buildDestination(d db.Destination) (destinations.Destination, error) {
	var secret []byte
	if len(d.EncryptedSecret) > 0 {
		var err error
		secret, err = crypto.Decrypt(s.MasterKey, d.EncryptedSecret)
		if err != nil {
			return nil, fmt.Errorf("decrypt destination secret: %w", err)
		}
	}
	return destinations.Build(d.Type, d.Name, d.ConfigJSON, secret)
}
