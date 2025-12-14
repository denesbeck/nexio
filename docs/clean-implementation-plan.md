# Implementation Plan: `nexio clean` Command

## Overview

Create a new command to remove orphaned blobs from the object store that are no longer referenced by any commit or staging entry.

---

## Files to Create

| File                      | Purpose                    |
| ------------------------- | -------------------------- |
| `cmd/nexio/clean.go`      | Command + helper functions |
| `cmd/nexio/clean_test.go` | Tests                      |

---

## `clean.go` Structure

```go
// Functions
CollectReferencedHashes() map[string]bool
CleanOrphanedBlobs(dryRun bool, verbose bool) CleanResult
RemoveEmptyShardDirs() int

// Types
type CleanResult struct {
    Count      int
    FreedBytes int64
    Errors     []error
}

// Cobra command
var cleanCmd = &cobra.Command{...}

// Flags
--dry-run, -n    Preview deletions without removing files
--verbose, -v    List each blob being deleted
```

---

## Function Details

### `CollectReferencedHashes() map[string]bool`

1. Glob `GetDir("commits") + "*/fileList.json"`
2. For each file, unmarshal `[]FileListEntry`, collect non-empty `BlobHash`
3. Read staging via `GetStagingLogsContent()`
4. For each entry where `Op != "REM"`, collect non-empty `BlobHash`
5. Return the map

### `CleanOrphanedBlobs(dryRun, verbose bool) CleanResult`

1. Call `CollectReferencedHashes()`
2. Walk `GetDir("objects")` using `filepath.Walk`
3. Skip directories, process files only
4. Reconstruct hash: `filepath.Base(dir) + filepath.Base(file)` → e.g., `ab` + `3f7c9e...` → `ab3f7c9e...`
5. If hash not referenced:
   - `stat` file for size
   - If not `dryRun`, delete file; on error, append to `Errors`
   - If `verbose`, print path
   - Increment `Count`, add to `FreedBytes`
6. Return `CleanResult`

### `RemoveEmptyShardDirs() int`

1. Walk `GetDir("objects")`
2. For each shard directory (2-char name), check if empty
3. If empty, `os.Remove()`
4. Return count of removed directories

### `cleanCmd.Run`

1. Parse flags
2. Call `CleanOrphanedBlobs(dryRun, verbose)`
3. Call `RemoveEmptyShardDirs()` (if not dry-run)
4. Print summary:
   - `Removed X blobs, freed Y MB` (or `Would remove...` for dry-run)
   - `Removed N empty shard directories`
   - If errors: `Failed to remove M blobs:`

---

## Output Examples

```bash
# Normal run
$ nexio clean
Removed 12 blobs, freed 1.4 MB
Removed 3 empty shard directories

# Dry run
$ nexio clean --dry-run
Would remove 12 blobs (1.4 MB)

# Verbose
$ nexio clean -v
Removing .nexio/objects/ab/3f7c9e2d1a8b4f...
Removing .nexio/objects/cd/ef12345678901...
Removed 2 blobs, freed 156 KB
Removed 1 empty shard directory

# Nothing to clean
$ nexio clean
No orphaned blobs found

# With errors
$ nexio clean
Removed 10 blobs, freed 1.2 MB
Failed to remove 2 blobs:
  .nexio/objects/ab/xxx: permission denied
  .nexio/objects/cd/yyy: permission denied
```

---

## Test Cases (`clean_test.go`)

| Test                         | Description                                           |
| ---------------------------- | ----------------------------------------------------- |
| `TestCleanNoOrphans`         | All blobs referenced → nothing deleted                |
| `TestCleanWithOrphans`       | Create unreferenced blobs → verify deleted            |
| `TestCleanDryRun`            | Dry run → blobs still exist after                     |
| `TestCleanEmptyShardRemoval` | After cleaning, empty shard dirs removed              |
| `TestCleanStagingReferences` | Blobs referenced only by staging are kept             |
| `TestCleanPartialFailure`    | Simulate permission error → continues, reports errors |

---

## Reference Collection Strategy

Instead of traversing branches → commits → file lists, we directly glob the commits directory:

```
commits/*/fileList.json  →  All committed blob references
staging/logs.json        →  All staged blob references (excluding REM ops)
```

This is simpler and more performant than following the branch indirection.
