package main

import (
	"os"
	"slices"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func init() {
	newCmd.Flags().StringVarP(&FromCommit, "from-commit", "c", "", "Commit to create branch from")
	newCmd.Flags().StringVarP(&FromBranch, "from-branch", "b", "", "Branch to create branch from")

	rootCmd.AddCommand(branchCmd)

	branchCmd.AddCommand(currentCmd)
	branchCmd.AddCommand(defaultCmd)
	branchCmd.AddCommand(newCmd)
	branchCmd.AddCommand(dropCmd)
	branchCmd.AddCommand(switchCmd)
}

var (
	FromCommit string
	FromBranch string
)

var branchCmd = &cobra.Command{
	Use:     "branch",
	Short:   "Branch management",
	Example: "nexio branch",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, args []string) {
		Debug("Starting branch command")
		runBranchCommand()
	},
}

var currentCmd = &cobra.Command{
	Use:     "current",
	Short:   "Get current branch",
	Example: "nexio branch current",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, args []string) {
		Debug("Starting current branch command")
		runCurrentCommand()
	},
}

var defaultCmd = &cobra.Command{
	Use:     "default",
	Short:   "Get default branch",
	Example: "nexio branch default",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, args []string) {
		Debug("Starting default branch command")
		runDefaultCommand()
	},
}

var newCmd = &cobra.Command{
	Use:     "new",
	Short:   "Create a new branch",
	Example: "nexio new <branch-name> --from-commit <commit-id> --from-branch <branch-name>",
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		Debug("Starting new branch command: name=%s, from-commit=%s, from-branch=%s", args[0], FromCommit, FromBranch)
		runNewCommand(args[0], FromCommit, FromBranch)
	},
}

var dropCmd = &cobra.Command{
	Use:     "drop",
	Short:   "Delete a branch",
	Example: "nexio drop <branch-name>",
	Args:    cobra.ArbitraryArgs,
	Run: func(_ *cobra.Command, args []string) {
		if len(args) == 0 {
			branchName := selectBranch("drop")
			if branchName == "" {
				return
			}
			Debug("Starting drop branch command: branch=%s", branchName)
			BreakLine()
			runDropCommand(branchName)
			BreakLine()
			return
		}
		Debug("Starting drop branch command with args: %v", args)
		for _, arg := range args {
			runDropCommand(arg)
		}
	},
}

var switchCmd = &cobra.Command{
	Use:     "switch",
	Short:   "Switch to a branch",
	Example: "nexio switch <branch-name>",
	Args:    cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		if len(args) == 0 {
			branchName := selectBranch("switch to")
			if branchName == "" {
				return
			}
			Debug("Starting switch branch command: branch=%s", branchName)
			BreakLine()
			runSwitchCommand(branchName)
			BreakLine()
			return
		}
		Debug("Starting switch branch command: branch=%s", args[0])
		runSwitchCommand(args[0])
	},
}

