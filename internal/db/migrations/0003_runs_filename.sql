-- Records the archive filename produced by a run, so the API can later
-- serve it back for download (from whichever local destination succeeded)
-- without having to reconstruct the name from started_at/schedule name,
-- which drift apart since the archive's own timestamp is generated
-- separately inside archiveDir. ADD COLUMN isn't safe to replay on every
-- startup (fails with "duplicate column name" the second time), so this
-- goes through the same schema_version gate as 0002, not a bare
-- CREATE-TABLE-IF-NOT-EXISTS file.
ALTER TABLE runs ADD COLUMN filename TEXT;
