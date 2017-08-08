PRAGMA foreign_keys = ON;

BEGIN;

CREATE TABLE IF NOT EXISTS token (
  id            INTEGER PRIMARY KEY,
  token         TEXT
);

CREATE TABLE IF NOT EXISTS file (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  is_dir        INTEGER,
  size          INTEGER,
  last_modified INTEGER,
  download_url  TEXT,
  can_trash     INTEGER
);

CREATE TABLE IF NOT EXISTS parent (
  file_id       TEXT REFERENCES file(id),
  parent_id     TEXT REFERENCES file(id)
);

CREATE INDEX parent_file_id ON parent(file_id);
CREATE INDEX parent_parent_id ON parent(parent_id);

COMMIT;