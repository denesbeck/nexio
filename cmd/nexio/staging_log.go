package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
)

type LogFileEntry struct {
	Id       string `json:"id"`
	Op       string `json:"op"`
	Path     string `json:"path"`
	BlobHash string `json:"blobHashField"`
}

var (
	add = color.New(color.FgGreen).SprintFunc()
	mod = color.New(color.FgYellow).SprintFunc()
	rem = color.New(color.FgRed).SprintFunc()
)

func LogOperation(id string, op string, path string, blobHash string) {
	DBLogOperation(id, op, path, blobHash)
}

func LogEntryLookup(op string, path string) (bool, *LogFileEntry) {
	return DBLogEntryLookup(op, path)
}

func IsStagingLogsEmpty() bool {
	Debug("Checking if staging logs are empty.")
	stagingLogs := GetSyncedStagingLogsContent()
	if len(*stagingLogs) == 0 {
		Debug("No staging logs found.")
		return true
	}
	return false
}

func RemoveLogEntry(id string) error {
	return DBRemoveLogEntry(id)
}

func TruncateLogs() {
	DBTruncateLogs()
}

func GetStagingLogsContent() (result *[]LogFileEntry) {
	logs := DBGetStagingLogs()
	return &logs
}

func GetSyncedStagingLogsContent() (result *[]LogFileEntry) {
	logs := DBGetSyncedStagingLogs()
	return &logs
}

func SortByOperationAndPath(content []LogFileEntry) (result *[]LogFileEntry) {
	Debug("Sorting log entries by operation and path")
	sort.Slice(content, func(i, j int) bool {
		if content[i].Op == "ADD" && content[j].Op == "MOD" {
			return true
		}
		if content[i].Op == "ADD" && content[j].Op == "REM" {
			return true
		}
		if content[i].Op == "MOD" && content[j].Op == "REM" {
			return true
		}
		if content[i].Op == content[j].Op {
			if content[i].Path < content[j].Path {
				return true
			}
		}
		return false
	})
	Debug("Log entries sorted successfully")
	return &content
}

func PrintLogs(content []LogFileEntry) {
	Debug("Printing %d log entries", len(content))
	sortedContent := SortByOperationAndPath(content)
	log := []string{}
	for _, logEntry := range *sortedContent {
		switch logEntry.Op {
		case "ADD":
			log = append(log, add(" "+logEntry.Op+":")+" "+logEntry.Path)
		case "MOD":
			log = append(log, mod(" "+logEntry.Op+":")+" "+logEntry.Path)
		case "REM":
			log = append(log, rem(" "+logEntry.Op+":")+" "+logEntry.Path)
		default:
			log = append(log, logEntry.Op+" "+logEntry.Path)
		}
	}
	TreeList(log, false)
	Debug("Log entries printed successfully")
}

func FormatLogs(content []LogFileEntry) string {
	Debug("Formatting %d log entries", len(content))
	if len(content) == 0 {
		return ""
	}

	sortedContent := SortByOperationAndPath(content)
	log := []string{}
	for _, logEntry := range *sortedContent {
		switch logEntry.Op {
		case "ADD":
			log = append(log, add(" "+logEntry.Op+":")+" "+logEntry.Path)
		case "MOD":
			log = append(log, mod(" "+logEntry.Op+":")+" "+logEntry.Path)
		case "REM":
			log = append(log, rem(" "+logEntry.Op+":")+" "+logEntry.Path)
		default:
			log = append(log, logEntry.Op+" "+logEntry.Path)
		}
	}

	// Format as tree structure
	var result strings.Builder
	for i, file := range log {
		if i == len(log)-1 {
			result.WriteString("  └── " + file)
		} else {
			result.WriteString("  ├── " + file + "\n")
		}
	}

	return result.String()
}

func CountOps(content []LogFileEntry) (add int, mod int, rem int) {
	add = 0
	mod = 0
	rem = 0
	for _, entry := range content {
		if entry.Op == "ADD" {
			add++
		}
		if entry.Op == "MOD" {
			mod++
		}
		if entry.Op == "REM" {
			rem++
		}
	}
	return add, mod, rem
}

// Checks if blob hash references in staging logs exist
func ValidateStagingIntegrity() []string {
	Debug("Validating staging integrity")
	logs := GetStagingLogsContent()
	orphanedIds := []string{}

	for _, entry := range *logs {
		// REM operations don't have blobs - they just mark files for removal
		if entry.Op == "REM" {
			continue
		}
		blobExists := BlobExists(entry.BlobHash)
		if !blobExists {
			Debug("Found orphaned log entry: %s (path: %s)", entry.Id, entry.Path)
			orphanedIds = append(orphanedIds, entry.Id)
		}
	}

	Debug("Found %d orphaned entries", len(orphanedIds))
	return orphanedIds
}

func CleanOrphanedStagingEntries() int {
	Debug("Cleaning orphaned staging entries")
	orphanedIds := ValidateStagingIntegrity()

	for _, id := range orphanedIds {
		RemoveLogEntry(id)
		Debug("Removed orphaned log entry: %s", id)
	}

	Debug("Cleaned %d orphaned entries", len(orphanedIds))
	return len(orphanedIds)
}

func GetUntrackedFiles() []string {
	Debug("Getting untracked files")

	var untracked []string
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Skip .nexio directory
			if strings.Contains(path, ".nexio") {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file is ignored by rules
		if ShouldIgnore(path) {
			return nil
		}

		// Check if already staged
		if IsFileStaged(path) {
			return nil
		}

		// Check if already committed
		if isCommitted, _ := GetFileMetadata(path); isCommitted {
			return nil
		}

		untracked = append(untracked, path)
		return nil
	})

	Debug("Found %d untracked files.", len(untracked))

	return untracked
}

func GetModifiedOrDeletedFiles() (modified []string, deleted []string) {
	Debug("Getting modified or deleted files")
	lastCommit := GetLastCommit()
	if lastCommit.Id == "" {
		return nil, nil
	}

	fileList := GetFileListContent(lastCommit.Id)

	for _, file := range *fileList {
		// Skip if staged already
		if IsFileStaged(file.Path) {
			continue
		}

		// Check if file exists in working directory
		if !FileExists(file.Path) {
			deleted = append(deleted, file.Path)
			continue
		}

		// Compare current file hash with committed blob hash
		currentHash, err := HashFile(file.Path)
		if err != nil {
			Debug("Failed to hash file %s: %s", file.Path, err.Error())
			continue
		}

		if currentHash != file.BlobHash {
			modified = append(modified, file.Path)
		}
	}

	return modified, deleted
}
