CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  id         TEXT PRIMARY KEY,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS loxone_credential (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  host               TEXT NOT NULL,
  port               INTEGER NOT NULL DEFAULT 21,
  username           TEXT NOT NULL,
  encrypted_password BLOB NOT NULL,
  updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS destinations (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  name             TEXT NOT NULL,
  type             TEXT NOT NULL CHECK (type IN ('local','webdav')),
  config_json      TEXT NOT NULL,
  encrypted_secret BLOB,
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS schedules (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL,
  cron_expr  TEXT NOT NULL,
  enabled    BOOLEAN NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS schedule_destinations (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  schedule_id     INTEGER NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
  destination_id  INTEGER NOT NULL REFERENCES destinations(id) ON DELETE CASCADE,
  retention_count INTEGER NOT NULL DEFAULT 0,
  UNIQUE (schedule_id, destination_id)
);

CREATE TABLE IF NOT EXISTS runs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  schedule_id INTEGER REFERENCES schedules(id) ON DELETE SET NULL,
  started_at  DATETIME NOT NULL,
  finished_at DATETIME,
  status      TEXT NOT NULL CHECK (status IN ('running','success','partial','failed')),
  size_bytes  INTEGER,
  error       TEXT
);

CREATE TABLE IF NOT EXISTS run_destination_results (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id         INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  destination_id INTEGER REFERENCES destinations(id) ON DELETE SET NULL,
  status         TEXT NOT NULL CHECK (status IN ('success','failed')),
  size_bytes     INTEGER,
  error          TEXT
);
