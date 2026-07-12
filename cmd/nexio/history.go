package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(historyCmd)
}

var historyCmd = &cobra.Command{
	Use:     "history",
	Aliases: []string{"hi"},
	Short:   "List all commits for the current branch",
	Example: "nexio history",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, args []string) {
		Debug("Starting history command")
		runHistoryCommand()
	},
}

type History struct {
	AuthorName  string
	AuthorEmail string
	Date        string
	Message     string
	Commits     []LogFileEntry
}

func runHistoryCommand() (returnCode int, history []History) {
	initialized := IsInitialized()
	if !initialized {
		color.Red(COMMON_RETURN_CODES[001])
		return 001, nil
	}

	commits := GetCommits()
	if len(commits) == 0 {
		Debug("%s", HISTORY_RETURN_CODES[402])
		Info("%s", HISTORY_RETURN_CODES[402])
		return 402, nil
	}
	if len(commits) > 20 {
		Debug("Limiting to last 20 commits")
		commits = commits[:20]
	}

	Debug("Displaying %d commits", len(commits))

	history = make([]History, 0, len(commits))

	BreakLine()
	for _, commit := range commits {
		Debug("Processing commit: %s", commit.Id)

		// Get metadata from DB
		metadata := DBGetCommitMetadata(commit.Id)

		author := metadata.Author.Name + " <" + metadata.Author.Email + ">"
		if metadata.Author.Name == "" || metadata.Author.Email == "" {
			author = "Unknown"
		}

		// Get commit logs from DB
		logs := DBGetCommitLogs(commit.Id)
		Debug("Displaying %d log entries for commit", len(logs))

		logsFormatted := FormatLogs(logs)
		boxContent := fmt.Sprintf(Icon("")+"  Author:  %s\n"+Icon("")+"  Date:    %s\n"+Icon("")+"  Message: %s",
			author,
			TimeAgo(commit.Timestamp),
			metadata.Message,
		)

		add, mod, rem := CountOps(logs)

		if logsFormatted != "" {
			boxContent += "\n\n" + Icon("") + "  Files: " + Code(fmt.Sprintf("+%d -%d ~%d", add, rem, mod)) + "\n" + logsFormatted
		}

		Box(Bold(StyledCommit(" "+commit.Id[:10])), boxContent)
		BreakLine()
		BreakLine()
		history = append(history, History{
			AuthorName:  metadata.Author.Name,
			AuthorEmail: metadata.Author.Email,
			Date:        commit.Timestamp,
			Message:     metadata.Message,
			Commits:     logs,
		})
	}
	Debug("History command completed successfully")
	return 401, history
}
