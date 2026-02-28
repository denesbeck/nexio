package main

import "errors"

var (
	errCommitNotFound = errors.New("Commit does not exist")
	errBranchExists   = errors.New("Branch already exists")
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
	return DBGetLastCommit()
}

func GetLastCommitByBranch(branch string) Commit {
	return DBGetLastCommitByBranch(branch)
}

func CountCommits() int {
	return DBCountCommits()
}

func GetCommits() *[]Commit {
	commits := DBGetCommits()
	return &commits
}

func GetFileListContent(commitId string) (result *[]FileListEntry) {
	files := DBGetFileListForCommit(commitId)
	return &files
}

func ProcessFileList(latestCommitId string, newCommitId string) {
	DBProcessFileList(latestCommitId, newCommitId)
}

func WriteCommitMetadata(commitId string, message string) {
	// In SQLite mode, metadata is stored as part of the commit record.
	// This is called from runCoreCommitCommand, but the actual metadata
	// (author, message) is written in DBRegisterCommit.
	// This function is now a no-op since we combine it with RegisterCommitForBranch.
	Debug("WriteCommitMetadata called for commit %s (handled by DBRegisterCommit)", commitId)
}

func RegisterCommitForBranch(commitId string) {
	// This is now handled by DBRegisterCommit which combines
	// commit creation + branch head update + metadata storage
	Debug("RegisterCommitForBranch called for commit %s", commitId)
}

// RegisterCommit creates a commit with metadata and registers it for the current branch
func RegisterCommit(commitId string, message string) {
	branch := GetCurrentBranchName()
	DBRegisterCommit(commitId, message, branch)
}

// HasUncommittedChanges checks if there are any uncommitted changes in the working directory
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

// CopyCommitsToBranch creates a new branch with head_commit pointing to the given commit.
// Since commits are shared across branches (no branch column), we just need to create
// the branch and set its head_commit.
func CopyCommitsToBranch(commitId string, targetBranch string) error {
	Debug("Creating branch %s from commit %s", targetBranch, commitId)

	// Verify the commit exists
	var exists int
	err := db.QueryRow("SELECT COUNT(*) FROM commits WHERE id = ?", commitId).Scan(&exists)
	if err != nil || exists == 0 {
		Debug("Commit does not exist: %s", commitId)
		return errCommitNotFound
	}

	// Check if branch already exists
	if DBBranchExists(targetBranch) {
		Debug("Branch already exists: %s", targetBranch)
		return errBranchExists
	}

	// Create the new branch pointing to the given commit
	if err := DBCreateBranch(targetBranch, false, false); err != nil {
		return err
	}
	DBUpdateBranchHead(targetBranch, commitId)

	return nil
}
