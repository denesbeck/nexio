package main

import (
	"os"
	"strconv"
	"testing"
)

// With SQLite, history is derived by walking the parent chain (parent_order 0)
// backward from the branch head and reversing, instead of a linked-list Next
// pointer. These tests verify the parent-based model.

// Test_ParentChain_Linear verifies that a linear sequence of commits forms a
// single-parent chain: the root has no parent, and each later commit's first
// parent is its immediate predecessor.
func Test_ParentChain_Linear(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	defer os.RemoveAll(namespace)

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	var ids []string
	for i := range 5 {
		file := namespace + "file" + strconv.Itoa(i) + ".txt"
		os.Create(file)
		runStageCommand(file, false)
		_, commitId := runCommitCommand("commit " + strconv.Itoa(i))
		ids = append(ids, commitId)
	}

	// Root commit has no parent.
	if fp := DBGetFirstParent(ids[0]); fp != "" {
		t.Errorf("Expected root commit to have no parent, got %q", fp)
	}
	if parents := DBGetParents(ids[0]); len(parents) != 0 {
		t.Errorf("Expected root commit to have 0 parents, got %d", len(parents))
	}

	// Every subsequent commit links back to its predecessor at order 0.
	for i := 1; i < len(ids); i++ {
		fp := DBGetFirstParent(ids[i])
		if fp != ids[i-1] {
			t.Errorf("Commit %d: expected first parent %q, got %q", i, ids[i-1], fp)
		}
		parents := DBGetParents(ids[i])
		if len(parents) != 1 {
			t.Fatalf("Commit %d: expected 1 parent, got %d", i, len(parents))
		}
		if parents[0].ParentId != ids[i-1] || parents[0].ParentOrder != 0 {
			t.Errorf("Commit %d: expected parent %q at order 0, got %q at order %d",
				i, ids[i-1], parents[0].ParentId, parents[0].ParentOrder)
		}
	}

	// History is returned oldest-first and matches the commit order.
	commits := GetCommits()
	if len(commits) != len(ids) {
		t.Fatalf("Expected %d commits, got %d", len(ids), len(commits))
	}
	for i, c := range commits {
		if c.Id != ids[i] {
			t.Errorf("History position %d: expected %q, got %q", i, ids[i], c.Id)
		}
	}
}

// Test_MultiParent verifies that a commit can carry more than one parent
// (a merge commit) with a deterministic order, and that DBAddParent appends
// additional parents without disturbing the first.
func Test_MultiParent(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	defer os.RemoveAll(namespace)

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Two ordinary commits: c1 (root) then c2 (first parent c1).
	f1 := namespace + "a.txt"
	os.Create(f1)
	runStageCommand(f1, false)
	_, c1 := runCommitCommand("first")

	f2 := namespace + "b.txt"
	os.Create(f2)
	runStageCommand(f2, false)
	_, c2 := runCommitCommand("second")

	// Simulate a merge: give c2 a second parent (c1 already occupies order 0).
	DBAddParent(c2, c1, 1)

	parents := DBGetParents(c2)
	if len(parents) != 2 {
		t.Fatalf("Expected 2 parents, got %d", len(parents))
	}
	// Ordered by parent_order: order 0 first, order 1 second.
	if parents[0].ParentOrder != 0 || parents[1].ParentOrder != 1 {
		t.Errorf("Expected orders [0,1], got [%d,%d]", parents[0].ParentOrder, parents[1].ParentOrder)
	}
	if parents[0].ParentId != c1 || parents[1].ParentId != c1 {
		t.Errorf("Expected both parents to be %q, got %q and %q", c1, parents[0].ParentId, parents[1].ParentId)
	}

	// The first parent (order 0) is unchanged by the append.
	if fp := DBGetFirstParent(c2); fp != c1 {
		t.Errorf("Expected first parent %q, got %q", c1, fp)
	}
}
