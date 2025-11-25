package main

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
)

type Commit struct {
	Id        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Next      string `json:"next"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CommitMetadata struct {
	Author  Author `json:"author"`
	Message string `json:"message"`
}

func GetLastCommit() Commit {
	Debug("Getting last commit")
	currentBranchName := GetCurrentBranchName()
	commit := GetLastCommitByBranch(currentBranchName)
	Debug("Last commit: %s", commit)
	return commit
}

func GetLastCommitByBranch(branch string) Commit {
	Debug("Getting last commit for branch: %s", branch)
	commits, err := os.ReadFile(dirs.Branches + branch + "/commits.json")
	if err != nil {
		Debug("Failed to read commits file")
		MustSucceed(err, "operation failed")
	}
	var content []Commit
	if err = json.Unmarshal(commits, &content); err != nil {
		Debug("Failed to unmarshal commits")
		MustSucceed(err, "operation failed")
	}
	if len(content) == 0 {
		Debug("No commits found for branch")
		return Commit{}
	}
	// The last commit is the one with an empty Next field
	for _, commit := range content {
		if commit.Next == "" {
			Debug("Last commit for branch: %s", commit.Id)
			return commit
		}
	}
	// Fallback: if no commit has empty Next (shouldn't happen), return the last in array
	Debug("Warning: No commit found with empty Next, using last in array")
	return content[len(content)-1]
}

func CountCommits() int {
	Debug("Counting all commits")
	currentBranchName := GetCurrentBranchName()
	commits, err := os.ReadFile(dirs.Branches + currentBranchName + "/commits.json")
	if err != nil {
		Debug("Failed to read commits file")
		MustSucceed(err, "operation failed")
	}
	var content []Commit
	if err = json.Unmarshal(commits, &content); err != nil {
		Debug("Failed to unmarshal commits")
		MustSucceed(err, "operation failed")
	}
	Debug("Counted %d commits", len(content))

	return len(content)
}

func GetCommits() *[]Commit {
	Debug("Getting all commits")
	currentBranchName := GetCurrentBranchName()
	commits, err := os.ReadFile(dirs.Branches + currentBranchName + "/commits.json")
	if err != nil {
		Debug("Failed to read commits file")
		MustSucceed(err, "operation failed")
	}
	var content []Commit
	if err = json.Unmarshal(commits, &content); err != nil {
		Debug("Failed to unmarshal commits")
		MustSucceed(err, "operation failed")
	}
	Debug("Retrieved %d commits", len(content))

	// Sort commits by following the linked list
	sortedCommits := sortCommitsByLinkedList(content)
	return &sortedCommits
}

// sortCommitsByLinkedList sorts commits by traversing the linked list from first to last
// The first commit has no predecessor (no other commit points to it)
// Each commit points to the next one via the Next field
func sortCommitsByLinkedList(commits []Commit) []Commit {
	if len(commits) == 0 {
		return commits
	}

	Debug("Sorting %d commits by linked list", len(commits))

	// Build a map for O(1) lookup: commitId -> Commit
	commitMap := make(map[string]Commit)
	// Track which commits are pointed to by another commit
	hasParent := make(map[string]bool)

	for _, commit := range commits {
		commitMap[commit.Id] = commit
		if commit.Next != "" {
			hasParent[commit.Next] = true
		}
	}

	// Find the first commit (the one that has no parent pointing to it)
	var firstCommit *Commit
	for _, commit := range commits {
		if !hasParent[commit.Id] {
			firstCommit = &commit
			Debug("Found first commit: %s", commit.Id)
			break
		}
	}

	if firstCommit == nil {
		Debug("Warning: Could not find first commit, returning unsorted")
		return commits
	}

	// Traverse the linked list from first to last
	sorted := make([]Commit, 0, len(commits))
	current := firstCommit
	visited := make(map[string]bool) // Prevent infinite loops

	for current != nil && !visited[current.Id] {
		sorted = append(sorted, *current)
		visited[current.Id] = true

		// Move to next commit
		if current.Next == "" {
			break
		}

		next, exists := commitMap[current.Next]
		if !exists {
			Debug("Warning: Broken linked list at commit %s -> %s", current.Id, current.Next)
			break
		}
		current = &next
	}

	Debug("Sorted %d commits in chronological order", len(sorted))
	return sorted
}

func GetFileListContent(commitId string) (result *[]FileListEntry) {
	Debug("Getting file list for commit: %s", commitId)
	fileList, err := os.ReadFile(dirs.Commits + commitId + "/fileList.json")
	if err != nil {
		Debug("Failed to read file list")
		MustSucceed(err, "operation failed")
	}
	var content []FileListEntry
	if len(fileList) > 0 {
		if err = json.Unmarshal(fileList, &content); err != nil {
			Debug("Failed to unmarshal file list")
			MustSucceed(err, "operation failed")
		}
	} else {
		content = []FileListEntry{}
		Debug("Empty file list")
		return &content
	}
	Debug("Retrieved %d files from commit", len(content))
	return &content
}

func ProcessFileList(latestCommitId string, newCommitId string) {
	Debug("Processing file list: latest=%s, new=%s", latestCommitId, newCommitId)
	var fileList *[]FileListEntry
	emptyFileList := []FileListEntry{}
	if latestCommitId == "" {
		Debug("No previous commit, starting with empty file list")
		fileList = &emptyFileList
	} else {
		fileListContent := GetFileListContent(latestCommitId)
		if fileListContent != nil {
			fileList = fileListContent
			Debug("Using file list from previous commit")
		} else {
			fileList = &emptyFileList
			Debug("No file list found in previous commit")
		}
	}

	stagingLogs := GetSyncedStagingLogsContent()

	for _, logEntry := range *stagingLogs {
		Debug("Processing staging log entry: op=%s, path=%s", logEntry.Op, logEntry.Path)
		switch logEntry.Op {
		case "REM":
			if len(*fileList) == 0 {
				Debug("Skipping REM operation - no files in list")
				continue
			}
			for i, entry := range *fileList {
				if entry.Path == logEntry.Path {
					Debug("Removing file from list: %s", entry.Path)
					*fileList = slices.Delete((*fileList), i, i+1)
					break
				}
			}
		case "ADD":
			Debug("Adding new file to list: %s", logEntry.Path)
			*fileList = append(*fileList, FileListEntry{Id: logEntry.Id, CommitId: newCommitId, Path: logEntry.Path})
			_, fileName := ParsePath(logEntry.Path)
			CopyFile(logEntry.Path, dirs.Commits+newCommitId+"/"+logEntry.Id+"/"+fileName)
		case "MOD":
			if len(*fileList) == 0 {
				Debug("Skipping MOD operation - no files in list")
				continue
			}
			for i, entry := range *fileList {
				if logEntry.Path == entry.Path {
					Debug("Updating file in list: %s", entry.Path)
					(*fileList)[i].Id = logEntry.Id
					(*fileList)[i].CommitId = newCommitId
				}
			}
			_, fileName := ParsePath(logEntry.Path)
			CopyFile(logEntry.Path, dirs.Commits+newCommitId+"/"+logEntry.Id+"/"+fileName)
		}
	}
	WriteJson(dirs.Commits+newCommitId+"/fileList.json", fileList)
	Debug("File list processed successfully")
}

func WriteCommitMetadata(commitId string, message string) {
	Debug("Writing commit metadata: id=%s, message=%s", commitId, message)
	config := GetConfig()
	author := Author{
		Name:  config.Name,
		Email: config.Email,
	}
	WriteJson(dirs.Commits+commitId+"/metadata.json", CommitMetadata{Author: author, Message: message})
	Debug("Commit metadata written successfully")
}

func RegisterCommitForBranch(commitId string) {
	Debug("Registering commit for branch: %s", commitId)
	currentBranchName := GetCurrentBranchName()

	err := WithLock(dirs.Branches+currentBranchName+"/commits", DefaultLockTimeout, func() error {
		commits, err := os.ReadFile(dirs.Branches + currentBranchName + "/commits.json")
		if err != nil {
			Debug("Failed to read commits file")
			return err
		}
		var content []Commit
		if err = json.Unmarshal(commits, &content); err != nil {
			Debug("Failed to unmarshal commits")
			return err
		}
		content = append(content, Commit{Id: commitId, Timestamp: GetTimestamp(), Next: ""})
		if len(content) > 1 {
			content[len(content)-2].Next = commitId
		}
		WriteJson(dirs.Branches+currentBranchName+"/commits.json", content)
		Debug("Commit registered successfully")
		return nil
	})

	if err != nil {
		MustSucceed(err, "operation failed")
	}
}

// HasUncommittedChanges checks if there are any uncommitted changes in the working directory
// Returns true if there are staged files, modified files, or deleted files
func HasUncommittedChanges() bool {
	Debug("Checking for uncommitted changes")

	// Check for staged files
	stagingLogs := GetStagingLogsContent()
	if len(*stagingLogs) > 0 {
		Debug("Found %d staged files", len(*stagingLogs))
		return true
	}

	// Check for modified or deleted files
	modified, deleted := GetModifiedOrDeletedFiles()
	if len(modified) > 0 || len(deleted) > 0 {
		Debug("Found %d modified and %d deleted files", len(modified), len(deleted))
		return true
	}

	Debug("No uncommitted changes found")
	return false
}

func CopyCommitsToBranch(commitId string, targetBranch string) error {
	Debug("Copying commits to branch: commit=%s, branch=%s", commitId, targetBranch)
	commits := GetCommits()
	commitIds := []string{}
	for _, commit := range *commits {
		commitIds = append(commitIds, commit.Id)
	}
	if !slices.Contains(commitIds, commitId) {
		Debug("Commit does not exist: %s", commitId)
		return errors.New("Commit does not exist")
	}

	if err := os.Mkdir(dirs.Branches+targetBranch, 0755); err != nil {
		Debug("Branch already exists: %s", targetBranch)
		return errors.New("Branch already exists")
	}

	index := FindIndex(commitIds, commitId)
	*commits = (*commits)[:(index + 1)]
	WriteJson(dirs.Branches+targetBranch+"/commits.json", *commits)
	Debug("Commits copied successfully")
	return nil
}
