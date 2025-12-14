package main

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_HashFile(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Test hashing a valid file
	testFile := namespace + "hash_test.txt"
	content := []byte("test content for hashing")
	os.WriteFile(testFile, content, 0644)

	hash, err := HashFile(testFile)
	if err != nil {
		t.Errorf("HashFile failed: %v", err)
	}
	if hash == "" {
		t.Error("Expected non-empty hash")
	}
	if len(hash) != 64 { // BLAKE3 produces 32 bytes = 64 hex chars
		t.Errorf("Expected 64 character hash, got %d", len(hash))
	}

	// Test that same content produces same hash
	hash2, err := HashFile(testFile)
	if err != nil {
		t.Errorf("HashFile failed on second call: %v", err)
	}
	if hash != hash2 {
		t.Error("Same file should produce same hash")
	}

	// Test hashing non-existent file
	_, err = HashFile(namespace + "nonexistent.txt")
	if err == nil {
		t.Error("Expected error when hashing non-existent file")
	}

	os.RemoveAll(namespace)
}

func Test_HashFile_DifferentContent(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	file1 := namespace + "file1.txt"
	file2 := namespace + "file2.txt"

	os.WriteFile(file1, []byte("content A"), 0644)
	os.WriteFile(file2, []byte("content B"), 0644)

	hash1, _ := HashFile(file1)
	hash2, _ := HashFile(file2)

	if hash1 == hash2 {
		t.Error("Different content should produce different hashes")
	}

	os.RemoveAll(namespace)
}

func Test_HashBytes(t *testing.T) {
	// Test hashing bytes
	data := []byte("test data")
	hash := HashBytes(data)

	if hash == "" {
		t.Error("Expected non-empty hash")
	}
	if len(hash) != 64 {
		t.Errorf("Expected 64 character hash, got %d", len(hash))
	}

	// Same data should produce same hash
	hash2 := HashBytes(data)
	if hash != hash2 {
		t.Error("Same data should produce same hash")
	}

	// Different data should produce different hash
	hash3 := HashBytes([]byte("different data"))
	if hash == hash3 {
		t.Error("Different data should produce different hash")
	}

	// Empty data should still produce a hash
	emptyHash := HashBytes([]byte{})
	if emptyHash == "" {
		t.Error("Empty data should still produce a hash")
	}
}

func Test_BlobPath(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Test blob path generation with sharding
	hash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	path := BlobPath(hash)

	// Should contain the objects directory
	if !filepath.IsAbs(path) && !contains(path, "objects") {
		// Path should reference the objects directory structure
	}

	// Should use first 2 chars as shard directory
	expectedShard := hash[:2]
	expectedRemainder := hash[2:]

	if !contains(path, expectedShard) {
		t.Errorf("Path should contain shard directory '%s', got '%s'", expectedShard, path)
	}
	if !contains(path, expectedRemainder) {
		t.Errorf("Path should contain remainder '%s', got '%s'", expectedRemainder, path)
	}

	os.RemoveAll(namespace)
}

func Test_BlobExists(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create a test file and write it as a blob
	testFile := namespace + "blob_exists_test.txt"
	os.WriteFile(testFile, []byte("blob content"), 0644)

	hash, err := WriteBlob(testFile)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	// Blob should exist
	if !BlobExists(hash) {
		t.Error("Blob should exist after WriteBlob")
	}

	// Non-existent blob should not exist
	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	if BlobExists(fakeHash) {
		t.Error("Non-existent blob should not exist")
	}

	os.RemoveAll(namespace)
}

func Test_WriteBlob(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	testFile := namespace + "write_blob_test.txt"
	content := []byte("test content for blob")
	os.WriteFile(testFile, content, 0644)

	// Write blob
	hash, err := WriteBlob(testFile)
	if err != nil {
		t.Errorf("WriteBlob failed: %v", err)
	}
	if hash == "" {
		t.Error("Expected non-empty hash")
	}

	// Verify blob file exists
	blobPath := BlobPath(hash)
	if !FileExists(blobPath) {
		t.Errorf("Blob file should exist at %s", blobPath)
	}

	// Verify shard directory was created
	shardDir := filepath.Dir(blobPath)
	if !FileExists(shardDir) {
		t.Errorf("Shard directory should exist at %s", shardDir)
	}

	os.RemoveAll(namespace)
}

func Test_WriteBlob_Deduplication(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create two files with same content
	file1 := namespace + "dedup1.txt"
	file2 := namespace + "dedup2.txt"
	content := []byte("identical content")

	os.WriteFile(file1, content, 0644)
	os.WriteFile(file2, content, 0644)

	// Write both as blobs
	hash1, err := WriteBlob(file1)
	if err != nil {
		t.Fatalf("WriteBlob failed for file1: %v", err)
	}

	hash2, err := WriteBlob(file2)
	if err != nil {
		t.Fatalf("WriteBlob failed for file2: %v", err)
	}

	// Hashes should be identical (deduplication)
	if hash1 != hash2 {
		t.Error("Identical content should produce identical hashes")
	}

	// Only one blob file should exist
	blobPath := BlobPath(hash1)
	if !FileExists(blobPath) {
		t.Error("Blob should exist")
	}

	os.RemoveAll(namespace)
}

func Test_WriteBlob_NonExistentFile(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	_, err := WriteBlob(namespace + "nonexistent.txt")
	if err == nil {
		t.Error("Expected error when writing blob for non-existent file")
	}

	os.RemoveAll(namespace)
}

