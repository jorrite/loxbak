-- destinations.type was created with CHECK (type IN ('local','webdav')),
-- which SQLite cannot ALTER in place — the only way to widen it is to
-- rebuild the table. This drops the hardcoded enum entirely: new
-- destination types (see internal/destinations/destination.go's registry)
-- are validated in Go against the registry, so adding one no longer needs
-- a schema change at all — the whole point of keeping destinations
-- pluggable. Gated by schema_version in migrations.go, so this only ever
-- runs once, not on every startup like the CREATE TABLE IF NOT EXISTS
-- migrations.
PRAGMA foreign_keys = OFF;

CREATE TABLE destinations_new (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  name             TEXT NOT NULL,
  type             TEXT NOT NULL,
  config_json      TEXT NOT NULL,
  encrypted_secret BLOB,
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO destinations_new (id, name, type, config_json, encrypted_secret, created_at, updated_at)
  SELECT id, name, type, config_json, encrypted_secret, created_at, updated_at FROM destinations;

DROP TABLE destinations;
ALTER TABLE destinations_new RENAME TO destinations;

PRAGMA foreign_keys = ON;
