package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cloneCmd)
}

var cloneCmd = &cobra.Command{
	Use:     "clone <remote-url> [local-dir]",
	Aliases: []string{"cln"},
	Short:   "Clone a remote repository from S3",
	Example: "nexio clone s3://my-bucket/nexio-repo\nnexio clone s3://my-bucket/nexio-repo ./local-dir",
	Args:    cobra.RangeArgs(1, 2),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Starting clone command")
		remoteURL := args[0]
		localDir := ""
		if len(args) > 1 {
			localDir = args[1]
		}
		runCloneCommand(remoteURL, localDir)
	},
}

func runCloneCommand(remoteURL, localDir string) {
	bucket, prefix, err := ParseRemoteURL(remoteURL)
	if err != nil {
		Fail("%s", err.Error())
		return
	}

	// Determine local directory
	if localDir == "" {
		// Use last segment of the prefix
		parts := strings.Split(prefix, "/")
		localDir = parts[len(parts)-1]
	}

	// Check if directory already has .nexio
	nexioDir := filepath.Join(localDir, ".nexio")
	if FileExists(nexioDir) {
		Fail("Directory %s already contains a Nexio repository.", localDir)
		return
	}

	ctx := context.Background()

	// Create S3 client
	client, err := NewS3Client(ctx)
	if err != nil {
		Fail("Failed to initialize S3 client: %s", err.Error())
		return
	}

	// Verify remote database exists
	exists, err := ObjectExists(ctx, client, bucket, S3Key(prefix, "index.db"))
	if err != nil {
		Fail("Failed to check remote repository: %s", err.Error())
		return
	}
	if !exists {
		Fail("Remote repository not found at %s", remoteURL)
		return
	}

	BreakLine()
	Info("Cloning from %s into ./%s...", remoteURL, localDir)

	// Create local directory structure
	if err := os.MkdirAll(nexioDir, 0755); err != nil {
		Fail("Failed to create directory: %s", err.Error())
		return
	}
	objectsDir := filepath.Join(nexioDir, "objects")
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		Fail("Failed to create objects directory: %s", err.Error())
		return
	}

	// Download index.db
	dbPath := filepath.Join(nexioDir, "index.db")
	Info("Downloading database...")
	if err := DownloadFile(ctx, client, bucket, S3Key(prefix, "index.db"), dbPath); err != nil {
		Fail("Failed to download database: %s", err.Error())
		os.RemoveAll(nexioDir)
		return
	}

	// Download all blobs
	blobCount, blobBytes, err := downloadAllBlobs(ctx, client, bucket, prefix, objectsDir)
	if err != nil {
		Fail("Failed to download blobs: %s", err.Error())
		os.RemoveAll(nexioDir)
		return
	}
	if blobCount > 0 {
		Info("Downloaded %d blob(s) (%s).", blobCount, formatSize(blobBytes))
	}

	// Create config.json with remote set
	configPath := filepath.Join(nexioDir, "config.json")
	cloneConfig := Config{
		Remote: remoteURL,
	}

	// Try to download remote config to get name/email
	remoteConfigData, err := DownloadBytes(ctx, client, bucket, S3Key(prefix, "config.json"))
	if err == nil {
		var remoteConfig Config
		if json.Unmarshal(remoteConfigData, &remoteConfig) == nil {
			// Keep name/email from remote config but always set remote URL
			cloneConfig.Name = remoteConfig.Name
			cloneConfig.Email = remoteConfig.Email
		}
	}

	configData, _ := json.Marshal(cloneConfig)
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		Fail("Failed to write config: %s", err.Error())
		os.RemoveAll(nexioDir)
		return
	}

	// Restore working directory files
	fileCount, err := restoreWorkingDirectory(dbPath, localDir)
	if err != nil {
		Fail("Failed to restore working directory: %s", err.Error())
		os.RemoveAll(nexioDir)
		return
	}
	if fileCount > 0 {
		Info("Restored %d file(s) to working directory.", fileCount)
	}

	BreakLine()
	Success("Clone complete.")
	BreakLine()
}

