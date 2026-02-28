# SQLite Implementation Plan

This document outlines the migration of Nexio's metadata storage from JSON files to SQLite, based on the benchmark findings in [sqlite-benchmark.md](./sqlite-benchmark.md).

**Status: Complete** -- All phases implemented and tested. 156 tests passing, 74.4% coverage.

## Overview

### Goals

1. Improve performance for large repositories (1000+ files)
2. Keep blob storage as files (already optimal)
3. Use pure Go SQLite (`modernc.org/sqlite`) to avoid CGO dependency

### Simplified Approach

Since Nexio is not in production, the original phased rollout was simplified:

- **Skipped Phase 2** (Feature Flags) -- no dual backends needed
- **Skipped Phase 3** (Migration Tool) -- no existing repositories to migrate
- Direct replacement of JSON functions in-place
- Removed old JSON storage directories (`staging/`, `commits/`, `branches/`)

## Database Schema

### Location

```
.nexio/
├── objects/          # Unchanged - blob storage
├── index.db          # SQLite database (all metadata)
└── config.json       # Unchanged - user config
```

### Tables

```sql
-- Branches
CREATE TABLE branches (
    name        TEXT PRIMARY KEY,
    head_commit TEXT,
    is_default  INTEGER DEFAULT 0,
    is_current  INTEGER DEFAULT 0,
    created_at  TEXT DEFAULT (datetime('now'))
);
CREATE INDEX idx_branches_current ON branches(is_current) WHERE is_current = 1;

-- Commits (shared across branches -- no branch column)
CREATE TABLE commits (
    id           TEXT PRIMARY KEY,
    parent_id    TEXT,
    timestamp    TEXT NOT NULL,
    message      TEXT NOT NULL,
    author_name  TEXT NOT NULL,
    author_email TEXT NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES commits(id)
);
CREATE INDEX idx_commits_parent ON commits(parent_id);
CREATE INDEX idx_commits_timestamp ON commits(timestamp);

-- Staging area
CREATE TABLE staging (
    id          TEXT PRIMARY KEY,
    op          TEXT NOT NULL CHECK (op IN ('ADD', 'MOD', 'REM')),
    path        TEXT NOT NULL,
    blob_hash   TEXT,
    created_at  TEXT DEFAULT (datetime('now'))
);
CREATE INDEX idx_staging_path ON staging(path);
CREATE INDEX idx_staging_op ON staging(op);

-- Files in commits
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

-- Commit logs snapshot (staging state at time of commit)
CREATE TABLE commit_logs (
    id          TEXT PRIMARY KEY,
    commit_id   TEXT NOT NULL,
    op          TEXT NOT NULL,
    path        TEXT NOT NULL,
    blob_hash   TEXT,
    FOREIGN KEY (commit_id) REFERENCES commits(id)
);
CREATE INDEX idx_commit_logs_commit ON commit_logs(commit_id);

-- Schema version for future migrations
CREATE TABLE schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT DEFAULT (datetime('now'))
);
INSERT OR IGNORE INTO schema_version (version) VALUES (1);
```

### Key Design Decision: Shared Commits

The original plan included a `branch` column on the `commits` table with a foreign key to `branches(name)`. This caused problems because:

- In the old JSON system, each branch had its own `commits.json`, so the same commit ID could appear in multiple branches
- With a `branch` column and `id` as primary key, a commit could only belong to one branch
- Creating a branch required duplicating commit rows, which is wasteful and error-prone

**Solution**: Commits have no `branch` column. Branch membership is determined by walking the `parent_id` chain from each branch's `head_commit`. Creating a branch simply points the new branch's `head_commit` to the same commit as the source branch. This is simpler, more correct, and more efficient.

## Implementation

### New Files Created

| File | Purpose |
|------|---------|
| `cmd/nexio/db.go` | Database connection, schema initialization, transactions |
| `cmd/nexio/db_staging.go` | Staging CRUD operations |
| `cmd/nexio/db_commits.go` | Commit CRUD, metadata, logs, parent-chain traversal |
| `cmd/nexio/db_branches.go` | Branch CRUD, drop with orphan cleanup |
| `cmd/nexio/db_files.go` | File list operations, referenced hash collection |

### Modified Files

