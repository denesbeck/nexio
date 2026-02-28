package main

type FileListEntry struct {
	Id       string `json:"id"`
	CommitId string `json:"commitId"`
	Path     string `json:"path"`
	BlobHash string `json:"blobHash"`
	Mode     uint32 `json:"mode"`
}

func IsFileStaged(filePath string) bool {
	return DBIsFileStaged(filePath)
}

func GetFileMetadata(filePath string) (isCommitted bool, metadata FileListEntry) {
	return DBGetFileMetadata(filePath)
}

func IsFileDeleted(filePath string) bool {
	Debug("Checking if file is deleted: %s", filePath)
	committed, _ := GetFileMetadata(filePath)
	existsInWorkdir := FileExists(filePath)
	isDeleted := committed && !existsInWorkdir
	Debug("File deletion status: committed=%v, exists=%v, isDeleted=%v", committed, existsInWorkdir, isDeleted)
	return isDeleted
}
