package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

var pushRemoteFlag string
var pushForceFlag bool

func init() {
	pushCmd.Flags().StringVar(&pushRemoteFlag, "remote", "", "Remote URL (overrides configured remote)")
	pushCmd.Flags().BoolVar(&pushForceFlag, "force", false, "Override remote lock")
	rootCmd.AddCommand(pushCmd)
}

var pushCmd = &cobra.Command{
	Use:     "push",
	Aliases: []string{"ps"},
	Short:   "Push commits to remote (S3)",
	Example: "nexio push\nnexio push --remote s3://my-bucket/nexio-repo",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		Debug("Starting push command")
		runPushCommand()
	},
}

func runPushCommand() {
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
	remoteURL, err := GetRemoteURL(pushRemoteFlag)
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
	Info("Pushing to %s...", remoteURL)

	// Acquire lock
	if err := AcquireLock(ctx, client, bucket, prefix, "push", pushForceFlag); err != nil {
		Fail("%s", err.Error())
		return
	}
	defer ReleaseLock(ctx, client, bucket, prefix)

	// Collect local commit IDs
	localCommitIds := collectAllLocalCommitIds()
	if len(localCommitIds) == 0 {
		Success("Nothing to push -- no commits found.")
		BreakLine()
		return
	}

	// Download remote index.db to diff commits
	remoteCommitIds, err := getRemoteCommitIds(ctx, client, bucket, prefix)
	if err != nil {
		Debug("No remote database found, treating as fresh remote: %v", err)
		remoteCommitIds = make(map[string]struct{})
	}

	// Find new commits (local but not remote)
	var newCommitIds []string
	for id := range localCommitIds {
		if _, exists := remoteCommitIds[id]; !exists {
			newCommitIds = append(newCommitIds, id)
		}
	}

	// Fast-forward check: verify remote HEAD is ancestor of local HEAD
	if len(remoteCommitIds) > 0 {
		if err := verifyFastForward(remoteCommitIds); err != nil {
			Fail("%s", err.Error())
			return
		}
	}

	// Upload missing blobs
	blobCount, blobBytes, err := uploadMissingBlobs(ctx, client, bucket, prefix, newCommitIds)
	if err != nil {
		Fail("Failed to upload blobs: %s", err.Error())
		return
	}

	// Upload index.db
	dbPath := filepath.Join(GetDir("root"), "index.db")
	if err := UploadFile(ctx, client, bucket, S3Key(prefix, "index.db"), dbPath); err != nil {
		Fail("Failed to upload database: %s", err.Error())
		return
	}

	// Upload config.json
	configPath := GetDir("config")
	if err := UploadFile(ctx, client, bucket, S3Key(prefix, "config.json"), configPath); err != nil {
		Fail("Failed to upload config: %s", err.Error())
		return
	}

	// Clean orphaned blobs locally
	CleanOrphanedBlobs(false, false)

	BreakLine()
	if len(newCommitIds) == 0 {
		Success("Everything up to date.")
	} else {
		Success("Uploaded %d new commit(s), %d blob(s) (%s).",
			len(newCommitIds), blobCount, formatSize(blobBytes))
	}
	Success("Push complete.")
	BreakLine()
}

// collectAllLocalCommitIds returns a set of all commit IDs across all branches.
func collectAllLocalCommitIds() map[string]struct{} {
	ids := make(map[string]struct{})
	branches := DBListBranches()
	for _, branch := range branches {
		for _, cid := range collectCommitIds(branch) {
			ids[cid] = struct{}{}
		}
	}
	return ids
}

// getRemoteCommitIds downloads the remote index.db and extracts all commit IDs.
func getRemoteCommitIds(ctx context.Context, client *s3.Client, bucket, prefix string) (map[string]struct{}, error) {
	Debug("Downloading remote database to extract commit IDs")

	// Download remote index.db to a temp file
	tmpFile, err := os.CreateTemp("", "nexio-remote-*.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	err = DownloadFile(ctx, client, bucket, S3Key(prefix, "index.db"), tmpPath)
	if err != nil {
		return nil, fmt.Errorf("remote database not found: %w", err)
	}

	// Open remote DB
	remoteDB, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote database: %w", err)
	}
	defer remoteDB.Close()

	// Query all commit IDs
	rows, err := remoteDB.Query("SELECT id FROM commits")
	if err != nil {
		return nil, fmt.Errorf("failed to query remote commits: %w", err)
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

	Debug("Found %d remote commits", len(ids))
	return ids, nil
}

// verifyFastForward checks that all remote commit IDs are present locally.
// If any remote commit is missing locally, the remote has diverged.
func verifyFastForward(remoteCommitIds map[string]struct{}) error {
	localIds := collectAllLocalCommitIds()
	for remoteId := range remoteCommitIds {
		if _, exists := localIds[remoteId]; !exists {
			return fmt.Errorf("remote has commits not present locally. Run nexio pull first")
		}
	}
	return nil
}

// uploadMissingBlobs uploads blobs referenced by the given commit IDs that don't exist remotely.
func uploadMissingBlobs(ctx context.Context, client *s3.Client, bucket, prefix string, commitIds []string) (int, int64, error) {
	Debug("Uploading missing blobs for %d commits", len(commitIds))

	// Collect all blob hashes from the new commits
	blobHashes := make(map[string]struct{})
	for _, commitId := range commitIds {
		files := DBGetFileListForCommit(commitId)
		for _, f := range files {
			if f.BlobHash != "" {
				blobHashes[f.BlobHash] = struct{}{}
			}
		}
	}

	uploaded := 0
	var totalBytes int64

	for hash := range blobHashes {
		// Check if blob already exists remotely
		remoteKey := S3Key(prefix, "objects/"+hash[:2]+"/"+hash[2:])
		exists, err := ObjectExists(ctx, client, bucket, remoteKey)
		if err != nil {
			return uploaded, totalBytes, fmt.Errorf("failed to check remote blob %s: %w", hash, err)
		}
		if exists {
			Debug("Blob %s already exists remotely, skipping", hash)
			continue
		}

		// Upload the blob
		localPath := BlobPath(hash)
		info, err := os.Stat(localPath)
		if err != nil {
			Debug("Local blob missing: %s", hash)
			continue
		}

		if err := UploadFile(ctx, client, bucket, remoteKey, localPath); err != nil {
			return uploaded, totalBytes, fmt.Errorf("failed to upload blob %s: %w", hash, err)
		}

		uploaded++
		totalBytes += info.Size()
	}

	Debug("Uploaded %d blobs (%d bytes)", uploaded, totalBytes)
	return uploaded, totalBytes, nil
}
