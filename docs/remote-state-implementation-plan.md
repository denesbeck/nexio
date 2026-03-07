# Remote State Management Implementation Plan

Add remote state management via AWS S3 to enable collaboration, backup, and multi-machine synchronization.

Addresses: [#5](https://github.com/denesbeck/code-sync/issues/5), [#6](https://github.com/denesbeck/code-sync/issues/6), [#14](https://github.com/denesbeck/code-sync/issues/14)

**Status: Pending**

## Overview

### Goals

1. Push local commits and blobs to a remote S3 bucket
2. Pull remote commits and blobs into a local repository
3. Clone a remote repository from S3 into a new local directory
4. Remote locking to prevent concurrent push/pull corruption
5. Minimal uploads/downloads -- only transfer what's missing

### Non-Goals

- Multi-backend support (only S3 for now; no abstraction layer)
- Merge conflict resolution (push fails if remote is ahead)
- Authentication management (relies on standard AWS credential chain)

---

## Remote URL Format

```
s3://<bucket>/<prefix>
```

Examples:

```
s3://my-bucket/nexio-repo
s3://my-bucket/team/project-alpha
```

The `<prefix>` acts as the repository root inside the bucket. All remote objects are stored under this prefix.

---

## Remote Storage Layout

The remote mirrors the local `.nexio/` structure inside the S3 prefix:

```
s3://<bucket>/<prefix>/
├── index.db              # Full SQLite database
├── objects/              # Content-addressable blob store (same sharding)
│   ├── ab/
│   │   └── 3f7c9e2d...  # Compressed blob (identical to local)
│   └── ...
├── config.json           # Repository config
└── nexio.lock            # Lock file for push/pull serialization
```

### Why upload the full `index.db`?

The SQLite database is small (typically < 1MB even for large repositories) and contains all metadata (commits, branches, files, commit_logs). Uploading it as a single file avoids the complexity of syncing individual tables or rows. On pull, the remote database is downloaded and merged into the local one.

---

## Remote Configuration

### Storage: `config.json`

Extend the existing `.nexio/config.json`:

```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "remote": "s3://my-bucket/nexio-repo"
}
```

### Config Type Update

```go
// config.go
type Config struct {
    Name   string `json:"name"`
    Email  string `json:"email"`
    Remote string `json:"remote,omitempty"`
}
```

### Setting the Remote

```bash
nexio config set remote s3://my-bucket/nexio-repo
nexio config get remote
```

---

## New Dependency

```
github.com/aws/aws-sdk-go-v2          # AWS SDK core
github.com/aws/aws-sdk-go-v2/config   # AWS credential/config loading
github.com/aws/aws-sdk-go-v2/service/s3
```

AWS credentials are resolved via the standard chain: environment variables, `~/.aws/credentials`, IAM roles, etc. Nexio does not manage credentials.

---

## Remote Locking

To prevent corruption from concurrent push/pull operations, a simple lock file mechanism is used.

### Lock File: `<prefix>/nexio.lock`

```json
{
  "holder": "John Doe <john@example.com>",
  "timestamp": "2026-03-06T12:00:00Z",
  "operation": "push"
}
```

### Acquire Lock

1. Check if `nexio.lock` exists in S3
2. If it exists, check the timestamp -- if older than 5 minutes, consider it stale and proceed (overwrite)
3. If it exists and is fresh, abort with: `Remote is locked by <holder> (<operation> started at <timestamp>). Use --force to override.`
4. Upload `nexio.lock` with the current user's info

### Release Lock

1. Delete `nexio.lock` from S3
2. Always release in a `defer` to handle panics/errors

---

## Commands

### `nexio push`

Upload local commits and blobs to the configured S3 remote.

```bash
nexio push
nexio push --remote s3://my-bucket/nexio-repo   # Override configured remote
nexio push --force                                # Override remote lock
```

#### Algorithm

1. **Validate** -- Ensure remote is configured (flag or config), no uncommitted staged changes
2. **Lock** -- Acquire remote lock
3. **Download remote `index.db`** -- to a temp file (if it exists)
4. **Diff commits** -- Compare local commit IDs with remote commit IDs. Collect commits that exist locally but not remotely.
5. **Fast-forward check** -- Verify the remote HEAD is an ancestor of the local HEAD (walk local parent chain). If not, abort with: `Remote has commits not present locally. Run nexio pull first.`
6. **Upload missing blobs** -- For each new commit, look up its `files` entries and upload any blob that doesn't exist remotely. Use `HeadObject` to check existence before uploading.
7. **Upload `index.db`** -- Upload the local database as the new remote database. This replaces the remote metadata entirely, which is safe because the fast-forward check ensures we're only adding to the remote history.
8. **Upload `config.json`** -- Sync config to remote
9. **Run `nexio clean`** -- Clean orphaned blobs locally before push (as noted in blob-storage.md)
10. **Release lock**

#### Output

```bash
$ nexio push
Pushing to s3://my-bucket/nexio-repo...
Uploading 3 new commits, 12 blobs (1.4 MB)
Push complete.
```

### `nexio pull`

Download remote commits and blobs from S3.

```bash
nexio pull
nexio pull --remote s3://my-bucket/nexio-repo
```

#### Algorithm

1. **Validate** -- Ensure remote is configured, no uncommitted staged changes
2. **Lock** -- Acquire remote lock
3. **Download remote `index.db`** -- to a temp file
4. **Diff commits** -- Compare remote commit IDs with local commit IDs. Collect commits that exist remotely but not locally.
5. **Fast-forward check** -- Verify the local HEAD is an ancestor of the remote HEAD. If not, abort with: `Local history has diverged from remote. This is currently unsupported.`
6. **Download missing blobs** -- For each new commit in the remote DB, look up its `files` entries and download any blob that doesn't exist locally.
7. **Merge database** -- Insert the new commits, files, and commit_logs from the remote DB into the local DB. Update branch HEAD pointers to match remote.
8. **Run `nexio clean`** -- Clean orphaned blobs locally after pull
9. **Release lock**

#### Database Merge Strategy

Open the remote `index.db` as an attached database:

```sql
ATTACH DATABASE '/tmp/remote_index.db' AS remote;
```

Then insert rows that don't exist locally:

```sql
-- Commits
INSERT OR IGNORE INTO commits SELECT * FROM remote.commits;

-- Files
INSERT OR IGNORE INTO files SELECT * FROM remote.files;

-- Commit logs
INSERT OR IGNORE INTO commit_logs SELECT * FROM remote.commit_logs;

-- Branches: update head_commit to remote's value
UPDATE branches SET head_commit = (
    SELECT head_commit FROM remote.branches WHERE remote.branches.name = branches.name
) WHERE name IN (SELECT name FROM remote.branches);

-- New branches from remote
INSERT OR IGNORE INTO branches SELECT * FROM remote.branches;
```

```sql
DETACH DATABASE remote;
```

#### Output

```bash
$ nexio pull
Pulling from s3://my-bucket/nexio-repo...
Downloading 5 new commits, 20 blobs (3.2 MB)
Pull complete.
```

### `nexio clone`

Clone a remote repository from S3 into a new local directory.

```bash
nexio clone s3://my-bucket/nexio-repo
nexio clone s3://my-bucket/nexio-repo ./local-dir
```

#### Algorithm

1. **Parse arguments** -- Extract remote URL and optional local directory (default: last segment of S3 prefix)
2. **Create target directory** -- `mkdir` the local directory, fail if it already contains `.nexio/`
3. **Initialize** -- Run `nexio init` logic in the target directory (creates `.nexio/`, `index.db`, default branch)
4. **Download `index.db`** -- Replace the freshly created `index.db` with the remote one
5. **Download all blobs** -- List all objects under `<prefix>/objects/` and download them into `.nexio/objects/`
6. **Download `config.json`** -- Download remote config, merge `remote` URL into it (preserve local name/email if set, set remote to the clone source)
7. **Restore working directory** -- Check out the HEAD commit of the current branch: for each file in the HEAD commit's `files` table, call `RestoreBlob()` to write the file to the working directory

#### Output

```bash
$ nexio clone s3://my-bucket/nexio-repo
Cloning from s3://my-bucket/nexio-repo into ./nexio-repo...
Downloading database...
Downloading 150 blobs (12.4 MB)...
Restoring working directory (45 files)...
Clone complete.
```

---

## New Files

| File | Purpose |
|------|---------|
| `cmd/nexio/remote.go` | S3 client initialization, `ParseRemoteURL()`, upload/download helpers |
| `cmd/nexio/remote_lock.go` | Lock acquisition, release, staleness check |
| `cmd/nexio/push.go` | `nexio push` command |
| `cmd/nexio/pull.go` | `nexio pull` command |
| `cmd/nexio/clone.go` | `nexio clone` command |
| `cmd/nexio/push_test.go` | Push tests |
| `cmd/nexio/pull_test.go` | Pull tests |
| `cmd/nexio/clone_test.go` | Clone tests |
| `cmd/nexio/remote_test.go` | S3 helper and lock tests |

## Modified Files

| File | Changes |
|------|---------|
| `go.mod` / `go.sum` | Add `aws-sdk-go-v2` dependencies |
| `cmd/nexio/config.go` | Add `set remote` / `get remote` subcommands |
| `cmd/nexio/config_helper.go` | No changes needed (Config struct is in config.go) |
| `cmd/nexio/root.go` | Add `clone` to the list of commands that skip DB opening (like `init` and `purge`) |
| `cmd/nexio/clean_helper.go` | Export `CleanOrphanedBlobs` for use in push/pull if not already exported |

---

## S3 Helper Functions (`remote.go`)

```go
// ParseRemoteURL parses "s3://bucket/prefix" into bucket and prefix
func ParseRemoteURL(url string) (bucket, prefix string, err error)

// NewS3Client creates an S3 client using the default AWS credential chain
func NewS3Client(ctx context.Context) (*s3.Client, error)

// UploadFile uploads a local file to S3
func UploadFile(ctx context.Context, client *s3.Client, bucket, key, localPath string) error

// DownloadFile downloads an S3 object to a local file
func DownloadFile(ctx context.Context, client *s3.Client, bucket, key, localPath string) error

// ObjectExists checks if an S3 object exists (HeadObject)
func ObjectExists(ctx context.Context, client *s3.Client, bucket, key string) (bool, error)

// ListObjects lists all objects under a prefix
func ListObjects(ctx context.Context, client *s3.Client, bucket, prefix string) ([]string, error)

// DeleteObject deletes an S3 object
func DeleteObject(ctx context.Context, client *s3.Client, bucket, key string) error

// GetRemoteURL resolves the remote URL from flag or config
func GetRemoteURL(flagValue string) (string, error)
```

---

## Command Tree Update

```
nexio (root)
├── ...existing commands...
├── push         (aliases: ps)    -- Push commits to remote
├── pull         (aliases: pl)    -- Pull commits from remote
└── clone        (aliases: cln)   -- Clone remote repository
```

---

## Implementation Order

### Phase 1: Foundation

1. Add AWS SDK dependencies to `go.mod`
2. Extend `Config` struct with `Remote` field
3. Add `nexio config set remote` / `nexio config get remote` subcommands
4. Implement `remote.go` -- S3 client, URL parsing, upload/download/list helpers
5. Implement `remote_lock.go` -- lock acquire/release

### Phase 2: Push

6. Implement `push.go` -- command, fast-forward check, blob diff, upload
7. Write `push_test.go`

### Phase 3: Pull

8. Implement `pull.go` -- command, blob diff, download, database merge
9. Write `pull_test.go`

### Phase 4: Clone

10. Implement `clone.go` -- command, full download, working directory restoration
11. Update `root.go` to skip DB opening for `clone`
12. Write `clone_test.go`

### Phase 5: Polish

13. Write `remote_test.go` for shared helpers
14. Run full test suite, fix any regressions
15. Update README.md to remove "No remote repository support" limitation

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No AWS credentials | `Failed to initialize S3 client: no credentials found. Configure AWS credentials.` |
| Bucket doesn't exist | `S3 bucket "xyz" does not exist or is not accessible.` |
| No remote configured | `No remote configured. Run: nexio config set remote s3://bucket/prefix` |
| Remote locked | `Remote is locked by <holder> (<op> at <time>). Use --force to override.` |
| Remote ahead (push) | `Remote has commits not present locally. Run nexio pull first.` |
| Local diverged (pull) | `Local history has diverged from remote. This is currently unsupported.` |
| Network error | `Failed to upload/download: <error>. Check your network connection.` |
| Uncommitted changes | `You have staged but uncommitted changes. Commit or unstage them first.` |

---

## Test Strategy

Testing S3 operations requires either a real bucket or a mock. Recommended approach:

- **Unit tests**: Mock the S3 client interface for URL parsing, lock logic, diff logic, and DB merge operations
- **Integration tests**: Use a real S3 bucket (gated behind `NEXIO_S3_TEST_BUCKET` env var) for end-to-end push/pull/clone flows
- **CI**: Integration tests run only when the env var is set (skipped by default)

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Full DB upload on push | Upload entire `index.db` | DB is small (< 1MB typically); avoids complex row-level sync |
| DB merge via ATTACH | Use SQLite ATTACH DATABASE | Native, fast, no custom merge logic needed |
| Lock file over DynamoDB | S3 object-based lock | Simpler, no extra AWS service; acceptable for small teams |
| Lock timeout | 5 minutes | Balances safety vs. stuck lock recovery |
| No backend abstraction | Direct S3 calls | YAGNI -- only S3 is needed now; easy to refactor later |
| AWS SDK v2 | `aws-sdk-go-v2` | Current generation, better performance, context support |
| Blob-level dedup on transfer | HeadObject check before upload | Minimizes data transfer; leverages existing content-addressing |
| Clean on push/pull | Auto-clean orphans | As designed in blob-storage.md lifecycle |

---

## Future Considerations

| Feature | Reason to Defer |
|---------|----------------|
| **DynamoDB locking** | Only needed for high-concurrency teams |
| **Multi-backend (GCS, Azure)** | Add backend interface when second provider is requested |
| **Partial clone** | Complexity; full clone is simple and sufficient for now |
| **Merge/rebase** | Significant complexity; linear-only history for now |
| **Parallel uploads** | Optimize later if push is slow; sequential is simpler |
| **Progress bars** | Nice UX improvement; add after core functionality works |
