package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	lockFileName    = "nexio.lock"
	lockStalePeriod = 5 * time.Minute
)

// RemoteLock represents the lock file stored in S3.
type RemoteLock struct {
	Holder    string `json:"holder"`
	Timestamp string `json:"timestamp"`
	Operation string `json:"operation"`
}

// AcquireLock attempts to acquire a remote lock in S3.
// If the lock exists and is not stale, it returns an error.
// If force is true, any existing lock is overwritten.
func AcquireLock(ctx context.Context, client *s3.Client, bucket, prefix, operation string, force bool) error {
	Debug("Acquiring remote lock for %s", operation)

	lockKey := S3Key(prefix, lockFileName)

	// Check if lock exists
	exists, err := ObjectExists(ctx, client, bucket, lockKey)
	if err != nil {
		return fmt.Errorf("failed to check lock status: %w", err)
	}

	if exists && !force {
		// Download and check lock
		data, err := DownloadBytes(ctx, client, bucket, lockKey)
		if err != nil {
			return fmt.Errorf("failed to read lock file: %w", err)
		}

		var lock RemoteLock
		if err := json.Unmarshal(data, &lock); err != nil {
			Debug("Failed to parse lock file, overwriting: %v", err)
			// Corrupted lock file -- overwrite it
		} else {
			// Check if stale
			lockTime, err := time.Parse(time.RFC3339, lock.Timestamp)
			if err == nil && time.Since(lockTime) < lockStalePeriod {
				return fmt.Errorf("remote is locked by %s (%s started at %s). Use --force to override",
					lock.Holder, lock.Operation, lock.Timestamp)
			}
			Debug("Lock is stale (older than %v), overwriting", lockStalePeriod)
		}
	}

	// Create lock
	config := GetConfig()
	holder := config.Name
	if config.Email != "" {
		holder += " <" + config.Email + ">"
	}
	if holder == "" {
		holder = "unknown"
	}

	lock := RemoteLock{
		Holder:    holder,
		Timestamp: GetTimestamp(),
		Operation: operation,
	}

	lockData, err := json.Marshal(lock)
	if err != nil {
		return fmt.Errorf("failed to create lock data: %w", err)
	}

	if err := UploadBytes(ctx, client, bucket, lockKey, lockData); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	Debug("Lock acquired successfully")
	return nil
}

// ReleaseLock releases the remote lock by deleting the lock file from S3.
func ReleaseLock(ctx context.Context, client *s3.Client, bucket, prefix string) {
	Debug("Releasing remote lock")

	lockKey := S3Key(prefix, lockFileName)
	if err := DeleteObject(ctx, client, bucket, lockKey); err != nil {
		Debug("Failed to release lock: %v", err)
		// Don't fail hard on lock release -- best effort
	} else {
		Debug("Lock released successfully")
	}
}