| File | Changes |
|------|---------|
| `go.mod` / `go.sum` | Added `modernc.org/sqlite` dependency |
| `dirs.go` | Removed `staging/`, `commits/`, `branches/` dirs; simplified to `root`, `objects`, `config` |
| `init_helper.go` | Calls `InitDB()` and `DBCreateBranch()` during init |
| `staging_log.go` | All functions delegate to DB functions |
| `commit_helper.go` | All functions delegate to DB functions; `CopyCommitsToBranch` creates branch pointing to commit |
| `branch_helper.go` | All functions delegate to DB functions |
| `status_helper.go` | Delegates to DB functions |
| `clean_helper.go` | `CollectReferencedHashes()` queries DB |
| `commit.go` | `runCoreCommitCommand` registers commit before inserting files/logs (FK ordering) |
| `branch.go` | Creating a branch just copies `head_commit` pointer; no commit row duplication |
| `history.go` | Uses `DBGetCommitMetadata` and `DBGetCommitLogs` |
| `root.go` | Added `PersistentPreRun`/`PersistentPostRun` hooks for DB lifecycle |

### Deleted Files

| File | Reason |
|------|--------|
| `lock.go` | File-based locking replaced by SQLite's built-in locking |

### Function Migration Map

#### staging_log.go

| Original Function | SQLite Replacement | Notes |
|---|---|---|
| `LogOperation()` | `DBLogOperation()` | No file locking needed |
| `LogEntryLookup()` | `DBLogEntryLookup()` | Uses index, O(log n) |
| `GetStagingLogsContent()` | `DBGetStagingLogs()` | Simple SELECT |
| `RemoveLogEntry()` | `DBRemoveLogEntry()` | No read-modify-write |
| `TruncateLogs()` | `DBTruncateLogs()` | DELETE is atomic |

#### status_helper.go

| Original Function | SQLite Replacement | Notes |
|---|---|---|
| `IsFileStaged()` | `DBIsFileStaged()` | Indexed lookup |
| `GetFileMetadata()` | `DBGetFileMetadata()` | Queries files table by commit + path |

#### commit_helper.go

| Original Function | SQLite Replacement | Notes |
|---|---|---|
| `GetLastCommit()` | `DBGetLastCommit()` | Queries `head_commit` from branches |
| `GetCommits()` | `DBGetCommits()` | Walks parent chain from head, reverses to chronological |
| `GetFileListContent()` | `DBGetFileListForCommit()` | Indexed by `commit_id` |
| `ProcessFileList()` | `DBProcessFileList()` | Generates new file IDs per commit |
| `RegisterCommit()` | `DBRegisterCommit()` | INSERT + update `branches.head_commit` |
| `CountCommits()` | `DBCountCommits()` | Walks parent chain from head |
| `CopyCommitsToBranch()` | Creates branch + sets `head_commit` | No commit duplication |

#### branch_helper.go

| Original Function | SQLite Replacement | Notes |
|---|---|---|
| `GetCurrentBranchName()` | `DBGetCurrentBranchName()` | `WHERE is_current = 1` |
| `GetDefaultBranchName()` | `DBGetDefaultBranchName()` | `WHERE is_default = 1` |
| `SetBranch()` | `DBSetBranch()` | UPDATE branches |
| `ListBranches()` | `DBListBranches()` | `SELECT name FROM branches` |
| `CreateBranchesMetadata()` | `DBCreateBranch()` | `INSERT INTO branches` |

#### clean_helper.go

| Original Function | SQLite Replacement | Notes |
|---|---|---|
| `CollectReferencedHashes()` | `DBCollectReferencedHashes()` | Queries files, staging, commit_logs tables |

## Technical Notes

### DB Lifecycle

- **CLI commands**: `PersistentPreRun` on `rootCmd` opens the DB for all commands except `init` and `purge`. `PersistentPostRun` closes it.
- **`init` command**: Opens the DB itself via `CreateDirs()` -> `InitDB()`.
- **Tests**: Each test calls `runInitCommand()` which calls `InitDB()`. `InitDB()` closes any existing connection before opening a new one, preventing stale connection issues.
- **Tests must run with `NEXIO_ENV=test`** to use the `__test__/` namespace (see `scripts/run-tests.sh`).

### Commit Ordering in `runCoreCommitCommand`

The commit record must be created **before** inserting files and commit logs, due to foreign key constraints on the `files` and `commit_logs` tables:

```
1. RegisterCommit()     -- INSERT into commits, UPDATE branch head
2. ProcessFileList()    -- INSERT into files (FK -> commits.id)
3. DBSaveCommitLogs()   -- INSERT into commit_logs (FK -> commits.id)
4. TruncateLogs()       -- DELETE from staging
```

### Branch Drop with Shared Commits

When dropping a branch, commits that are also reachable from other branches must not be deleted. `DBDropBranch` collects commit IDs reachable from the branch being dropped, compares with commits reachable from all other branches, and only deletes truly orphaned commits and their associated files/logs.

## References

- [SQLite Benchmark Results](./sqlite-benchmark.md)
- [modernc.org/sqlite Documentation](https://pkg.go.dev/modernc.org/sqlite)
- [SQLite WAL Mode](https://www.sqlite.org/wal.html)
