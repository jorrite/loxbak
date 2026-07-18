// Package destinations defines the pluggable backup-destination interface
// and a small constructor registry. Adding a new destination type (e.g. S3)
// means implementing Destination, registering a constructor here, and
// adding a matching config form on the frontend — nothing else in the
// scheduler or DB schema needs to change.
package destinations

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// BackupMeta describes a single backup archive being handed to a
// destination for storage.
type BackupMeta struct {
	// ScheduleName is the human-readable name of the schedule that
	// produced this backup (used to namespace/organize stored archives).
	ScheduleName string
	// Filename is the suggested archive filename, e.g.
	// "2026-07-17T120000Z.zip".
	Filename string
	// Size is the archive size in bytes, if known ahead of time (0 if
	// unknown / streamed).
	Size int64
}

// Destination is a pluggable backup storage target.
type Destination interface {
	// Name returns the configured, human-readable name of this
	// destination instance (as stored in destinations.name).
	Name() string
	// Store writes the archive read from r to this destination.
	Store(ctx context.Context, meta BackupMeta, r io.Reader) error
}

// BackupEntry describes one backup archive actually present at a
// destination, as reported by that destination itself (not from local DB
// history) — this is what makes it possible to see and manage backups that
// predate loxbak tracking them, or that a run's history row was since
// pruned/deleted.
type BackupEntry struct {
	Filename string
	Size     int64
	ModTime  time.Time
}

// Lister is optionally implemented by a Destination that can enumerate and
// delete its own stored backups. Not every destination type has to
// implement it — retention enforcement is a no-op for one that doesn't, and
// the Backups page simply has nothing to show/delete for it, rather than
// either being a hard requirement.
type Lister interface {
	// List returns every backup archive currently stored at this
	// destination.
	List(ctx context.Context) ([]BackupEntry, error)
	// Delete removes one backup archive by filename.
	Delete(ctx context.Context, filename string) error
}

// Reader is optionally implemented by a Destination that can stream one of
// its own stored backups back out, for the Backups page's download action.
type Reader interface {
	Open(ctx context.Context, filename string) (io.ReadCloser, error)
}

// Prune deletes a Lister's own backups for one schedule beyond the newest
// keep, matching by filenamePrefix (the sanitized schedule-name prefix
// shared by every archive that schedule produces — see
// scheduler.SanitizeName — since one destination's directory can hold
// backups from multiple schedules), and returns the filenames deleted.
// Shared by every destination type rather than each reimplementing the same
// prefix-match/sort/delete-beyond-keep logic on top of its own List/Delete.
func Prune(ctx context.Context, lister Lister, filenamePrefix string, keep int) ([]string, error) {
	if keep <= 0 {
		return nil, nil
	}

	entries, err := lister.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("destinations: prune: list: %w", err)
	}

	var matched []BackupEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Filename, filenamePrefix) {
			matched = append(matched, e)
		}
	}
	// Archive filenames end in a lexically-sortable timestamp
	// (20060102T150405Z), so sorting the filename string itself is enough
	// to order them newest-first without parsing anything.
	sort.Slice(matched, func(i, j int) bool { return matched[i].Filename > matched[j].Filename })

	if len(matched) <= keep {
		return nil, nil
	}

	var deleted []string
	for _, e := range matched[keep:] {
		if err := lister.Delete(ctx, e.Filename); err != nil {
			return deleted, fmt.Errorf("destinations: prune: delete %s: %w", e.Filename, err)
		}
		deleted = append(deleted, e.Filename)
	}
	return deleted, nil
}

// Constructor builds a Destination from its stored JSON config and decrypted
// secret bytes (e.g. a WebDAV password). configJSON and secret come
// straight from the destinations table (config_json, encrypted_secret —
// already decrypted by the caller).
type Constructor func(name string, configJSON string, secret []byte) (Destination, error)

// registry maps a destinations.type value to its Constructor.
var registry = map[string]Constructor{}

// Register adds a constructor for the given destination type. It is called
// from each destination implementation's init() function.
func Register(typ string, ctor Constructor) {
	registry[typ] = ctor
}

// Build constructs a Destination instance for the given type, using the
// registered Constructor.
func Build(typ, name, configJSON string, secret []byte) (Destination, error) {
	ctor, ok := registry[typ]
	if !ok {
		return nil, fmt.Errorf("destinations: unknown destination type %q", typ)
	}
	return ctor(name, configJSON, secret)
}

// Types returns the list of currently registered destination type names.
func Types() []string {
	types := make([]string, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	return types
}
