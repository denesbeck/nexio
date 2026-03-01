package main

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(unstageCmd)
}

var unstageCmd = &cobra.Command{
	Use:     "unstage",
	Aliases: []string{"usg"},
	Short:   "Unstage the selected files from the staging area",
	Example: "nexio unstage <path/to/your/file>",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		for _, arg := range args {
			runUnstageCommand(arg)
		}
	},
}

func runUnstageCommand(filePath string) {
	initialized := IsInitialized()
	if !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return
	}

	isLogged, logEntry := LogEntryLookup("*", filePath)

	if isLogged {
		err := RemoveLogEntry(logEntry.Id)
		if err != nil {
			Debug("Error removing log entry: %s", err.Error())
			Fail("%s", COMMON_RETURN_CODES[005])
			return
		}
		Success("%s", UNSTAGE_RETURN_CODES[801])
	} else {
		Info("%s", UNSTAGE_RETURN_CODES[802])
	}
}
