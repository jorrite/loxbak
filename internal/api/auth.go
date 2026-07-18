package api

import (
	"encoding/json"
	"net/http"

	"loxbak/internal/crypto"
	"loxbak/internal/db"
	"loxbak/internal/ftp"
	"loxbak/internal/session"
)

type loginRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type meResponse struct {
	Host     string `json:"host"`
	Username string `json:"username"`
}

// handleLogin handles POST /api/login. It validates the given Miniserver
// credentials with a real FTP login attempt; on success it stores the
// (encrypted) credential, opens a session, and sets the session cookie —
// there is no separate loxbak user/password, the Miniserver credentials
// are the login.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Host == "" || req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "host, username, and password are required")
		return
	}
	if req.Port == 0 {
		req.Port = 21
	}

	if err := ftp.Validate(r.Context(), req.Host, req.Port, req.Username, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	encryptedPassword, err := crypto.Encrypt(s.MasterKey, []byte(req.Password))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store credential")
		return
	}

	if err := db.UpsertCredential(s.DB, req.Host, req.Port, req.Username, encryptedPassword); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store credential")
		return
	}

	token, err := session.Create(s.DB, s.SessionKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	session.SetCookie(w, token)

	writeJSON(w, http.StatusOK, meResponse{Host: req.Host, Username: req.Username})
}

// handleLogout handles POST /api/logout: deletes the session row (if any)
// and clears the cookie. Always succeeds, even if there was no valid
// session to begin with.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(session.CookieName); err == nil {
		if id, ok := session.IDFromCookie(cookie.Value); ok {
			_ = db.DeleteSession(s.DB, id)
		}
	}
	session.ClearCookie(w)
	w.WriteHeader(http.StatusOK)
}

// handleMe handles GET /api/me: returns the stored Loxone credential's
// host/username if the session cookie is valid. Wrapped in requireSession,
// so by the time this runs the cookie has already been validated.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	cred, err := db.GetCredential(s.DB)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, meResponse{Host: cred.Host, Username: cred.Username})
}
