// Package api implements loxbak's HTTP API: session auth, schedules,
// destinations, and run history, plus serving the built frontend.
package api

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"loxbak/internal/scheduler"
)

// Server bundles everything the HTTP handlers need.
type Server struct {
	DB         *sql.DB
	MasterKey  string
	SessionKey string
	Scheduler  *scheduler.Scheduler
	httpServer *http.Server
}

// New constructs a Server and its underlying http.Server, ready to
// ListenAndServe.
func New(sqlDB *sql.DB, masterKey, sessionKey string, sched *scheduler.Scheduler, addr string) *Server {
	s := &Server{
		DB:         sqlDB,
		MasterKey:  masterKey,
		SessionKey: sessionKey,
		Scheduler:  sched,
	}

	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           withLogging(withCORS(s.Router())),
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// ListenAndServe starts the HTTP server. It blocks until the server stops
// (returning http.ErrServerClosed on a graceful Shutdown).
func (s *Server) ListenAndServe() error {
	slog.Info("starting HTTP server", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server, waiting for in-flight
// requests to complete or ctx to be cancelled.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("api: shutdown: %w", err)
	}
	return nil
}
