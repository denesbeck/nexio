package main

import (
	"os"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "Initialize the Nexio version control system",
	Example: "nexio init",
	Args:    cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		Debug("Starting init command")
		runInitCommand()
	},
}

func runInitCommand() {
	if _, err := os.Stat(dirs.Root); !os.IsNotExist(err) {
		Debug("%s", COMMON_RETURN_CODES[003])
		Fail(COMMON_RETURN_CODES[003])
		return
	}

	Debug("Creating staging directories")
	if err := os.MkdirAll(dirs.StagingAdded, os.ModePerm); err != nil {
		Debug("Failed to create staging/added directory")
		MustSucceed(err, "operation failed")
	}
	if err := os.MkdirAll(dirs.StagingModified, os.ModePerm); err != nil {
		Debug("Failed to create staging/modified directory")
		MustSucceed(err, "operation failed")
	}
	if err := os.MkdirAll(dirs.StagingRemoved, os.ModePerm); err != nil {
		Debug("Failed to create staging/removed directory")
		MustSucceed(err, "operation failed")
	}

	Debug("Creating staging logs file")
	f, err := os.Create(dirs.StagingLogs)
	if err != nil {
		Debug("Failed to create staging logs file")
		MustSucceed(err, "operation failed")
	}
	_, err = f.WriteString("[]")
	if err != nil {
		Debug("Failed to write initial staging logs")
		MustSucceed(err, "operation failed")
	}
	f.Close()

	Debug("Creating commits directory")
	if err := os.MkdirAll(dirs.Commits, os.ModePerm); err != nil {
		Debug("Failed to create commits directory")
		MustSucceed(err, "operation failed")
	}

	Debug("Creating default branch directory")
	if err := os.MkdirAll(dirs.DefaultBranch, os.ModePerm); err != nil {
		Debug("Failed to create default branch directory")
		MustSucceed(err, "operation failed")
	}
	Debug("Creating default branch commits file")
	f, err = os.Create(dirs.DefaultBranchCommits)
	if err != nil {
		Debug("Failed to create default branch commits file")
		MustSucceed(err, "operation failed")
	}
	_, err = f.WriteString("[]")
	if err != nil {
		Debug("Failed to write initial default branch commits")
		MustSucceed(err, "operation failed")
	}
	f.Close()

	Debug("Creating branches metadata")
	CreateBranchesMetadata()

	Debug("Creating config file")
	f, err = os.Create(dirs.Config)
	if err != nil {
		Debug("Failed to create config file")
		MustSucceed(err, "operation failed")
	}
	_, err = f.WriteString("{ \"name\": \"\", \"email\": \"\" }")
	if err != nil {
		Debug("Failed to write initial config")
		MustSucceed(err, "operation failed")
	}
	f.Close()

	Debug("Nexio initialized successfully")
	BreakLine()
	Info("Initializing Nexio Repository")
	BreakLine()

	messages := []string{
		"Creating directory structure...",
		"Setting up branches...         ",
		"Creating config file...        "}
	stop := Spinner(messages, false)
	if os.Getenv("NEXIO_ENV") != "test" {
		time.Sleep(1 * time.Second)
	}
	stop()

	BreakLine()
	Success("Repository initialized successfully!")
	BreakLine()

	Text(".nexio created", "  ")
	Text("Default branch: main", "  ")
	Text("User: Not configured", "  ")

	BreakLine()
	List("Next steps:", []string{
		"nexio config set name \"Your Name\"",
		"nexio config set email \"you@example.com\"",
		"nexio add <file>",
		"nexio commit -m \"Initial commit\""}, true)

	BreakLine()
	Text("Learn more: "+Code("nexio --help"), "")
	BreakLine()
}
