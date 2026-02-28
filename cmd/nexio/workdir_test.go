package main

import (
	"os"
	"testing"
)

func Test_Workdir(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()
	bFile := namespace + "b.txt"
	os.Create(bFile)
	runStageCommand(bFile, false)
	runCommitCommand("b")
	statusCode, content := runWorkdirCommand()
	if statusCode != 301 {
		t.Errorf("Expected 301, got %d", statusCode)
	}
	if len(content) != 1 {
		t.Errorf("Expected 1 file, got %d", len(content))
	}
	if content[0].Path != bFile {
		t.Errorf("Expected %s, got %s", bFile, content[0].Path)
	}

	aFile := namespace + "a.txt"
	cFile := namespace + "c.txt"
	os.Create(aFile)
	os.Create(cFile)
	runStageCommand(aFile, false)
	runStageCommand(cFile, false)
	runCommitCommand("ac")
	statusCode, content = runWorkdirCommand()
	if statusCode != 301 {
		t.Errorf("Expected 301, got %d", statusCode)
	}
	if len(content) != 3 {
		t.Errorf("Expected 3 files, got %d", len(content))
	}
	if content[0].Path != aFile && content[1].Path != bFile && content[2].Path != cFile {
		t.Errorf("Expected %s, got %s", aFile, content[0].Path)
		t.Errorf("Expected %s, got %s", bFile, content[1].Path)
		t.Errorf("Expected %s, got %s", cFile, content[2].Path)
	}

	os.RemoveAll(namespace)
}

func Test_Workdir_NotInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	statusCode, content := runWorkdirCommand()
	if statusCode != 001 {
		t.Errorf("Expected 001, got %d", statusCode)
	}

	if content != nil {
		t.Errorf("Expected nil content, got %v", content)
	}

	os.RemoveAll(namespace)
}

func Test_Workdir_NoCommits(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()

	statusCode, content := runWorkdirCommand()
	if statusCode != 302 {
		t.Errorf("Expected 302, got %d", statusCode)
	}

	if len(content) != 0 {
		t.Errorf("Expected 0 files, got %d", len(content))
	}

	os.RemoveAll(namespace)
}