// downloadAllBlobs downloads all objects from the remote objects/ prefix.
func downloadAllBlobs(ctx context.Context, client *s3.Client, bucket, prefix, objectsDir string) (int, int64, error) {
	Debug("Downloading all blobs from remote")

	objectsPrefix := S3Key(prefix, "objects")
	keys, err := ListObjects(ctx, client, bucket, objectsPrefix)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list remote blobs: %w", err)
	}

	downloaded := 0
	var totalBytes int64

	for _, key := range keys {
		// Extract relative path from the key: prefix/objects/ab/cdef... -> ab/cdef...
		rel := strings.TrimPrefix(key, objectsPrefix+"/")
		if rel == "" || rel == key {
			continue
		}

		localPath := filepath.Join(objectsDir, rel)

		// Ensure shard directory exists
		shardDir := filepath.Dir(localPath)
		if err := os.MkdirAll(shardDir, 0755); err != nil {
			return downloaded, totalBytes, fmt.Errorf("failed to create shard directory: %w", err)
		}

		if err := DownloadFile(ctx, client, bucket, key, localPath); err != nil {
			Debug("Failed to download blob %s: %v", key, err)
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

// restoreWorkingDirectory checks out the HEAD commit of the current branch.
func restoreWorkingDirectory(dbPath, targetDir string) (int, error) {
	Debug("Restoring working directory from HEAD commit")

	// Open the downloaded database
	cloneDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open database: %w", err)
	}
	defer cloneDB.Close()

	// Enable foreign keys
	cloneDB.Exec("PRAGMA foreign_keys=ON")

	// Get the current branch HEAD commit
	var headCommit sql.NullString
	err = cloneDB.QueryRow(
		"SELECT head_commit FROM branches WHERE is_current = 1",
	).Scan(&headCommit)
	if err != nil || !headCommit.Valid || headCommit.String == "" {
		Debug("No current branch HEAD found, skipping working directory restore")
		return 0, nil
	}

	// Get all files for the HEAD commit
	rows, err := cloneDB.Query(
		"SELECT path, blob_hash, mode FROM files WHERE commit_id = ?",
		headCommit.String,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query files: %w", err)
	}
	defer rows.Close()

	count := 0
	objectsDir := filepath.Join(targetDir, ".nexio", "objects")

	for rows.Next() {
		var path, blobHash string
		var mode uint32
		if err := rows.Scan(&path, &blobHash, &mode); err != nil {
			Debug("Failed to scan file entry: %v", err)
			continue
		}

		if blobHash == "" {
			continue
		}

		// Construct the blob path within the clone's .nexio/objects
		blobPath := filepath.Join(objectsDir, blobHash[:2], blobHash[2:])
		destPath := filepath.Join(targetDir, path)

		// Read and decompress blob
		data, err := readBlobFromPath(blobPath)
		if err != nil {
			Debug("Failed to read blob for %s: %v", path, err)
			continue
		}

		// Ensure parent directory exists
		destDir := filepath.Dir(destPath)
		if destDir != "." {
			if err := os.MkdirAll(destDir, 0755); err != nil {
				Debug("Failed to create directory %s: %v", destDir, err)
				continue
			}
		}

		// Write file with appropriate mode
		fileMode := os.FileMode(mode)
		if fileMode == 0 {
			fileMode = 0644
		}
		if err := os.WriteFile(destPath, data, fileMode); err != nil {
			Debug("Failed to write file %s: %v", destPath, err)
			continue
		}

		count++
	}

	Debug("Restored %d files", count)
	return count, nil
}

// readBlobFromPath reads and decompresses a blob from a specific file path.
// This is needed for clone since the blob store is at a non-standard location.
func readBlobFromPath(blobPath string) ([]byte, error) {
	compressed, err := os.ReadFile(blobPath)
	if err != nil {
		return nil, err
	}

	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}
