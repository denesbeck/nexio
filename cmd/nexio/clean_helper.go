package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func CollectReferencedHashes() map[string]struct{} {
	files, err := filepath.Glob(filepath.Join(GetDir("commits"), "*", "fileList.json"))
	if err != nil {
		Debug("Failed to retrieve file lists: %s", err)
		MustSucceed(err, "operation failed")
	}

	duplicateHashes := 0
	hashMap := make(map[string]struct{})

	Debug("Reading file lists...")
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			Debug("Failed to read file list: %s", err)
			MustSucceed(err, "operation failed")
		}

		var fileList []FileListEntry
		if err = json.Unmarshal(content, &fileList); err != nil {
			Debug("Failed to unmarshal file list: %s", err)
			MustSucceed(err, "operation failed")
		}
		for _, entry := range fileList {
			if _, exists := hashMap[entry.BlobHash]; exists {
				duplicateHashes++
				continue
			}

			hashMap[entry.BlobHash] = struct{}{}
		}
	}

	Debug("Reading staging logs...")
	content := GetStagingLogsContent()

	for _, entry := range *content {
		if _, exists := hashMap[entry.BlobHash]; exists {
			duplicateHashes++
			continue
		}

		hashMap[entry.BlobHash] = struct{}{}
	}

	Debug("Found %d duplicate referenced hashes", duplicateHashes)
	Debug("Found %d unique referenced hashes", len(hashMap))

	return hashMap
}

func CleanOrphanedShards(hashes map[string]struct{}, dryRun bool, verbose bool) (int, int) {
	Debug("Collecting shard prefixes...")
	prefixes := make(map[string]struct{})
	failed := 0
	for hash := range hashes {
		if len(hash) < 2 {
			continue
		}
		prefix := hash[:2]
		prefixes[prefix] = struct{}{}
	}

	Debug("Found %d unique shard prefixes", len(prefixes))

	Debug("Checking for orphaned shards...")
	shards, err := os.ReadDir(GetDir("objects"))
	if err != nil {
		Debug("Failed to read object store: %s", err)
		MustSucceed(err, "operation failed")
	}
	deleted := 0
	for _, shard := range shards {
		if shard.IsDir() {
			shardPrefix := shard.Name()
			if _, exists := prefixes[shardPrefix]; !exists {
				if verbose {
					Info("Removing orphaned shard: %s", shard.Name())
				}
				if !dryRun {
					err := os.RemoveAll(filepath.Join(GetDir("objects"), shard.Name()))
					if err != nil {
						Debug("Failed to remove orphaned shard: %s", err)
						failed++
					}
				}
				deleted++
			}
		}
	}

	Debug("Removed %d orphaned shards (dry-run: %v)", deleted, dryRun)

	if failed > 0 {
		Debug("Failed to remove %d orphaned shards", failed)
	}

	return deleted, failed
}

func CleanOrphanedBlobs(dryRun bool, verbose bool) (int64, int, int) {
	hashes := CollectReferencedHashes()
	freedBytes := int64(0)
	deletedBlobs := 0
	failedBlobs := 0
	failedDirs := 0

	Debug("Cleaning orphaned shards...")
	CleanOrphanedShards(hashes, dryRun, verbose)

	shards, err := os.ReadDir(GetDir("objects"))
	if err != nil {
		Debug("Failed to read object store: %s", err)
		MustSucceed(err, "operation failed")
	}

	for _, shard := range shards {
		if shard.IsDir() {
			blobs, err := os.ReadDir(filepath.Join(GetDir("objects"), shard.Name()))
			if err != nil {
				Debug("Failed to read shard: %s", err)
				MustSucceed(err, "operation failed")
			}
			objectCount := len(blobs)
			for _, blob := range blobs {
				if _, exists := hashes[blob.Name()]; !exists {
					if verbose {
						Info("Removing orphaned blob: %s", blob.Name())
					}
					if !dryRun {
						err := os.Remove(filepath.Join(GetDir("objects"), shard.Name(), blob.Name()))
						if err != nil {
							Debug("Failed to remove orphaned blob: %s", err)
							failedBlobs++
						}
					}
					info, err := blob.Info()
					if err != nil {
						Debug("Failed to get blob info: %s", err)
					} else {
						freedBytes += info.Size()
					}
					objectCount--
					deletedBlobs++
					if objectCount == 0 {
						if verbose {
							Info("Removing orphaned shard: %s", shard.Name())
						}
						if !dryRun {
							err := os.Remove(filepath.Join(GetDir("objects"), shard.Name()))
							if err != nil {
								Debug("Failed to remove orphaned shard: %s", err)
								failedDirs++
							}

						}
					}
				}
			}
		}
	}

	if failedBlobs > 0 {
		Debug("Failed to remove %d orphaned blobs", failedBlobs)
	}
	if failedDirs > 0 {
		Debug("Failed to remove %d orphaned shards", failedDirs)
	}
	Debug("Removed %d orphaned blobs (dry-run: %v)", deletedBlobs, dryRun)
	Debug("Freed %d bytes (dry-run: %v)", freedBytes, dryRun)

	return freedBytes, deletedBlobs, failedBlobs
}
