package destinations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path"
	"strconv"

	goftp "github.com/jlaffaye/ftp"

	"loxbak/internal/ftp"
)

func init() {
	Register("ftp", newFTP)
}

// ftpConfig is the config_json shape for an "ftp" destination — a remote
// FTP server backups get uploaded to. Distinct from the Miniserver's own
// FTP server, which is the backup *source* (see internal/ftp), not a
// destination.
type ftpConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	// Dir is the remote directory backups are written into. Defaults to
	// "/" if empty.
	Dir string `json:"dir"`
}

// FTP stores backup archives on a remote FTP(S) server, reusing the same
// dial/explicit-TLS-then-plain-fallback logic internal/ftp uses to fetch
// from the Miniserver — same protocol, opposite direction (upload).
type FTP struct {
	name     string
	host     string
	port     int
	username string
	password string
	dir      string
}

func newFTP(name, configJSON string, secret []byte) (Destination, error) {
	var cfg ftpConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("destinations: ftp: invalid config: %w", err)
	}
	if cfg.Host == "" {
		return nil, fmt.Errorf("destinations: ftp: config missing host")
	}
	if cfg.Port == 0 {
		cfg.Port = 21
	}
	dir := cfg.Dir
	if dir == "" {
		dir = "/"
	}

	return &FTP{
		name:     name,
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: string(secret),
		dir:      dir,
	}, nil
}

// Name implements Destination.
func (f *FTP) Name() string { return f.name }

// Store implements Destination by uploading the archive to
// <dir>/<filename> on the remote FTP server via STOR, creating the
// directory first if needed.
func (f *FTP) Store(ctx context.Context, meta BackupMeta, r io.Reader) error {
	conn, err := f.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Quit()

	// Best-effort: MakeDir errors on a directory that already exists on
	// most servers, which isn't a real failure — only the Stor below is
	// treated as fatal.
	_ = conn.MakeDir(f.dir)

	remotePath := path.Join(f.dir, meta.Filename)
	if err := conn.Stor(remotePath, r); err != nil {
		return fmt.Errorf("destinations: ftp: store %s: %w", remotePath, err)
	}

	return nil
}

// List implements Lister by listing this destination's directory directly
// on the remote FTP server — not from any DB history — so it also surfaces
// archives that predate loxbak tracking runs at all.
func (f *FTP) List(ctx context.Context) ([]BackupEntry, error) {
	conn, err := f.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Quit()

	entries, err := conn.List(f.dir)
	if err != nil {
		return nil, fmt.Errorf("destinations: ftp: list %s: %w", f.dir, err)
	}

	var out []BackupEntry
	for _, e := range entries {
		if e.Type != goftp.EntryTypeFile {
			continue
		}
		out = append(out, BackupEntry{Filename: e.Name, Size: int64(e.Size), ModTime: e.Time})
	}
	return out, nil
}

// Delete implements Lister.
func (f *FTP) Delete(ctx context.Context, filename string) error {
	conn, err := f.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Quit()

	remotePath := path.Join(f.dir, filename)
	if err := conn.Delete(remotePath); err != nil {
		return fmt.Errorf("destinations: ftp: delete %s: %w", remotePath, err)
	}
	return nil
}

// connect dials and logs into the remote FTP server, shared by Store,
// List, and Delete rather than each redoing the same dial+login.
func (f *FTP) connect(ctx context.Context) (*goftp.ServerConn, error) {
	addr := net.JoinHostPort(f.host, strconv.Itoa(f.port))

	conn, err := ftp.Connect(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("destinations: ftp: connect: %w", err)
	}
	if err := conn.Login(f.username, f.password); err != nil {
		conn.Quit()
		return nil, fmt.Errorf("destinations: ftp: login: %w", err)
	}
	return conn, nil
}
