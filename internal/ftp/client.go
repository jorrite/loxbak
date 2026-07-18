// Package ftp wraps github.com/jlaffaye/ftp to talk to a Loxone Miniserver's
// FTP(S) server: validating credentials on login, and incrementally
// mirroring the Miniserver's project files down to a persistent local
// directory for archiving. Mirror is rsync-shaped on purpose — only
// changed files are re-downloaded and files gone from the Miniserver are
// pruned locally — so a scheduled run's FTP work is proportional to what
// changed since the last run, not the whole tree.
//
// Loxone Miniservers may require explicit TLS (FTPES / "AUTH TLS") rather
// than plain FTP or implicit FTPS, and firmware >= 16.1 disables FTP access
// by default. Both cases are handled/surfaced here.
package ftp

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	goftp "github.com/jlaffaye/ftp"
)

// DefaultTimeout bounds how long a single dial/login attempt may take.
const DefaultTimeout = 15 * time.Second

// Connect dials an FTP server, preferring explicit TLS (FTPES) and falling
// back to a plain connection. Exported so internal/destinations' FTP
// destination (uploading to a remote FTP server) can reuse the same
// dial/TLS-negotiation logic used to fetch from the Miniserver, rather than
// duplicating it.
func Connect(ctx context.Context, addr string) (*goftp.ServerConn, error) {
	// Try explicit TLS first.
	//nolint:gosec // InsecureSkipVerify: Miniservers typically use a
	// self-signed or LAN-only certificate; there is no public CA chain to
	// validate against for a local appliance.
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	conn, err := goftp.Dial(addr,
		goftp.DialWithContext(ctx),
		goftp.DialWithTimeout(DefaultTimeout),
		goftp.DialWithExplicitTLS(tlsConfig),
	)
	if err == nil {
		return conn, nil
	}
	tlsErr := err

	// Fall back to a plain (non-TLS) connection for older Miniservers /
	// firmware that doesn't support AUTH TLS at all.
	conn, err = goftp.Dial(addr,
		goftp.DialWithContext(ctx),
		goftp.DialWithTimeout(DefaultTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("connect (tried explicit TLS: %v, then plain): %w", tlsErr, err)
	}
	return conn, nil
}

// classifyConnectErr turns a low-level dial error into a message that
// distinguishes "FTP is disabled on the Miniserver" from other network
// failures, since firmware >= 16.1 disables FTP by default and this is by
// far the most common misconfiguration users will hit. Classification is
// done via substring matching on the underlying error text, which is
// portable across the platforms this project targets (linux/amd64,
// linux/arm64, darwin for local dev) without depending on syscall-specific
// error types.
func classifyConnectErr(err error) error {
	msg := strings.ToLower(err.Error())
	if containsAny(msg, "connection refused", "i/o timeout", "no route to host", "timeout") {
		return fmt.Errorf(
			"couldn't reach the Miniserver's FTP service — check that FTP access is enabled in the Miniserver's settings (disabled by default since firmware 16.1): %w",
			err,
		)
	}
	return fmt.Errorf("connect to Miniserver: %w", err)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// Validate attempts a real FTP login against the Miniserver and returns a
// clear, wrapped error distinguishing "FTP disabled / unreachable" from
// "bad credentials".
func Validate(ctx context.Context, host string, port int, username, password string) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	conn, err := Connect(ctx, addr)
	if err != nil {
		return classifyConnectErr(err)
	}
	defer conn.Quit()

	if err := conn.Login(username, password); err != nil {
		return fmt.Errorf("login failed — check the Miniserver username and password: %w", err)
	}

	return nil
}

// Mirror incrementally syncs the Miniserver's FTP filesystem into localDir,
// which is expected to persist across calls (a temp dir defeats the point).
// Unchanged files (same size and mtime, within mtimeTolerance) are left
// alone rather than re-downloaded, and files/directories no longer present
// on the Miniserver are pruned from localDir — the same
// download-only-what-changed, delete-what's-gone shape as `rsync`. The
// caller is responsible for archiving localDir's contents after Mirror
// returns; unlike the old temp-dir version, Mirror never deletes localDir
// itself.
func Mirror(ctx context.Context, host string, port int, username, password, localDir string) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	conn, err := Connect(ctx, addr)
	if err != nil {
		return classifyConnectErr(err)
	}
	defer conn.Quit()

	if err := conn.Login(username, password); err != nil {
		return fmt.Errorf("login failed — check the Miniserver username and password: %w", err)
	}

	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return fmt.Errorf("create mirror dir: %w", err)
	}

	return mirrorDir(ctx, conn, "/", localDir)
}

// maxRetries and retryBaseDelay govern the exponential backoff applied to a
// single directory listing or file download before it's given up on and
// skipped. Not every failure is the permanent, stable-across-retries kind
// (like /sys's 550 permission errors) — the Miniserver is a small embedded
// device whose FTP service can drop a command under load, and those
// transient errors are worth a few retries rather than an immediate skip.
const (
	maxRetries     = 5
	retryBaseDelay = 500 * time.Millisecond
)

