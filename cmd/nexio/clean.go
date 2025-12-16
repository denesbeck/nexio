package main

import (
	"strconv"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVarP(&DryRun, "dry-run", "n", false, "Preview deletions without removing giles")
	cleanCmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "List each blob being deleted")

}

var DryRun bool
var Verbose bool

var cleanCmd = &cobra.Command{
	Use:     "clean",
	Short:   "Clean orphaned blobs from the object store",
	Example: "nexio clean",
	Run: func(_ *cobra.Command, args []string) {
		runCleanCommand()
	},
}

func runCleanCommand() {
	initialized := IsInitialized()
	if !initialized {
		Fail(COMMON_RETURN_CODES[001])
		return
	}
	freedBytes, deletedBlobs, failedBlobs := CleanOrphanedBlobs(DryRun, Verbose)
	BreakLine()
	Success(CLEAN_RETURN_CODES[1001])
	BreakLine()
	Text("Freed "+formatSize(freedBytes), "")
	Text("Deleted "+strconv.Itoa(deletedBlobs)+" blobs", "")
	Text("Failed to delete "+strconv.Itoa(failedBlobs)+" blobs", "")
	BreakLine()
}
