# Blob-Based Storage Optimization

## Overview

Transform Nexio from raw file storage to a **content-addressable blob store** for maximum storage efficiency and performance.

---

## Optimization Stack

| Component       | Technology                         | Purpose                                           |
| --------------- | ---------------------------------- | ------------------------------------------------- |
| **Hashing**     | BLAKE3 (`lukechampine.com/blake3`) | Fastest cryptographic hash, enables deduplication |
| **Compression** | Zlib (level 6)                     | 50-90% size reduction for text files              |
| **Sharding**    | 2-character prefix                 | Distributes blobs across ~256 subdirectories      |

---

## How It Works

```
1. File: src/main.go (10KB)
        |
        v
2. BLAKE3 hash -> "ab3f7c9e2d1a8b4f6e..."
        |
        v
3. Zlib compress -> ~3KB
        |
        v
4. Shard path -> .nexio/objects/ab/3f7c9e2d1a8b4f6e...
        |
        v
5. Dedup check -> Skip write if blob exists
```

---

## New Directory Structure

```
.nexio/
├── objects/                        # NEW: Content-addressable blob store
│   ├── 00/
│   ├── 01/
│   ├── ...
│   ├── ab/
│   │   ├── 3f7c9e2d1a8b4f6e...     # Compressed blob
│   │   └── cdef123456789012...     # Compressed blob
│   ├── ...
│   └── ff/
├── staging/
│   └── logs.json                   # Enhanced with blobHash field
├── commits/
│   └── <commit-hash>/
│       ├── fileList.json           # Enhanced with blobHash + mode
│       ├── metadata.json           # Unchanged
│       └── logs.json               # Unchanged
├── branches/
│   └── ...                         # Unchanged
└── config.json                     # Unchanged
```

**Key changes:**

- Raw file copies removed from `staging/added/`, `staging/modified/`, `staging/removed/`
- Raw file copies removed from `commits/<hash>/<file-id>/<filename>`
- All file content stored in `objects/` with deduplication

---

## Updated Data Structures

### FileListEntry (status_helper.go)

```go
type FileListEntry struct {
    Id       string `json:"id"`
    CommitId string `json:"commitId"`
    Path     string `json:"path"`
    BlobHash string `json:"blobHash"`  // NEW: BLAKE3 hash
    Mode     uint32 `json:"mode"`      // NEW: File permissions
}
```

### LogFileEntry (staging_log.go)

```go
type LogFileEntry struct {
    Id       string `json:"id"`
    Op       string `json:"op"`
    Path     string `json:"path"`
    BlobHash string `json:"blobHash"`  // NEW: BLAKE3 hash
}
```

---

## New Blob Module

**File:** `cmd/nexio/blob.go`

| Function                                                     | Description                                                 |
| ------------------------------------------------------------ | ----------------------------------------------------------- |
| `HashFile(path string) (string, error)`                      | Compute BLAKE3 hash of file (streaming, memory efficient)   |
| `HashBytes(data []byte) string`                              | Compute BLAKE3 hash of byte slice                           |
| `BlobPath(hash string) string`                               | Return sharded path: `ab3f...` -> `.nexio/objects/ab/3f...` |
| `BlobExists(hash string) bool`                               | Check if blob exists (for deduplication)                    |
| `WriteBlob(path string) (string, error)`                     | Hash, compress, store blob. Skip if exists. Return hash.    |
| `ReadBlob(hash string) ([]byte, error)`                      | Read and decompress blob content                            |
| `RestoreBlob(hash, destPath string, mode os.FileMode) error` | Decompress blob to destination with permissions             |

---

## New Clean Command

**File:** `cmd/nexio/clean.go`

```bash
nexio clean
```

Removes orphaned blobs not referenced by any commit or staging log.

**Logic:**

1. Collect all blob hashes referenced in:
   - All commits' `fileList.json`
   - Current staging `logs.json`
2. Walk `.nexio/objects/**/*`
3. Delete any blob not in the referenced set
4. Report: "Cleaned X blobs, freed Y MB"

---

## Blob Lifecycle

