package main

import (
	"os"
	"testing"
)

func Test_Init(t *testing.T) {
	os.RemoveAll(namespace)

	runInitCommand()
	for _, dir := range GetDirs() {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s not created", dir)
		}
	}
	os.RemoveAll(namespace)
}

func Test_IsInitialized(t *testing.T) {
	os.RemoveAll(namespace)

	if IsInitialized() {
		t.Errorf("Nexio initialized")
	}

	runInitCommand()
	if !IsInitialized() {
		t.Errorf("Nexio not initialized")
	}

	os.RemoveAll(namespace)
}

func Test_GetFiles(t *testing.T) {
	files := GetFiles()

	// Should return a non-empty list of file paths
	if len(files) == 0 {
		t.Error("Expected GetFiles to return at least one file path")
	}

	// All returned paths should be for files (based on dirs configuration)
	for _, file := range files {
		// Each path should be non-empty
		if file == "" {
			t.Error("GetFiles returned an empty path")
		}
	}

	// Should include known files like staging logs and config
	foundStagingLogs := false
	foundConfig := false
	for _, file := range files {
		if strContains(file, "logs.json") {
			foundStagingLogs = true
		}
		if strContains(file, "config.json") {
			foundConfig = true
		}
	}

	if !foundStagingLogs {
		t.Error("Expected GetFiles to include staging logs file")
	}
	if !foundConfig {
		t.Error("Expected GetFiles to include config file")
	}
}

func Test_GetDir(t *testing.T) {
	// Test known directory IDs
	tests := []struct {
		id       string
		contains string
	}{
		{"root", ".nexio"},
		{"objects", "objects"},
		{"staging", "staging"},
		{"commits", "commits"},
		{"branches", "branches"},
		{"config", "config.json"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			path := GetDir(tt.id)
			if path == "" {
				t.Errorf("GetDir(%q) returned empty string", tt.id)
			}
			if !strContains(path, tt.contains) {
				t.Errorf("GetDir(%q) = %q, expected to contain %q", tt.id, path, tt.contains)
			}
		})
	}

	// Test unknown ID returns empty string
	unknownPath := GetDir("nonexistent_id")
	if unknownPath != "" {
		t.Errorf("GetDir with unknown ID should return empty string, got %q", unknownPath)
	}
}

func Test_GetRoot(t *testing.T) {
	root := GetRoot()
	if root == "" {
		t.Error("GetRoot should return non-empty string")
	}
	if !strContains(root, ".nexio") {
		t.Errorf("GetRoot should contain .nexio, got %q", root)
	}
}

// Helper function for string contains check
func strContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
