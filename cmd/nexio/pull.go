package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

var pullRemoteFlag string
var pullForceFlag bool

func init() {
	pullCmd.Flags().StringVar(&pullRemoteFlag, "remote", "", "Remote URL (overrides configured remote)")
	pullCmd.Flags().BoolVar(&pullForceFlag, "force", false, "Override remote lock")
	rootCmd.AddCommand(pullCmd)
}

var pullCmd = &cobra.Command{
	Use:     "pull",
	Aliases: []string{"pl"},
	Short:   "Pull commits from remote (S3)",
	Example: "nexio pull\nnexio pull --remote s3://my-bucket/nexio-repo",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		Debug("Starting pull command")
		runPullCommand()
	},
}

func runPullCommand() {
	if !IsInitialized() {
		Fail("%s", COMMON_RETURN_CODES[001])
		return
	}

	// Check for uncommitted staged changes
	stagingLogs := GetStagingLogsContent()
	if len(stagingLogs) > 0 {
		Fail("You have staged but uncommitted changes. Commit or unstage them first.")
		return
	}

	// Resolve remote URL
	remoteURL, err := GetRemoteURL(pullRemoteFlag)
	if err != nil {
		Fail("%s", err.Error())
		return
	}

	bucket, prefix, err := ParseRemoteURL(remoteURL)
	if err != nil {
		Fail("%s", err.Error())
		return
	}

	ctx := context.Background()

	// Create S3 client
	client, err := NewS3Client(ctx)
	if err != nil {
		Fail("Failed to initialize S3 client: %s", err.Error())
		return
	}

	BreakLine()
	Info("Pulling from %s...", remoteURL)

	// Acquire lock
	if err := AcquireLock(ctx, client, bucket, prefix, "pull", pullForceFlag); err != nil {
		Fail("%s", err.Error())
		return
	}
	defer ReleaseLock(ctx, client, bucket, prefix)

	// Download remote index.db to a temp file
	tmpFile, err := os.CreateTemp("", "nexio-remote-*.db")
	if err != nil {
		Fail("Failed to create temp file: %s", err.Error())
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	err = DownloadFile(ctx, client, bucket, S3Key(prefix, "index.db"), tmpPath)
	if err != nil {
		Fail("Remote database not found. Nothing to pull.")
		return
	}

	// Open remote DB to diff commits
	remoteDB, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		Fail("Failed to open remote database: %s", err.Error())
		return
	}

	// Get remote commit IDs
	remoteCommitIds, err := queryCommitIds(remoteDB)
	if err != nil {
		remoteDB.Close()
		Fail("Failed to read remote commits: %s", err.Error())
		return
	}

	// Get local commit IDs
	localCommitIds := collectAllLocalCommitIds()

	// Find new commits (remote but not local)
	var newCommitIds []string
	for id := range remoteCommitIds {
		if _, exists := localCommitIds[id]; !exists {
			newCommitIds = append(newCommitIds, id)
		}
	}

	// Fast-forward check: verify all local commits exist in remote
	for localId := range localCommitIds {
		if _, exists := remoteCommitIds[localId]; !exists {
			remoteDB.Close()
			Fail("Local history has diverged from remote. This is currently unsupported.")
			return
		}
	}

	remoteDB.Close()

	if len(newCommitIds) == 0 {
		BreakLine()
		Success("Already up to date.")
		BreakLine()
		return
	}

	// Capture old HEAD commit before merge (for working directory sync)
	oldHeadCommitId := DBGetHeadCommitForBranch(DBGetCurrentBranchName())

	// Download missing blobs
	blobCount, blobBytes, err := downloadMissingBlobs(ctx, client, bucket, prefix, tmpPath, newCommitIds)
	if err != nil {
		Fail("Failed to download blobs: %s", err.Error())
		return
	}

	// Merge remote database into local
	if err := mergeRemoteDB(tmpPath); err != nil {
		Fail("Failed to merge remote database: %s", err.Error())
		return
	}

	// Sync working directory to match the updated HEAD
	fileCount, err := syncWorkingDirectory(oldHeadCommitId)
	if err != nil {
		Fail("Failed to sync working directory: %s", err.Error())
		return
	}
	if fileCount > 0 {
		Info("Updated %d file(s) in working directory.", fileCount)
	}

	// Clean orphaned blobs locally
	CleanOrphanedBlobs(false, false)

	BreakLine()
	Success("Downloaded %d new commit(s), %d blob(s) (%s).",
		len(newCommitIds), blobCount, formatSize(blobBytes))
	Success("Pull complete.")
	BreakLine()
}

