package main

import (
	"database/sql"
	"os"
)

// DBGetFileMetadata gets file info from the latest commit on the current branch
func DBGetFileMetadata(filePath string) (bool, FileListEntry) {
	Debug("Getting file metadata: %s", filePath)
	lastCommit := DBGetLastCommit()
	if lastCommit.Id == "" {
		Debug("No commits found")
		return false, FileListEntry{}
	}

	var entry FileListEntry
	err := db.QueryRow(
		"SELECT id, commit_id, path, blob_hash, mode FROM files WHERE commit_id = ? AND path = ?",
		lastCommit.Id, filePath,
	).Scan(&entry.Id, &entry.CommitId, &entry.Path, &entry.BlobHash, &entry.Mode)
	if err != nil {
		Debug("File not found in commit: %v", err)
		return false, FileListEntry{}
	}
	Debug("File (%s) found in commit: %s", entry.Id, entry.CommitId)
	return true, entry
}

// DBGetFileListForCommit returns all files tracked in a specific commit
func DBGetFileListForCommit(commitId string) []FileListEntry {
	Debug("Getting file list for commit: %s", commitId)
	rows, err := db.Query(
		"SELECT id, commit_id, path, blob_hash, mode FROM files WHERE commit_id = ?",
		commitId,
	)
	if err != nil {
		Debug("Failed to get file list: %v", err)
		MustSucceed(err, "operation failed")
	}
	defer rows.Close()

	var files []FileListEntry
	for rows.Next() {
		var f FileListEntry
		if err := rows.Scan(&f.Id, &f.CommitId, &f.Path, &f.BlobHash, &f.Mode); err != nil {
			Debug("Failed to scan file entry: %v", err)
			MustSucceed(err, "operation failed")
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		MustSucceed(err, "operation failed")
	}
	if files == nil {
		files = []FileListEntry{}
	}
	Debug("Retrieved %d files from commit", len(files))
	return files
}

// DBProcessFileListTx builds the file list for a new commit by applying staging changes to the previous commit's file list
func DBProcessFileListTx(tx *sql.Tx, latestCommitId string, newCommitId string, stagingLogs []LogFileEntry) error {
	Debug("Processing file list: latest=%s, new=%s", latestCommitId, newCommitId)

	// Get previous file list
	var prevFiles []FileListEntry
	if latestCommitId != "" {
		prevFiles = DBGetFileListForCommit(latestCommitId)
	}

	// Build a map for quick lookup
	fileMap := make(map[string]FileListEntry)
	for _, f := range prevFiles {
		fileMap[f.Path] = f
	}

	// Apply staging changes
	for _, logEntry := range stagingLogs {
		Debug("Processing staging log entry: op=%s, path=%s", logEntry.Op, logEntry.Path)
		switch logEntry.Op {
		case "REM":
			delete(fileMap, logEntry.Path)
			Debug("Removed file from list: %s", logEntry.Path)
		case "ADD":
			info, err := os.Stat(logEntry.Path)
			if err != nil {
				Debug("Failed to get file info: %v", err)
				return err
			}
			mode := uint32(info.Mode().Perm())
			fileMap[logEntry.Path] = FileListEntry{
				Id:       logEntry.Id,
				CommitId: newCommitId,
				Path:     logEntry.Path,
				BlobHash: logEntry.BlobHash,
				Mode:     mode,
			}
			Debug("Added new file to list: %s", logEntry.Path)
		case "MOD":
			if entry, exists := fileMap[logEntry.Path]; exists {
				entry.Id = logEntry.Id
				entry.CommitId = newCommitId
				entry.BlobHash = logEntry.BlobHash
				fileMap[logEntry.Path] = entry
				Debug("Updated file in list: %s", logEntry.Path)
			}
		}
	}

	// Insert all files for the new commit
	// Generate new IDs for each file entry since each commit gets its own file records
	for _, f := range fileMap {
		newId := GenRandHex(20)
		_, err := tx.Exec(
			"INSERT INTO files (id, commit_id, path, blob_hash, mode) VALUES (?, ?, ?, ?, ?)",
			newId, newCommitId, f.Path, f.BlobHash, f.Mode,
		)
		if err != nil {
			Debug("Failed to insert file: %v", err)
			return err
		}
	}
	Debug("File list processed successfully")
	return nil
}

// DBGetSyncedStagingLogs returns staging logs after syncing with filesystem
// Removes entries for non-REM ops whose files no longer exist
func DBGetSyncedStagingLogs() []LogFileEntry {
	Debug("Getting synced staging logs content")
	logs := DBGetStagingLogs()

	diff := false
	for _, entry := range logs {
		if entry.Op == "REM" {
			continue
		}
		if !FileExists(entry.Path) {
			diff = true
			DBRemoveLogEntry(entry.Id)
		}
	}

	if diff {
		Debug("Refetching staged files after cleanup...")
		logs = DBGetStagingLogs()
	}
	return logs
}

// DBCollectReferencedHashes returns all blob hashes referenced by files and staging
func DBCollectReferencedHashes() map[string]struct{} {
	Debug("Collecting referenced hashes")
	hashes := make(map[string]struct{})
	duplicateHashes := 0

	// Get hashes from files table
	rows, err := db.Query("SELECT DISTINCT blob_hash FROM files WHERE blob_hash != ''")
	if err != nil {
		Debug("Failed to query file hashes: %v", err)
		MustSucceed(err, "operation failed")
	}
	for rows.Next() {
		var h string
		rows.Scan(&h)
		hashes[h] = struct{}{}
	}
	rows.Close()

	// Get hashes from staging table
	Debug("Reading staging logs...")
	rows, err = db.Query("SELECT DISTINCT blob_hash FROM staging WHERE blob_hash != '' AND blob_hash IS NOT NULL")
	if err != nil {
		Debug("Failed to query staging hashes: %v", err)
		MustSucceed(err, "operation failed")
	}
	for rows.Next() {
		var h string
		rows.Scan(&h)
		if _, exists := hashes[h]; exists {
			duplicateHashes++
		}
		hashes[h] = struct{}{}
	}
	rows.Close()

	// Also get hashes from commit_logs table
	rows, err = db.Query("SELECT DISTINCT blob_hash FROM commit_logs WHERE blob_hash != '' AND blob_hash IS NOT NULL")
	if err != nil {
		Debug("Failed to query commit log hashes: %v", err)
		MustSucceed(err, "operation failed")
	}
	for rows.Next() {
		var h string
		rows.Scan(&h)
		if _, exists := hashes[h]; exists {
			duplicateHashes++
		}
		hashes[h] = struct{}{}
	}
	rows.Close()

	Debug("Found %d duplicate referenced hashes", duplicateHashes)
	Debug("Found %d unique referenced hashes", len(hashes))

	return hashes
}

// DBGetHeadCommitForBranch returns the head commit ID for a given branch, or empty string
func DBGetHeadCommitForBranch(branch string) string {
	var headCommit sql.NullString
	err := db.QueryRow("SELECT head_commit FROM branches WHERE name = ?", branch).Scan(&headCommit)
	if err != nil || !headCommit.Valid {
		return ""
	}
	return headCommit.String
}
