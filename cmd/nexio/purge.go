package main

import (
	"os"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(purgeCmd)
}

var purgeCmd = &cobra.Command{
	Use:     "purge",
	Short:   "Purge Nexio and all its data. THIS COMMAND IS IRREVERSIBLE!",
	Example: "nexio purge",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, args []string) {
		runPurgeCommand()
	},
}

func runPurgeCommand() {
	initialized := IsInitialized()
	if !initialized {
		Fail(COMMON_RETURN_CODES[001])
		return
	}
	BreakLine()
	Warning("WARNING: Destructive Operation!")
	BreakLine()
	List("This will permanently delete:", []string{
		"All commits and history",
		"All branches",
		"All staged changes",
		"Configuration files"}, false)

	BreakLine()
	Text("Location: "+Code(dirs.Root), "")
	BreakLine()
	repoSize := GetRepositorySize()
	Text(Bold("Size: ")+repoSize, "")
	BreakLine()

	var result bool
	if os.Getenv("NEXIO_ENV") != "test" {
		var err error
		result, err = pterm.DefaultInteractiveConfirm.Show(pterm.Yellow("?") + pterm.Cyan(" Are you sure you want to continue?"))

		if err != nil {
			Debug("Failed to get user input for purge command.")
			MustSucceed(err, "operation failed")
		}
	} else {
		result = true
	}

	BreakLine()
	if !result {
		Debug("User cancelled purge command.")
		Info(PURGE_RETURN_CODES[902])
		BreakLine()
		return
	}

	if namespace == "" {
		os.RemoveAll(dirs.Root)
	} else {
		os.RemoveAll(namespace)
	}

	messages := []string{
		"Deleting Nexio data...",
		"Deleting configuration...         ",
	}
	stop := Spinner(messages, false)
	if os.Getenv("NEXIO_ENV") != "test" {
		time.Sleep(1 * time.Second)
	}
	stop()

	BreakLine()
	Success(PURGE_RETURN_CODES[901])
	BreakLine()
}
