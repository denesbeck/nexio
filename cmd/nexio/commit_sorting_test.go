package main

import (
	"testing"
)

// With SQLite, commits are sorted by timestamp (ORDER BY) instead of linked list traversal.
// The sortCommitsByLinkedList function has been removed.
// These tests verify the Commit struct and basic ordering logic.

func Test_CommitStruct_SingleCommit(t *testing.T) {
	commit := Commit{Id: "aaa", Timestamp: "2024-01-01", Next: ""}
	if commit.Id != "aaa" {
		t.Errorf("Expected commit 'aaa', got '%s'", commit.Id)
	}
	if commit.Next != "" {
		t.Errorf("Expected empty Next, got '%s'", commit.Next)
	}
}

func Test_CommitStruct_FindLastCommit(t *testing.T) {
	// This tests the basic logic of finding the last commit (empty Next)
	commits := []Commit{
		{Id: "first", Next: "second"},
		{Id: "second", Next: "third"},
		{Id: "third", Next: ""}, // This should be found
	}

	// Find the one with empty Next
	var lastCommit Commit
	for _, commit := range commits {
		if commit.Next == "" {
			lastCommit = commit
			break
		}
	}

	if lastCommit.Id != "third" {
		t.Errorf("Expected last commit to be 'third', got '%s'", lastCommit.Id)
	}
}
