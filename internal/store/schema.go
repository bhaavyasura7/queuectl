package store

import "database/sql"

const migration = `
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS jobs (
  id              TEXT PRIMARY KEY,
  command         TEXT NOT NULL,
  state           TEXT NOT NULL CHECK (state IN ('pending','processing','completed','failed','dead')),
  priority        TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','critical')),
  run_at          TEXT,
  timeout         INTEGER,
  attempts        INTEGER NOT NULL DEFAULT 0,
  max_retries     INTEGER NOT NULL,
  backoff_base    REAL NOT NULL,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL,
  next_attempt_at TEXT,
  claimed_at      TEXT,
  heartbeat_at    TEXT,
  worker_id       TEXT,
  last_error      TEXT,
  stdout          TEXT,
  stderr          TEXT
);

CREATE INDEX IF NOT EXISTS idx_jobs_claimable ON jobs(state, next_attempt_at, run_at);

CREATE TABLE IF NOT EXISTS workers (
  pid         INTEGER PRIMARY KEY,
  hostname    TEXT NOT NULL,
  started_at  TEXT NOT NULL,
  status      TEXT NOT NULL DEFAULT 'running'
);

CREATE TABLE IF NOT EXISTS config (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

INSERT OR IGNORE INTO config(key, value) VALUES ('max-retries', '3');
INSERT OR IGNORE INTO config(key, value) VALUES ('backoff-base', '2');
`

func runMigration(db *sql.DB) error {
	_, err := db.Exec(migration)
	return err
}
