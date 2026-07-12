package main

import (
	"sort"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(workdirCmd)
}

var workdirCmd = &cobra.Command{
	Use:     "workdir",
	Aliases: []string{"wd"},
	Short:   "List the files that are committed",
	Example: "nexio workdir",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, args []string) {
		runWorkdirCommand()
	},
}

func runWorkdirCommand() (returnCode int, workdirContent []FileListEntry) {
	if initialized := IsInitialized(); !initialized {
		Fail("%s", COMMON_RETURN_CODES[001])
		return 001, nil
	}

	commitId := GetLastCommit().Id
	if commitId == "" {
		Info("%s", WORKDIR_RETURN_CODES[302])
		return 302, nil
	}

	// If there is a commit, there should be at least one file
	content := GetFileListContent(commitId)

	sort.Slice(content, func(i, j int) bool {
		return content[i].Path < content[j].Path
	})

	files := []string{}
	for _, record := range content {
		files = append(files, record.Path)
	}

	list := GenerateLeveledList(files)
	BreakLine()
	output := Tree(list, ".               ", false)

	Box(Bold(StyledBoxHeader("󱓜 Tracked files")), output)
	BreakLine()
	BreakLine()

	return 301, content
}
