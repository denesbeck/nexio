package main

import (
	"os"
	"testing"
)

func Test_Purge(t *testing.T) {
	// Clean __test__ namespace
	os.RemoveAll(namespace)

	// Initialize Nexio
	runInitCommand()

	// Check if directories are created
	for _, dir := range GetDirs() {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s not created", dir)
		}
	}

	// Purge Nexio
	runPurgeCommand()

	// Check if directories are purged
	for _, dir := range GetDirs() {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("Directory %s not purged", dir)
		}
	}

	if IsInitialized() {
		t.Errorf("Nexio not purged")
	}

	os.RemoveAll(namespace)
}
