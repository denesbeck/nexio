package main

import (
	"os"
	"strconv"
	"testing"
)

func Test_History(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	setConfig("name", "testuser")
	setConfig("email", "test@test.com")

	for i := 1; i <= 10; i++ {
		if i > 5 {
			setConfig("name", "testuserX")
			setConfig("email", "testX@test.com")
		}
		fileName := namespace + strconv.Itoa(i) + ".txt"
		os.Create(fileName)
		if i%2 == 0 {
			os.Create(fileName + "a")
			runAddCommand(fileName+"a", false)
		}
		runAddCommand(fileName, false)
		runCommitCommand("Commit " + strconv.Itoa(i))
	}

	statusCode, history := runHistoryCommand()
	if statusCode != 401 {
		t.Errorf("Expected 401, got %d", statusCode)
	}

	if len(history) != 10 {
		t.Errorf("Expected 10 commits, got %d", len(history))
	}

	for i, commit := range history {
		if commit.Message != "Commit "+strconv.Itoa(i+1) {
			t.Errorf("Expected commit message 'Commit %d', got '%s'", i+1, commit.Message)
		}

		expectedAuthor := "testuser <test@test.com>"
		if i >= 5 {
			expectedAuthor = "testuserX <testX@test.com>"
		}

		actualAuthor := commit.AuthorName + " <" + commit.AuthorEmail + ">"

		if actualAuthor != expectedAuthor {
			t.Errorf("Expected author %s, got '%s'", expectedAuthor, actualAuthor)
		}

		if i%2 == 0 && len(commit.Commits) != 1 {
			t.Errorf("Expected 1 file in commit %d, got %d", i+1, len(commit.Commits))
		}
		if i%2 > 0 && len(commit.Commits) != 2 {
			t.Errorf("Expected 2 file in commit %d, got %d", i+1, len(commit.Commits))
		}

	}

	os.RemoveAll(namespace)
}

func Test_History_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	statusCode, history := runHistoryCommand()
	if statusCode != 001 {
		t.Errorf("Expected 001, got %d", statusCode)
	}

	if history != nil {
		t.Errorf("Expected nil history, got %v", history)
	}

	os.RemoveAll(namespace)
}

func Test_History_NoCommits(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	statusCode, history := runHistoryCommand()
	if statusCode != 402 {
		t.Errorf("Expected 402, got %d", statusCode)
	}

	if len(history) != 0 {
		t.Errorf("Expected 0 commits, got %d", len(history))
	}

	os.RemoveAll(namespace)
}
