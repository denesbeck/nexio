# Diff Viewing Implementation Plan

Add a `nexio diff` command that shows, as a unified diff, what changed between a
file's committed version and its working-tree or staged version.

Addresses: [#19](https://github.com/denesbeck/nexio/issues/19) — *Implement diff viewing* (labels: `feature`, `critical`)

Builds on: [#18](https://github.com/denesbeck/nexio/issues/18) parent tracking (done) — diff reads the last commit on the current branch via the existing head/parent walk.

**Status: Pending**

## Overview

### Goals

1. `nexio diff [file...]` — unified diff of **working tree vs last commit** (unstaged changes).
2. `nexio diff --staged [file...]` — unified diff of **staging area vs last commit** (what the next commit will contain).
3. Standard unified-diff output (`--- / +++ / @@` hunks) with colored `+`/`-` lines.
4. Correct handling of added, modified, and deleted files, and binary files.

### Non-Goals

- Diffing between two arbitrary commits or branches (`nexio diff <a>..<b>`). The data model supports it (parent walk), but it's a separate feature.
- Word- or character-level intra-line diffing. Line-level only.
- Rename/copy detection.
- Patch application (`nexio apply`).

### Current State

There is no `diff` command. The pieces it needs already exist:

- `ReadBlob(hash) ([]byte, error)` (`blob.go:103`) — decompresses a stored blob back to original content. This yields the **committed** and **staged** versions of a file.
- `DBGetFileMetadata(path) (bool, FileListEntry)` (`db_files.go`) — the file's row (incl. `blob_hash`) in the **last commit** on the current branch.
- `DBLogEntryLookup("*", path) (bool, *LogFileEntry)` (`db_staging.go:21`) — the file's **staging** row (incl. `blob_hash` and `op`), if any.
- `GetModifiedOrDeletedFiles() (modified, deleted []string)` (`staging_log.go:231`) — already compares working-tree hashes against committed `blob_hash`; the natural file list for the default (unstaged) mode.
- `GetSyncedStagingLogsContent() []LogFileEntry` — the file list for `--staged` mode.
- Command/flag wiring via cobra (`status.go`, `stage.go` are templates); output helpers in `ui.go`; `add/mod/rem` color funcs in `staging_log.go:19`.

---

## The three "sides" of a file

Every diff compares two of these three content sources for a path:

| Side | Source | Resolver |
| ---- | ------ | -------- |
| **committed** | `blob_hash` from the last commit's `files` row | `DBGetFileMetadata` → `ReadBlob` |
| **staged** | `blob_hash` from the `staging` row (`REM` ⇒ empty) | `DBLogEntryLookup` → `ReadBlob` |
| **working** | bytes on disk | `os.ReadFile(path)` |

Each resolver returns `(content []byte, exists bool)`. "Missing" sides drive add/delete rendering:

- committed missing → left side empty → **all additions** (new file).
- working/staged missing (or `REM`) → right side empty → **all deletions**.

### Mode → comparison

| Command | Left (`---`) | Right (`+++`) |
| ------- | ------------ | ------------- |
| `nexio diff` | committed | working |
| `nexio diff --staged` | committed | staged |

This keeps nexio's simple model intact: default shows "what I could stage," `--staged` shows "what I'm about to commit."

---

## File selection

- **With `[file...]` args** — diff exactly those paths (validated/normalized like `stage`, via `ValidatePath` and the `namespace` prefix in tests). A path with no difference prints nothing (or a "no changes" note when a single explicit file is requested).
- **Without args**:
  - default → `GetModifiedOrDeletedFiles()` (modified + deleted tracked files).
  - `--staged` → every path in `GetSyncedStagingLogsContent()`.

---

## Design Decision: diff engine

Three viable options (both `sergi/go-diff` and `pmezard/go-difflib` are already transitive entries in `go.sum`):

1. **From-scratch LCS line diff** — a ~100-line Myers/LCS line differ plus a unified-hunk builder. Matches the project's "built from scratch" ethos (README line 7), adds no direct dependency, and gives full control over the `@@` hunk grouping and coloring. **Recommended.**
2. **`pmezard/go-difflib`** — a Python-`difflib` port that emits unified diffs (hunks, context, headers) directly; least code, but pulls a real dependency into `go.mod` and cedes output control.
3. **`sergi/go-diff`** (diff-match-patch) — character-level; wrong granularity for a line-oriented unified diff.

The plan assumes **option 1**. If we'd rather not own the hunk-grouping logic, option 2 is a drop-in fallback; the command/resolver layer is identical either way. Decision to confirm before coding.

### Engine shape (option 1)

```go
// diff_helper.go
type diffOp struct { kind byte; text string } // ' ' context, '-' del, '+' add

// lcsLineDiff returns the edit script between two line slices.
func lcsLineDiff(a, b []string) []diffOp

// unifiedDiff renders ops into hunks with `contextLines` (default 3) of context,
// producing `@@ -l,s +l,s @@` headers.
func unifiedDiff(aPath, bPath string, a, b []string, contextLines int) string
```

Split content into lines preserving the "\ No newline at end of file" case (track whether the final line had a trailing `\n`).

---

## Binary files

Before diffing, scan both sides for a NUL byte (Git's heuristic) or invalid UTF-8. If either side is binary, skip the line diff and print `Binary files a/<path> and b/<path> differ` (return code 1204). This avoids dumping control characters to the terminal.

---

## Output format

```
diff --nexio a/<path> b/<path>
--- a/<path>
+++ b/<path>
@@ -1,4 +1,5 @@
 unchanged line
-removed line        (red)
+added line          (green)
 unchanged line
```

- Header + `@@` lines: dim/cyan via `pterm` (reuse `ui.go` helpers).
- `+`/`-` lines: the existing `add`/`rem` color funcs (`staging_log.go:19-22`).
- New file: `--- /dev/null` (or `a/<path>` with "(new file)"); deleted: `+++ /dev/null`.
- Multiple files: concatenate per-file diffs with a blank line between.

---

## Files to Add / Change

| File | Change |
| ---- | ------ |
| `cmd/nexio/diff.go` (new) | `diffCmd` cobra command (alias `df`), `--staged`/`-s` bool flag, arg handling, `runDiffCommand(paths, staged)` orchestration + display. Follows `status.go`. |
| `cmd/nexio/diff_helper.go` (new) | Side resolvers (`resolveCommitted` / `resolveStaged` / `resolveWorking`), `isBinary`, `lcsLineDiff`, `unifiedDiff`, per-file `buildFileDiff`. |
| `cmd/nexio/return_codes.go` | New `DIFF_RETURN_CODES` in the **12xx** band (next free after remote 11xx). |
| `cmd/nexio/diff_test.go` (new) | Add/mod/rem, staged vs unstaged, no-change, untracked, binary, multi-file, hunk correctness, root-repo (no commits). |
| `README.md` | Add `diff` to the command table and a usage example; mention in Key Features. |

`push`/`pull`/`clone`/schema are untouched — diff is read-only.

## Return codes (`DIFF_RETURN_CODES`, 12xx)

```
1201: "Diff generated successfully."
1202: "No changes to display."
1203: "File is not tracked."      // no committed, staged, or working version
1204: "Binary files differ."
```

---

## Function Details

### `resolveCommitted(path string) ([]byte, bool)`
`ok, entry := DBGetFileMetadata(path)`; if `!ok` return `(nil, false)`. Else `ReadBlob(entry.BlobHash)`. Root repo (no commits) ⇒ `DBGetFileMetadata` returns `false` ⇒ new-file diff.

### `resolveStaged(path string) ([]byte, bool)`
`ok, e := DBLogEntryLookup("*", path)`; `!ok` ⇒ `(nil, false)`. `e.Op == "REM"` ⇒ `([]byte{}, true)` (deletion). Else `ReadBlob(e.BlobHash)`.

### `resolveWorking(path string) ([]byte, bool)`
`os.ReadFile(path)`; `os.IsNotExist` ⇒ `(nil, false)`.

### `buildFileDiff(path string, staged bool) (string, int)`
Resolve left = committed, right = working|staged. If neither side exists ⇒ `1203`. If equal bytes ⇒ `1202` (empty diff). If either binary ⇒ `1204`. Else split to lines and `unifiedDiff` ⇒ `1201`.

### `runDiffCommand(paths []string, staged bool) int`
Guard `IsInitialized`. Resolve the file set (explicit args, or the mode's default list). For each, `buildFileDiff` and print. Aggregate: if nothing printed, show `1202`.

---

## Testing (`diff_test.go`, `NEXIO_ENV=test`)

1. **Modified file (unstaged)** — commit a file, edit it on disk, `runDiffCommand([f], false)` shows the `-old/+new` lines with a correct `@@` header.
2. **Staged modification** — commit, edit, stage, `runDiffCommand([f], true)` shows the staged change; `false` mode (working == staged) shows the same content.
3. **Added file** — new untracked file stage; `--staged` shows all-additions against `/dev/null`.
4. **Deleted file** — commit, `os.Remove`; default diff shows all-deletions.
5. **No change** — committed file unchanged ⇒ `1202`, no output.
6. **Untracked, no arg match** — path never tracked ⇒ `1203`.
7. **Binary** — file with NUL bytes ⇒ `1204`, no line dump.
8. **Multiple files** — two modified files ⇒ two diff blocks.
9. **Root repo** — no commits yet; diff of a working file renders as a new-file addition.
10. **Hunk correctness** — a change in the middle of a long file produces one hunk with 3 lines of surrounding context and correct `-l,s +l,s` counts.
11. Full suite green (`scripts/run-tests.sh`).

---

## Rollout

1. Resolvers + engine (`diff_helper.go`) with unit tests for `lcsLineDiff`/`unifiedDiff` on plain string slices (no repo needed).
2. Command + display (`diff.go`), return codes.
3. Integration tests (`diff_test.go`) over real init/stage/commit flows.
4. README + command table.

Read-only and additive: no schema change, no migration, and no impact on existing commands.
