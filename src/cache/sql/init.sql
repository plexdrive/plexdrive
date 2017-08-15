PRAGMA foreign_keys = ON;

BEGIN;

CREATE TABLE IF NOT EXISTS token (
  id            INTEGER PRIMARY KEY,
  token         TEXT
);

CREATE TABLE IF NOT EXISTS file (
  inode         INTEGER PRIMARY KEY AUTOINCREMENT,
  id            TEXT UNIQUE,
  name          TEXT NOT NULL,
  is_dir        INTEGER,
  size          TEXT,
  last_modified TEXT,
  download_url  TEXT,
  can_trash     INTEGER
);
CREATE INDEX file_id ON file(id);

CREATE TABLE IF NOT EXISTS parent (
  file_id       TEXT REFERENCES file(id),
  parent_id     TEXT REFERENCES file(id),
  PRIMARY KEY (file_id, parent_id)
);

CREATE INDEX parent_file_id ON parent(file_id);
CREATE INDEX parent_parent_id ON parent(parent_id);

UPDATE sqlite_sequence SET seq = 1 WHERE name = 'file';

COMMIT;