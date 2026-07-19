package main

import (
	"database/sql"
	"errors"
	"os"
	"testing"
)

// countRows runs a COUNT(*)-style query and returns the single integer result.
func countRows(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q failed: %v", query, err)
	}
	return n
}

// Test_Commit_AtomicRollback is the core guarantee of issue #16: if any step of
// a commit fails, the whole commit is rolled back and the repository is left in
// its previous consistent state. The failure is injected at the very end of the
// transaction — after every write has been issued — so this proves that even a
// last-moment failure unwinds the commit row, its parent link, the branch-head
// advance, the file list, the commit logs, and the staging truncate together.
func Test_Commit_AtomicRollback(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Baseline: one committed file establishes a branch head to roll back to.
	fileA := namespace + "a.txt"
	os.WriteFile(fileA, []byte("a"), 0644)
	runStageCommand(fileA, false)
	_, baseCommitId := runCommitCommand("base commit")

	baseHead := GetLastCommit().Id
	baseCount := CountCommits()
	if baseHead != baseCommitId || baseCount != 1 {
		t.Fatalf("unexpected baseline: head=%s count=%d", baseHead, baseCount)
	}

	// Stage a second file so the (about-to-fail) commit has real work to do.
	fileB := namespace + "b.txt"
	os.WriteFile(fileB, []byte("b"), 0644)
	runStageCommand(fileB, false)
	stagedBefore := len(DBGetStagingLogs())
	if stagedBefore == 0 {
		t.Fatal("expected staged entries before the commit attempt")
	}

	// Drive the exact commit sequence, but return an injected error after all
	// four steps. WithTransaction must roll every one of them back.
	newCommitId := GenRandHex(20)
	branch := GetCurrentBranchName()
	stagingLogs := GetSyncedStagingLogsContent()
	injected := errors.New("injected failure")

	err := WithTransaction(func(tx *sql.Tx) error {
		if err := DBRegisterCommitTx(tx, newCommitId, "doomed commit", branch); err != nil {
			return err
		}
		if err := DBProcessFileListTx(tx, baseCommitId, newCommitId, stagingLogs); err != nil {
			return err
		}
		if err := DBSaveCommitLogs(tx, newCommitId, stagingLogs); err != nil {
			return err
		}
		if err := DBTruncateLogsTx(tx); err != nil {
			return err
		}
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("expected the injected error to surface, got %v", err)
	}

	// The branch head must not have advanced.
	if got := GetLastCommit().Id; got != baseHead {
		t.Errorf("branch head advanced on failure: want %s, got %s", baseHead, got)
	}
	if got := CountCommits(); got != baseCount {
		t.Errorf("commit count changed on failure: want %d, got %d", baseCount, got)
	}

	// No partial rows from the doomed commit may survive.
	for _, tbl := range []struct {
		name  string
		query string
	}{
		{"commits", "SELECT COUNT(*) FROM commits WHERE id = ?"},
		{"commit_parents", "SELECT COUNT(*) FROM commit_parents WHERE commit_id = ?"},
		{"files", "SELECT COUNT(*) FROM files WHERE commit_id = ?"},
		{"commit_logs", "SELECT COUNT(*) FROM commit_logs WHERE commit_id = ?"},
	} {
		if n := countRows(t, tbl.query, newCommitId); n != 0 {
			t.Errorf("orphan %s rows after rollback: %d", tbl.name, n)
		}
	}

	// Staging must be preserved so the user can retry.
	if got := len(DBGetStagingLogs()); got != stagedBefore {
		t.Errorf("staging not preserved after rollback: want %d, got %d", stagedBefore, got)
	}

	os.RemoveAll(namespace)
}

// Test_Commit_RetryAfterRollback verifies that after a rolled-back commit the
// staging area is still intact and a normal commit succeeds cleanly, leaving no
// leftovers from the failed attempt.
func Test_Commit_RetryAfterRollback(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	fileA := namespace + "a.txt"
	os.WriteFile(fileA, []byte("a"), 0644)
	runStageCommand(fileA, false)
	_, baseCommitId := runCommitCommand("base commit")

	fileB := namespace + "b.txt"
	os.WriteFile(fileB, []byte("b"), 0644)
	runStageCommand(fileB, false)

	// Simulate a failed commit attempt that rolls back.
	doomedId := GenRandHex(20)
	branch := GetCurrentBranchName()
	stagingLogs := GetSyncedStagingLogsContent()
	_ = WithTransaction(func(tx *sql.Tx) error {
		if err := DBRegisterCommitTx(tx, doomedId, "doomed", branch); err != nil {
			return err
		}
		if err := DBProcessFileListTx(tx, baseCommitId, doomedId, stagingLogs); err != nil {
			return err
		}
		return errors.New("boom")
	})

	// Retry with a real commit — it must succeed and be clean.
	code, retryId := runCommitCommand("retry commit")
	if code != 702 {
		t.Fatalf("expected retry commit to succeed (702), got %d", code)
	}
	if got := CountCommits(); got != 2 {
		t.Errorf("expected 2 commits after retry, got %d", got)
	}
	// Exactly two commit rows total: baseline + retry. The doomed id must not persist.
	if total := countRows(t, "SELECT COUNT(*) FROM commits"); total != 2 {
		t.Errorf("expected exactly 2 commit rows (no leftovers), got %d", total)
	}
	if n := countRows(t, "SELECT COUNT(*) FROM commits WHERE id = ?", doomedId); n != 0 {
		t.Errorf("doomed commit id leaked into commits table: %d rows", n)
	}
	if got := DBGetFirstParent(retryId); got != baseCommitId {
		t.Errorf("retry commit first parent: want %s, got %s", baseCommitId, got)
	}
	if files := GetFileListContent(retryId); len(files) != 2 {
		t.Errorf("expected 2 files in retry commit, got %d", len(files))
	}

	os.RemoveAll(namespace)
}

// Test_Commit_RootHasNoParents confirms the first commit in a repository is a
// root: it commits atomically and writes zero commit_parents rows.
func Test_Commit_RootHasNoParents(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	file := namespace + "root.txt"
	os.WriteFile(file, []byte("root"), 0644)
	runStageCommand(file, false)
	code, commitId := runCommitCommand("root commit")
	if code != 702 {
		t.Fatalf("expected 702, got %d", code)
	}

	if parents := DBGetParents(commitId); len(parents) != 0 {
		t.Errorf("root commit should have no parents, got %d", len(parents))
	}
	if n := countRows(t, "SELECT COUNT(*) FROM commit_parents"); n != 0 {
		t.Errorf("expected no commit_parents rows for a root-only repo, got %d", n)
	}
	if got := DBGetFirstParent(commitId); got != "" {
		t.Errorf("root commit first parent should be empty, got %q", got)
	}

	os.RemoveAll(namespace)
}
