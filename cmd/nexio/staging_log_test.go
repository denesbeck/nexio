package main

import (
	"os"
	"strings"
	"testing"
)

func Test_LogOperation(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	id := GenRandHex(20)
	path := namespace + "test.txt"
	os.WriteFile(path, []byte("test"), 0644)

	// Log an ADD operation
	LogOperation(id, "ADD", path, "testhash")

	// Check if it was logged using LogEntryLookup
	found, logEntry := LogEntryLookup("ADD", path)
	if !found {
		t.Errorf("Expected to find logged operation")
	}
	if logEntry.Id != id {
		t.Errorf("Expected id %s, got %s", id, logEntry.Id)
	}
	if logEntry.Op != "ADD" {
		t.Errorf("Expected operation ADD, got %s", logEntry.Op)
	}

	os.RemoveAll(namespace)
}

func Test_RemoveLogEntry(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	id := GenRandHex(20)
	path := namespace + "test.txt"
	os.WriteFile(path, []byte("test"), 0644)

	// Log an operation
	LogOperation(id, "ADD", path, "testhash")

	// Verify it's there
	found, _ := LogEntryLookup("ADD", path)
	if !found {
		t.Errorf("Expected to find log entry before removal")
	}

	// Remove it
	RemoveLogEntry(id)

	// Verify it's gone
	found, _ = LogEntryLookup("ADD", path)
	if found {
		t.Errorf("Expected log entry to be removed")
	}

	os.RemoveAll(namespace)
}

func Test_LogEntryLookup(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Test when entry doesn't exist
	found, _ := LogEntryLookup("ADD", "nonexistent.txt")
	if found {
		t.Errorf("Expected not to find nonexistent entry")
	}

	// Add an entry and test
	id := GenRandHex(20)
	path := namespace + "test.txt"
	os.WriteFile(path, []byte("test"), 0644)
	LogOperation(id, "MOD", path, "testhash")

	found, logEntry := LogEntryLookup("MOD", path)
	if !found {
		t.Errorf("Expected to find entry")
	}
	if logEntry.Id != id {
		t.Errorf("Expected id %s, got %s", id, logEntry.Id)
	}
	if logEntry.Op != "MOD" {
		t.Errorf("Expected operation MOD, got %s", logEntry.Op)
	}

	os.RemoveAll(namespace)
}

func Test_FormatLogs(t *testing.T) {
	// Test with empty log entries
	emptyLogs := []LogFileEntry{}
	result := FormatLogs(emptyLogs)
	if result != "" {
		t.Errorf("Expected empty string for empty logs, got '%s'", result)
	}

	// Test with single log entry
	singleLog := []LogFileEntry{
		{Id: "test1", Op: "ADD", Path: "file1.txt"},
	}
	result = FormatLogs(singleLog)
	if !strings.Contains(result, "└──") {
		t.Errorf("Expected tree format with └── for single entry, got '%s'", result)
	}
	if !strings.Contains(result, "file1.txt") {
		t.Errorf("Expected result to contain 'file1.txt', got '%s'", result)
	}

	// Test with multiple log entries
	multipleLogs := []LogFileEntry{
		{Id: "test1", Op: "ADD", Path: "file1.txt"},
		{Id: "test2", Op: "MOD", Path: "file2.txt"},
		{Id: "test3", Op: "REM", Path: "file3.txt"},
	}
	result = FormatLogs(multipleLogs)
	if !strings.Contains(result, "├──") {
		t.Errorf("Expected tree format with ├── for multiple entries, got '%s'", result)
	}
	if !strings.Contains(result, "└──") {
		t.Errorf("Expected tree format with └── for last entry, got '%s'", result)
	}
	if !strings.Contains(result, "file1.txt") || !strings.Contains(result, "file2.txt") || !strings.Contains(result, "file3.txt") {
		t.Errorf("Expected result to contain all file names, got '%s'", result)
	}

	// Verify sorting (ADD should come before MOD which should come before REM)
	lines := strings.Split(result, "\n")
	addIndex := -1
	modIndex := -1
	remIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "ADD") {
			addIndex = i
		}
		if strings.Contains(line, "MOD") {
			modIndex = i
		}
		if strings.Contains(line, "REM") {
			remIndex = i
		}
	}
	if addIndex > modIndex {
		t.Errorf("Expected ADD to come before MOD in sorted output")
	}
	if modIndex > remIndex {
		t.Errorf("Expected MOD to come before REM in sorted output")
	}
}

