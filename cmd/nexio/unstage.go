package main

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(unstageCmd)
}

type UnstageResult struct {
	FilePath   string
	ReturnCode int
	Success    bool
}

var unstageCmd = &cobra.Command{
	Use:     "unstage",
	Aliases: []string{"usg"},
	Short:   "Unstage the selected files from the staging area",
	Example: "nexio unstage <path/to/your/file>\nnexio unstage file1 file2 file3\nnexio unstage .",
	Args:    cobra.MinimumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		initialized := IsInitialized()
		if !initialized {
			Fail("%s", COMMON_RETURN_CODES[001])
			return
		}

		filePaths := expandUnstageFilePaths(args)

		results := make([]UnstageResult, 0, len(filePaths))
		for _, filePath := range filePaths {
			result := runUnstageCommand(filePath)
			results = append(results, result)
		}
		DisplayUnstageResults(results)
	},
}

func expandUnstageFilePaths(args []string) []string {
	var filePaths []string

	for _, arg := range args {
		if arg == "." {
			Debug("Unstaging all staged files")
			stagedFiles := GetStagingLogsContent()
			for _, entry := range stagedFiles {
				filePaths = append(filePaths, entry.Path)
			}
		} else {
			filePaths = append(filePaths, arg)
		}
	}

	return filePaths
}

func runUnstageCommand(filePath string) UnstageResult {
	result := UnstageResult{FilePath: filePath}

	isLogged, logEntry := LogEntryLookup("*", filePath)

	if isLogged {
		err := RemoveLogEntry(logEntry.Id)
		if err != nil {
			Debug("Error removing log entry: %s", err.Error())
			result.ReturnCode = 803
			return result
		}
		result.ReturnCode = 801
		result.Success = true
	} else {
		result.ReturnCode = 802
	}

	return result
}
