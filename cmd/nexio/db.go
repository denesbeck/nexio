package main

import (
	"database/sql"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// InitDB opens or creates the SQLite database and initializes the schema.
// If a connection is already open, it closes it first.
func InitDB() error {
	// Close any existing connection before opening a new one
	CloseDB()

	dbPath := filepath.Join(GetDir("root"), "index.db")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	// Enable WAL mode for better concurrent performance
	if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}

	// Enable foreign keys
	if _, err = db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return err
	}

	return initSchema()
}

// CloseDB closes the database connection
func CloseDB() {
	if db != nil {
		db.Close()
	}
}

// initSchema creates all tables and indexes if they don't exist
func initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS branches (
		name        TEXT PRIMARY KEY,
		head_commit TEXT,
		is_default  INTEGER DEFAULT 0,
		is_current  INTEGER DEFAULT 0,
		created_at  TEXT DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_branches_current ON branches(is_current) WHERE is_current = 1;

	CREATE TABLE IF NOT EXISTS commits (
		id           TEXT PRIMARY KEY,
		timestamp    TEXT NOT NULL,
		message      TEXT NOT NULL,
		author_name  TEXT NOT NULL,
		author_email TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_commits_timestamp ON commits(timestamp);

	CREATE TABLE IF NOT EXISTS commit_parents (
		commit_id    TEXT NOT NULL,
		parent_id    TEXT NOT NULL,
		parent_order INTEGER NOT NULL,   -- 0 = first parent, 1+ = merge parents
		PRIMARY KEY (commit_id, parent_order),
		FOREIGN KEY (commit_id) REFERENCES commits(id),
		FOREIGN KEY (parent_id) REFERENCES commits(id)
	);
	CREATE INDEX IF NOT EXISTS idx_commit_parents_commit ON commit_parents(commit_id);

	CREATE TABLE IF NOT EXISTS staging (
		id          TEXT PRIMARY KEY,
		op          TEXT NOT NULL CHECK (op IN ('ADD', 'MOD', 'REM')),
		path        TEXT NOT NULL,
		blob_hash   TEXT,
		created_at  TEXT DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_staging_path ON staging(path);
	CREATE INDEX IF NOT EXISTS idx_staging_op ON staging(op);

	CREATE TABLE IF NOT EXISTS files (
		id          TEXT PRIMARY KEY,
		commit_id   TEXT NOT NULL,
		path        TEXT NOT NULL,
		blob_hash   TEXT NOT NULL,
		mode        INTEGER NOT NULL,
		FOREIGN KEY (commit_id) REFERENCES commits(id)
	);
	CREATE INDEX IF NOT EXISTS idx_files_commit_id ON files(commit_id);
	CREATE INDEX IF NOT EXISTS idx_files_commit_path ON files(commit_id, path);
	CREATE INDEX IF NOT EXISTS idx_files_blob_hash ON files(blob_hash);
	CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);

	CREATE TABLE IF NOT EXISTS commit_logs (
		id          TEXT PRIMARY KEY,
		commit_id   TEXT NOT NULL,
		op          TEXT NOT NULL,
		path        TEXT NOT NULL,
		blob_hash   TEXT,
		FOREIGN KEY (commit_id) REFERENCES commits(id)
	);
	CREATE INDEX IF NOT EXISTS idx_commit_logs_commit ON commit_logs(commit_id);

	CREATE TABLE IF NOT EXISTS schema_version (
		version     INTEGER PRIMARY KEY,
		applied_at  TEXT DEFAULT (datetime('now'))
	);
	INSERT OR IGNORE INTO schema_version (version) VALUES (2);
	`
	_, err := db.Exec(schema)
	return err
}

// WithTransaction wraps operations in a database transaction
func WithTransaction(fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
