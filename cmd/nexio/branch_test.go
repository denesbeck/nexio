package main

import (
	"os"
	"slices"
	"strconv"
	"testing"
)

func Test_NewBranchCmd(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	runNewCommand("test-branch", "", "")

	branches := ListBranches()
	if !slices.Contains(branches, "test-branch") {
		t.Error("Branch `test-branch` not found")
	}

	os.RemoveAll(namespace)
}

func Test_NewBranchFromCommit(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()
	runNewCommand("test-branch", "", "")

	for i := 1; i <= 100; i++ {
		os.Create(namespace + "file" + strconv.Itoa(i) + ".txt")
		runStageCommand(namespace+"file"+strconv.Itoa(i)+".txt", false)
		runCommitCommand("test commit " + strconv.Itoa(i))
	}

	selectedCommit := GetLastCommit().Id

	for i := 101; i <= 200; i++ {
		os.Create(namespace + "file" + strconv.Itoa(i) + ".txt")
		runStageCommand(namespace+"file"+strconv.Itoa(i)+".txt", false)
		runCommitCommand("test commit " + strconv.Itoa(i))
	}
	countCommitsOriginalBranch := len(GetCommits())

	if countCommitsOriginalBranch != 200 {
		t.Errorf("Expected 200 commits, got %d", countCommitsOriginalBranch)
	}

	t.Log(GetCommits())
	t.Log(selectedCommit)
	runNewCommand("test-branch-1", selectedCommit, "")

	countCommitsNewBranch := len(GetCommits())
	if countCommitsNewBranch != 100 {
		t.Errorf("Expected 100 commits, got %d", countCommitsNewBranch)
	}

	lastCommitNewBranch := GetLastCommit().Id
	if selectedCommit != lastCommitNewBranch {
		t.Errorf("Expected last commit to be %s, got %s", selectedCommit, lastCommitNewBranch)
	}

	os.RemoveAll(namespace)
}

func Test_NewBranchFromBranch(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()
	runNewCommand("test-branch", "", "")
	for i := 1; i < 4; i++ {
		os.Create(namespace + "file" + strconv.Itoa(i) + ".txt")
		runStageCommand(namespace+"file"+strconv.Itoa(i)+".txt", false)
		runCommitCommand("test commit " + strconv.Itoa(i))
	}
	lastCommitOriginalBranch := GetLastCommit().Id

	runNewCommand("test-branch-1", "", "test-branch")
	commits := GetCommits()
	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}
	lastCommitNewBranch := GetLastCommit().Id
	if lastCommitOriginalBranch != lastCommitNewBranch {
		t.Errorf("Expected last commit to be %s, got %s", lastCommitOriginalBranch, lastCommitNewBranch)
	}

	os.RemoveAll(namespace)
}

func Test_SwitchNonExistingBranch(t *testing.T) {
}

