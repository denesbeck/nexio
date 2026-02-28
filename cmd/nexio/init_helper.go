package main

import (
	"os"
)

func IsInitialized() bool {
	Debug("Checking if Nexio is initialized")
	if _, err := os.Stat(GetRoot()); !os.IsNotExist(err) {
		Debug("Nexio is initialized")
		return true
	}
	Debug("Nexio is not initialized")
	return false
}

func CreateDirs() {
	for _, dir := range dirs {
		if !dir.IsFile {
			Debug("Creating %s directory", dir.Name)
			if err := os.MkdirAll(dir.Path, os.ModePerm); err != nil {
				Debug("Failed to create %s directory", dir.Name)
				MustSucceed(err, "operation failed")
			}
		} else {
			Debug("Creating %s file", dir.Name)
			f, err := os.Create(dir.Path)
			if err != nil {
				Debug("Failed to create %s file", dir.Name)
				MustSucceed(err, "operation failed")
			}
			_, err = f.WriteString(dir.Content)
			if err != nil {
				Debug("Failed to write %s file", dir.Name)
				MustSucceed(err, "operation failed")
			}
			f.Close()
		}
	}

	// Initialize SQLite database
	Debug("Initializing SQLite database")
	if err := InitDB(); err != nil {
		Debug("Failed to initialize database")
		MustSucceed(err, "operation failed")
	}

	// Create initial branch
	Debug("Creating initial branch: %s", InitBranch)
	if err := DBCreateBranch(InitBranch, true, true); err != nil {
		Debug("Failed to create initial branch")
		MustSucceed(err, "operation failed")
	}
}
