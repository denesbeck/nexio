package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// Commit type definitions for interactive mode
type CommitType struct {
	Name        string
	Description string
}

var commitTypes = []CommitType{
	{Name: "feat", Description: "A new feature"},
	{Name: "fix", Description: "A bug fix"},
	{Name: "docs", Description: "Documentation changes"},
	{Name: "style", Description: "Code style (formatting, etc.)"},
	{Name: "refactor", Description: "Code restructuring"},
	{Name: "perf", Description: "Performance improvements"},
	{Name: "test", Description: "Adding tests"},
	{Name: "chore", Description: "Maintenance tasks"},
}

func init() {
	commitCmd.Flags().StringVarP(&Message, "message", "m", "", "Commit message (optional, interactive mode if not provided)")

	rootCmd.AddCommand(commitCmd)
}

var Message string

var commitCmd = &cobra.Command{
	Use:     "commit",
	Short:   "Record changes to the repository",
	Example: "nexio commit -m <your commit message>\n  nexio commit  (interactive mode)",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, args []string) {
		if Message == "" {
			Debug("No message provided, starting interactive mode")
			runInteractiveCommit()
		} else {
			Debug("Starting commit command with message: %s", Message)
			runCommitCommand(Message)
		}
	},
}

func runInteractiveCommit() {
	initialized := IsInitialized()
	if !initialized {
		Fail(COMMON_RETURN_CODES[001])
		return
	}

	// Clean up any orphaned staging entries from previous failed operations
	CleanOrphanedStagingEntries()

	empty := IsStagingLogsEmpty()
	if empty {
		Debug("No changes staged for commit.")
		Fail(COMMIT_RETURN_CODES[701])
		return
	}

	if os.Getenv("NEXIO_ENV") == "test" {
		Debug("Test environment detected, skipping interactive mode")
		runCommitCommand("test: auto-generated commit message")
		return
	}

	BreakLine()

	options := make([]string, len(commitTypes))
	for i, ct := range commitTypes {
		options[i] = fmt.Sprintf("%-10s %s", ct.Name+":", ct.Description)
	}

	selectedType, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultText(pterm.Yellow("?") + pterm.Cyan(" Select commit type")).
		Show()
	if err != nil {
		Debug("Failed to get commit type selection: %v", err)
		Fail("Failed to get commit type selection")
		return
	}

	commitTypeName := strings.Split(selectedType, ":")[0]
	commitTypeName = strings.TrimSpace(commitTypeName)
	Debug("Selected commit type: %s", commitTypeName)

	BreakLine()

	description, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText(pterm.Yellow("?") + pterm.Cyan(" Description") + pterm.Gray(" (50 chars)")).
		Show()
	if err != nil {
		Debug("Failed to get short description: %v", err)
		Fail("Failed to get short description")
		return
	}

	description = strings.TrimSpace(description)
	if description == "" {
		Fail("Description cannot be empty")
		return
	}

	if len(description) > 50 {
		description = description[:50]
	}
	Debug("Description: %s", description)

	commitMessage := fmt.Sprintf("%s: %s", commitTypeName, description)

	stagingLogs := GetSyncedStagingLogsContent()
	addCount, modCount, remCount := CountOps(*stagingLogs)

	returnCode, commitId := runCoreCommitCommand(commitMessage)

	if returnCode == 702 && commitId != "" {
		displayInteractiveCommitSuccess(commitId, addCount, modCount, remCount)
	}
}

func displayInteractiveCommitSuccess(commitId string, addCount, modCount, remCount int) {
	BreakLine()

	// // Success message with short commit ID
	shortCommitId := commitId
	if len(commitId) > 10 {
		shortCommitId = commitId[:10]
	}

	messages := []string{
		"Registering commit (" + StyledCommit(shortCommitId) + ")...        "}
	stop := Spinner(messages, false)
	if os.Getenv("NEXIO_ENV") != "test" {
		time.Sleep(1 * time.Second)
	}
	stop()

	BreakLine()

	Info("Changes:")

	addStyle := pterm.NewStyle(pterm.FgLightGreen)
	modStyle := pterm.NewStyle(pterm.FgLightYellow)
	remStyle := pterm.NewStyle(pterm.FgLightRed)

	TreeList([]string{
		addStyle.Sprintf("%d", addCount) + " files added",
		modStyle.Sprintf("%d", modCount) + " files modified",
		remStyle.Sprintf("%d", remCount) + " files removed"}, false)

	BreakLine()

	// Repository size
	repoSize := GetRepositorySize()
	Text(Bold("Repository size: ")+repoSize, "")
	BreakLine()
}

func runCommitCommand(message string) (returnCode int, commitId string) {
	initialized := IsInitialized()
	if !initialized {
		Fail(COMMON_RETURN_CODES[001])
		return 001, ""
	}

	// Clean up any orphaned staging entries from previous failed operations
	CleanOrphanedStagingEntries()

	empty := IsStagingLogsEmpty()
	if empty {
		Debug("No changes staged for commit.")
		Fail(COMMIT_RETURN_CODES[701])
		return 701, ""
	}

	_, commitId = runCoreCommitCommand(message)

	Success(COMMIT_RETURN_CODES[702])
	return 702, commitId
}

func runCoreCommitCommand(message string) (int, string) {
	newCommitId := GenRandHex(20)
	latestCommitId := GetLastCommit().Id
	Debug("Creating new commit: id=%s, parent=%s", newCommitId, latestCommitId)

	ProcessFileList(latestCommitId, newCommitId)
	Debug("Processed file list for commit")

	WriteCommitMetadata(newCommitId, message)
	Debug("Wrote commit metadata")

	if err := CopyFile(dirs.StagingLogs, dirs.Commits+newCommitId+"/logs.json"); err != nil {
		Debug("Failed to copy staging logs to commit")
		MustSucceed(err, "operation failed")
	}

	Debug("Copied staging logs to commit")

	TruncateLogs()
	if err := EmptyDir(dirs.StagingAdded); err != nil {
		Debug("Failed to empty staging added directory")
		MustSucceed(err, "operation failed")
	}
	if err := EmptyDir(dirs.StagingModified); err != nil {
		Debug("Failed to empty staging modified directory")
		MustSucceed(err, "operation failed")
	}
	if err := EmptyDir(dirs.StagingRemoved); err != nil {
		Debug("Failed to empty staging removed directory")
		MustSucceed(err, "operation failed")
	}
	Debug("Cleaned up staging area")

	RegisterCommitForBranch(newCommitId)
	Debug("Registered commit for current branch")

	return 702, newCommitId
}
