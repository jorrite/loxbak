package destinations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"

	"github.com/studio-b12/gowebdav"
)

func init() {
	Register("webdav", newWebDAV)
}

// webdavConfig is the config_json shape for a "webdav" destination,
// generic to any WebDAV server. The password is not part of this struct
// — it's stored separately (encrypted) and passed in as secret.
type webdavConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	// Dir is the remote directory backups are written into, relative to
	// URL. Defaults to "/" if empty.
	Dir string `json:"dir"`
}

// WebDAV stores backup archives on a remote WebDAV server via
// github.com/studio-b12/gowebdav.
type WebDAV struct {
	name   string
	client *gowebdav.Client
	dir    string
}

func newWebDAV(name, configJSON string, secret []byte) (Destination, error) {
	var cfg webdavConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("destinations: webdav: invalid config: %w", err)
	}
	if cfg.URL == "" {
		return nil, fmt.Errorf("destinations: webdav: config missing url")
	}

	dir := cfg.Dir
	if dir == "" {
		dir = "/"
	}

	client := gowebdav.NewClient(cfg.URL, cfg.Username, string(secret))

	return &WebDAV{name: name, client: client, dir: dir}, nil
}

// Name implements Destination.
func (w *WebDAV) Name() string { return w.name }

// Store implements Destination by streaming the archive to
// <dir>/<filename> on the WebDAV server, creating the directory if needed.
func (w *WebDAV) Store(ctx context.Context, meta BackupMeta, r io.Reader) error {
	if err := w.client.MkdirAll(w.dir, 0o755); err != nil {
		return fmt.Errorf("destinations: webdav: mkdir %s: %w", w.dir, err)
	}

	remotePath := path.Join(w.dir, meta.Filename)

	var err error
	if meta.Size > 0 {
		err = w.client.WriteStreamWithLength(remotePath, r, meta.Size, 0o644)
	} else {
		err = w.client.WriteStream(remotePath, r, 0o644)
	}
	if err != nil {
		return fmt.Errorf("destinations: webdav: write %s: %w", remotePath, err)
	}

	return nil
}

// List implements Lister by reading this destination's directory directly
// off the WebDAV server — not from any DB history — so it also surfaces
// archives that predate loxbak tracking runs at all.
func (w *WebDAV) List(_ context.Context) ([]BackupEntry, error) {
	infos, err := w.client.ReadDir(w.dir)
	if err != nil {
		return nil, fmt.Errorf("destinations: webdav: list %s: %w", w.dir, err)
	}

	var out []BackupEntry
	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		out = append(out, BackupEntry{Filename: info.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	return out, nil
}

// Delete implements Lister.
func (w *WebDAV) Delete(_ context.Context, filename string) error {
	remotePath := path.Join(w.dir, filename)
	if err := w.client.Remove(remotePath); err != nil {
		return fmt.Errorf("destinations: webdav: delete %s: %w", remotePath, err)
	}
	return nil
}
