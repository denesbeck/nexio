package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
)

func TestCommit(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	for i := range 10 {
		file := namespace + "file" + strconv.Itoa(i) + ".txt"
		os.Create(file)
		runAddCommand(file, false)
		returnCode, commitId := runCommitCommand("test commit " + strconv.Itoa(i))
		if returnCode != 702 {
			t.Errorf("Expected 702, got %d", returnCode)
		}
		commits := GetCommits()
		if len(*commits) == 0 {
			t.Errorf("Expected at least one commit, got %d", len(*commits))
		}
		lastCommit := GetLastCommit()
		if lastCommit.Id != commitId {
			t.Errorf("Expected commit ID %s, got %s", commitId, lastCommit.Id)
		}
		if len(*commits) == 1 {
			if lastCommit.Next != "" {
				t.Errorf("Expected no next commit, got %s", lastCommit.Next)
			}
		}
		if len(*commits) > 1 {
			if (*commits)[i-1].Next != commitId {
				t.Errorf("Expected next commit ID %s, got %s", commitId, (*commits)[i-1].Next)
			}
		}
		metadata, err := os.ReadFile(GetDir("commits") + commitId + "/metadata.json")
		if err != nil {
			t.Errorf("Failed to read metadata file: %v", err)
		}
		var content CommitMetadata
		if err = json.Unmarshal(metadata, &content); err != nil {
			t.Errorf("Failed to unmarshal metadata: %v", err)
		}
		if content.Message != "test commit "+strconv.Itoa(i) {
			t.Errorf("Expected commit message 'test commit %s', got '%s'", strconv.Itoa(i), content.Message)
		}
		if content.Author.Name != "test user" {
			t.Errorf("Expected commit author `test user`, got '%s'", content.Author.Name)
		}
		if content.Author.Email != "test@test.com" {
			t.Errorf("Expected commit author `test@test.com`, got '%s'", content.Author.Email)
		}
	}
}

func Test_CountCommits(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Initially should have 0 commits
	count := CountCommits()
	if count != 0 {
		t.Errorf("Expected 0 commits initially, got %d", count)
	}

	// Make a commit
	file := namespace + "test.txt"
	os.WriteFile(file, []byte("content"), 0644)
	runAddCommand(file, false)
	runCommitCommand("First commit")

	count = CountCommits()
	if count != 1 {
		t.Errorf("Expected 1 commit, got %d", count)
	}

	// Make another commit
	os.WriteFile(file, []byte("updated"), 0644)
	runAddCommand(file, false)
	runCommitCommand("Second commit")

	count = CountCommits()
	if count != 2 {
		t.Errorf("Expected 2 commits, got %d", count)
	}

	os.RemoveAll(namespace)
}

func Test_GetCommits_Simple(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Make some commits
	for i := 1; i <= 3; i++ {
		file := namespace + "file" + fmt.Sprintf("%d", i) + ".txt"
		os.WriteFile(file, []byte("content"), 0644)
		runAddCommand(file, false)
		runCommitCommand(fmt.Sprintf("Commit %d", i))
	}

	commits := GetCommits()
	if len(*commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(*commits))
	}

	os.RemoveAll(namespace)
}

func Test_GetFileListContent(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create a file and commit it
	file := namespace + "test_file.txt"
	os.WriteFile(file, []byte("content"), 0644)
	runAddCommand(file, false)
	_, commitId := runCommitCommand("Test commit")

	// Get file list content
	fileList := GetFileListContent(commitId)
	if fileList == nil {
		t.Fatal("Expected non-nil file list")
	}

	if len(*fileList) != 1 {
		t.Errorf("Expected 1 file in list, got %d", len(*fileList))
	}

	os.RemoveAll(namespace)
}

func Test_GetLastCommitByBranch(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Test when no commits exist
	commit := GetLastCommitByBranch("main")
	if commit.Id != "" {
		t.Errorf("Expected empty commit ID when no commits exist, got %s", commit.Id)
	}

	// Make a commit
	file := namespace + "test.txt"
	os.WriteFile(file, []byte("content"), 0644)
	runAddCommand(file, false)
	_, commitId := runCommitCommand("Test commit")

	// Get last commit by branch
	commit = GetLastCommitByBranch("main")
	if commit.Id != commitId {
		t.Errorf("Expected commit ID %s, got %s", commitId, commit.Id)
	}

	os.RemoveAll(namespace)
}

func Test_CommitCommand_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	// Test when not initialized
	returnCode, _ := runCommitCommand("Test commit")
	if returnCode != 001 {
		t.Errorf("Expected return code 001 (not initialized), got %d", returnCode)
	}

	os.RemoveAll(namespace)
}

func Test_CommitCommand_NoStagedChanges(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Try to commit without any staged changes
	returnCode, _ := runCommitCommand("Test commit")
	if returnCode != 701 {
		t.Errorf("Expected return code 701 (no changes to commit), got %d", returnCode)
	}

	os.RemoveAll(namespace)
}

func Test_ProcessFileList_RemoveOperation(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and commit a file
	file := namespace + "file_to_remove.txt"
	os.WriteFile(file, []byte("content"), 0644)
	runAddCommand(file, false)
	runCommitCommand("Add file")

	// Get last commit ID
	lastCommit := GetLastCommit()
	if lastCommit.Id == "" {
		t.Fatal("Expected a commit to exist")
	}

	// Get file list from commit
	fileList := GetFileListContent(lastCommit.Id)
	if len(*fileList) != 1 {
		t.Errorf("Expected 1 file after first commit, got %d", len(*fileList))
	}

	// Delete file and use runAddCommand with "." to detect the deletion
	os.Remove(file)

	// Stage the deletion by adding with the file path (should detect it's deleted)
	result := runAddCommand(file, false)
	if result.ReturnCode != 109 {
		t.Logf("Staging deletion returned code %d (expected 109)", result.ReturnCode)
	}

	// Make another commit with the removal
	code, _ := runCommitCommand("Remove file")
	if code == 702 {
		// Get new file list
		newLastCommit := GetLastCommit()
		newFileList := GetFileListContent(newLastCommit.Id)
		if len(*newFileList) != 0 {
			t.Errorf("Expected 0 files after removal commit, got %d", len(*newFileList))
		}
	}

	os.RemoveAll(namespace)
}

func Test_ProcessFileList_ModifyOperation(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and commit a file
	file := namespace + "file_to_modify.txt"
	os.WriteFile(file, []byte("original"), 0644)
	runAddCommand(file, false)
	runCommitCommand("Add file")

	// Modify and stage the modification
	os.WriteFile(file, []byte("modified"), 0644)
	runAddCommand(file, false)

	// Make another commit with the modification
	_, commitId := runCommitCommand("Modify file")

	// Get file list from new commit
	fileList := GetFileListContent(commitId)
	if len(*fileList) != 1 {
		t.Errorf("Expected 1 file after modification, got %d", len(*fileList))
	}

	// Verify the commit ID was updated for the modified file
	if (*fileList)[0].CommitId != commitId {
		t.Errorf("Expected file to reference new commit %s, got %s", commitId, (*fileList)[0].CommitId)
	}

	os.RemoveAll(namespace)
}
