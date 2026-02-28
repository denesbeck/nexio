package main

import (
	"os"
	"testing"
)

func Test_SwitchBranch_BlocksWithStagedFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create two branches
	runNewCommand("branch-a", "", "")
	runNewCommand("branch-b", "", "")

	// Switch to branch-a
	runSwitchCommand("branch-a")

	// Create and stage a file
	file := namespace + "file.txt"
	os.Create(file)
	runStageCommand(file, false)

	// Try to switch to branch-b with staged changes
	statusCode := runSwitchCommand("branch-b")

	if statusCode != 214 {
		t.Errorf("Expected status code 214 (uncommitted changes), got %d", statusCode)
	}

	// Verify we're still on branch-a
	currentBranch := GetCurrentBranchName()
	if currentBranch != "branch-a" {
		t.Errorf("Expected to still be on branch-a, got %s", currentBranch)
	}

	// File should still exist
	if !FileExists(file) {
		t.Error("File should still exist after blocked branch switch")
	}

	os.RemoveAll(namespace)
}

func Test_SwitchBranch_BlocksWithModifiedFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create file and commit it
	file := namespace + "file.txt"
	os.WriteFile(file, []byte("original content"), 0644)
	runStageCommand(file, false)
	runCommitCommand("Initial commit")

	// Create new branch (this switches to it)
	runNewCommand("branch-b", "", "")

	// Switch back to main
	runSwitchCommand("main")

	// Modify the file (without staging)
	os.WriteFile(file, []byte("modified content"), 0644)

	// Try to switch branches with modified file
	statusCode := runSwitchCommand("branch-b")

	if statusCode != 214 {
		t.Errorf("Expected status code 214 (uncommitted changes), got %d", statusCode)
	}

	// Verify we're still on main branch
	currentBranch := GetCurrentBranchName()
	if currentBranch != "main" {
		t.Errorf("Expected to still be on main, got %s", currentBranch)
	}

	os.RemoveAll(namespace)
}

func Test_SwitchBranch_BlocksWithDeletedFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create file and commit it
	file := namespace + "file.txt"
	os.WriteFile(file, []byte("content"), 0644)
	runStageCommand(file, false)
	runCommitCommand("Initial commit")

	// Create new branch (this switches to it)
	runNewCommand("branch-b", "", "")

	// Switch back to main
	runSwitchCommand("main")

	// Delete the file (without staging)
	os.Remove(file)

	// Try to switch branches with deleted file
	statusCode := runSwitchCommand("branch-b")

	if statusCode != 214 {
		t.Errorf("Expected status code 214 (uncommitted changes), got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_SwitchBranch_AllowsWithNoChanges(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create file and commit it
	file := namespace + "file.txt"
	os.WriteFile(file, []byte("content"), 0644)
	runStageCommand(file, false)
	runCommitCommand("Initial commit")

	// Create new branch from current commit (this switches to it)
	runNewCommand("branch-b", "", "")

	// We're already on branch-b, so switch back to main first
	runSwitchCommand("main")

	// Now switch to branch-b should succeed with no uncommitted changes
	statusCode := runSwitchCommand("branch-b")

	if statusCode != 213 {
		t.Errorf("Expected status code 213 (success), got %d", statusCode)
	}

	// Verify we switched successfully
	currentBranch := GetCurrentBranchName()
	if currentBranch != "branch-b" {
		t.Errorf("Expected to be on branch-b, got %s", currentBranch)
	}

	os.RemoveAll(namespace)
}

func Test_SwitchBranch_AllowsAfterCommitting(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create file and commit it
	file := namespace + "file.txt"
	os.WriteFile(file, []byte("content"), 0644)
	runStageCommand(file, false)
	runCommitCommand("Initial commit")

	// Create new branch (this switches to it)
	runNewCommand("branch-b", "", "")

	// Switch back to main
	runSwitchCommand("main")

	// Modify and stage file
	os.WriteFile(file, []byte("modified"), 0644)
	runStageCommand(file, false)

	// First attempt should fail
	statusCode := runSwitchCommand("branch-b")
	if statusCode != 214 {
		t.Errorf("Expected status code 214 before commit, got %d", statusCode)
	}

	// Commit the changes
	runCommitCommand("Modify file")

	// Now switch should succeed
	statusCode = runSwitchCommand("branch-b")
	if statusCode != 213 {
		t.Errorf("Expected status code 213 after commit, got %d", statusCode)
	}

	// Verify we switched successfully
	currentBranch := GetCurrentBranchName()
	if currentBranch != "branch-b" {
		t.Errorf("Expected to be on branch-b, got %s", currentBranch)
	}

	os.RemoveAll(namespace)
}

func Test_HasUncommittedChanges_DetectsStagedFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Initially no uncommitted changes
	if HasUncommittedChanges() {
		t.Error("Should have no uncommitted changes initially")
	}

	// Stage a file
	file := namespace + "file.txt"
	os.Create(file)
	runStageCommand(file, false)

	// Should detect staged file
	if !HasUncommittedChanges() {
		t.Error("Should detect staged file as uncommitted change")
	}

	os.RemoveAll(namespace)
}

func Test_HasUncommittedChanges_DetectsModifiedFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and commit file
	file := namespace + "file.txt"
	os.WriteFile(file, []byte("original"), 0644)
	runStageCommand(file, false)
	runCommitCommand("Initial commit")

	// No uncommitted changes after commit
	if HasUncommittedChanges() {
		t.Error("Should have no uncommitted changes after commit")
	}

	// Modify file
	os.WriteFile(file, []byte("modified"), 0644)

	// Should detect modification
	if !HasUncommittedChanges() {
		t.Error("Should detect modified file as uncommitted change")
	}

	os.RemoveAll(namespace)
}
