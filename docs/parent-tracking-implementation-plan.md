# Parent Tracking Implementation Plan

Extend commit parent tracking to support **multiple parents**, so a merge commit can reference both the current branch head and the merged branch head.

Addresses: [#18](https://github.com/denesbeck/nexio/issues/18) — prerequisite for [#19](https://github.com/denesbeck/nexio/issues/19) (diff) and [#1](https://github.com/denesbeck/nexio/issues/1) (merge)

**Status: Pending**

## Overview

### Goals

1. Allow a commit to reference more than one parent (for merge commits).
2. Store **all** parent relations in a single normalized table, ordered so the first parent is unambiguous.
3. Carry multi-parent data across `push` / `pull` / `clone`.

### Non-Goals

- Implementing `merge` itself (#1) — this only provides the data model it needs.
- Octopus merges (3+ parents). The design allows N parents, but no command will create more than 2.
- Backward compatibility with pre-existing dev databases (see below).

### Current State

Commits store a single nullable `parent_id` column (`db.go:60`), set at commit time to the previous branch head (`db_commits.go:157`). Every traversal — history, count, branch walking, push/pull — follows this single pointer. The chain is strictly linear; there is no way to represent a commit with two parents.

---

## Design Decision: fully normalized, drop `parent_id`

Nexio is **not released yet**, so we treat this as a breaking schema change and do not preserve the old column or migrate existing dev repos. All parent relations — including the first parent — move into a dedicated `commit_parents` table, keyed by `parent_order`:

- `parent_order = 0` → first parent (the linear-history parent).
- `parent_order = 1, 2, …` → additional merge parents.

`commits.parent_id` and its index (`idx_commits_parent`) are **removed**. A commit's parents are always read from `commit_parents`, ordered by `parent_order`. This keeps a single source of truth, cleanly supports N parents, and avoids the denormalization of caching the first parent in two places.

---

## Schema Changes

### `commits` table — remove `parent_id`

```sql
CREATE TABLE IF NOT EXISTS commits (
    id           TEXT PRIMARY KEY,
    timestamp    TEXT NOT NULL,
    message      TEXT NOT NULL,
    author_name  TEXT NOT NULL,
    author_email TEXT NOT NULL
);
```

Drop the `parent_id` column, its `FOREIGN KEY`, and `CREATE INDEX ... idx_commits_parent`.

### New table `commit_parents`

```sql
CREATE TABLE IF NOT EXISTS commit_parents (
    commit_id    TEXT NOT NULL,
    parent_id    TEXT NOT NULL,
    parent_order INTEGER NOT NULL,   -- 0 = first parent, 1+ = merge parents
    PRIMARY KEY (commit_id, parent_order),
    FOREIGN KEY (commit_id) REFERENCES commits(id),
    FOREIGN KEY (parent_id) REFERENCES commits(id)
);
CREATE INDEX IF NOT EXISTS idx_commit_parents_commit ON commit_parents(commit_id);
```

### No migration runner

Because the schema is (re)created idempotently via `CREATE TABLE IF NOT EXISTS` on every `InitDB` (`db.go:36` → `initSchema`), and we are **not** preserving old databases, no v1→v2 backfill is written. Existing dev repos should be recreated (`nexio purge` + `init`, or delete `.nexio/`). Bump `schema_version` to `2` to mark the format break:

```sql
INSERT OR IGNORE INTO schema_version (version) VALUES (2);
```

---

## Files to Change

| File | Change |
| ---- | ------ |
| `cmd/nexio/db.go` | Remove `parent_id` from `commits`; add `commit_parents` table + index; bump `schema_version` to 2 |
| `cmd/nexio/db_commits.go` | Rewrite parent reads/writes to use `commit_parents`: `DBRegisterCommit` inserts `parent_order = 0`; new `DBGetParents`, `DBGetFirstParent`, `DBAddParent`; update `DBGetLastCommitByBranch`, `DBCountCommits`, `DBGetCommitsByBranch` |
| `cmd/nexio/db_branches.go` | Update `collectCommitIds` (and any sibling walk) to read first parent from `commit_parents` |
| `cmd/nexio/commit_helper.go` | Extend `Commit` with `Parents []string` (JSON `parents`); drop reliance on the single-parent read |
| `cmd/nexio/pull.go` | Remove `parent_id` from the `commits` merge insert; add a `commit_parents` merge insert |
| `cmd/nexio/db_commits_test.go` (new) | Tests for multi-parent read/write, first-parent ordering, linear-history parity |

`push.go` and `clone.go` need **no change**: push uploads the full `index.db`, and clone copies the whole DB file — the new table travels with them automatically. The only row-by-row table merge is in `pull.go` (`pull.go:352`).

### Call sites that currently read `commits.parent_id`

All must switch to `commit_parents` (first parent = `parent_order = 0`):

- `db_commits.go:34` (`DBGetLastCommitByBranch`)
- `db_commits.go:62` (`DBCountCommits`)
- `db_commits.go:103`, `:113` (`DBGetCommitsByBranch`)
- `db_branches.go:207` (`collectCommitIds`)
- `db_commits.go:157` (`DBRegisterCommit` — write)
- `pull.go:354` (merge insert)

---

## Function Details

### `DBGetFirstParent(commitId string) string`
```sql
SELECT parent_id FROM commit_parents WHERE commit_id = ? AND parent_order = 0
```
Returns `""` if the commit has no parent (root commit). Replaces every inlined `SELECT parent_id FROM commits` used for linear walks.

### `DBGetParents(commitId string) []string`
```sql
SELECT parent_id FROM commit_parents WHERE commit_id = ? ORDER BY parent_order
```
Returns the full ordered parent list. Used by merge, `history` display, and integrity checks.

### `DBAddParent(commitId, parentId string, order int)`
Inserts one `commit_parents` row. `DBRegisterCommit` calls it with `order = 0`; the future `merge` command calls it again with `order = 1` for the second parent.

### `DBRegisterCommit` (rewritten)
1. Resolve the current branch head (as today).
2. Insert the commit row **without** `parent_id`.
3. If the head is non-empty, `DBAddParent(commitId, head, 0)`.
4. Update branch head (as today).

Wrap steps 2–4 in the existing `WithTransaction` helper so the commit and its parent row are atomic.

---

## Push / Pull / Clone

- **push** — full `index.db` upload; no change.
- **clone** — full DB file copy; no change.
- **pull** — in the merge transaction (`pull.go:352`), drop `parent_id` from the `commits` insert and add, **after** it (so FK targets exist):

```sql
INSERT OR IGNORE INTO commit_parents (commit_id, parent_id, parent_order)
SELECT commit_id, parent_id, parent_order FROM remote.commit_parents;
```

---

## Testing

1. **Fresh repo** — `init` creates `commits` (no `parent_id`) and `commit_parents`; `schema_version` = 2.
2. **Linear history parity** — a normal sequence of commits produces one `commit_parents` row per commit at `parent_order = 0`; `history`, `status`, and count output are identical to before.
3. **Root commit** — the first commit has zero `commit_parents` rows; `DBGetFirstParent` returns `""`.
4. **Multi-parent read** — insert a commit with rows `(C, A, 0)` and `(C, B, 1)`; assert `DBGetParents(C) == [A, B]` and `DBGetFirstParent(C) == A`.
5. **Pull round-trip** — push a repo containing a multi-parent commit, pull into a fresh clone, assert `commit_parents` rows transfer and `DBGetParents` matches.
6. Full existing suite (`scripts/run-tests.sh`) passes.

---

## Rollout

1. Schema change (`db.go`): drop `parent_id`, add `commit_parents`, bump version.
2. Read/write helpers (`db_commits.go`, `db_branches.go`) + `Commit.Parents`.
3. Pull merge update (`pull.go`).
4. Tests.

Steps 1–2 are inert for normal usage (every commit writes exactly one `parent_order = 0` row until `merge` lands), so this can merge safely ahead of #19 and #1.
