package main

import (
	"fmt"
	"strconv"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:     "status",
	Short:   "List the files that are staged for commit",
	Example: "nexio status",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, args []string) {
		Debug("Starting status command")
		runStatusCommand()
	},
}

func runStatusCommand() (returnCode int, stagingLogs []LogFileEntry) {
	if initialized := IsInitialized(); !initialized {
		Fail(COMMON_RETURN_CODES[001])
		return 001, nil
	}
	content := GetStagingLogsContent()
	currentBranch := GetCurrentBranchName()
	commitCount := CountCommits()
	lastCommit := GetLastCommit()
	BreakLine()
	Box(Bold("Status"), fmt.Sprintf(pterm.FgCyan.Sprint(" ")+"Branch: %s\n"+pterm.FgCyan.Sprint(" ")+"Commits: %d\n"+pterm.FgCyan.Sprint(" ")+"Last commit: %s", currentBranch, commitCount, TimeAgo(lastCommit.Timestamp)))
	BreakLine()
	if len(*content) != 0 {
		Debug("Found %d files staged for commit.", len(*content))
		BreakLine()
		Info("Staged changes " + "(" + strconv.Itoa(len(*content)) + ")")
		PrintLogs(*content)
	} else {
		Debug("%s", STATUS_RETURN_CODES[501])
	}

	modified, deleted := GetModifiedOrDeletedFiles()
	if len(modified) > 0 || len(deleted) > 0 {
		Debug("Found %d tracked files that have been modified or deleted.", len(modified)+len(deleted))
		BreakLine()
		Info("Unstaged changes " + "(" + strconv.Itoa(len(modified)+len(deleted)) + ")")
		for i, file := range modified {
			modified[i] = pterm.FgYellow.Sprint(" MOD: ") + file
		}
		for i, file := range deleted {
			deleted[i] = pterm.FgRed.Sprint(" REM: ") + file
		}
		TreeList(modified, false)
		TreeList(deleted, false)
	} else {
		Debug("%s", STATUS_RETURN_CODES[503])
	}

	untracked := GetUntrackedFiles()
	if len(untracked) != 0 {
		BreakLine()
		Info("Untracked files " + "(" + strconv.Itoa(len(untracked)) + ")")
		TreeList(untracked, true)
		BreakLine()
		Text("Use "+Code("nexio add <file>...")+" to track", "")
	} else {
		Debug("%s", STATUS_RETURN_CODES[504])
	}

	if len(*content) == 0 && len(modified) == 0 && len(deleted) == 0 && len(untracked) == 0 {
		Debug("%s", STATUS_RETURN_CODES[505])
		BreakLine()
		Info(STATUS_RETURN_CODES[505])
	}
	BreakLine()
	Debug("Status command completed successfully")
	return 502, *content
}
