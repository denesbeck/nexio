package main

import (
	"os"
	"testing"
)

func Test_IsFileStaged(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	file := namespace + "file.txt"
	os.Create(file)
	runAddCommand(file, false)
	if !IsFileStaged(file) {
		t.Errorf("Expected file %s to be staged", file)
	}
	if IsFileStaged(namespace + "nonexistent.txt") {
		t.Errorf("Expected file %s to not be staged", namespace+"nonexistent.txt")
	}
	os.RemoveAll(namespace)
}

func Test_IsFileDeleted(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	file := namespace + "file.txt"
	os.Create(file)
	runAddCommand(file, false)
	runCommitCommand("test commit")

	os.Remove(file)

	isCommitted, _, _ := GetFileMetadata(file)

	if !isCommitted {
		t.Errorf("Expected file %s to be committed", file)
	}

	if !IsFileDeleted(file) {
		t.Errorf("Expected file %s to be deleted", file)
	}

	if IsFileDeleted(namespace + "nonexistent.txt") {
		t.Errorf("Expected file %s to not be deleted", namespace+"nonexistent.txt")
	}
	os.RemoveAll(namespace)
}

func Test_GetFileMetadata(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	file := namespace + "file.txt"
	os.Create(file)
	runAddCommand(file, false)
	runCommitCommand("test commit")

	isCommitted, commitId, fileId := GetFileMetadata(file)

	if !isCommitted {
		t.Errorf("Expected file %s to be committed", file)
	}

	if commitId == "" {
		t.Errorf("Expected commit ID to not be empty")
	}

	if fileId == "" {
		t.Errorf("Expected file ID to not be empty")
	}

	isCommitted, _, _ = GetFileMetadata(namespace + "nonexistent.txt")
	if isCommitted {
		t.Errorf("Expected file %s to not be committed", namespace+"nonexistent.txt")
	}
	os.RemoveAll(namespace)
}

func Test_StatusCommand(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()
	file := namespace + "file.txt"
	os.Create(file)
	runAddCommand(file, false)

	returnCode, stagingLogs := runStatusCommand()

	if returnCode != 502 {
		t.Errorf("Expected return code 502, got %d", returnCode)
	}

	if len(stagingLogs) == 0 {
		t.Errorf("Expected staging logs to not be empty")
	}

	runRemoveCommand(file)

	returnCode, stagingLogs = runStatusCommand()

	if returnCode != 502 {
		t.Errorf("Expected return code 502, got %d", returnCode)
	}

	if len(stagingLogs) != 0 {
		t.Errorf("Expected staging logs to be empty")
	}

	os.RemoveAll(namespace)
}

func Test_StatusCommand_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	// Test when not initialized
	returnCode, _ := runStatusCommand()
	if returnCode != 001 {
		t.Errorf("Expected return code 001 (not initialized), got %d", returnCode)
	}

	os.RemoveAll(namespace)
}

func Test_StatusCommand_WithModifiedFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and commit a file
	file := namespace + "modified_test.txt"
	os.WriteFile(file, []byte("original content"), 0644)
	runAddCommand(file, false)
	runCommitCommand("Initial commit")

	// Modify the file
	os.WriteFile(file, []byte("modified content"), 0644)

	// Run status - should show the file as modified
	returnCode, _ := runStatusCommand()
	if returnCode != 502 {
		t.Errorf("Expected return code 502, got %d", returnCode)
	}

	os.RemoveAll(namespace)
}

func Test_StatusCommand_WithDeletedFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and commit a file
	file := namespace + "deleted_test.txt"
	os.WriteFile(file, []byte("content"), 0644)
	runAddCommand(file, false)
	runCommitCommand("Initial commit")

	// Delete the file
	os.Remove(file)

	// Run status - should show the file as deleted
	returnCode, _ := runStatusCommand()
	if returnCode != 502 {
		t.Errorf("Expected return code 502, got %d", returnCode)
	}

	os.RemoveAll(namespace)
}

func Test_StatusCommand_EmptyRepository(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Run status with no files - should return empty staging
	returnCode, stagingLogs := runStatusCommand()
	if returnCode != 502 {
		t.Errorf("Expected return code 502, got %d", returnCode)
	}
	if len(stagingLogs) != 0 {
		t.Errorf("Expected empty staging logs, got %d entries", len(stagingLogs))
	}

	os.RemoveAll(namespace)
}