func selectBranch(action string) string {
	initialized := IsInitialized()
	if !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return ""
	}

	branches := ListBranches()
	if len(branches) == 0 {
		Fail("No branches available")
		return ""
	}

	if len(branches) == 1 {
		switch action {
		case "switch to":
			Info("No other branches available to switch to.")
		case "drop":
			Info("No branches available to drop.")
		}
		return ""
	}

	currentBranch := GetCurrentBranchName()
	defaultBranch := GetDefaultBranchName()

	options := make([]string, len(branches))
	for i, branch := range branches {
		option := branch
		if branch == defaultBranch {
			option += " 󰨐"
		}
		if branch == currentBranch {
			option += " (current)"
		}
		options[i] = option
	}

	BreakLine()
	selectedOption, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultText(pterm.Yellow("?") + pterm.Cyan(" Select branch to "+action)).
		Show()
	if err != nil {
		Debug("Failed to get branch selection: %v", err)
		Fail("Failed to get branch selection")
		return ""
	}

	// Extract branch name from option (remove markers)
	branchName := selectedOption
	if idx := indexOf(branchName, " 󰨐"); idx != -1 {
		branchName = branchName[:idx]
	}
	if idx := indexOf(branchName, " (current)"); idx != -1 {
		branchName = branchName[:idx]
	}

	return branchName
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func runBranchCommand() {
	initialized := IsInitialized()
	if !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return
	}

	branches := ListBranches()
	if len(branches) == 0 {
		Debug("%s", BRANCH_RETURN_CODES[217])
		Fail("%s", BRANCH_RETURN_CODES[217])
		return
	}

	currentBranchName := GetCurrentBranchName()
	defaultBranchName := GetDefaultBranchName()
	Debug("Current branch: %s, Default branch: %s", currentBranchName, defaultBranchName)

	branchList := []string{}
	for _, branchName := range branches {
		formattedBranchName := branchName
		if branchName == defaultBranchName {
			formattedBranchName = formattedBranchName + " 󰨐"
		}
		if branchName == currentBranchName {
			formattedBranchName = pterm.LightBlue(formattedBranchName)
		}
		branchList = append(branchList, formattedBranchName)
	}

	BreakLine()
	Info("Branches:")
	TreeList(branchList, true)
	BreakLine()
	Text("legend: 󰨐 = default, "+pterm.LightBlue("blue")+" = current", "")
	BreakLine()
	Debug("Branch command completed successfully")
}

func runCurrentCommand() {
	initialized := IsInitialized()
	if !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return
	}

	currentBranchName := GetCurrentBranchName()
	Debug("Current branch: %s", currentBranchName)
	Info("Current branch: %s", StyledBranch(currentBranchName))
}

func runDefaultCommand() {
	initialized := IsInitialized()
	if !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return
	}

	defaultBranchName := GetDefaultBranchName()
	Debug("Default branch: %s", defaultBranchName)
	Info("Default branch: %s", StyledBranch(defaultBranchName))
}

func runNewCommand(branchName string, fromCommit string, fromBranch string) int {
	initialized := IsInitialized()
	if !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return 001
	}

	if !IsValidBranchName(branchName) {
		Debug("Invalid branch name: %s", branchName)
		Fail("%s", BRANCH_RETURN_CODES[201])
		return 201
	}

	if fromCommit != "" && fromBranch != "" {
		Debug("Cannot create branch from both commit and branch")
		Fail("%s", BRANCH_RETURN_CODES[202])
		return 202
	}

	var srcBranch string
	if fromBranch != "" {
		srcBranch = fromBranch
		branches := ListBranches()
		if !slices.Contains(branches, srcBranch) {
			Debug("Source branch does not exist: %s", srcBranch)
			Fail("%s", BRANCH_RETURN_CODES[203])
			return 203
		}
	} else {
		srcBranch = GetCurrentBranchName()
	}

	if fromCommit != "" {
		Debug("Creating branch from commit: %s", fromCommit)
		err := CopyCommitsToBranch(fromCommit, branchName)
		if err != nil {
			Debug("Failed to create branch from commit: %v", err)
			Fail("%s", BRANCH_RETURN_CODES[204])
			return 204
		}
	} else {
		Debug("Creating branch from branch: %s", srcBranch)
		// Check if branch already exists
		if DBBranchExists(branchName) {
			Debug("Branch already exists: %s", branchName)
			Fail("%s", BRANCH_RETURN_CODES[205])
			return 205
		}

		// Create new branch
		if err := DBCreateBranch(branchName, false, false); err != nil {
			Debug("Failed to create branch: %v", err)
			Fail("%s", BRANCH_RETURN_CODES[205])
			return 205
		}

		// Point the new branch's head to the same commit as the source branch.
		// Commits are shared across branches — no copying needed.
		srcHead := DBGetHeadCommitForBranch(srcBranch)
		if srcHead != "" {
			DBUpdateBranchHead(branchName, srcHead)
		}
	}
	Debug("Branch created successfully: %s", branchName)
	Success("%s", BRANCH_RETURN_CODES[206])
	runSwitchCommand(branchName)
	return 206
}