// queryCommitIds queries all commit IDs from a database connection.
func queryCommitIds(database *sql.DB) (map[string]struct{}, error) {
	rows, err := database.Query("SELECT id FROM commits")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids[id] = struct{}{}
	}
	return ids, nil
}

// downloadMissingBlobs downloads blobs referenced by new commits that don't exist locally.
func downloadMissingBlobs(ctx context.Context, client *s3.Client, bucket, prefix, remoteDBPath string, commitIds []string) (int, int64, error) {
	Debug("Downloading missing blobs for %d commits", len(commitIds))

	// Open the remote DB to get file lists for new commits
	remoteDB, err := sql.Open("sqlite", remoteDBPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open remote database: %w", err)
	}
	defer remoteDB.Close()

	// Collect blob hashes from new commits
	blobHashes := make(map[string]struct{})
	for _, commitId := range commitIds {
		rows, err := remoteDB.Query(
			"SELECT blob_hash FROM files WHERE commit_id = ? AND blob_hash != ''",
			commitId,
		)
		if err != nil {
			Debug("Failed to query files for commit %s: %v", commitId, err)
			continue
		}
		for rows.Next() {
			var hash string
			if err := rows.Scan(&hash); err != nil {
				continue
			}
			blobHashes[hash] = struct{}{}
		}
		rows.Close()
	}

	downloaded := 0
	var totalBytes int64

	for hash := range blobHashes {
		// Check if blob already exists locally
		if BlobExists(hash) {
			Debug("Blob %s already exists locally, skipping", hash)
			continue
		}

		// Download the blob
		remoteKey := S3Key(prefix, "objects/"+hash[:2]+"/"+hash[2:])
		localPath := BlobPath(hash)

		// Ensure shard directory exists
		shardDir := filepath.Dir(localPath)
		if err := os.MkdirAll(shardDir, 0755); err != nil {
			return downloaded, totalBytes, fmt.Errorf("failed to create shard directory: %w", err)
		}

		if err := DownloadFile(ctx, client, bucket, remoteKey, localPath); err != nil {
			// Blob might not exist remotely (orphaned reference) -- skip
			Debug("Failed to download blob %s: %v", hash, err)
			continue
		}

		info, err := os.Stat(localPath)
		if err == nil {
			totalBytes += info.Size()
		}

		downloaded++
	}

	Debug("Downloaded %d blobs (%d bytes)", downloaded, totalBytes)
	return downloaded, totalBytes, nil
}

