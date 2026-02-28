package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "nexio",
	Short: "Nexio (Nexio) is a version control system inspired by Git",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize database for all commands except init and purge
		if cmd.Name() != "init" && cmd.Name() != "purge" && cmd.Parent() != nil {
			if IsInitialized() {
				if err := InitDB(); err != nil {
					Debug("Failed to initialize database: %v", err)
					MustSucceed(err, "failed to open database")
				}
			}
		}
		// For purge: still need DB to be open so we can close it properly
		if cmd.Name() == "purge" && IsInitialized() {
			if err := InitDB(); err != nil {
				Debug("Failed to initialize database for purge: %v", err)
				// Don't fail for purge - we still want to delete the directory
			}
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		CloseDB()
	},
}

func Execute() {
	rootCmd.Execute()
}
