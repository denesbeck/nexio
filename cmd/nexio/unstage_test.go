package main

import (
	"os"
	"testing"
)

func TestUnstage(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	file := namespace + "file.txt"
	os.Create(file)

	runStageCommand(file, false)
	isLogged, logEntry := LogEntryLookup("*", file)
	if !isLogged || logEntry.Op != "ADD" {
		t.Errorf("Expected log entry to be added with ADD operation, got %s", logEntry.Op)
	}

	runUnstageCommand(file)
	isLogged, _ = LogEntryLookup("*", file)
	if isLogged {
		t.Errorf("Expected log entry to be removed")
	}

	os.RemoveAll(namespace)
}
