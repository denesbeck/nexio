package main

import (
	"encoding/json"
	"os"
)

type FileListEntry struct {
	Id       string `json:"id"`
	CommitId string `json:"commitId"`
	Path     string `json:"path"`
	BlobHash string `json:"blobHash"`
	Mode     uint32 `json:"mode"`
}

func IsFileStaged(filePath string) bool {
	Debug("Checking if file is staged: %s", filePath)
	logs, err := os.ReadFile(GetDir("staging_logs_file"))
	if err != nil {
		Debug("Failed to read staging logs")
		MustSucceed(err, "operation failed")
	}

	if len(logs) == 0 {
		Debug("No staging logs found")
		return false
	}
	var content []LogFileEntry
	if err = json.Unmarshal(logs, &content); err != nil {
		Debug("Failed to unmarshal staging logs")
		MustSucceed(err, "operation failed")
	}
	for _, entry := range content {
		if entry.Path == filePath {
			Debug("File is staged with operation: %s", entry.Op)
			return true
		}
	}
	Debug("File is not staged")
	return false
}

func GetFileMetadata(filePath string) (isCommitted bool, metadata FileListEntry) {
	Debug("Getting file metadata: %s", filePath)
	latestCommitId := GetLastCommit().Id

	Debug("Latest commit ID: %s", latestCommitId)
	if latestCommitId == "" {
		Debug("No commits found")
		return false, FileListEntry{}
	}
	fileList, err := os.ReadFile(GetDir("commits") + latestCommitId + "/fileList.json")
	if err != nil {
		Debug("Failed to read file list")
		MustSucceed(err, "operation failed")
	}

	var content []FileListEntry
	if err = json.Unmarshal(fileList, &content); err != nil {
		Debug("Failed to unmarshal file list")
		MustSucceed(err, "operation failed")
	}
	for _, fileListEntry := range content {
		if fileListEntry.Path == filePath {
			Debug("File (%s) found in commit: %s", fileListEntry.Id, fileListEntry.CommitId)
			return true, fileListEntry
		}
	}
	Debug("File not found in any commit")
	return false, FileListEntry{}
}

func IsFileDeleted(filePath string) bool {
	Debug("Checking if file is deleted: %s", filePath)
	committed, _ := GetFileMetadata(filePath)
	existsInWorkdir := FileExists(filePath)
	isDeleted := committed && !existsInWorkdir
	Debug("File deletion status: committed=%v, exists=%v, isDeleted=%v", committed, existsInWorkdir, isDeleted)
	return isDeleted
}