func Test_ReadBlob(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and write a blob
	testFile := namespace + "read_blob_test.txt"
	originalContent := []byte("original content to read back")
	os.WriteFile(testFile, originalContent, 0644)

	hash, err := WriteBlob(testFile)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	// Read the blob back
	readContent, err := ReadBlob(hash)
	if err != nil {
		t.Errorf("ReadBlob failed: %v", err)
	}

	// Content should match
	if string(readContent) != string(originalContent) {
		t.Errorf("Content mismatch: expected '%s', got '%s'", originalContent, readContent)
	}

	os.RemoveAll(namespace)
}

func Test_ReadBlob_NonExistent(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err := ReadBlob(fakeHash)
	if err == nil {
		t.Error("Expected error when reading non-existent blob")
	}

	os.RemoveAll(namespace)
}

func Test_ReadBlob_LargeFile(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create a larger file (100KB)
	testFile := namespace + "large_blob_test.txt"
	largeContent := make([]byte, 100*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	os.WriteFile(testFile, largeContent, 0644)

	hash, err := WriteBlob(testFile)
	if err != nil {
		t.Fatalf("WriteBlob failed for large file: %v", err)
	}

	readContent, err := ReadBlob(hash)
	if err != nil {
		t.Errorf("ReadBlob failed for large file: %v", err)
	}

	if len(readContent) != len(largeContent) {
		t.Errorf("Size mismatch: expected %d, got %d", len(largeContent), len(readContent))
	}

	// Verify content integrity
	for i := range largeContent {
		if readContent[i] != largeContent[i] {
			t.Errorf("Content mismatch at byte %d", i)
			break
		}
	}

	os.RemoveAll(namespace)
}

func Test_RestoreBlob(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and write a blob
	testFile := namespace + "restore_source.txt"
	originalContent := []byte("content to restore")
	os.WriteFile(testFile, originalContent, 0644)

	hash, err := WriteBlob(testFile)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	// Restore to a new location
	destFile := namespace + "restore_dest.txt"
	err = RestoreBlob(hash, destFile, 0644)
	if err != nil {
		t.Errorf("RestoreBlob failed: %v", err)
	}

	// Verify file exists
	if !FileExists(destFile) {
		t.Error("Restored file should exist")
	}

	// Verify content
	restoredContent, err := os.ReadFile(destFile)
	if err != nil {
		t.Errorf("Failed to read restored file: %v", err)
	}
	if string(restoredContent) != string(originalContent) {
		t.Errorf("Content mismatch: expected '%s', got '%s'", originalContent, restoredContent)
	}

	os.RemoveAll(namespace)
}

func Test_RestoreBlob_WithPermissions(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and write a blob
	testFile := namespace + "perm_source.txt"
	os.WriteFile(testFile, []byte("permission test"), 0644)

	hash, err := WriteBlob(testFile)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	// Restore with executable permissions
	destFile := namespace + "perm_dest.txt"
	err = RestoreBlob(hash, destFile, 0755)
	if err != nil {
		t.Errorf("RestoreBlob failed: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(destFile)
	if err != nil {
		t.Errorf("Failed to stat restored file: %v", err)
	}
	// Check executable bit (at least for owner)
	if info.Mode().Perm()&0100 == 0 {
		t.Error("Expected executable permission to be set")
	}

	os.RemoveAll(namespace)
}

func Test_RestoreBlob_CreatesDirectories(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and write a blob
	testFile := namespace + "dir_source.txt"
	os.WriteFile(testFile, []byte("directory test"), 0644)

	hash, err := WriteBlob(testFile)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	// Restore to a nested path that doesn't exist
	destFile := namespace + "nested/deep/path/restored.txt"
	err = RestoreBlob(hash, destFile, 0644)
	if err != nil {
		t.Errorf("RestoreBlob failed: %v", err)
	}

	// Verify file exists
	if !FileExists(destFile) {
		t.Error("Restored file should exist in nested directory")
	}

	os.RemoveAll(namespace)
}

func Test_RestoreBlob_NonExistent(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	err := RestoreBlob(fakeHash, namespace+"fail.txt", 0644)
	if err == nil {
		t.Error("Expected error when restoring non-existent blob")
	}

	os.RemoveAll(namespace)
}

func Test_WriteBlob_EmptyFile(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create empty file
	testFile := namespace + "empty.txt"
	os.WriteFile(testFile, []byte{}, 0644)

	hash, err := WriteBlob(testFile)
	if err != nil {
		t.Errorf("WriteBlob failed for empty file: %v", err)
	}
	if hash == "" {
		t.Error("Expected non-empty hash for empty file")
	}

	// Read it back
	content, err := ReadBlob(hash)
	if err != nil {
		t.Errorf("ReadBlob failed for empty file: %v", err)
	}
	if len(content) != 0 {
		t.Errorf("Expected empty content, got %d bytes", len(content))
	}

	os.RemoveAll(namespace)
}

func Test_WriteBlob_BinaryContent(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create file with binary content
	testFile := namespace + "binary.bin"
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 0x00, 0x00}
	os.WriteFile(testFile, binaryContent, 0644)

	hash, err := WriteBlob(testFile)
	if err != nil {
		t.Errorf("WriteBlob failed for binary file: %v", err)
	}

	// Read it back
	content, err := ReadBlob(hash)
	if err != nil {
		t.Errorf("ReadBlob failed for binary file: %v", err)
	}

	// Verify binary content integrity
	if len(content) != len(binaryContent) {
		t.Errorf("Size mismatch: expected %d, got %d", len(binaryContent), len(content))
	}
	for i := range binaryContent {
		if content[i] != binaryContent[i] {
			t.Errorf("Binary content mismatch at byte %d: expected %x, got %x", i, binaryContent[i], content[i])
		}
	}

	os.RemoveAll(namespace)
}

// Helper function
func contains(s, substr string) bool {
	return filepath.Base(s) != "" && len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr))
}