| Action                   | Behavior                                             |
| ------------------------ | ---------------------------------------------------- |
| `nexio add`              | Write blob to `objects/`, record hash in staging log |
| `nexio remove` / unstage | Remove from staging log, leave blob                  |
| `nexio commit`           | Reference blob hash in `fileList.json`               |
| `nexio clean`            | Delete all orphaned blobs                            |
| `nexio push`             | Run clean before pushing (don't upload orphans)      |
| `nexio pull`             | Run clean after pulling (remove new orphans)         |

---

## Files to Modify

| File                         | Changes                                                          |
| ---------------------------- | ---------------------------------------------------------------- |
| `go.mod`                     | Add `lukechampine.com/blake3` dependency                         |
| `cmd/nexio/blob.go`          | **NEW** - All blob storage functions                             |
| `cmd/nexio/clean.go`         | **NEW** - Orphaned blob cleanup command                          |
| `cmd/nexio/dirs.go`          | Add `Objects` field to `Dirs` struct                             |
| `cmd/nexio/init_helper.go`   | Create `objects/` directory on `nexio init`                      |
| `cmd/nexio/staging_log.go`   | Add `BlobHash` field to `LogFileEntry`                           |
| `cmd/nexio/status_helper.go` | Add `BlobHash` and `Mode` fields to `FileListEntry`              |
| `cmd/nexio/add_helper.go`    | Replace `CopyFile()` with `WriteBlob()`, remove staging dirs     |
| `cmd/nexio/commit_helper.go` | Store blob hash instead of copying files                         |
| `cmd/nexio/branch.go`        | Use `RestoreBlob()` instead of `CopyFile()` for file restoration |
| `cmd/nexio/file.go`          | Add `IsModifiedByHash()` for fast hash-based comparison          |

---

## Storage Efficiency

| Scenario                            | Before (Raw)       | After (Blobs)          | Savings         |
| ----------------------------------- | ------------------ | ---------------------- | --------------- |
| 10 commits, same 1MB file unchanged | 10MB               | ~300KB                 | **97%**         |
| 100KB source file                   | 100KB              | ~30KB                  | **70%**         |
| 10 identical files across project   | 1MB                | 100KB                  | **90%**         |
| 10,000 objects                      | 1 directory (slow) | ~39 files/shard (fast) | **O(1) lookup** |

---

## Performance Comparison

| Operation               | Before              | After                      |
| ----------------------- | ------------------- | -------------------------- |
| **File comparison**     | Byte-by-byte (slow) | Hash comparison (instant)  |
| **Duplicate detection** | None                | Automatic via content hash |
| **Storage per commit**  | Full file copies    | Only new/changed blobs     |
| **Directory listing**   | Degrades with scale | Constant via sharding      |

---

## Design Decisions

| Decision         | Choice                            | Rationale                                          |
| ---------------- | --------------------------------- | -------------------------------------------------- |
| Hash algorithm   | BLAKE3                            | 3-4x faster than SHA-256, cryptographically secure |
| Compression      | Zlib level 6                      | Good balance of speed and compression ratio        |
| Shard prefix     | 2 characters                      | ~256 directories, handles millions of objects      |
| Staging storage  | Hash reference only               | Most efficient, no duplicate storage               |
| Orphan cleanup   | `nexio clean` + auto on push/pull | Clean before upload, after download                |
| Chunking         | Not implemented                   | Overhead exceeds benefit for source code files     |
| Migration        | Not supported                     | Fresh repos only                                   |
| File permissions | Full `uint32`                     | Preserves exact Unix permissions                   |

---

## Implementation Order

1. Add BLAKE3 dependency
2. Update `dirs.go` with Objects path
3. Create `blob.go` with all blob functions
4. Update `init_helper.go` to create objects directory
5. Update `staging_log.go` with BlobHash field
6. Update `status_helper.go` with BlobHash and Mode fields
7. Update `add_helper.go` to use WriteBlob
8. Update `commit_helper.go` to store blob references
9. Update `branch.go` to restore from blobs
10. Update `file.go` with hash-based comparison
11. Create `clean.go` with nexio clean command
12. Run tests and fix issues

---

## Not Included (Future Considerations)

| Feature                     | Reason to Defer                                     |
| --------------------------- | --------------------------------------------------- |
| **Chunking**                | Complexity overhead; source files are small         |
| **Delta compression**       | Significant complexity; whole-file dedup sufficient |
| **Packfiles**               | Only needed for very large repos (100k+ objects)    |
| **Migration**               | Fresh implementation; no legacy repos to support    |
| **Auto garbage collection** | Will run automatically on push/pull                 |
