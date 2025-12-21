# SQLite vs JSON Storage Benchmark

## Overview

This document analyzes the performance implications of using SQLite vs the current JSON file-based storage for Nexio's metadata (staging logs, commit file lists, etc.).

## Methodology

A benchmark program was created to compare both approaches across different dataset sizes (100, 1,000, 5,000, 10,000 files). The benchmark tests:

1. **Staging Operations** - Adding files to staging and looking up staged files
2. **Commit File Lists** - Looking up files within a commit
3. **Garbage Collection** - Collecting all referenced blob hashes across commits

### Test Environment

- Platform: macOS (darwin)
- SQLite: `modernc.org/sqlite` (pure Go, no CGO required)
- Each timing averaged over 100 iterations for lookups

### Current JSON Implementation

The current approach stores metadata in JSON files:

- `staging/logs.json` - Array of staged file entries
- `commits/<id>/fileList.json` - Array of files per commit
- `branches/<name>/commits.json` - Linked list of commits

Each operation reads the entire file, parses JSON, performs the operation, and rewrites the file.

### SQLite Implementation

The benchmark uses a single SQLite database with indexed tables:

```sql
CREATE TABLE staging (
    id TEXT PRIMARY KEY,
    op TEXT,
    path TEXT,
    blob_hash TEXT
);
CREATE INDEX idx_staging_path ON staging(path);

CREATE TABLE files (
    id TEXT PRIMARY KEY,
    commit_id TEXT,
    path TEXT,
    blob_hash TEXT,
    mode INTEGER
);
CREATE INDEX idx_files_commit_path ON files(commit_id, path);
CREATE INDEX idx_files_blob ON files(blob_hash);
```

## Results

### Staging Operations

| Files | JSON Add All | SQLite Add (batch) | Improvement | JSON Lookup | SQLite Lookup | Improvement |
|-------|--------------|-------------------|-------------|-------------|---------------|-------------|
| 100 | 10ms | 418µs | 24x | 93µs | 10µs | 9.5x |
| 1,000 | 508ms | 2.2ms | 233x | 763µs | 12µs | 66x |
| 5,000 | 12s | 12.6ms | 955x | 4ms | 9µs | 440x |
| 10,000 | 48s | 23ms | 2,070x | 8ms | 10µs | 798x |

### Commit File List Lookups

| Files | JSON | SQLite | Improvement |
|-------|------|--------|-------------|
| 100 | 116µs | 10µs | 11.5x |
| 1,000 | 967µs | 10µs | 97x |
| 5,000 | 4.6ms | 11µs | 403x |
| 10,000 | 9ms | 11µs | 840x |

### Garbage Collection (50 commits)

| Files/Commit | JSON | SQLite | Improvement |
|--------------|------|--------|-------------|
| 10 | 1.07ms | 129µs | 8.3x |
| 100 | 5.4ms | 1.4ms | 3.9x |
| 500 | 24ms | 6.5ms | 3.7x |
| 1,000 | 48ms | 14ms | 3.5x |

## Analysis

### Why JSON Performance Degrades

The current JSON approach suffers from O(n²) complexity for sequential operations:

1. **Each `nexio add`**: Read entire file -> Parse JSON -> Append -> Serialize -> Write entire file
2. **Each lookup**: Read entire file -> Parse JSON -> Linear scan through all entries

As the number of files grows, each operation takes longer, and the total time grows quadratically.

### Why SQLite Scales

1. **B-tree indexes**: O(log n) lookups instead of O(n) linear scans
2. **Batched writes**: Single transaction for multiple inserts
3. **No full-file rewrites**: Appends are O(1) amortized
4. **Query optimization**: SQLite's query planner uses indexes efficiently

### Complexity Comparison

| Operation | JSON | SQLite |
|-----------|------|--------|
| Add single entry | O(n) - rewrite entire file | O(log n) - B-tree insert |
| Add n entries sequentially | O(n²) | O(n log n) |
| Lookup by path | O(n) - linear scan | O(log n) - index lookup |
| Collect all hashes | O(commits × files) | O(files) - single query |

## Real-World Impact

### `nexio status` on 10,000 file repository

Current (JSON):
- Check if each file is staged: 10,000 × 8ms = **~80 seconds**
- Check each file in commit: 10,000 × 9ms = **~90 seconds**
- **Total: ~3 minutes** just for lookups

With SQLite:
- Check if each file is staged: 10,000 × 10µs = **~100ms**
- Check each file in commit: 10,000 × 11µs = **~110ms**
- **Total: ~200ms**

### `nexio add .` on 5,000 files

- Current JSON: **12 seconds**
- With SQLite (batched): **13 milliseconds**

### `nexio clean` on repository with 50 commits × 1,000 files

- Current JSON: 48ms (reading 50 separate JSON files)
- With SQLite: 14ms (single indexed query)

## Recommendation

| Repo Size | Current Performance | Recommendation |
|-----------|--------------------|--------------------|
| < 500 files | Acceptable | Optional migration |
| 500-2,000 files | Noticeable lag | Recommended |
| > 2,000 files | Unusable | Essential |

### Proposed Hybrid Architecture

Keep blobs as files (content-addressable storage is already optimal), use SQLite for metadata:

```
.nexio/
├── objects/          # Keep as files (blobs need streaming I/O)
│   ├── ab/
│   │   └── cdef1234...
│   └── ff/
├── index.db          # NEW: SQLite database for metadata
│   ├── staging       # (id, op, path, blob_hash)
│   ├── files         # (commit_id, path, blob_hash, mode)
│   ├── commits       # (id, branch, parent_id, timestamp, message, author)
│   └── branches      # (name, head_commit, is_default, is_current)
└── config.json       # Keep as JSON (rarely accessed, human-editable)
```

### Trade-offs

| Aspect | JSON (Current) | SQLite |
|--------|----------------|--------|
| Performance at scale | Poor | Excellent |
| Human readability | Easy to inspect | Requires sqlite3 CLI |
| Corruption recovery | Manual JSON editing | SQLite recovery tools |
| Dependencies | None | `modernc.org/sqlite` (pure Go) |
| Concurrent access | File locking | Built-in ACID transactions |

## Reproducing the Benchmark

The benchmark script is available at `scripts/benchmark_storage.go`. To run:

```bash
cd scripts
go run benchmark_storage.go
```

Requirements:
- Go 1.21+
- `go get modernc.org/sqlite`

The script will output markdown-formatted tables comparing JSON vs SQLite performance across all dataset sizes.
