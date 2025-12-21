# SQLite Implementation Plan

This document outlines the plan for migrating Nexio's metadata storage from JSON files to SQLite, based on the benchmark findings in [sqlite-benchmark.md](./sqlite-benchmark.md).

## Overview

### Goals

1. Improve performance for large repositories (1000+ files)
2. Maintain backward compatibility during migration
3. Keep blob storage as files (already optimal)
4. Use pure Go SQLite (`modernc.org/sqlite`) to avoid CGO dependency

### Non-Goals

- Changing the blob storage mechanism
- Modifying the CLI interface
- Breaking existing repositories (migration path required)

## Database Schema

### Location

```
.nexio/
├── objects/          # Unchanged - blob storage
├── index.db          # NEW - SQLite database
├── config.json       # Unchanged - user config
└── staging/          # DEPRECATED after migration
    └── logs.json
```

### Tables

```sql
-- Staging area (replaces staging/logs.json)
CREATE TABLE staging (
    id          TEXT PRIMARY KEY,
    op          TEXT NOT NULL CHECK (op IN ('ADD', 'MOD', 'REM')),
    path        TEXT NOT NULL,
    blob_hash   TEXT,
    created_at  TEXT DEFAULT (datetime('now'))
);
CREATE INDEX idx_staging_path ON staging(path);
CREATE INDEX idx_staging_op ON staging(op);

-- Files in commits (replaces commits/<id>/fileList.json)
CREATE TABLE files (
    id          TEXT PRIMARY KEY,
    commit_id   TEXT NOT NULL,
    path        TEXT NOT NULL,
    blob_hash   TEXT NOT NULL,
    mode        INTEGER NOT NULL,
    FOREIGN KEY (commit_id) REFERENCES commits(id)
);
CREATE INDEX idx_files_commit_id ON files(commit_id);
CREATE INDEX idx_files_commit_path ON files(commit_id, path);
CREATE INDEX idx_files_blob_hash ON files(blob_hash);
CREATE INDEX idx_files_path ON files(path);

-- Commits (replaces branches/<name>/commits.json and commits/<id>/metadata.json)
CREATE TABLE commits (
    id          TEXT PRIMARY KEY,
    branch      TEXT NOT NULL,
    parent_id   TEXT,               -- NULL for first commit (replaces linked list)
    timestamp   TEXT NOT NULL,
    message     TEXT NOT NULL,
    author_name TEXT NOT NULL,
    author_email TEXT NOT NULL,
    FOREIGN KEY (branch) REFERENCES branches(name),
    FOREIGN KEY (parent_id) REFERENCES commits(id)
);
CREATE INDEX idx_commits_branch ON commits(branch);
CREATE INDEX idx_commits_parent ON commits(parent_id);
CREATE INDEX idx_commits_timestamp ON commits(timestamp);

-- Branches (replaces branches/metadata.json)
CREATE TABLE branches (
    name        TEXT PRIMARY KEY,
    head_commit TEXT,               -- Latest commit on this branch
    is_default  INTEGER DEFAULT 0,
    is_current  INTEGER DEFAULT 0,
    created_at  TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (head_commit) REFERENCES commits(id)
);
CREATE INDEX idx_branches_current ON branches(is_current) WHERE is_current = 1;

-- Commit logs snapshot (replaces commits/<id>/logs.json)
CREATE TABLE commit_logs (
    id          TEXT PRIMARY KEY,
    commit_id   TEXT NOT NULL,
    op          TEXT NOT NULL,
    path        TEXT NOT NULL,
    blob_hash   TEXT,
    FOREIGN KEY (commit_id) REFERENCES commits(id)
);
CREATE INDEX idx_commit_logs_commit ON commit_logs(commit_id);

-- Schema version for migrations
CREATE TABLE schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT DEFAULT (datetime('now'))
);
INSERT INTO schema_version (version) VALUES (1);
```

## Implementation Phases

### Phase 1: Database Layer (New Files)

Create a new database abstraction layer without modifying existing code.

#### New Files to Create

