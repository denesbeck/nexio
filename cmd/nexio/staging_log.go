package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/fatih/color"
)

type LogFileEntry struct {
	Id   string `json:"id"`
	Op   string `json:"op"`
	Path string `json:"path"`
}

var (
	add = color.New(color.FgGreen).SprintFunc()
	mod = color.New(color.FgYellow).SprintFunc()
	rem = color.New(color.FgRed).SprintFunc()
)

func LogOperation(id string, op string, path string) {
	Debug("Logging operation: id=%s, op=%s, path=%s", id, op, path)

	err := WithLock(dirs.StagingLogs, DefaultLockTimeout, func() error {
		logs, err := os.ReadFile(dirs.StagingLogs)
		if err != nil {
			Debug("Failed to read staging logs")
			return err
		}
		var content []LogFileEntry
		if len(logs) > 0 {
			if err = json.Unmarshal(logs, &content); err != nil {
				Debug("Failed to unmarshal staging logs")
				return err
			}
		}
		content = append(content, LogFileEntry{
			Id:   id,
			Op:   op,
			Path: path,
		})
		WriteJson(dirs.StagingLogs, content)
		Debug("Operation logged successfully")
		return nil
	})

	if err != nil {
		MustSucceed(err, "operation failed")
	}
}

func LogEntryLookup(op string, path string) (isLogged bool, logId string, operation string) {
	Debug("Looking up log entry: op=%s, path=%s", op, path)
	logs, err := os.ReadFile(dirs.StagingLogs)
	if err != nil {
		Debug("Failed to read staging logs")
		MustSucceed(err, "operation failed")
	}
	var content []LogFileEntry
	if len(logs) > 0 {
		if err = json.Unmarshal(logs, &content); err != nil {
			Debug("Failed to unmarshal staging logs")
			MustSucceed(err, "operation failed")
		}
		for _, entry := range content {
			// Consider op "*" as a wildcard.
			if op == "*" && entry.Path == path || entry.Op == op && entry.Path == path {
				Debug("Found log entry: id=%s, op=%s", entry.Id, entry.Op)
				return true, entry.Id, entry.Op
			}
		}
	}
	Debug("No matching log entry found")
	return false, "", ""
}

func IsStagingLogsEmpty() bool {
	Debug("Checking if staging logs are empty")
	logs, err := os.ReadFile(dirs.StagingLogs)
	if err != nil {
		Debug("Failed to read staging logs")
		MustSucceed(err, "operation failed")
	}
	var content []LogFileEntry
	if len(logs) > 0 {
		if err = json.Unmarshal(logs, &content); err != nil {
			Debug("Failed to unmarshal staging logs")
			MustSucceed(err, "operation failed")
		}
		if len(content) == 0 {
			Debug("Staging logs are empty")
			return true
		}
		Debug("Staging logs are not empty")
		return false
	}
	Debug("Staging logs file is empty")
	return true
}

func RemoveLogEntry(id string) {
	Debug("Removing log entry: id=%s", id)

	err := WithLock(dirs.StagingLogs, DefaultLockTimeout, func() error {
		logs, err := os.ReadFile(dirs.StagingLogs)
		if err != nil {
			Debug("Failed to read staging logs")
			return err
		}
		var content []LogFileEntry
		if len(logs) > 0 {
			if err = json.Unmarshal(logs, &content); err != nil {
				Debug("Failed to unmarshal staging logs")
				return err
			}
		}
		for i, entry := range content {
			if entry.Id == id {
				Debug("Found and removing log entry: id=%s, op=%s", entry.Id, entry.Op)
				content = slices.Delete(content, i, i+1)
				break
			}
		}
		WriteJson(dirs.StagingLogs, content)
		Debug("Log entry removed successfully")
		return nil
	})

	if err != nil {
		MustSucceed(err, "operation failed")
	}
}

func TruncateLogs() {
	Debug("Truncating staging logs")

	err := WithLock(dirs.StagingLogs, DefaultLockTimeout, func() error {
		WriteJson(dirs.StagingLogs, []LogFileEntry{})
		Debug("Staging logs truncated successfully")
		return nil
	})

	if err != nil {
		MustSucceed(err, "operation failed")
	}
}

func GetStagingLogsContent() (result *[]LogFileEntry) {
	Debug("Getting staging logs content")
	logs, err := os.ReadFile(dirs.StagingLogs)
	if err != nil {
		Debug("Failed to read staging logs")
		MustSucceed(err, "operation failed")
	}
	var content []LogFileEntry
	if len(logs) > 0 {
		if err = json.Unmarshal(logs, &content); err != nil {
			Debug("Failed to unmarshal staging logs")
			MustSucceed(err, "operation failed")
		}
	} else {
		content = []LogFileEntry{}
		Debug("Staging logs are empty")
		return &content
	}
	Debug("Retrieved %d log entries", len(content))
	return &content
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
			log = append(log, add(" "+logEntry.Op+":")+" "+logEntry.Path)
		case "MOD":
			log = append(log, mod(" "+logEntry.Op+":")+" "+logEntry.Path)
		case "REM":
			log = append(log, rem(" "+logEntry.Op+":")+" "+logEntry.Path)
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
			log = append(log, add(" "+logEntry.Op+":")+" "+logEntry.Path)
		case "MOD":
			log = append(log, mod(" "+logEntry.Op+":")+" "+logEntry.Path)
		case "REM":
			log = append(log, rem(" "+logEntry.Op+":")+" "+logEntry.Path)
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

// ValidateStagingIntegrity checks if staging logs match actual staged files
// Returns list of orphaned file IDs that should be cleaned up
func ValidateStagingIntegrity() []string {
	Debug("Validating staging integrity")
	logs := GetStagingLogsContent()
	orphanedIds := []string{}

	// Check if all logged files exist in staging
	for _, entry := range *logs {
		var stagingPath string
		switch entry.Op {
		case "ADD":
			stagingPath = dirs.StagingAdded + entry.Id
		case "MOD":
			stagingPath = dirs.StagingModified + entry.Id
		case "REM":
			stagingPath = dirs.StagingRemoved + entry.Id
		}

		if _, err := os.Stat(stagingPath); os.IsNotExist(err) {
			Debug("Found orphaned log entry: %s (path: %s)", entry.Id, entry.Path)
			orphanedIds = append(orphanedIds, entry.Id)
		}
	}

	Debug("Found %d orphaned entries", len(orphanedIds))
	return orphanedIds
}

// CleanOrphanedStagingEntries removes log entries that don't have corresponding staged files
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
		if isCommitted, _, _ := GetFileMetadata(path); isCommitted {
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

		_, fileName := ParsePath(file.Path)

		if isModified, _ := IsModified(file.Path, dirs.Commits+file.CommitId+"/"+file.Id+"/"+fileName); isModified {
			modified = append(modified, file.Path)
		}
	}

	return modified, deleted
}
