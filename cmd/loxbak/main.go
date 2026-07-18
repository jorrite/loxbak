// Command loxbak runs the loxbak backup server: HTTP API, scheduler, and
// (in the final build) the embedded static frontend, all in one process.
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"loxbak/internal/api"
	"loxbak/internal/db"
	"loxbak/internal/scheduler"
	"loxbak/internal/session"
)

const settingsSessionSecretKey = "session_signing_secret"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	masterKey := os.Getenv("MASTER_KEY")
	if masterKey == "" {
		slog.Error("MASTER_KEY environment variable is required but not set")
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		slog.Error("failed to create data directory", "dir", dataDir, "error", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(dataDir, "loxbak.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	sessionKey, err := ensureSessionSecret(sqlDB)
	if err != nil {
		slog.Error("failed to establish session signing secret", "error", err)
		os.Exit(1)
	}

	sched := scheduler.New(sqlDB, masterKey)
	if err := sched.Reload(); err != nil {
		slog.Error("failed to load schedules", "error", err)
		os.Exit(1)
	}
	sched.Start()

	server := api.New(sqlDB, masterKey, sessionKey, sched, ":"+port)

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sched.Stop(shutdownCtx)

	return server.Shutdown(shutdownCtx)
}

// ensureSessionSecret returns the persisted session-signing secret,
// generating and storing one on first run if it doesn't exist yet. Unlike
// MASTER_KEY, this is not env-configured — one less thing to set up.
func ensureSessionSecret(sqlDB *sql.DB) (string, error) {
	if existing, ok, err := db.GetSetting(sqlDB, settingsSessionSecretKey); err != nil {
		return "", err
	} else if ok {
		return existing, nil
	}

	secret, err := session.GenerateSecret()
	if err != nil {
		return "", err
	}
	if err := db.SetSetting(sqlDB, settingsSessionSecretKey, secret); err != nil {
		return "", err
	}
	return secret, nil
}
