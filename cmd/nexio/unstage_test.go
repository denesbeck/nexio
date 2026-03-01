package main

import (
	"os"
	"testing"
)

func TestUnstage(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"
	os.Create(file)

	runStageCommand(file, false)
	isLogged, logEntry := LogEntryLookup("*", file)
	if !isLogged || logEntry.Op != "ADD" {
		t.Errorf("Expected log entry to be added with ADD operation, got %s", logEntry.Op)
	}

	result := runUnstageCommand(file)
	if result.ReturnCode != 801 {
		t.Errorf("Expected return code 801, got %d", result.ReturnCode)
	}
	isLogged, _ = LogEntryLookup("*", file)
	if isLogged {
		t.Errorf("Expected log entry to be removed")
	}

	os.RemoveAll(namespace)
}

func TestUnstageNotStaged(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "not_staged.txt"
	os.Create(file)

	result := runUnstageCommand(file)
	if result.ReturnCode != 802 {
		t.Errorf("Expected return code 802, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func TestUnstageMultipleFiles(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file1 := namespace + "file1.txt"
	file2 := namespace + "file2.txt"
	file3 := namespace + "file3.txt"
	os.Create(file1)
	os.Create(file2)
	os.Create(file3)

	runStageCommand(file1, false)
	runStageCommand(file2, false)
	runStageCommand(file3, false)

	// Unstage using multiple args
	filePaths := expandUnstageFilePaths([]string{file1, file2, file3})
	if len(filePaths) != 3 {
		t.Errorf("Expected 3 file paths, got %d", len(filePaths))
	}

	for _, fp := range filePaths {
		result := runUnstageCommand(fp)
		if result.ReturnCode != 801 {
			t.Errorf("Expected return code 801 for %s, got %d", fp, result.ReturnCode)
		}
	}

	// Verify all files are unstaged
	for _, fp := range []string{file1, file2, file3} {
		isLogged, _ := LogEntryLookup("*", fp)
		if isLogged {
			t.Errorf("Expected %s to be unstaged", fp)
		}
	}

	os.RemoveAll(namespace)
}

func TestUnstageWildcard(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file1 := namespace + "a.txt"
	file2 := namespace + "b.txt"
	file3 := namespace + "c.txt"
	os.Create(file1)
	os.Create(file2)
	os.Create(file3)

	runStageCommand(file1, false)
	runStageCommand(file2, false)
	runStageCommand(file3, false)

	// Verify all files are staged
	for _, fp := range []string{file1, file2, file3} {
		isLogged, _ := LogEntryLookup("*", fp)
		if !isLogged {
			t.Errorf("Expected %s to be staged", fp)
		}
	}

	// Expand "." wildcard - should return all staged files
	filePaths := expandUnstageFilePaths([]string{"."})
	if len(filePaths) < 3 {
		t.Errorf("Expected at least 3 file paths from wildcard, got %d", len(filePaths))
	}

	// Unstage all
	for _, fp := range filePaths {
		runUnstageCommand(fp)
	}

	// Verify all files are unstaged
	for _, fp := range []string{file1, file2, file3} {
		isLogged, _ := LogEntryLookup("*", fp)
		if isLogged {
			t.Errorf("Expected %s to be unstaged after wildcard unstage", fp)
		}
	}

	os.RemoveAll(namespace)
}

func TestExpandUnstageFilePaths(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	// Test with specific files (not ".")
	result := expandUnstageFilePaths([]string{"file1.txt", "file2.txt"})
	if len(result) != 2 {
		t.Errorf("Expected 2 files, got %d", len(result))
	}
	if result[0] != "file1.txt" || result[1] != "file2.txt" {
		t.Errorf("Expected [file1.txt, file2.txt], got %v", result)
	}

	os.RemoveAll(namespace)
}

func TestExpandUnstageFilePathsWithDot(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	// Stage some files
	file1 := namespace + "x.txt"
	file2 := namespace + "y.txt"
	os.Create(file1)
	os.Create(file2)

	runStageCommand(file1, false)
	runStageCommand(file2, false)

	// "." should return all staged file paths
	result := expandUnstageFilePaths([]string{"."})
	if len(result) < 2 {
		t.Errorf("Expected at least 2 files from wildcard, got %d", len(result))
	}

	// Verify the staged files are in the result
	found := map[string]bool{}
	for _, fp := range result {
		found[fp] = true
	}
	if !found[file1] {
		t.Errorf("Expected %s in expanded paths", file1)
	}
	if !found[file2] {
		t.Errorf("Expected %s in expanded paths", file2)
	}

	os.RemoveAll(namespace)
}