// syncWorkingDirectory updates the working directory to match the current HEAD commit.
// It compares the old HEAD's file list with the new HEAD's file list and:
//   - Restores files that are new or have changed blob hashes
//   - Removes files that are no longer tracked in the new HEAD
func syncWorkingDirectory(oldHeadCommitId string) (int, error) {
	Debug("Syncing working directory after pull")

	// Get new HEAD commit
	newHeadCommitId := DBGetHeadCommitForBranch(DBGetCurrentBranchName())
	if newHeadCommitId == "" {
		Debug("No HEAD commit after merge, nothing to sync")
		return 0, nil
	}

	// Build map of old HEAD files (path -> blobHash)
	oldFiles := make(map[string]string)
	if oldHeadCommitId != "" {
		for _, f := range DBGetFileListForCommit(oldHeadCommitId) {
			oldFiles[f.Path] = f.BlobHash
		}
	}

	// Get new HEAD files
	newFiles := DBGetFileListForCommit(newHeadCommitId)

	count := 0

	// Build set of new file paths for deletion check
	newFilePaths := make(map[string]struct{})
	for _, f := range newFiles {
		newFilePaths[f.Path] = struct{}{}
	}

	// Restore new or changed files
	for _, f := range newFiles {
		if f.BlobHash == "" {
			continue
		}

		oldHash, existed := oldFiles[f.Path]
		if existed && oldHash == f.BlobHash {
			// File unchanged, skip
			continue
		}

		// File is new or has a different blob hash -- restore it
		fileMode := os.FileMode(f.Mode)
		if fileMode == 0 {
			fileMode = 0644
		}
		if err := RestoreBlob(f.BlobHash, f.Path, fileMode); err != nil {
			Debug("Failed to restore %s: %v", f.Path, err)
			continue
		}
		count++
	}

	// Remove files that were in old HEAD but not in new HEAD
	for oldPath := range oldFiles {
		if _, exists := newFilePaths[oldPath]; !exists {
			Debug("Removing file no longer tracked: %s", oldPath)
			os.Remove(oldPath)
			count++
		}
	}

	Debug("Synced %d files in working directory", count)
	return count, nil
}

// mergeRemoteDB merges the remote database into the local database using ATTACH.
func mergeRemoteDB(remoteDBPath string) error {
	Debug("Merging remote database into local")

	// Escape single quotes in path for SQL
	escapedPath := strings.ReplaceAll(remoteDBPath, "'", "''")

	// Attach remote database
	_, err := db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS remote", escapedPath))
	if err != nil {
		return fmt.Errorf("failed to attach remote database: %w", err)
	}
	defer db.Exec("DETACH DATABASE remote")

	// Merge within a transaction
	return WithTransaction(func(tx *sql.Tx) error {
		// Insert new commits
		_, err := tx.Exec(`
			INSERT OR IGNORE INTO commits (id, timestamp, message, author_name, author_email)
			SELECT id, timestamp, message, author_name, author_email FROM remote.commits
		`)
		if err != nil {
			return fmt.Errorf("failed to merge commits: %w", err)
		}

		// Insert commit parents
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO commit_parents (commit_id, parent_id, parent_order)
			SELECT commit_id, parent_id, parent_order FROM remote.commit_parents
		`)
		if err != nil {
			return fmt.Errorf("failed to merge commit parents: %w", err)
		}

		// Insert new files
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO files (id, commit_id, path, blob_hash, mode)
			SELECT id, commit_id, path, blob_hash, mode FROM remote.files
		`)
		if err != nil {
			return fmt.Errorf("failed to merge files: %w", err)
		}

		// Insert new commit logs
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO commit_logs (id, commit_id, op, path, blob_hash)
			SELECT id, commit_id, op, path, blob_hash FROM remote.commit_logs
		`)
		if err != nil {
			return fmt.Errorf("failed to merge commit logs: %w", err)
		}

		// Update branch head commits to match remote
		_, err = tx.Exec(`
			UPDATE branches SET head_commit = (
				SELECT head_commit FROM remote.branches WHERE remote.branches.name = branches.name
			) WHERE name IN (SELECT name FROM remote.branches)
		`)
		if err != nil {
			return fmt.Errorf("failed to update branch heads: %w", err)
		}

		// Insert new branches from remote
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO branches (name, head_commit, is_default, is_current, created_at)
			SELECT name, head_commit, is_default, 0, created_at FROM remote.branches
		`)
		if err != nil {
			return fmt.Errorf("failed to merge new branches: %w", err)
		}

		Debug("Database merge complete")
		return nil
	})
}
