package main

import (
	"github.com/spf13/cobra"
)

func init() {
	addCmd.Flags().BoolVarP(&Force, "force", "f", false, "Disregard the rules defined in `.nexio.rules.yml`")

	rootCmd.AddCommand(addCmd)
}

var Force bool

type AddResult struct {
	FilePath   string
	ReturnCode int
	Message    string
	Success    bool
}

var addCmd = &cobra.Command{
	Use:     "add",
	Short:   "Add the selected files to the staging area",
	Example: "nexio add <path/to/your/file>\nnexio add file1 file2 file3\nnexio add .",
	Args:    cobra.MinimumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Starting add command with args: %v", args)

		initialized := IsInitialized()
		if !initialized {
			Fail(COMMON_RETURN_CODES[001])
			return
		}

		filePaths, err := ExpandFilePaths(args)
		if err != nil {
			Fail("Failed to expand file paths: " + err.Error())
			return
		}

		Debug("Processing %d files", len(filePaths))

		results := make([]AddResult, 0, len(filePaths))
		for _, filePath := range filePaths {
			result := runAddCommand(filePath, Force)
			results = append(results, result)
		}
		DisplayAddResults(results)
	},
}

func runAddCommand(filePath string, force bool) AddResult {
	result := AddResult{FilePath: filePath}
	returnCode := runAddCommandInternal(filePath, force, &result)
	result.ReturnCode = returnCode
	return result
}

func runAddCommandInternal(filePath string, force bool, _ *AddResult) int {
	Debug("Processing file: %s", filePath)

	if err := ValidatePath(filePath); err != nil {
		Debug("Path is invalid: %s", err.Error())
		return 004
	}

	if !force {
		shouldIgnore := ShouldIgnore(filePath)
		if shouldIgnore {
			return 002
		}
	}

	generatedId := GenRandHex(20)
	Debug("Generated ID for file: %s", generatedId)

	hashedFile, err := HashFile(filePath)
	if err != nil {
		Debug("Error hashing file: %s", err.Error())
		Fail(COMMON_RETURN_CODES[005])
	}

	fileStaged := IsFileStaged(filePath)
	if fileStaged {
		Debug("File is already staged: %s", filePath)
		exists := FileExists(filePath)

		added, logEntry := LogEntryLookup("ADD", filePath)
		if added {
			if !exists {
				Debug("File was added but no longer exists, removing from staging")
				if err := RemoveLogEntry(logEntry.Id); err != nil {
					Debug("Error removing log entry: %s", err.Error())
					Fail(COMMON_RETURN_CODES[005])
				}
				return 101
			}

			modified := hashedFile != logEntry.BlobHash
			if modified {
				Debug("File was added and modified, updating staging")
				// Writing new blob, no new log entry needs to be created
				WriteBlob(filePath) // WARN:[clean-blob] The old blob remains, needs to be cleaned
				return 102
			}
			Debug("File was added but not modified")
			return 103
		}

		modified, logEntry := LogEntryLookup("MOD", filePath)
		if modified {
			if !exists {
				Debug("File was modified but no longer exists, removing from staging")
				if err := RemoveLogEntry(logEntry.Id); err != nil {
					Debug("Error removing log entry: %s", err.Error())
					Fail(COMMON_RETURN_CODES[005])
				}
				LogOperation(generatedId, "REM", filePath, hashedFile)
				return 104
			}
			modified := hashedFile != logEntry.BlobHash
			if modified {
				Debug("File was modified and changed, updating staging")
				_, err := WriteBlob(filePath)
				if err != nil {
					Debug("Error writing blob: %s", err.Error())
					Fail(COMMON_RETURN_CODES[005])
				}
				return 105
			}
			Debug("File was modified but not changed, staging is up-to-date")
			return 106
		}
		removed, logEntry := LogEntryLookup("REM", filePath)
		if removed {
			if exists {
				Debug("File was removed but exists again, checking modifications")
				if err := RemoveLogEntry(logEntry.Id); err != nil {
					Debug("Error removing log entry: %s", err.Error())
					Fail(COMMON_RETURN_CODES[005])
				}
				_, metadata := GetFileMetadata(filePath)
				modified := hashedFile != metadata.BlobHash
				if modified {
					Debug("File was removed but modified, adding back as modified")
					if err := StageAndLog(generatedId, filePath, "modified"); err != nil {
						Debug("Error staging file: %s", err.Error())
						Fail(COMMON_RETURN_CODES[005])
						return 114
					}
					return 107
				}
				Debug("File was removed but exists again without modifications, removed from staging")
				return 113
			} else {
				Debug("File was removed and still doesn't exist")
				return 108
			}
		}
	} else {
		Debug("File is not staged, checking commit status")
		isCommitted, metadata := GetFileMetadata(filePath)
		isDeleted := IsFileDeleted(filePath)
		if isDeleted {
			Debug("File was committed but deleted, staging for removal")
			blobHash, err := WriteBlob(filePath)
			if err != nil {
				Debug("Error writing blob: %s", err.Error())
				Fail(COMMON_RETURN_CODES[005])
				return 114
			}
			LogOperation(generatedId, "REM", filePath, blobHash)
			return 109
		}

		if isCommitted {
			modified := hashedFile != metadata.BlobHash
			if modified {
				Debug("File was committed and modified, staging as modified")
				if err := StageAndLog(generatedId, filePath, "modified"); err != nil {
					Debug("Error staging file: %s", err.Error())
					Fail(COMMON_RETURN_CODES[005])
					return 114
				}
				return 110
			} else {
				Debug("File was committed but not modified")
				return 111
			}
		} else {
			Debug("File is new, staging as added")
			if err := StageAndLog(generatedId, filePath, "added"); err != nil {
				Debug("Error staging file: %s", err.Error())
				Fail(COMMON_RETURN_CODES[005])
				return 114
			}
			return 112
		}
	}
	return 100 // Fallback: Should not occur
}
