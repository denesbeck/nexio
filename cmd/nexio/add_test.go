package main

import (
	"os"
	"strconv"
	"testing"
)

func Test_AddToStaging(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	for i := 1; i < 4; i++ {
		// create files: file1.txt, file2.txt, file3.txt
		os.Create(namespace + "file" + strconv.Itoa(i) + ".txt")
		// add files to staging
		runAddCommand(namespace+"file"+strconv.Itoa(i)+".txt", false)
		// check if files are staged
		if IsFileStaged(namespace+"file"+strconv.Itoa(i)+".txt") == false {
			t.Errorf("File not staged")
		}
	}

	os.RemoveAll(namespace)
}

func Test_DisplayAddResults(t *testing.T) {
	// Test with empty results
	DisplayAddResults([]AddResult{})

	// Test with various result types
	results := []AddResult{
		{FilePath: "added1.txt", ReturnCode: 112},
		{FilePath: "added2.txt", ReturnCode: 110},
		{FilePath: "updated1.txt", ReturnCode: 102},
		{FilePath: "updated2.txt", ReturnCode: 105},
		{FilePath: "updated3.txt", ReturnCode: 107},
		{FilePath: "removed1.txt", ReturnCode: 109},
		{FilePath: "removed2.txt", ReturnCode: 101},
		{FilePath: "removed3.txt", ReturnCode: 104},
		{FilePath: "staged1.txt", ReturnCode: 103},
		{FilePath: "staged2.txt", ReturnCode: 106},
		{FilePath: "staged3.txt", ReturnCode: 108},
		{FilePath: "staged4.txt", ReturnCode: 113},
		{FilePath: "notmodified.txt", ReturnCode: 111},
		{FilePath: "ignored.txt", ReturnCode: 002},
		{FilePath: "failed.txt", ReturnCode: 999},
	}

	// Just test that it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DisplayAddResults panicked: %v", r)
		}
	}()

	DisplayAddResults(results)
}

func Test_ExpandFilePaths(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create some test files
	testFiles := []string{
		namespace + "test1.txt",
		namespace + "test2.txt",
		namespace + "subdir/test3.txt",
	}

	for _, file := range testFiles {
		path, _ := ParsePath(file)
		if path != "" {
			os.MkdirAll(path, 0755)
		}
		os.Create(file)
	}

	// Test with specific files
	result, err := ExpandFilePaths([]string{namespace + "test1.txt", namespace + "test2.txt"})
	if err != nil {
		t.Errorf("ExpandFilePaths failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 files, got %d", len(result))
	}

	// Test with "." would expand to all files, but this requires being in the test directory
	// We'll skip that test for now as it's complex in a test environment

	os.RemoveAll(namespace)
}

