package destinations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func init() {
	Register("local", newLocal)
}

// localConfig is the config_json shape for a "local" destination.
type localConfig struct {
	// no fields yet: the destination directory is always derived from
	// DATA_DIR and the destination's name. Kept as a struct (rather than
	// no config at all) so the frontend has a stable shape to submit even
	// if it's empty today.
}

// Local writes backup archives into $DATA_DIR/backups/<destination-name>/.
type Local struct {
	name    string
	dataDir string
}

func newLocal(name, configJSON string, _ []byte) (Destination, error) {
	var cfg localConfig
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return nil, fmt.Errorf("destinations: local: invalid config: %w", err)
		}
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	return &Local{name: name, dataDir: dataDir}, nil
}

// Name implements Destination.
func (l *Local) Name() string { return l.name }

// Store implements Destination by writing the archive under
// $DATA_DIR/backups/<name>/<filename>.
func (l *Local) Store(_ context.Context, meta BackupMeta, r io.Reader) error {
	dir := filepath.Join(l.dataDir, "backups", l.name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("destinations: local: mkdir %s: %w", dir, err)
	}

	path := filepath.Join(dir, meta.Filename)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("destinations: local: create %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("destinations: local: write %s: %w", path, err)
	}

	return nil
}

// List implements Lister by reading this destination's directory directly
// — not from any DB history — so it also surfaces archives that predate
// loxbak tracking runs at all.
func (l *Local) List(_ context.Context) ([]BackupEntry, error) {
	dir := filepath.Join(l.dataDir, "backups", l.name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("destinations: local: list: read dir %s: %w", dir, err)
	}

	var out []BackupEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("destinations: local: list: stat %s: %w", e.Name(), err)
		}
		out = append(out, BackupEntry{Filename: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	return out, nil
}

// Delete implements Lister.
func (l *Local) Delete(_ context.Context, filename string) error {
	path := filepath.Join(l.dataDir, "backups", l.name, filename)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("destinations: local: delete %s: %w", path, err)
	}
	return nil
}

// Open implements Reader.
func (l *Local) Open(_ context.Context, filename string) (io.ReadCloser, error) {
	path := filepath.Join(l.dataDir, "backups", l.name, filename)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("destinations: local: open %s: %w", path, err)
	}
	return f, nil
}
