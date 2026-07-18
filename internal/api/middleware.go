package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"loxbak/internal/session"
)

type contextKey string

const sessionContextKey contextKey = "session"

// withCORS allows cross-origin requests from a local dev frontend (e.g.
// `next dev` on :3000 talking to this server on :8080). It only reflects
// an Origin back when its host is localhost/127.0.0.1 — same-origin
// requests (the normal production case, where this binary serves the
// embedded frontend itself) don't send a matching Origin and are
// unaffected either way.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if u, err := url.Parse(origin); err == nil {
			host := u.Hostname()
			if host == "localhost" || host == "127.0.0.1" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// withLogging logs each request's method, path, status code, and duration.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration", time.Since(start),
		)
	})
}

// statusWriter captures the status code written so withLogging can report it.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// writeErrorLog logs an internal error that doesn't need to (or already
// has) surfaced to the client.
func writeErrorLog(msg string, err error) {
	slog.Error(msg, "error", err)
}

// requireSession is middleware that validates the signed session cookie and,
// on success, stores the resolved db.Session in the request context for
// handlers to use. On failure it responds 401 and does not call next.
func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, err := session.Validate(r, s.DB, s.SessionKey)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		ctx := context.WithValue(r.Context(), sessionContextKey, sess)
		next(w, r.WithContext(ctx))
	}
}