| File | Purpose |
|------|---------|
| `cmd/nexio/db.go` | Database connection, initialization, migrations |
| `cmd/nexio/db_staging.go` | Staging operations (replaces `staging_log.go` functions) |
| `cmd/nexio/db_commits.go` | Commit operations (replaces `commit_helper.go` functions) |
| `cmd/nexio/db_branches.go` | Branch operations (replaces `branch_helper.go` functions) |
| `cmd/nexio/db_files.go` | File list operations (replaces `status_helper.go` functions) |

#### db.go - Core Database Functions

```go
package main

import (
    "database/sql"
    "os"
    "path/filepath"
    
    _ "modernc.org/sqlite"
)

var db *sql.DB

// InitDB opens or creates the database
func InitDB() error {
    dbPath := filepath.Join(GetDir("root"), "index.db")
    
    var err error
    db, err = sql.Open("sqlite", dbPath)
    if err != nil {
        return err
    }
    
    // Enable WAL mode for better concurrent performance
    _, err = db.Exec("PRAGMA journal_mode=WAL")
    if err != nil {
        return err
    }
    
    // Enable foreign keys
    _, err = db.Exec("PRAGMA foreign_keys=ON")
    if err != nil {
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

// initSchema creates tables if they don't exist
func initSchema() error {
    schema := `
    CREATE TABLE IF NOT EXISTS staging (...);
    CREATE TABLE IF NOT EXISTS files (...);
    -- etc.
    `
    _, err := db.Exec(schema)
    return err
}

// WithTransaction wraps operations in a transaction
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
```

#### db_staging.go - Staging Operations

```go
package main

// DBLogOperation adds an entry to staging (replaces LogOperation)
func DBLogOperation(id, op, path, blobHash string) error {
    _, err := db.Exec(
        "INSERT INTO staging (id, op, path, blob_hash) VALUES (?, ?, ?, ?)",
        id, op, path, blobHash,
    )
    return err
}

// DBLogEntryLookup finds a staging entry (replaces LogEntryLookup)
func DBLogEntryLookup(op, path string) (bool, *LogFileEntry) {
    var e LogFileEntry
    var query string
    var args []interface{}
    
    if op == "*" {
        query = "SELECT id, op, path, blob_hash FROM staging WHERE path = ?"
        args = []interface{}{path}
    } else {
        query = "SELECT id, op, path, blob_hash FROM staging WHERE op = ? AND path = ?"
        args = []interface{}{op, path}
    }
    
    err := db.QueryRow(query, args...).Scan(&e.Id, &e.Op, &e.Path, &e.BlobHash)
    if err != nil {
        return false, nil
    }
    return true, &e
}

// DBIsFileStaged checks if a file is in staging (replaces IsFileStaged)
func DBIsFileStaged(path string) bool {
    var count int
    db.QueryRow("SELECT COUNT(*) FROM staging WHERE path = ?", path).Scan(&count)
    return count > 0
}

// DBGetStagingLogs returns all staging entries (replaces GetStagingLogsContent)
func DBGetStagingLogs() ([]LogFileEntry, error) {
    rows, err := db.Query("SELECT id, op, path, blob_hash FROM staging ORDER BY rowid")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var entries []LogFileEntry
    for rows.Next() {
        var e LogFileEntry
        rows.Scan(&e.Id, &e.Op, &e.Path, &e.BlobHash)
        entries = append(entries, e)
    }
    return entries, nil
}

// DBRemoveLogEntry removes a staging entry (replaces RemoveLogEntry)
func DBRemoveLogEntry(id string) error {
    _, err := db.Exec("DELETE FROM staging WHERE id = ?", id)
    return err
}

// DBTruncateLogs clears all staging entries (replaces TruncateLogs)
func DBTruncateLogs() error {
    _, err := db.Exec("DELETE FROM staging")
    return err
}
```

#### db_files.go - File Operations

