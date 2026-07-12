package main

import (
	"os"
	"path/filepath"
	"strings"
)

func DisplayStageResults(results []StageResult) {
	if len(results) == 0 {
		Debug("Results length is 0.")
		Info("No files to stage.")
		return
	}

	var added, updated, removed, ignored, alreadyStaged, notModified, failed []string

	for _, r := range results {
		switch r.ReturnCode {
		case 112, 110: // File added to staging (new file or modified committed file)
			added = append(added, r.FilePath)
		case 109: // File deleted from filesystem (committed file)
			removed = append(removed, r.FilePath)
		case 102, 105, 107: // Staged file updated
			updated = append(updated, r.FilePath)
		case 101, 104: // File deleted from filesystem (was staged)
			removed = append(removed, r.FilePath)
		case 103, 106, 108, 113: // File already staged or restored to original state
			alreadyStaged = append(alreadyStaged, r.FilePath)
		case 111: // File not modified
			notModified = append(notModified, r.FilePath)
		case 002: // Ignored by rules
			ignored = append(ignored, r.FilePath)
		default:
			failed = append(failed, r.FilePath)
		}
	}

	if len(added)+len(updated)+len(removed)+len(alreadyStaged)+len(failed) == 0 {
		Info("Nothing to stage.")
		Debug("Nothing to stage.")
		return
	}

	if len(added) > 0 {
		BreakLine()
		Success("󰐙 Added to staging %s", FormatFileCount(len(added)))
		list := GenerateLeveledList(added)
		Tree(list, ".", true)
	}

	if len(updated) > 0 {
		BreakLine()
		Success("󰓦 Updated in staging %s", FormatFileCount(len(updated)))
		list := GenerateLeveledList(updated)
		Tree(list, ".", true)
	}

	if len(removed) > 0 {
		BreakLine()
		Info("󰍷 Removed from filesystem %s", FormatFileCount(len(removed)))
		list := GenerateLeveledList(removed)
		Tree(list, ".", true)
	}

	if len(alreadyStaged) > 0 {
		BreakLine()
		Info(" Already staged %s", FormatFileCount(len(alreadyStaged)))
		list := GenerateLeveledList(alreadyStaged)
		Tree(list, ".", true)
	}

	if len(failed) > 0 {
		BreakLine()
		Fail(" Failed (%s)", FormatFileCount(len(failed)))
		list := GenerateLeveledList(failed)
		Tree(list, ".", true)
	}
}

func StageAndLog(id string, path string, op string) error {
	Debug("Staging and logging file: id=%s, path=%s, op=%s", id, path, op)
	logOperations := map[string]string{
		"added":    "ADD",
		"modified": "MOD",
		"removed":  "REM",
	}
	blobHash, err := WriteBlob(path)
	if err != nil {
		Debug("Failed to write blob")
		return err
	}
	LogOperation(id, logOperations[op], path, blobHash)
	return nil
}

func ExpandFilePaths(args []string) ([]string, error) {
	var filePaths []string

	for _, arg := range args {
		if arg == "." {
			Debug("Expanding current directory recursively")
			err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
				if err != nil {
					Debug("Error walking path %s: %v", path, err)
					MustSucceed(err, "operation failed")
				}

				if info.IsDir() {
					return nil
				}

				if strings.HasPrefix(path, ".nexio") {
					return nil
				}

				filePaths = append(filePaths, path)
				return nil
			})
			if err != nil {
				return nil, err
			}

			stagedFiles := GetStagingLogsContent()
			for _, entry := range stagedFiles {
				if entry.Op == "ADD" || entry.Op == "MOD" {
					if !FileExists(entry.Path) {
						Debug("Found staged file that no longer exists: %s", entry.Path)
						filePaths = append(filePaths, entry.Path)
					}
				}
			}

			_, deletedFiles := GetModifiedOrDeletedFiles()
			for _, deletedFile := range deletedFiles {
				Debug("Found committed file that was deleted: %s", deletedFile)
				filePaths = append(filePaths, deletedFile)
			}
		} else {
			filePaths = append(filePaths, arg)
		}
	}

	Debug("Expanded to %d files", len(filePaths))
	return filePaths, nil
}