func Test_CountOps(t *testing.T) {
	// Test with empty log entries
	emptyLogs := []LogFileEntry{}
	add, mod, rem := CountOps(emptyLogs)
	if add != 0 || mod != 0 || rem != 0 {
		t.Errorf("Expected all counts to be 0 for empty logs, got add=%d, mod=%d, rem=%d", add, mod, rem)
	}

	// Test with mixed operations
	mixedLogs := []LogFileEntry{
		{Id: "test1", Op: "ADD", Path: "file1.txt"},
		{Id: "test2", Op: "ADD", Path: "file2.txt"},
		{Id: "test3", Op: "MOD", Path: "file3.txt"},
		{Id: "test4", Op: "REM", Path: "file4.txt"},
		{Id: "test5", Op: "REM", Path: "file5.txt"},
		{Id: "test6", Op: "REM", Path: "file6.txt"},
	}
	add, mod, rem = CountOps(mixedLogs)
	if add != 2 {
		t.Errorf("Expected 2 ADD operations, got %d", add)
	}
	if mod != 1 {
		t.Errorf("Expected 1 MOD operation, got %d", mod)
	}
	if rem != 3 {
		t.Errorf("Expected 3 REM operations, got %d", rem)
	}

	// Test with only ADD operations
	addOnlyLogs := []LogFileEntry{
		{Id: "test1", Op: "ADD", Path: "file1.txt"},
		{Id: "test2", Op: "ADD", Path: "file2.txt"},
		{Id: "test3", Op: "ADD", Path: "file3.txt"},
	}
	add, mod, rem = CountOps(addOnlyLogs)
	if add != 3 || mod != 0 || rem != 0 {
		t.Errorf("Expected add=3, mod=0, rem=0, got add=%d, mod=%d, rem=%d", add, mod, rem)
	}
}