func runDropCommand(branchName string) int {
	initialized := IsInitialized()
	if !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return 001
	}

	branches := ListBranches()
	if !slices.Contains(branches, branchName) {
		Debug("Branch does not exist: %s", branchName)
		Fail("%s", BRANCH_RETURN_CODES[207])
		return 207
	}

	if currentBranchName := GetCurrentBranchName(); currentBranchName == branchName {
		Debug("Cannot delete current branch: %s", branchName)
		Fail("%s", BRANCH_RETURN_CODES[208])
		return 208
	}

	if defaultBranchName := GetDefaultBranchName(); defaultBranchName == branchName {
		Debug("Cannot delete default branch: %s", branchName)
		Fail("%s", BRANCH_RETURN_CODES[209])
		return 209
	}

	if err := DBDropBranch(branchName); err != nil {
		Debug("Failed to delete branch: %s", branchName)
		Fail("%s", BRANCH_RETURN_CODES[207])
		return 207
	}
	Debug("Branch deleted successfully: %s", branchName)
	Success("%s", BRANCH_RETURN_CODES[210])
	return 210
}

func runSwitchCommand(branchName string) int {
	initialized := IsInitialized()
	if !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return 001
	}

	currentBranch := GetCurrentBranchName()
	if currentBranch == branchName {
		Debug("Already on branch: %s", branchName)
		Fail("%s", BRANCH_RETURN_CODES[211])
		return 211
	}

	branches := ListBranches()
	if !slices.Contains(branches, branchName) {
		Debug("Branch does not exist: %s", branchName)
		Fail("%s", BRANCH_RETURN_CODES[212])
		return 212
	}

	// Check for uncommitted changes before switching
	if HasUncommittedChanges() {
		BreakLine()
		Fail("%s", BRANCH_RETURN_CODES[214])
		BreakLine()

		// Show what changes would be lost
		stagingLogs := GetSyncedStagingLogsContent()
		if len(*stagingLogs) > 0 {
			Info("Staged changes (%d)", len(*stagingLogs))
			PrintLogs(*stagingLogs)
		}
		BreakLine()

		modified, deleted := GetModifiedOrDeletedFiles()
		if len(modified) > 0 || len(deleted) > 0 {
			Info("Unstaged changes (%d)", len(modified)+len(deleted))
			for i, file := range modified {
				modified[i] = pterm.FgYellow.Sprint(" MOD: ") + file
			}
			for i, file := range deleted {
				deleted[i] = pterm.FgRed.Sprint(" REM: ") + file
			}
			TreeList(modified, false)
			TreeList(deleted, false)
			BreakLine()
		}
		Warning("Please commit or stash your changes before switching branches.")
		BreakLine()

		return 214
	}

	Debug("Remove tracked files for current branch %s before switching.", currentBranch)

	oldBranchCommitId := GetLastCommitByBranch(currentBranch).Id
	// Remove the tracked files from the old branch
	if oldBranchCommitId != "" {
		fileList := GetFileListContent(oldBranchCommitId)
		for _, file := range *fileList {
			RemoveFile("./" + file.Path)
		}
	}

	newBranchCommitId := GetLastCommitByBranch(branchName).Id
	Debug("Switching to commit: %s", newBranchCommitId)

	// Restore the tracked files from the new branch
	if newBranchCommitId != "" {
		fileList := GetFileListContent(newBranchCommitId)
		for _, file := range *fileList {
			err := RestoreBlob(file.BlobHash, "./"+file.Path, os.FileMode(file.Mode))
			if err != nil {
				Debug("Failed to restore file %s: %v", file.Path, err)
				Fail("Failed to restore file: %s", file.Path)
				return 215
			}
		}
	}
	SetBranch(branchName, "current")
	Debug("Switched to branch: %s", branchName)
	Info("%s%s", BRANCH_RETURN_CODES[213], branchName)
	return 213
}