// withRetry calls fn until it succeeds or has been tried once plus
// maxRetries times, waiting an exponentially increasing delay (500ms, 1s,
// 2s, 4s, 8s) between attempts. It returns the last error once retries are
// exhausted, or nil as soon as one attempt succeeds.
func withRetry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; ; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if attempt >= maxRetries {
			return err
		}
		delay := retryBaseDelay * time.Duration(1<<attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// excludedRemoteDirs are skipped entirely rather than mirrored. /sys
// contains Loxone firmware/NOR-flash images — not project configuration —
// and is the source of both the per-file "550 Permission denied" and the
// directory-listing "550 Can't check for file existence" errors seen in
// practice; Loxone's own Exosphere cloud backup targets project/config
// state, not firmware, so excluding it here mirrors that same scope.
var excludedRemoteDirs = map[string]bool{
	"/sys": true,
}

// mtimeTolerance accounts for FTP directory listings' typically
// minute-level (not second-level) timestamp resolution: comparing exact
// timestamps would treat every file as "changed" on every run and defeat
// the incremental sync entirely.
const mtimeTolerance = 2 * time.Minute

// mirrorDir recursively lists and downloads the remote filesystem by hand
// rather than using goftp's Walker: Walker.Next() aborts the *entire* walk
// the moment List() fails on any one directory (confirmed by reading
// jlaffaye/ftp's walker.go — it sets a terminal error and stops, abandoning
// every other unvisited directory too), which is far too fragile against a
// Miniserver FTP account that can't list/read parts of its own tree. Here, a
// List() failure on one directory is logged and only that subtree is
// skipped — sibling directories still get mirrored.
//
// It also does the incremental part of Mirror: files already present
// locally with a matching size/mtime are left alone, and anything present
// locally but no longer reported by the Miniserver is removed, so localDir
// converges to match the remote tree instead of just accumulating files
// forever.
func mirrorDir(ctx context.Context, conn *goftp.ServerConn, remoteDir, localDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if excludedRemoteDirs[remoteDir] {
		return nil
	}

	localDirPath := filepath.Join(localDir, remoteDir)
	stale := localEntryNames(localDirPath)

	var entries []*goftp.Entry
	err := withRetry(ctx, func() error {
		var listErr error
		entries, listErr = conn.List(remoteDir)
		return listErr
	})
	if err != nil {
		slog.Warn("skipping directory that could not be listed after retries", "path", remoteDir, "attempts", maxRetries+1, "error", err)
		return nil
	}

	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		delete(stale, entry.Name) // still present remotely — don't prune it below
		remotePath := path.Join(remoteDir, entry.Name)
		localPath := filepath.Join(localDir, remotePath)

		switch entry.Type {
		case goftp.EntryTypeFolder:
			if err := os.MkdirAll(localPath, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", remotePath, err)
			}
			if err := mirrorDir(ctx, conn, remotePath, localDir); err != nil {
				return err
			}
		case goftp.EntryTypeFile:
			if fileUpToDate(localPath, entry) {
				continue
			}
			// A single unreadable file (Loxone's /sys/ tree in particular
			// contains firmware/NOR-flash images the FTP account often
			// can't read: 550 "Permission denied") shouldn't sink the
			// entire backup — retry with backoff, then skip it and keep
			// mirroring the rest.
			if err := withRetry(ctx, func() error { return downloadFile(conn, remotePath, localPath, entry.Time) }); err != nil {
				slog.Warn("skipping file that could not be downloaded after retries", "path", remotePath, "attempts", maxRetries+1, "error", err)
			}
		default:
			// Skip symlinks and anything else we don't understand.
		}
	}

	for name := range stale {
		stalePath := filepath.Join(localDirPath, name)
		if err := os.RemoveAll(stalePath); err != nil {
			slog.Warn("failed to prune locally mirrored file no longer on Miniserver", "path", stalePath, "error", err)
		}
	}
	return nil
}

// localEntryNames returns the names of localDir's direct children, or an
// empty (never nil) map if localDir doesn't exist yet — the common case for
// a directory the Miniserver just created since the last sync.
func localEntryNames(localDir string) map[string]bool {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return map[string]bool{}
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
	}
	return names
}

// fileUpToDate reports whether the local copy of a file already matches
// the remote entry closely enough to skip re-downloading it, using the
// same size-and-mtime "quick check" rsync uses by default rather than
// hashing file contents.
func fileUpToDate(localPath string, entry *goftp.Entry) bool {
	info, err := os.Stat(localPath)
	if err != nil || info.IsDir() {
		return false
	}
	if uint64(info.Size()) != entry.Size {
		return false
	}
	diff := info.ModTime().Sub(entry.Time)
	if diff < 0 {
		diff = -diff
	}
	return diff <= mtimeTolerance
}

func downloadFile(conn *goftp.ServerConn, remotePath, localPath string, remoteTime time.Time) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	resp, err := conn.Retr(remotePath)
	if err != nil {
		return fmt.Errorf("retr: %w", err)
	}
	defer resp.Close()

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	// Stamp the local file with the remote's mtime so the next sync's
	// fileUpToDate check can recognize it as unchanged instead of
	// re-downloading it every run.
	if !remoteTime.IsZero() {
		if err := os.Chtimes(localPath, remoteTime, remoteTime); err != nil {
			return fmt.Errorf("set mtime: %w", err)
		}
	}
	return nil
}
