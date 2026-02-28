package main

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_CleanCommand_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	returnCode, _, _, _ := runCleanCommand()

	if returnCode != 001 {
		t.Errorf("Expected return code 001 (not initialized), got %d", returnCode)
	}

	os.RemoveAll(namespace)
}

func Test_CleanCommand_Initialized(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	returnCode, freedBytes, deletedBlobs, failedBlobs := runCleanCommand()

	if returnCode != 1001 {
		t.Errorf("Expected return code 1001 (success), got %d", returnCode)
	}
	if freedBytes != 0 {
		t.Errorf("Expected 0 freed bytes on empty repo, got %d", freedBytes)
	}
	if deletedBlobs != 0 {
		t.Errorf("Expected 0 deleted blobs on empty repo, got %d", deletedBlobs)
	}
	if failedBlobs != 0 {
		t.Errorf("Expected 0 failed blobs on empty repo, got %d", failedBlobs)
	}

	os.RemoveAll(namespace)
}

func Test_CollectReferencedHashes_EmptyRepository(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	hashes := CollectReferencedHashes()

	if len(hashes) != 0 {
		t.Errorf("Expected 0 referenced hashes in empty repository, got %d", len(hashes))
	}

	os.RemoveAll(namespace)
}

func Test_CollectReferencedHashes_WithCommits(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit a file
	file1 := namespace + "test1.txt"
	os.WriteFile(file1, []byte("content1"), 0644)
	hash1, _ := HashFile(file1)

	runStageCommand(file1, false)
	runCommitCommand("Test commit 1")

	// Create and commit another file
	file2 := namespace + "test2.txt"
	os.WriteFile(file2, []byte("content2"), 0644)
	hash2, _ := HashFile(file2)

	runStageCommand(file2, false)
	runCommitCommand("Test commit 2")

	hashes := CollectReferencedHashes()

	// Should have both blob hashes
	if _, exists := hashes[hash1]; !exists {
		t.Errorf("Expected hash %s to be referenced", hash1)
	}
	if _, exists := hashes[hash2]; !exists {
		t.Errorf("Expected hash %s to be referenced", hash2)
	}

	os.RemoveAll(namespace)
}

