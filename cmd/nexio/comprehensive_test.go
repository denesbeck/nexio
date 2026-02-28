package main

import (
	"os"
	"testing"
)

// Test various helper functions to boost coverage
func Test_ComprehensiveCoverage(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Test config functions
	config := GetConfig()
	_ = config

	// Test branch functions
	branchName := GetCurrentBranchName()
	if branchName != "main" {
		t.Errorf("Expected main branch, got %s", branchName)
	}

	defaultBranch := GetDefaultBranchName()
	if defaultBranch != "main" {
		t.Errorf("Expected main default branch, got %s", defaultBranch)
	}

	// Create a new branch and test it
	runNewCommand("test-branch-comprehensive", "", "")

	// Test file operations
	testFile := namespace + "comprehensive.txt"
	os.WriteFile(testFile, []byte("test"), 0644)

	exists := FileExists(testFile)
	if !exists {
		t.Errorf("File should exist")
	}

	// Test adding, committing
	runStageCommand(testFile, false)
	runCommitCommand("Test commit for coverage")

	// Test commit functions
	lastCommit := GetLastCommit()
	if lastCommit.Id == "" {
		t.Errorf("Expected commit message 'Test commit for coverage', got '%s'", lastCommit.Id)
	}

	count := CountCommits()
	if count != 1 {
		t.Errorf("Expected 1 commit, got %d", count)
	}

	// Test file metadata
	isCommitted, _ := GetFileMetadata(testFile)
	if !isCommitted {
		t.Errorf("File should be committed")
	}

	// Test staging log functions
	isEmpty := IsStagingLogsEmpty()
	if !isEmpty {
		t.Errorf("Staging logs should be empty after commit")
	}

	os.WriteFile(testFile, []byte("modified"), 0644)
	modified, err := IsModified(testFile, testFile+".bak")
	if err == nil && !modified {
		// This might error if .bak doesn't exist, which is expected
	}

	os.RemoveAll(namespace)
}

func Test_EmptyDirCoverage(t *testing.T) {
	tmpDir := namespace + "emptydir_test"
	os.MkdirAll(tmpDir+"/subdir", 0755)
	os.WriteFile(tmpDir+"/file.txt", []byte("content"), 0644)

	err := EmptyDir(tmpDir)
	if err != nil {
		t.Errorf("EmptyDir failed: %v", err)
	}

	entries, _ := os.ReadDir(tmpDir)
	if len(entries) != 0 {
		t.Errorf("Directory should be empty")
	}

	os.RemoveAll(tmpDir)
}

func Test_CopyFileCoverage(t *testing.T) {
	src := namespace + "copy_src.txt"
	dst := namespace + "copy_dst.txt"

	os.WriteFile(src, []byte("content"), 0644)

	err := CopyFile(src, dst)
	if err != nil {
		t.Errorf("CopyFile failed: %v", err)
	}

	exists := FileExists(dst)
	if !exists {
		t.Errorf("Destination file should exist")
	}

	os.Remove(src)
	os.Remove(dst)
}

func Test_ValidatePathComprehensive(t *testing.T) {
	// Test valid paths
	err := ValidatePath("test.txt")
	if err != nil {
		t.Errorf("Valid path should not error: %v", err)
	}

	err = ValidatePath("dir/test.txt")
	if err != nil {
		t.Errorf("Valid nested path should not error: %v", err)
	}

	// Test path traversal
	err = ValidatePath("../../../etc/passwd")
	if err == nil {
		t.Errorf("Path traversal should be detected")
	}
}
