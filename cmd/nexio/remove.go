package main

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(removeCmd)
}

var removeCmd = &cobra.Command{
	Use:     "remove",
	Short:   "Remove the selected files from the staging area",
	Example: "nexio remove <path/to/your/file>",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		for _, arg := range args {
			runRemoveCommand(arg)
		}
	},
}

func runRemoveCommand(filePath string) {
	initialized := IsInitialized()
	if !initialized {
		Fail(COMMON_RETURN_CODES[001])
		return
	}

	isLogged, logEntry := LogEntryLookup("*", filePath)

	if isLogged {
		err := RemoveLogEntry(logEntry.Id)
		if err != nil {
			Debug("Error removing log entry: %s", err.Error())
			Fail(COMMON_RETURN_CODES[005])
			return
		}
		Success(REMOVE_RETURN_CODES[801])
	} else {
		Info(REMOVE_RETURN_CODES[802])
	}
}