func Test_CollectReferencedHashes_WithStagingLogs(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create and stage a file (don't commit)
	file := namespace + "staged.txt"
	os.WriteFile(file, []byte("staged content"), 0644)
	hash, _ := HashFile(file)

	runStageCommand(file, false)

	hashes := CollectReferencedHashes()

	// Should have the staged blob hash
	if _, exists := hashes[hash]; !exists {
		t.Errorf("Expected staged hash %s to be referenced", hash)
	}

	os.RemoveAll(namespace)
}

func Test_CollectReferencedHashes_MixedCommitsAndStaging(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit a file
	committedFile := namespace + "committed.txt"
	os.WriteFile(committedFile, []byte("committed content"), 0644)
	committedHash, _ := HashFile(committedFile)

	runStageCommand(committedFile, false)
	runCommitCommand("Test commit")

	// Create and stage another file (don't commit)
	stagedFile := namespace + "staged.txt"
	os.WriteFile(stagedFile, []byte("staged content"), 0644)
	stagedHash, _ := HashFile(stagedFile)

	runStageCommand(stagedFile, false)

	hashes := CollectReferencedHashes()

	// Should have both hashes
	if _, exists := hashes[committedHash]; !exists {
		t.Errorf("Expected committed hash %s to be referenced", committedHash)
	}
	if _, exists := hashes[stagedHash]; !exists {
		t.Errorf("Expected staged hash %s to be referenced", stagedHash)
	}

	os.RemoveAll(namespace)
}

func Test_CollectReferencedHashes_DeduplicatesHashes(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create two files with identical content (same hash)
	content := []byte("identical content")
	file1 := namespace + "file1.txt"
	file2 := namespace + "file2.txt"
	os.WriteFile(file1, content, 0644)
	os.WriteFile(file2, content, 0644)

	hash1, _ := HashFile(file1)
	hash2, _ := HashFile(file2)

	if hash1 != hash2 {
		t.Fatalf("Expected identical content to produce same hash")
	}

	runStageCommand(file1, false)
	runCommitCommand("Commit file1")

	runStageCommand(file2, false)
	runCommitCommand("Commit file2")

	hashes := CollectReferencedHashes()

	// Should have the hash only once (deduplication)
	if _, exists := hashes[hash1]; !exists {
		t.Errorf("Expected hash %s to be referenced", hash1)
	}

	// Map should only have 1 entry for this hash
	count := 0
	for h := range hashes {
		if h == hash1 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected hash to appear once in map, got %d", count)
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedShards_NoOrphans(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit a file
	file := namespace + "test.txt"
	os.WriteFile(file, []byte("test content"), 0644)
	runStageCommand(file, false)
	runCommitCommand("Test commit")

	hashes := CollectReferencedHashes()
	deleted, failed := CleanOrphanedShards(hashes, false, false)

	if deleted != 0 {
		t.Errorf("Expected 0 deleted shards, got %d", deleted)
	}
	if failed != 0 {
		t.Errorf("Expected 0 failed deletions, got %d", failed)
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedShards_WithOrphans(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit a file
	file := namespace + "test.txt"
	os.WriteFile(file, []byte("test content"), 0644)
	runStageCommand(file, false)
	runCommitCommand("Test commit")

	// Create orphan shard directories manually
	orphanShard1 := filepath.Join(GetDir("objects"), "zz")
	orphanShard2 := filepath.Join(GetDir("objects"), "yy")
	os.MkdirAll(orphanShard1, 0755)
	os.MkdirAll(orphanShard2, 0755)

	hashes := CollectReferencedHashes()
	deleted, failed := CleanOrphanedShards(hashes, false, false)

	if deleted != 2 {
		t.Errorf("Expected 2 deleted shards, got %d", deleted)
	}
	if failed != 0 {
		t.Errorf("Expected 0 failed deletions, got %d", failed)
	}

	// Verify orphan shards are actually gone
	if FileExists(orphanShard1) {
		t.Errorf("Orphan shard %s should have been deleted", orphanShard1)
	}
	if FileExists(orphanShard2) {
		t.Errorf("Orphan shard %s should have been deleted", orphanShard2)
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedShards_DryRun(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit a file
	file := namespace + "test.txt"
	os.WriteFile(file, []byte("test content"), 0644)
	runStageCommand(file, false)
	runCommitCommand("Test commit")

	// Create orphan shard directory
	orphanShard := filepath.Join(GetDir("objects"), "zz")
	os.MkdirAll(orphanShard, 0755)

	hashes := CollectReferencedHashes()
	deleted, failed := CleanOrphanedShards(hashes, true, false) // dryRun=true

	if deleted != 1 {
		t.Errorf("Expected 1 deleted (reported) shard in dry-run, got %d", deleted)
	}
	if failed != 0 {
		t.Errorf("Expected 0 failed deletions, got %d", failed)
	}

	// Verify orphan shard is still there (dry-run should not delete)
	if !FileExists(orphanShard) {
		t.Errorf("Orphan shard %s should NOT have been deleted in dry-run", orphanShard)
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedBlobs_NoOrphans(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit files
	file1 := namespace + "test1.txt"
	file2 := namespace + "test2.txt"
	os.WriteFile(file1, []byte("content1"), 0644)
	os.WriteFile(file2, []byte("content2"), 0644)

	runStageCommand(file1, false)
	runCommitCommand("Commit 1")

	runStageCommand(file2, false)
	runCommitCommand("Commit 2")

	freedBytes, deletedBlobs, failedBlobs := CleanOrphanedBlobs(false, false)

	if freedBytes != 0 {
		t.Errorf("Expected 0 freed bytes, got %d", freedBytes)
	}
	if deletedBlobs != 0 {
		t.Errorf("Expected 0 deleted blobs, got %d", deletedBlobs)
	}
	if failedBlobs != 0 {
		t.Errorf("Expected 0 failed deletions, got %d", failedBlobs)
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedBlobs_WithOrphans(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit a file
	file := namespace + "test.txt"
	os.WriteFile(file, []byte("test content"), 0644)
	committedHash, _ := HashFile(file)
	runStageCommand(file, false)
	runCommitCommand("Test commit")

	// Create orphan blob manually using WriteBlob on a temp file
	// We need to create an orphan in the SAME shard as the committed file
	// to test blob-level cleanup (not shard-level cleanup)
	orphanFile := namespace + "orphan.txt"

	// Keep trying different content until we get a hash with the same prefix
	var orphanHash string
	for i := 0; i < 1000; i++ {
		content := []byte("orphan content " + string(rune('a'+i%26)) + string(rune('0'+i/26)))
		os.WriteFile(orphanFile, content, 0644)
		hash, _ := HashFile(orphanFile)
		if hash[:2] == committedHash[:2] {
			orphanHash, _ = WriteBlob(orphanFile)
			break
		}
	}
	os.Remove(orphanFile)

	if orphanHash == "" {
		t.Skip("Could not create orphan blob in same shard as committed blob")
	}

	// Verify the orphan blob exists
	if !BlobExists(orphanHash) {
		t.Fatalf("Orphan blob should exist before cleaning")
	}

	freedBytes, deletedBlobs, failedBlobs := CleanOrphanedBlobs(false, false)

	if deletedBlobs != 1 {
		t.Errorf("Expected 1 deleted blob, got %d", deletedBlobs)
	}
	if failedBlobs != 0 {
		t.Errorf("Expected 0 failed deletions, got %d", failedBlobs)
	}
	if freedBytes <= 0 {
		t.Errorf("Expected positive freed bytes, got %d", freedBytes)
	}

	// Verify orphan blob is gone
	if BlobExists(orphanHash) {
		t.Errorf("Orphan blob %s should have been deleted", orphanHash)
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedBlobs_DryRun(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit a file
	file := namespace + "test.txt"
	os.WriteFile(file, []byte("test content"), 0644)
	committedHash, _ := HashFile(file)
	runStageCommand(file, false)
	runCommitCommand("Test commit")

	// Create orphan blob in the SAME shard as the committed file
	orphanFile := namespace + "orphan.txt"
	var orphanHash string
	for i := 0; i < 1000; i++ {
		content := []byte("orphan dry run content " + string(rune('a'+i%26)) + string(rune('0'+i/26)))
		os.WriteFile(orphanFile, content, 0644)
		hash, _ := HashFile(orphanFile)
		if hash[:2] == committedHash[:2] {
			orphanHash, _ = WriteBlob(orphanFile)
			break
		}
	}
	os.Remove(orphanFile)

	if orphanHash == "" {
		t.Skip("Could not create orphan blob in same shard as committed blob")
	}

	freedBytes, deletedBlobs, failedBlobs := CleanOrphanedBlobs(true, false) // dryRun=true

	if deletedBlobs != 1 {
		t.Errorf("Expected 1 deleted (reported) blob in dry-run, got %d", deletedBlobs)
	}
	if failedBlobs != 0 {
		t.Errorf("Expected 0 failed deletions, got %d", failedBlobs)
	}
	if freedBytes <= 0 {
		t.Errorf("Expected positive freed bytes calculated, got %d", freedBytes)
	}

	// Verify orphan blob is still there (dry-run should not delete)
	if !BlobExists(orphanHash) {
		t.Errorf("Orphan blob %s should NOT have been deleted in dry-run", orphanHash)
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedBlobs_RemovesEmptyShards(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit a file (this creates a referenced blob in some shard)
	file := namespace + "test.txt"
	os.WriteFile(file, []byte("test content"), 0644)
	committedHash, _ := HashFile(file)
	runStageCommand(file, false)
	runCommitCommand("Test commit")

	// Create orphan blob in a DIFFERENT shard than the committed file
	// This tests the shard-level cleanup when all blobs in a shard are orphaned
	orphanFile := namespace + "orphan.txt"
	var orphanHash string
	for i := 0; i < 1000; i++ {
		content := []byte("unique orphan shard content " + string(rune('a'+i%26)) + string(rune('0'+i/26)))
		os.WriteFile(orphanFile, content, 0644)
		hash, _ := HashFile(orphanFile)
		if hash[:2] != committedHash[:2] { // Different shard
			orphanHash, _ = WriteBlob(orphanFile)
			break
		}
	}
	os.Remove(orphanFile)

	if orphanHash == "" {
		t.Skip("Could not create orphan blob in different shard from committed blob")
	}

	// Get the shard directory for the orphan
	orphanShardDir := filepath.Dir(BlobPath(orphanHash))

	// Verify shard exists
	if !FileExists(orphanShardDir) {
		t.Fatalf("Orphan shard directory should exist before cleaning")
	}

	// The orphan is in its own shard, so CleanOrphanedShards will remove the entire shard
	// This means deletedBlobs will be 0 (shard cleanup doesn't count individual blobs)
	CleanOrphanedBlobs(false, false)

	// Verify orphan blob is gone (either via shard or blob cleanup)
	if BlobExists(orphanHash) {
		t.Errorf("Orphan blob should have been deleted")
	}

	// Verify the orphan shard directory is gone
	if FileExists(orphanShardDir) {
		t.Errorf("Orphan shard directory should have been deleted")
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedBlobs_MultipleOrphans(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit a file
	file := namespace + "test.txt"
	os.WriteFile(file, []byte("test content"), 0644)
	committedHash, _ := HashFile(file)
	runStageCommand(file, false)
	runCommitCommand("Test commit")

	// Create multiple orphan blobs in the SAME shard as the committed file
	orphanHashes := []string{}
	orphanFile := namespace + "orphan.txt"
	contentIndex := 0
	for len(orphanHashes) < 3 && contentIndex < 10000 {
		content := []byte("multi orphan content " + string(rune('a'+contentIndex%26)) + string(rune('0'+(contentIndex/26)%10)) + string(rune('A'+(contentIndex/260)%26)))
		os.WriteFile(orphanFile, content, 0644)
		hash, _ := HashFile(orphanFile)
		if hash[:2] == committedHash[:2] && hash != committedHash {
			h, _ := WriteBlob(orphanFile)
			orphanHashes = append(orphanHashes, h)
		}
		contentIndex++
	}
	os.Remove(orphanFile)

	if len(orphanHashes) < 2 {
		t.Skip("Could not create enough orphan blobs in same shard")
	}

	expectedDeleted := len(orphanHashes)
	_, deletedBlobs, failedBlobs := CleanOrphanedBlobs(false, false)

	if deletedBlobs != expectedDeleted {
		t.Errorf("Expected %d deleted blobs, got %d", expectedDeleted, deletedBlobs)
	}
	if failedBlobs != 0 {
		t.Errorf("Expected 0 failed deletions, got %d", failedBlobs)
	}

	// Verify all orphan blobs are gone
	for _, hash := range orphanHashes {
		if BlobExists(hash) {
			t.Errorf("Orphan blob %s should have been deleted", hash)
		}
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedBlobs_PreservesReferencedBlobs(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	setConfig("name", "test user")
	setConfig("email", "test@test.com")

	// Create and commit multiple files
	files := []string{
		namespace + "file1.txt",
		namespace + "file2.txt",
		namespace + "file3.txt",
	}
	hashes := []string{}

	for i, f := range files {
		content := []byte("content for file " + string(rune('1'+i)))
		os.WriteFile(f, content, 0644)
		hash, _ := HashFile(f)
		hashes = append(hashes, hash)
		runStageCommand(f, false)
		runCommitCommand("Commit " + string(rune('1'+i)))
	}

	// Create an orphan blob in the same shard as the first committed file
	orphanFile := namespace + "orphan.txt"
	var orphanHash string
	for i := 0; i < 1000; i++ {
		content := []byte("preserve test orphan " + string(rune('a'+i%26)) + string(rune('0'+i/26)))
		os.WriteFile(orphanFile, content, 0644)
		hash, _ := HashFile(orphanFile)
		if hash[:2] == hashes[0][:2] && hash != hashes[0] {
			orphanHash, _ = WriteBlob(orphanFile)
			break
		}
	}
	os.Remove(orphanFile)

	CleanOrphanedBlobs(false, false)

	// Verify all referenced blobs still exist
	for i, hash := range hashes {
		if !BlobExists(hash) {
			t.Errorf("Referenced blob %s for file %s should NOT have been deleted", hash, files[i])
		}
	}

	// Verify orphan is gone (if we created one)
	if orphanHash != "" && BlobExists(orphanHash) {
		t.Errorf("Orphan blob should have been deleted")
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedBlobs_PreservesStagedBlobs(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Stage a file but don't commit
	stagedFile := namespace + "staged.txt"
	os.WriteFile(stagedFile, []byte("staged content"), 0644)
	stagedHash, _ := HashFile(stagedFile)
	runStageCommand(stagedFile, false)

	// Create an orphan blob in the SAME shard as the staged file
	orphanFile := namespace + "orphan.txt"
	var orphanHash string
	for i := 0; i < 1000; i++ {
		content := []byte("staged orphan content " + string(rune('a'+i%26)) + string(rune('0'+i/26)))
		os.WriteFile(orphanFile, content, 0644)
		hash, _ := HashFile(orphanFile)
		if hash[:2] == stagedHash[:2] && hash != stagedHash {
			orphanHash, _ = WriteBlob(orphanFile)
			break
		}
	}
	os.Remove(orphanFile)

	if orphanHash == "" {
		t.Skip("Could not create orphan blob in same shard as staged blob")
	}

	freedBytes, deletedBlobs, _ := CleanOrphanedBlobs(false, false)

	if deletedBlobs != 1 {
		t.Errorf("Expected 1 deleted blob, got %d", deletedBlobs)
	}
	if freedBytes <= 0 {
		t.Errorf("Expected positive freed bytes, got %d", freedBytes)
	}

	// Verify staged blob still exists
	if !BlobExists(stagedHash) {
		t.Errorf("Staged blob %s should NOT have been deleted", stagedHash)
	}

	// Verify orphan is gone
	if BlobExists(orphanHash) {
		t.Errorf("Orphan blob should have been deleted")
	}

	os.RemoveAll(namespace)
}

func Test_CleanOrphanedBlobs_EmptyObjectStore(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Clean on empty object store should not error
	freedBytes, deletedBlobs, failedBlobs := CleanOrphanedBlobs(false, false)

	if freedBytes != 0 {
		t.Errorf("Expected 0 freed bytes, got %d", freedBytes)
	}
	if deletedBlobs != 0 {
		t.Errorf("Expected 0 deleted blobs, got %d", deletedBlobs)
	}
	if failedBlobs != 0 {
		t.Errorf("Expected 0 failed deletions, got %d", failedBlobs)
	}

	os.RemoveAll(namespace)
}