func Test_DropBranch(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()
	runNewCommand("test-branch", "", "")
	runNewCommand("test-branch-1", "", "")

	statusCode := runDropCommand("test-branch")

	if statusCode != 210 {
		t.Errorf("Expected 210, got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_DropDefaultBranch(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()
	runNewCommand("test-branch", "", "")
	runNewCommand("test-branch-1", "", "")

	setDefaultBranch("test-branch")

	runSwitchCommand("test-branch-1")

	statusCode := runDropCommand("test-branch")

	if statusCode != 209 {
		t.Errorf("Expected 209, got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_DropCurrentBranch(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()
	runNewCommand("test-branch", "", "")

	statusCode := runDropCommand("test-branch")

	if statusCode != 208 {
		t.Errorf("Expected 208, got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_NewBranchAlreadyExists(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	statusCode := runNewCommand("test-branch", "", "")
	if statusCode != 206 {
		t.Errorf("Expected 206, got %d", statusCode)
	}

	statusCode = runNewCommand("test-branch", "", "")
	if statusCode != 205 {
		t.Errorf("Expected 205, got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_NewBranchInvalidName(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	statusCode := runNewCommand("test-branch", "", "")
	if statusCode != 206 {
		t.Errorf("Expected 206, got %d", statusCode)
	}

	statusCode = runNewCommand("test branch %#^@#&", "", "")
	if statusCode != 201 {
		t.Errorf("Expected 201, got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_SwitchBranchAlreadyOnTarget(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	runNewCommand("test-branch", "", "")

	statusCode := runSwitchCommand("test-branch")

	if statusCode != 211 {
		t.Errorf("Expected 211, got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_SwitchBranchDoesNotExist(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	runNewCommand("test-branch", "", "")

	statusCode := runSwitchCommand("test-branch-1")

	if statusCode != 212 {
		t.Errorf("Expected 212, got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_SwitchBranchCurrent(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	runNewCommand("test-branch", "", "")
	runNewCommand("test-branch-1", "", "")

	statusCode := runSwitchCommand("test-branch")

	if statusCode != 213 {
		t.Errorf("Expected 213, got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_RunBranchCommand(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create some branches
	runNewCommand("test-branch-1", "", "")
	runNewCommand("test-branch-2", "", "")

	// Test runBranchCommand - should list all branches
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runBranchCommand panicked: %v", r)
		}
	}()

	runBranchCommand()

	os.RemoveAll(namespace)
}

func Test_RunBranchCommand_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runBranchCommand panicked: %v", r)
		}
	}()

	runBranchCommand()

	os.RemoveAll(namespace)
}

func Test_RunCurrentCommand(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	runNewCommand("test-branch", "", "")

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runCurrentCommand panicked: %v", r)
		}
	}()

	runCurrentCommand()

	os.RemoveAll(namespace)
}

func Test_RunCurrentCommand_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runCurrentCommand panicked: %v", r)
		}
	}()

	runCurrentCommand()

	os.RemoveAll(namespace)
}

func Test_RunDefaultCommand(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runDefaultCommand panicked: %v", r)
		}
	}()

	runDefaultCommand()

	os.RemoveAll(namespace)
}

func Test_RunDefaultCommand_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runDefaultCommand panicked: %v", r)
		}
	}()

	runDefaultCommand()

	os.RemoveAll(namespace)
}

func Test_RunNewCommand_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	statusCode := runNewCommand("test-branch", "", "")
	if statusCode != 001 {
		t.Errorf("Expected status code 001 (not initialized), got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_RunNewCommand_BothFromCommitAndBranch(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	statusCode := runNewCommand("test-branch", "abc123", "main")
	if statusCode != 202 {
		t.Errorf("Expected status code 202 (can't use both flags), got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_RunNewCommand_FromNonExistentBranch(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	statusCode := runNewCommand("test-branch", "", "nonexistent-branch")
	if statusCode != 203 {
		t.Errorf("Expected status code 203 (branch not found), got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_RunDropCommand_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	statusCode := runDropCommand("test-branch")
	if statusCode != 001 {
		t.Errorf("Expected status code 001 (not initialized), got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_RunDropCommand_BranchNotFound(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	statusCode := runDropCommand("nonexistent-branch")
	if statusCode != 207 {
		t.Errorf("Expected status code 207 (branch not found), got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

func Test_RunSwitchCommand_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	statusCode := runSwitchCommand("test-branch")
	if statusCode != 001 {
		t.Errorf("Expected status code 001 (not initialized), got %d", statusCode)
	}

	os.RemoveAll(namespace)
}

// Test: new, switch, drop, current, default
func Test_Branching(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	runNewCommand("test-branch1", "", "")
	runNewCommand("test-branch2", "", "")
	runNewCommand("test-branch3", "", "")

	runSwitchCommand("test-branch2")

	currentBranchName := GetCurrentBranchName()

	if currentBranchName != "test-branch2" {
		t.Errorf("Expected current branch to be `test-branch2`, got '%s'", currentBranchName)
	}

	runSwitchCommand("test-branch1")

	currentBranchName = GetCurrentBranchName()

	if currentBranchName != "test-branch1" {
		t.Errorf("Expected current branch to be `test-branch1`, got '%s'", currentBranchName)
	}

	runSwitchCommand("test-branch3")

	currentBranchName = GetCurrentBranchName()

	if currentBranchName != "test-branch3" {
		t.Errorf("Expected current branch to be `test-branch3`, got '%s'", currentBranchName)
	}

	defaultBranchName := GetDefaultBranchName()
	if defaultBranchName != "main" {
		t.Errorf("Expected default branch to be `main`, got '%s'", defaultBranchName)
	}

	SetBranch("test-branch2", "default")

	defaultBranchName = GetDefaultBranchName()
	if defaultBranchName != "test-branch2" {
		t.Errorf("Expected default branch to be `test-branch2`, got '%s'", defaultBranchName)
	}

	runDropCommand("test-branch1")

	// Verify branch is deleted from DB
	branches := ListBranches()
	if slices.Contains(branches, "test-branch1") {
		t.Error("branch `test-branch1` not deleted")
	}

	os.RemoveAll(namespace)
}

func Test_indexOf(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected int
	}{
		{"found at start", "hello world", "hello", 0},
		{"found in middle", "hello world", "lo wo", 3},
		{"found at end", "hello world", "world", 6},
		{"not found", "hello world", "xyz", -1},
		{"empty substring", "hello", "", 0},
		{"substring longer than string", "hi", "hello", -1},
		{"exact match", "hello", "hello", 0},
		{"single char found", "hello", "e", 1},
		{"single char not found", "hello", "x", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexOf(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("indexOf(%q, %q) = %d, expected %d", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}