```go
package main

// DBGetFileMetadata gets file info from latest commit (replaces GetFileMetadata)
func DBGetFileMetadata(path string) (bool, FileListEntry) {
    var e FileListEntry
    
    // Get the head commit of current branch, then find the file
    err := db.QueryRow(`
        SELECT f.id, f.commit_id, f.path, f.blob_hash, f.mode
        FROM files f
        JOIN branches b ON b.head_commit = f.commit_id
        WHERE b.is_current = 1 AND f.path = ?
    `, path).Scan(&e.Id, &e.CommitId, &e.Path, &e.BlobHash, &e.Mode)
    
    if err != nil {
        return false, FileListEntry{}
    }
    return true, e
}

// DBGetFileListForCommit gets all files in a commit (replaces GetFileListContent)
func DBGetFileListForCommit(commitId string) ([]FileListEntry, error) {
    rows, err := db.Query(`
        SELECT id, commit_id, path, blob_hash, mode 
        FROM files 
        WHERE commit_id = ?
    `, commitId)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var files []FileListEntry
    for rows.Next() {
        var f FileListEntry
        rows.Scan(&f.Id, &f.CommitId, &f.Path, &f.BlobHash, &f.Mode)
        files = append(files, f)
    }
    return files, nil
}

// DBCollectReferencedHashes gets all blob hashes (replaces CollectReferencedHashes)
func DBCollectReferencedHashes() (map[string]struct{}, error) {
    hashes := make(map[string]struct{})
    
    // Get hashes from files
    rows, err := db.Query("SELECT DISTINCT blob_hash FROM files WHERE blob_hash != ''")
    if err != nil {
        return nil, err
    }
    for rows.Next() {
        var h string
        rows.Scan(&h)
        hashes[h] = struct{}{}
    }
    rows.Close()
    
    // Get hashes from staging
    rows, err = db.Query("SELECT DISTINCT blob_hash FROM staging WHERE blob_hash != ''")
    if err != nil {
        return nil, err
    }
    for rows.Next() {
        var h string
        rows.Scan(&h)
        hashes[h] = struct{}{}
    }
    rows.Close()
    
    return hashes, nil
}
```

### Phase 2: Feature Flag Integration

Add a feature flag to switch between JSON and SQLite backends.

```go
// In config.go or a new file

var useDB = false // Will be enabled via flag or config

func init() {
    // Check environment variable or config
    if os.Getenv("NEXIO_USE_SQLITE") == "1" {
        useDB = true
    }
}

// Wrapper functions that delegate to the appropriate backend
func LogOperation(id, op, path, blobHash string) {
    if useDB {
        DBLogOperation(id, op, path, blobHash)
    } else {
        jsonLogOperation(id, op, path, blobHash) // renamed original
    }
}
```

### Phase 3: Migration Tool

Create a migration command to convert existing repositories.

```go
// cmd/nexio/migrate.go

func MigrateToSQLite() error {
    // 1. Initialize database
    if err := InitDB(); err != nil {
        return err
    }
    
    // 2. Migrate branches
    branches := ListBranches()
    for _, branch := range branches {
        migrateBranch(branch)
    }
    
    // 3. Migrate staging
    stagingLogs := GetStagingLogsContent()
    for _, entry := range *stagingLogs {
        DBLogOperation(entry.Id, entry.Op, entry.Path, entry.BlobHash)
    }
    
    // 4. Mark migration complete
    WriteJson(GetDir("root")+"/migration.json", map[string]bool{"sqlite": true})
    
    return nil
}

func migrateBranch(branch string) error {
    return WithTransaction(func(tx *sql.Tx) error {
        // Read commits.json
        commits := getCommitsForBranch(branch)
        
        for _, commit := range commits {
            // Insert commit
            tx.Exec(`INSERT INTO commits (...) VALUES (...)`, ...)
            
            // Read and insert file list
            fileList := GetFileListContent(commit.Id)
            for _, f := range *fileList {
                tx.Exec(`INSERT INTO files (...) VALUES (...)`, ...)
            }
            
            // Read and insert commit logs
            logs := getCommitLogs(commit.Id)
            for _, l := range logs {
                tx.Exec(`INSERT INTO commit_logs (...) VALUES (...)`, ...)
            }
        }
        
        return nil
    })
}
```

### Phase 4: Replace JSON Backend

Once SQLite is stable, make it the default and deprecate JSON functions.

1. Remove feature flag, always use SQLite
2. Keep JSON functions for migration only
3. Update `nexio init` to create database instead of JSON files
4. Remove old JSON files after successful migration

## Files to Modify

