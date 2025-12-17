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
	if IsInitialized() {
		Debug("%s", COMMON_RETURN_CODES[003])
		Fail("%s", COMMON_RETURN_CODES[003])
		return
	}

	BreakLine()
	Info("Initializing Nexio Repository")
	BreakLine()

	Debug("Creating directory structure")
	CreateDirs()

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

	Debug("Nexio initialized successfully")
}
