# Atomic Commit Implementation Plan

Make `commit` **all-or-nothing**: either every step of a commit succeeds and the
repository advances to a new consistent state, or the commit fails and the
repository is left exactly as it was before.

Addresses: [#16](https://github.com/denesbeck/nexio/issues/16) — *Add atomic commits with rollback* (labels: `refactor`, `security`)

**Status: Implemented** — commit now runs as a single transaction; rollback / retry / root-commit tests added in `commit_atomic_test.go`. See **Outcome** at the bottom for how the landed code differs from the original design sketch.

## Overview

### Goals

1. Wrap the entire commit write sequence in a **single SQLite transaction** so a
   failure at any step rolls back all of it.
2. Never update a branch head to a commit whose file list / logs / parent links
   are incomplete.
3. Surface commit failures as clean errors that leave the repo in its previous
   consistent state, instead of `os.Exit`-ing mid-write.

### Non-Goals

- Rolling back **blob files** on disk. Blobs are content-addressed and written at
  *stage* time (`stage_helper.go:88` → `WriteBlob`), not at commit time. A failed
  commit leaves no dangling *new* blobs, because commit writes none. Any blob
  already on disk from staging is either reused by the retry or reclaimed by the
  existing GC (`purge`) — it never corrupts state. See [[blob-storage]].
- Crash-safety of SQLite's own on-disk file. WAL mode (`db.go:27`) plus SQLite's
  atomic-commit guarantee already cover power loss / process kill *between*
  transactions; this plan makes the *application-level* commit a single such
  transaction.
- Changing the user-facing command or output. `nexio commit -m "…"` behaves
  identically on success.

### Current State

`runCoreCommitCommand` (`commit.go:213`) performs a commit as **four independent,
auto-committed steps**:

1. `RegisterCommit` → `DBRegisterCommit` (`db_commits.go:134`) — inserts the
   `commits` row **and** its `parent_order = 0` link inside a `WithTransaction`
   (atomic pair), then calls `DBUpdateBranchHead` **outside** that transaction.
2. `ProcessFileList` → `DBProcessFileList` (`db_files.go:61`) — inserts the new
   commit's `files` rows, one `db.Exec` each (autocommit).
3. `DBSaveCommitLogs` (`db_commits.go:230`) — inserts `commit_logs` rows, one
   `db.Exec` each (autocommit).
4. `TruncateLogs` → `DBTruncateLogs` (`db_staging.go:101`) — clears `staging`.

Every DB error routes through `MustSucceed` → `FatalError` → `os.Exit(1)`
(`error_handler.go:19`), so a failure mid-sequence **kills the process with the
partial writes already committed**. Concretely:

- The branch head is advanced in step 1 *before* files exist. If step 2 fails
  (e.g. `os.Stat` on a staged path, or a disk-full `INSERT`), the head now points
  at a commit with **no / partial `files` rows** — `history`, `status`, and
  checkout all read a corrupt commit.
- Step 2 or 3 failing partway leaves a commit with **some** file/log rows.
- `TruncateLogs` failing leaves stale `staging` rows (minor, but inconsistent).

The issue references `commit.go:45-76`; that range is now `runInteractiveCommit`.
The actual write sequence lives in `runCoreCommitCommand` (`commit.go:213-234`).

---

## Design Decision: one transaction, thread `tx` through

Because commit performs **only DB writes** (no blob I/O), a single
`*sql.Tx` spanning steps 1–4 gives us full atomicity for free — SQLite rolls the
whole thing back on any error, and nothing external needs undoing.

The blocker today is that every helper writes through the global `db` in
autocommit mode and aborts via `os.Exit`. Two things must change:

1. **Thread a `*sql.Tx`** through the commit write path so all statements join
   one transaction. Statements that run on the global `db` while a write
   transaction is open would execute *outside* it (defeating rollback) and, under
   SQLite's single-writer model, risk `SQLITE_BUSY`.
2. **Return errors instead of exiting** from the tx-scoped helpers, so
   `WithTransaction` can roll back and the caller can report a clean failure.
   `os.Exit` inside a transaction body would abandon the tx with no rollback.

We keep the existing autocommit helpers (`DBUpdateBranchHead`,
`DBProcessFileList`, …) for their **other** callers (`branch.go:318`,
`CopyCommitsToBranch`) and add `tx`-scoped, error-returning variants used only by
commit. This is the smallest change that isolates the new behavior to the commit
path.

### Reconcile staging *before* the transaction

`DBProcessFileList` calls `DBGetSyncedStagingLogs` (`db_files.go:130`), which
*deletes* staging rows for vanished files. That reconciliation is a pre-commit
cleanup, not part of the atomic unit — run it (and capture the resulting log
snapshot) **before** opening the transaction, then feed the snapshot into the
tx-scoped file/log inserts. This keeps the transaction body free of filesystem
side effects and makes it a pure, retryable DB unit.

---

## Files to Change

| File | Change |
| ---- | ------ |
| `cmd/nexio/db.go` | No schema change. Optionally add a `WithTransactionResult`-style helper only if a return value is needed; the existing `WithTransaction` (`db.go:121`) already covers the flow. |
| `cmd/nexio/db_commits.go` | Add tx-scoped, error-returning variants: `dbRegisterCommitTx(tx, commitId, message, branch, head)` (insert `commits` row + `parent_order = 0` link, **no** head update inside), `dbSaveCommitLogsTx(tx, commitId, logs)`. |
| `cmd/nexio/db_files.go` | Add `dbProcessFileListTx(tx, latestCommitId, newCommitId, stagingLogs)` — same logic as `DBProcessFileList` but inserts via `tx.Exec` and takes the already-synced staging snapshot as an argument (no `DBGetSyncedStagingLogs` call inside). |
| `cmd/nexio/db_branches.go` | Add `dbUpdateBranchHeadTx(tx, branch, commitId)`; keep `DBUpdateBranchHead` for `branch.go` / `CopyCommitsToBranch`. |
| `cmd/nexio/db_staging.go` | Add `dbTruncateLogsTx(tx)`; keep `DBTruncateLogs`. |
| `cmd/nexio/commit.go` | Rewrite `runCoreCommitCommand` to reconcile staging, then run steps 1–4 inside one `WithTransaction`, returning an error/return-code on failure instead of exiting mid-way. |
| `cmd/nexio/commit_helper.go` | Adjust `RegisterCommit` / `ProcessFileList` wrappers if their signatures move under the tx path (or leave them for non-commit callers). |
| `cmd/nexio/commit_test.go` | Add rollback / atomicity tests (see **Testing**). |

No other command writes commits, so the blast radius is the commit path only.

---

## Rewritten Commit Flow

`runCoreCommitCommand` (`commit.go:213`) becomes:

```go
func runCoreCommitCommand(message string) (int, string) {
    newCommitId := GenRandHex(20)
    latestCommitId := GetLastCommit().Id
    branch := GetCurrentBranchName()

    // Pre-transaction reconciliation: prune staging entries for vanished files
    // and take a stable snapshot to commit against.
    stagingLogs := GetSyncedStagingLogsContent()

    // Resolve current head once; it becomes the new commit's first parent.
    head := DBGetHeadCommitForBranch(branch)

    err := WithTransaction(func(tx *sql.Tx) error {
        // 1. commit row + parent_order = 0 link
        if err := dbRegisterCommitTx(tx, newCommitId, message, branch, head); err != nil {
            return err
        }
        // 2. file list for the new commit
        if err := dbProcessFileListTx(tx, latestCommitId, newCommitId, stagingLogs); err != nil {
            return err
        }
        // 3. persist staging logs as commit logs
        if err := dbSaveCommitLogsTx(tx, newCommitId, stagingLogs); err != nil {
            return err
        }
        // 4. advance branch head — only after 1–3 succeed
        if err := dbUpdateBranchHeadTx(tx, branch, newCommitId); err != nil {
            return err
        }
        // 5. clear staging
        return dbTruncateLogsTx(tx)
    })
    if err != nil {
        Debug("Commit failed, rolled back: %v", err)
        return <commit-failed-code>, ""   // repo unchanged
    }

    return 702, newCommitId
}
```

Key ordering change: **branch head is updated last**, inside the same
transaction, so the head never transiently points at an incomplete commit even
mid-transaction — and rolls back to the prior head on any failure.

### Error handling

The tx-scoped helpers **return** errors rather than calling `MustSucceed`. On
rollback, `runCoreCommitCommand` returns a failure code and the callers
(`runCommitCommand` at `commit.go:176`, `runInteractiveCommit` at `commit.go:55`)
render a `Fail(...)` message instead of printing the success spinner. Add a
`COMMIT_RETURN_CODES` entry (e.g. `703 → "Commit failed; repository unchanged"`)
in `return_codes.go` and gate the existing success UI on `returnCode == 702`
(the interactive path already does this at `commit.go:128`).

---

## Function Details

### `dbRegisterCommitTx(tx, commitId, message, branch, head string) error`
Same body as the current `WithTransaction` block in `DBRegisterCommit`
(`db_commits.go:149-169`) but using the passed-in `tx` and **without** the
trailing `DBUpdateBranchHead` — the head update moves to step 4 of the outer
transaction. Reads config/timestamp as today; inserts the `commits` row, and if
`head != ""`, inserts `(commitId, head, 0)` into `commit_parents`.

### `dbProcessFileListTx(tx, latestCommitId, newCommitId string, stagingLogs []LogFileEntry) error`
Same file-map merge logic as `DBProcessFileList` (`db_files.go:61`), but:
- takes the **already-synced** `stagingLogs` as an argument (caller reconciled),
- inserts each `files` row via `tx.Exec`,
- returns the first error (including the `os.Stat` failure on an ADD path)
  instead of `MustSucceed`.

### `dbSaveCommitLogsTx(tx, commitId string, logs []LogFileEntry) error`
`DBSaveCommitLogs` (`db_commits.go:230`) with `tx.Exec` and error return.

### `dbUpdateBranchHeadTx(tx, branch, commitId string) error`
`DBUpdateBranchHead` (`db_branches.go:221`) with `tx.Exec` and error return.

### `dbTruncateLogsTx(tx) error`
`DBTruncateLogs` (`db_staging.go:101`) with `tx.Exec` and error return.

---

## Testing

Add to `commit_test.go` (env `NEXIO_ENV=test`):

1. **Happy path unchanged** — stage + commit produces the `commits` row,
   `commit_parents` `(new, head, 0)` link, full `files` + `commit_logs` rows,
   advanced branch head, and empty `staging`. Existing commit tests still pass.
2. **Rollback on file-insert failure** — inject a failure in the file-insert
   step (e.g. stage a path, `os.Remove` it so `os.Stat` fails inside
   `dbProcessFileListTx`), run commit, then assert **nothing changed**: no new
   `commits` row, branch head equals the pre-commit head, no orphan
   `commit_parents` / `files` / `commit_logs` rows, and `staging` still holds the
   original entries.
3. **Head never advances on failure** — after an injected failure,
   `GetLastCommit().Id` equals the previous head.
4. **Staging preserved on failure** — `staging` is intact after a rolled-back
   commit, so the user can retry.
5. **Retry succeeds** — after a rolled-back commit, fixing the condition and
   re-running `commit` produces a clean single commit (no leftovers from the
   failed attempt).
6. **Root commit** — first-ever commit (empty `head`) commits atomically with
   zero `commit_parents` rows.
7. Full suite green: `scripts/run-tests.sh`.

> Injecting a mid-transaction failure may warrant a small seam (e.g. a test-only
> hook or an unwritable staged path). Prefer the `os.Stat`-on-removed-file route
> in test 2 since it needs no production code change.

---

## Rollout

1. Add tx-scoped helper variants (`db_commits.go`, `db_files.go`,
   `db_branches.go`, `db_staging.go`) — additive, no behavior change yet.
2. Rewrite `runCoreCommitCommand` to use the single `WithTransaction` and
   error-return path; add the failure return code + UI gating.
3. Tests (rollback, retry, head-invariance).
4. Manual soak: `disk-full` simulation (small tmpfs) and `SIGKILL` mid-commit to
   confirm the repo reopens clean.

Steps 1–2 are self-contained to the commit path; no schema migration and no
change to `push` / `pull` / `clone`, which copy the whole `index.db`.

---

## Outcome (as implemented)

The landed change follows the design above; a few details differ from the sketch:

- **`runCoreCommitCommand`** (`commit.go`) now syncs staging **once** via
  `GetSyncedStagingLogsContent()` before the transaction, then runs the four
  steps inside one `WithTransaction`, in order: `DBRegisterCommitTx` →
  `DBProcessFileListTx` → `DBSaveCommitLogs` → `DBTruncateLogsTx`. On any error it
  returns `703, ""` after the automatic rollback; on success `702, newCommitId`.
- **`DBSaveCommitLogs` was repurposed in place** to take a `*sql.Tx` and return an
  `error`, rather than adding a separate `DBSaveCommitLogsTx` — it had no callers
  outside the commit path, so a second variant would have been dead weight.
- **New tx-scoped helpers**: `DBRegisterCommitTx`, `DBProcessFileListTx`,
  `DBUpdateBranchHeadTx`, `DBTruncateLogsTx` — each writes via `tx.Exec` and
  returns errors instead of `MustSucceed`/`os.Exit`.
- **The branch-head update stays inside `DBRegisterCommitTx`** (not hoisted to a
  separate final step). Once everything shares one transaction, intra-transaction
  ordering is immaterial to atomicity, so this is equivalent and simpler.
- **`DBProcessFileListTx` takes the pre-synced `stagingLogs` snapshot** as a
  parameter and does **not** call `DBGetSyncedStagingLogs` itself — the mutating
  sync stays out of the transaction, and file list + commit logs are built from
  the same snapshot so they cannot diverge.
- **Dead code removed**: the pre-refactor non-transactional `DBRegisterCommit`,
  `DBProcessFileList`, and the `RegisterCommit` / `ProcessFileList` wrappers were
  deleted once the tx path replaced them.

### Tests

Landed in **`commit_atomic_test.go`** (not `commit_test.go`), env `NEXIO_ENV=test`:

- `Test_Commit_AtomicRollback` — drives the full four-step sequence and injects a
  failure **after** all writes, then asserts the branch head is unchanged, commit
  count is unchanged, there are zero orphan `commits` / `commit_parents` / `files`
  / `commit_logs` rows for the doomed id, and staging is preserved. This is the
  deterministic proof of rollback (covers plan tests 2–4).
- `Test_Commit_RetryAfterRollback` — after a rolled-back attempt, a normal commit
  succeeds, yielding exactly two commit rows (no leftovers), correct parent, and
  the expected file list (plan test 5).
- `Test_Commit_RootHasNoParents` — first commit is a root with zero
  `commit_parents` rows and an empty first parent (plan test 6).

The happy path (plan test 1) remains covered by the existing `commit_test.go`
suite, which still passes.

> **Note on the injected-failure approach.** The plan suggested triggering a real
> `os.Stat` failure by removing a staged file. In the final flow the pre-transaction
> `GetSyncedStagingLogsContent()` prunes vanished files *before* the tx, so that
> path is only reachable via a TOCTOU race and isn't deterministic. The tests
> instead drive the real tx helpers and return an injected error, which
> deterministically exercises `WithTransaction`'s rollback without a production
> seam.

### Coverage

The tx-scoped commit helpers land at ~65–90% statement coverage; the uncovered
lines are the individual error-return branches, reachable only by faulting a
specific `tx.Exec`. Project-wide the remaining coverage gap is the remote/S3 layer
(`remote.go`, `push.go`, `pull.go`, `clone.go`), which is unrelated to this change
and needs a live S3 to exercise.