func Test_GetSyncedStagingLogsContent(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Test 1: Empty staging logs - should return empty array
	t.Run("EmptyStagingLogs", func(t *testing.T) {
		result := GetSyncedStagingLogsContent()
		if result == nil {
			t.Errorf("Expected non-nil result")
		}
		if len(result) != 0 {
			t.Errorf("Expected empty staging logs, got %d entries", len(result))
		}
	})

	// Test 2: All files exist - should return all entries unchanged
	t.Run("AllFilesExist", func(t *testing.T) {
		// Create and add multiple files
		file1Path := namespace + "test1.txt"
		file2Path := namespace + "test2.txt"
		file3Path := namespace + "test3.txt"

		os.WriteFile(file1Path, []byte("content1"), 0644)
		os.WriteFile(file2Path, []byte("content2"), 0644)
		os.WriteFile(file3Path, []byte("content3"), 0644)

		id1 := GenRandHex(20)
		id2 := GenRandHex(20)
		id3 := GenRandHex(20)

		LogOperation(id1, "ADD", file1Path, "hash1")
		LogOperation(id2, "MOD", file2Path, "hash2")
		LogOperation(id3, "REM", file3Path, "hash3")

		result := GetSyncedStagingLogsContent()

		if len(result) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(result))
		}

		// Verify all entries are still present
		foundIds := make(map[string]bool)
		for _, entry := range result {
			foundIds[entry.Id] = true
		}

		if !foundIds[id1] || !foundIds[id2] || !foundIds[id3] {
			t.Errorf("Expected all entries to be present")
		}

		// Cleanup
		os.Remove(file1Path)
		os.Remove(file2Path)
		os.Remove(file3Path)
		TruncateLogs()
	})

	// Test 3: Some files deleted - should remove entries for deleted files
	t.Run("SomeFilesDeleted", func(t *testing.T) {
		// Create and add files
		existingFile := namespace + "existing.txt"
		deletedFile1 := namespace + "deleted1.txt"
		deletedFile2 := namespace + "deleted2.txt"

		os.WriteFile(existingFile, []byte("exists"), 0644)
		os.WriteFile(deletedFile1, []byte("will be deleted"), 0644)
		os.WriteFile(deletedFile2, []byte("will be deleted too"), 0644)

		existingId := GenRandHex(20)
		deletedId1 := GenRandHex(20)
		deletedId2 := GenRandHex(20)

		// Stage the files
		StageAndLog(existingId, existingFile, "added")
		StageAndLog(deletedId1, deletedFile1, "added")
		StageAndLog(deletedId2, deletedFile2, "modified")

		// Verify all 3 entries exist before deletion
		logsBeforeDelete := GetStagingLogsContent()
		if len(logsBeforeDelete) != 3 {
			t.Errorf("Expected 3 entries before deletion, got %d", len(logsBeforeDelete))
		}

		// Delete files from filesystem
		os.Remove(deletedFile1)
		os.Remove(deletedFile2)

		// Call GetSyncedStagingLogsContent - should clean up deleted files
		result := GetSyncedStagingLogsContent()

		// Should only have 1 entry remaining (existingFile)
		if len(result) != 1 {
			t.Errorf("Expected 1 entry after sync, got %d", len(result))
		}

		if result[0].Id != existingId {
			t.Errorf("Expected remaining entry to have id %s, got %s", existingId, result[0].Id)
		}

		if result[0].Path != existingFile {
			t.Errorf("Expected remaining entry to have path %s, got %s", existingFile, result[0].Path)
		}

		// Cleanup
		os.Remove(existingFile)
		TruncateLogs()
	})

	// Test 4: All files deleted - should return empty logs
	t.Run("AllFilesDeleted", func(t *testing.T) {
		file1 := namespace + "temp1.txt"
		file2 := namespace + "temp2.txt"

		os.WriteFile(file1, []byte("temp1"), 0644)
		os.WriteFile(file2, []byte("temp2"), 0644)

		id1 := GenRandHex(20)
		id2 := GenRandHex(20)

		StageAndLog(id1, file1, "added")
		StageAndLog(id2, file2, "modified")

		// Delete all files
		os.Remove(file1)
		os.Remove(file2)

		result := GetSyncedStagingLogsContent()

		if len(result) != 0 {
			t.Errorf("Expected empty logs after all files deleted, got %d entries", len(result))
		}

		TruncateLogs()
	})

	// Test 5: Mixed operations (ADD, MOD, REM) with some deleted
	t.Run("MixedOperationsWithDeletions", func(t *testing.T) {
		addedExisting := namespace + "added_existing.txt"
		addedDeleted := namespace + "added_deleted.txt"
		modifiedExisting := namespace + "modified_existing.txt"
		modifiedDeleted := namespace + "modified_deleted.txt"
		removedExisting := namespace + "removed_existing.txt"

		os.WriteFile(addedExisting, []byte("ae"), 0644)
		os.WriteFile(addedDeleted, []byte("ad"), 0644)
		os.WriteFile(modifiedExisting, []byte("me"), 0644)
		os.WriteFile(modifiedDeleted, []byte("md"), 0644)
		os.WriteFile(removedExisting, []byte("re"), 0644)

		idAE := GenRandHex(20)
		idAD := GenRandHex(20)
		idME := GenRandHex(20)
		idMD := GenRandHex(20)
		idRE := GenRandHex(20)

		StageAndLog(idAE, addedExisting, "added")
		StageAndLog(idAD, addedDeleted, "added")
		StageAndLog(idME, modifiedExisting, "modified")
		StageAndLog(idMD, modifiedDeleted, "modified")
		StageAndLog(idRE, removedExisting, "removed")

		// Delete some files
		os.Remove(addedDeleted)
		os.Remove(modifiedDeleted)

		result := GetSyncedStagingLogsContent()

		// Should have 3 entries remaining
		if len(result) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(result))
		}

		// Verify correct entries remain
		foundIds := make(map[string]bool)
		for _, entry := range result {
			foundIds[entry.Id] = true
		}

		if !foundIds[idAE] {
			t.Errorf("Expected added_existing entry to remain")
		}
		if !foundIds[idME] {
			t.Errorf("Expected modified_existing entry to remain")
		}
		if !foundIds[idRE] {
			t.Errorf("Expected removed_existing entry to remain")
		}
		if foundIds[idAD] {
			t.Errorf("Expected added_deleted entry to be removed")
		}
		if foundIds[idMD] {
			t.Errorf("Expected modified_deleted entry to be removed")
		}

		// Cleanup
		os.Remove(addedExisting)
		os.Remove(modifiedExisting)
		os.Remove(removedExisting)
		TruncateLogs()
	})

	// Test 6: Verify log entries are removed when file is deleted
	t.Run("VerifyLogEntriesRemovedOnFileDelete", func(t *testing.T) {
		testFile := namespace + "log_removal_test.txt"
		os.WriteFile(testFile, []byte("test"), 0644)

		testId := GenRandHex(20)
		StageAndLog(testId, testFile, "added")

		// Verify log entry exists
		found, _ := LogEntryLookup("ADD", testFile)
		if !found {
			t.Errorf("Expected log entry to exist before sync")
		}

		// Delete the original file
		os.Remove(testFile)

		// Call GetSyncedStagingLogsContent - should remove log entry for deleted file
		result := GetSyncedStagingLogsContent()

		// Verify log entry is removed
		if len(result) != 0 {
			t.Errorf("Expected log to be empty after sync, got %d entries", len(result))
		}

		// Verify we can't find the entry anymore
		found, _ = LogEntryLookup("ADD", testFile)
		if found {
			t.Errorf("Expected log entry to be removed")
		}

		TruncateLogs()
	})

	os.RemoveAll(namespace)
}