| File | Changes |
|------|---------|
| `go.mod` | Add `modernc.org/sqlite` dependency |
| `dirs.go` | Add `db` directory constant |
| `init.go` | Call `InitDB()` during initialization |
| `main.go` | Add `defer CloseDB()` |
| `staging_log.go` | Rename functions, add wrappers |
| `status_helper.go` | Rename functions, add wrappers |
| `commit_helper.go` | Rename functions, add wrappers |
| `branch_helper.go` | Rename functions, add wrappers |
| `clean_helper.go` | Use new `DBCollectReferencedHashes()` |

## Function Migration Map

### staging_log.go

| Current Function | SQLite Replacement | Notes |
|-----------------|-------------------|-------|
| `LogOperation()` | `DBLogOperation()` | No file locking needed |
| `LogEntryLookup()` | `DBLogEntryLookup()` | Uses index, O(log n) |
| `IsFileStaged()` (in status_helper.go) | `DBIsFileStaged()` | Uses index |
| `GetStagingLogsContent()` | `DBGetStagingLogs()` | Simple SELECT |
| `RemoveLogEntry()` | `DBRemoveLogEntry()` | No read-modify-write |
| `TruncateLogs()` | `DBTruncateLogs()` | DELETE is atomic |

### status_helper.go

| Current Function | SQLite Replacement | Notes |
|-----------------|-------------------|-------|
| `IsFileStaged()` | `DBIsFileStaged()` | Indexed lookup |
| `GetFileMetadata()` | `DBGetFileMetadata()` | JOIN with branches |

### commit_helper.go

| Current Function | SQLite Replacement | Notes |
|-----------------|-------------------|-------|
| `GetLastCommit()` | `DBGetLastCommit()` | Query head_commit from branches |
| `GetCommits()` | `DBGetCommits()` | ORDER BY timestamp, no linked list |
| `GetFileListContent()` | `DBGetFileListForCommit()` | Indexed by commit_id |
| `ProcessFileList()` | `DBProcessFileList()` | Batch inserts in transaction |
| `RegisterCommitForBranch()` | `DBRegisterCommit()` | Update branches.head_commit |
| `CountCommits()` | `DBCountCommits()` | SELECT COUNT(*) |

### branch_helper.go

| Current Function | SQLite Replacement | Notes |
|-----------------|-------------------|-------|
| `GetCurrentBranchName()` | `DBGetCurrentBranch()` | WHERE is_current = 1 |
| `GetDefaultBranchName()` | `DBGetDefaultBranch()` | WHERE is_default = 1 |
| `SetBranch()` | `DBSetBranch()` | UPDATE branches |
| `ListBranches()` | `DBListBranches()` | SELECT name FROM branches |
| `CreateBranchesMetadata()` | `DBCreateBranch()` | INSERT INTO branches |

### clean_helper.go

| Current Function | SQLite Replacement | Notes |
|-----------------|-------------------|-------|
| `CollectReferencedHashes()` | `DBCollectReferencedHashes()` | Single query, huge speedup |

## Testing Strategy

1. **Unit Tests**: Test each DB function independently
2. **Integration Tests**: Test full workflows (add -> commit -> branch)
3. **Migration Tests**: Test JSON -> SQLite migration
4. **Performance Tests**: Verify benchmark improvements
5. **Compatibility Tests**: Ensure CLI behavior unchanged

## Rollout Plan

1. **Week 1**: Implement Phase 1 (database layer)
2. **Week 2**: Implement Phase 2 (feature flag) + testing
3. **Week 3**: Beta testing with feature flag
4. **Week 4**: Implement Phase 3 (migration tool)
5. **Week 5**: Phase 4 (make SQLite default)

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Data corruption during migration | Backup JSON files before migration |
| SQLite file locking issues | Use WAL mode, test concurrent access |
| Performance regression | Benchmark before/after each change |
| Breaking changes | Feature flag allows rollback |

## Open Questions

1. Should we keep JSON export capability for debugging?
2. Should migration be automatic or require explicit command?
3. How to handle partial migrations (interrupted)?
4. Should we vacuum the database periodically?

## References

- [SQLite Benchmark Results](./sqlite-benchmark.md)
- [modernc.org/sqlite Documentation](https://pkg.go.dev/modernc.org/sqlite)
- [SQLite WAL Mode](https://www.sqlite.org/wal.html)