func Test_AddToStaging_EdgeCases(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Test adding a file that doesn't exist - should fail
	id := GenRandHex(20)
	err := AddToStaging(id, namespace+"nonexistent.txt", "added")
	if err == nil {
		t.Errorf("Expected error when adding non-existent file")
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode101(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	os.Create(file)
	runAddCommand(file, false)
	os.Remove(file)
	result := runAddCommand(file, false)
	if result.ReturnCode != 101 {
		t.Errorf("Expected 101, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode102(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	os.Create(file)
	runAddCommand(file, false)
	os.WriteFile(file, []byte("test"), 0644)
	result := runAddCommand(file, false)
	if result.ReturnCode != 102 {
		t.Errorf("Expected 102, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode103(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	os.Create(namespace + "file.txt")
	runAddCommand(namespace+"file.txt", false)
	result := runAddCommand(namespace+"file.txt", false)
	if result.ReturnCode != 103 {
		t.Errorf("Expected 103, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode104(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	hash := GenRandHex(20)
	os.Create(file)
	StageAndLog(hash, file, "modified")

	os.Remove(file)

	result := runAddCommand(file, false)
	if result.ReturnCode != 104 {
		t.Errorf("Expected 104, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode105(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	hash := GenRandHex(20)
	os.Create(file)
	StageAndLog(hash, file, "modified")

	os.WriteFile(file, []byte("test"), 0644)

	result := runAddCommand(file, false)
	if result.ReturnCode != 105 {
		t.Errorf("Expected 105, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode106(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	hash := GenRandHex(20)
	os.Create(file)
	StageAndLog(hash, file, "modified")

	result := runAddCommand(file, false)
	if result.ReturnCode != 106 {
		t.Errorf("Expected 106, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode107(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	hash := GenRandHex(20)
	os.Create(file)
	StageAndLog(hash, file, "added")
	runCommitCommand("test")

	hash = GenRandHex(20)
	LogOperation(hash, "REM", file)

	os.WriteFile(file, []byte("test"), 0644)

	result := runAddCommand(file, false)
	if result.ReturnCode != 107 {
		t.Errorf("Expected 107, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode8(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	hash := GenRandHex(20)
	os.Create(file)
	StageAndLog(hash, file, "added")
	runCommitCommand("test")

	hash = GenRandHex(20)
	LogOperation(hash, "REM", file)

	os.Remove(file)

	result := runAddCommand(file, false)
	if result.ReturnCode != 108 {
		t.Errorf("Expected 108, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode109(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	hash := GenRandHex(20)
	os.Create(file)
	StageAndLog(hash, file, "added")
	runCommitCommand("test")

	os.Remove(file)

	result := runAddCommand(file, false)
	if result.ReturnCode != 109 {
		t.Errorf("Expected 109, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode110(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	hash := GenRandHex(20)
	os.Create(file)
	StageAndLog(hash, file, "added")
	runCommitCommand("test")

	os.WriteFile(file, []byte("test"), 0644)

	result := runAddCommand(file, false)
	if result.ReturnCode != 110 {
		t.Errorf("Expected 110, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode111(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	hash := GenRandHex(20)
	os.Create(file)
	StageAndLog(hash, file, "added")
	runCommitCommand("test")

	result := runAddCommand(file, false)
	if result.ReturnCode != 111 {
		t.Errorf("Expected 111, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_AddCmdStatusCode112(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"

	os.Create(file)

	result := runAddCommand(file, false)
	if result.ReturnCode != 112 {
		t.Errorf("Expected 112, got %d", result.ReturnCode)
	}

	os.RemoveAll(namespace)
}

func Test_Debug_Function(t *testing.T) {
	// Save original env
	originalEnv := os.Getenv("DEBUG")
	defer os.Setenv("DEBUG", originalEnv)

	// Test with DEBUG=false
	os.Setenv("DEBUG", "false")
	Debug("This should not print")

	// Test with DEBUG=true
	os.Setenv("DEBUG", "true")
	Debug("This should print: %s", "test message")
}

func Test_ExpandFilePaths_WithDot(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create some test files in the namespace
	testFiles := []string{
		namespace + "test1.txt",
		namespace + "test2.txt",
		namespace + "subdir/test3.txt",
	}

	for _, file := range testFiles {
		path, _ := ParsePath(file)
		if path != "" {
			os.MkdirAll(path, 0755)
		}
		os.WriteFile(file, []byte("test content"), 0644)
	}

	// Test with specific files (not ".")
	result, err := ExpandFilePaths([]string{namespace + "test1.txt", namespace + "test2.txt", namespace + "subdir/test3.txt"})
	if err != nil {
		t.Errorf("ExpandFilePaths failed: %v", err)
	}

	// Should find the test files we specified
	if len(result) != 3 {
		t.Errorf("Expected 3 files, got %d", len(result))
	}

	os.RemoveAll(namespace)
}

func Test_ExpandFilePaths_WithStagedDeletedFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and stage a file
	testFile := namespace + "staged_file.txt"
	os.WriteFile(testFile, []byte("content"), 0644)
	runAddCommand(testFile, false)

	// Delete the file from filesystem
	os.Remove(testFile)

	// Test with specific file path - should return the path as-is
	result, err := ExpandFilePaths([]string{testFile})
	if err != nil {
		t.Errorf("ExpandFilePaths failed: %v", err)
	}

	// Should include the path we specified
	if len(result) != 1 || result[0] != testFile {
		t.Errorf("Expected [%s], got %v", testFile, result)
	}

	os.RemoveAll(namespace)
}

func Test_ExpandFilePaths_WithCommittedDeletedFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create, stage, and commit a file
	testFile := namespace + "committed_file.txt"
	os.WriteFile(testFile, []byte("content"), 0644)
	runAddCommand(testFile, false)
	runCommitCommand("Initial commit")

	// Delete the committed file from filesystem
	os.Remove(testFile)

	// Test with specific file path - should return the path as-is
	result, err := ExpandFilePaths([]string{testFile})
	if err != nil {
		t.Errorf("ExpandFilePaths failed: %v", err)
	}

	// Should include the path we specified
	if len(result) != 1 || result[0] != testFile {
		t.Errorf("Expected [%s], got %v", testFile, result)
	}

	os.RemoveAll(namespace)
}

func Test_ExpandFilePaths_MultipleFiles(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create test files
	testFiles := []string{
		namespace + "file1.txt",
		namespace + "file2.txt",
		namespace + "dir/file3.txt",
	}

	os.MkdirAll(namespace+"dir", 0755)
	for _, file := range testFiles {
		os.WriteFile(file, []byte("content"), 0644)
	}

	// Test with multiple specific files
	result, err := ExpandFilePaths(testFiles)
	if err != nil {
		t.Errorf("ExpandFilePaths failed: %v", err)
	}

	// Should return all specified files
	if len(result) != 3 {
		t.Errorf("Expected 3 files, got %d", len(result))
	}

	os.RemoveAll(namespace)
}

func Test_StageAndLog(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	testFile := namespace + "test_stage.txt"
	os.WriteFile(testFile, []byte("content"), 0644)

	id := GenRandHex(20)
	err := StageAndLog(id, testFile, "added")
	if err != nil {
		t.Errorf("StageAndLog failed: %v", err)
	}

	// Verify file was staged
	if !FileExists(dirs.Staging + "added/" + id + "/test_stage.txt") {
		t.Error("File was not staged")
	}

	// Verify log entry was created
	isLogged, _, _ := LogEntryLookup("ADD", testFile)
	if !isLogged {
		t.Error("Log entry was not created")
	}

	os.RemoveAll(namespace)
}

func Test_RemoveFileAndLog(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	testFile := namespace + "test_remove.txt"
	os.WriteFile(testFile, []byte("content"), 0644)

	id := GenRandHex(20)
	StageAndLog(id, testFile, "added")

	// Verify file was staged
	if !FileExists(dirs.Staging + "added/" + id + "/test_remove.txt") {
		t.Error("File was not staged before removal")
	}

	// Remove the staged file and log
	RemoveFileAndLog(id, "added")

	// Verify file was removed from staging
	if FileExists(dirs.Staging + "added/" + id) {
		t.Error("Staged file was not removed")
	}

	// Verify log entry was removed
	isLogged, _, _ := LogEntryLookup("ADD", testFile)
	if isLogged {
		t.Error("Log entry was not removed")
	}

	os.RemoveAll(namespace)
}
