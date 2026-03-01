package main

import (
	"github.com/spf13/cobra"
)

func init() {
	stageCmd.Flags().BoolVarP(&Force, "force", "f", false, "Disregard the rules defined in `.nexio.rules.yml`")

	rootCmd.AddCommand(stageCmd)
}

var Force bool

type StageResult struct {
	FilePath   string
	ReturnCode int
	Message    string
	Success    bool
}

var stageCmd = &cobra.Command{
	Use:     "stage",
	Aliases: []string{"sg"},
	Short:   "Stage the selected files for commit",
	Example: "nexio stage <path/to/your/file>\nnexio stage file1 file2 file3\nnexio stage .",
	Args:    cobra.MinimumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Starting stage command with args: %v", args)

		initialized := IsInitialized()
		if !initialized {
			Fail("%s", COMMON_RETURN_CODES[001])
			return
		}

		filePaths, err := ExpandFilePaths(args)
		if err != nil {
			Fail("Failed to expand file paths: %s", err.Error())
			return
		}

		Debug("Processing %d files", len(filePaths))

		results := make([]StageResult, 0, len(filePaths))
		for _, filePath := range filePaths {
			result := runStageCommand(filePath, Force)
			results = append(results, result)
		}
		DisplayStageResults(results)
	},
}

func runStageCommand(filePath string, force bool) StageResult {
	result := StageResult{FilePath: filePath}
	returnCode := runStageCommandInternal(filePath, force, &result)
	result.ReturnCode = returnCode
	return result
}

func runStageCommandInternal(filePath string, force bool, _ *StageResult) int {
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
		Fail("%s", COMMON_RETURN_CODES[005])
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
					Fail("%s", COMMON_RETURN_CODES[005])
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
					Fail("%s", COMMON_RETURN_CODES[005])
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
					Fail("%s", COMMON_RETURN_CODES[005])
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
					Fail("%s", COMMON_RETURN_CODES[005])
				}
				_, metadata := GetFileMetadata(filePath)
				modified := hashedFile != metadata.BlobHash
				if modified {
					Debug("File was removed but modified, adding back as modified")
					if err := StageAndLog(generatedId, filePath, "modified"); err != nil {
						Debug("Error staging file: %s", err.Error())
						Fail("%s", COMMON_RETURN_CODES[005])
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
			LogOperation(generatedId, "REM", filePath, "")
			return 109
		}

		if isCommitted {
			modified := hashedFile != metadata.BlobHash
			if modified {
				Debug("File was committed and modified, staging as modified")
				if err := StageAndLog(generatedId, filePath, "modified"); err != nil {
					Debug("Error staging file: %s", err.Error())
					Fail("%s", COMMON_RETURN_CODES[005])
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
				Fail("%s", COMMON_RETURN_CODES[005])
				return 114
			}
			return 112
		}
	}
	return 100 // Fallback: Should not occur
}
