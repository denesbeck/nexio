package main

import (
	"os"
	"testing"
)

func Test_patternToRegexp(t *testing.T) {
	tests := []struct {
		pattern     string
		testPath    string
		shouldMatch bool
		description string
	}{
		{"*.txt", "file.txt", true, "simple wildcard should match"},
		{"*.txt", "dir/file.txt", true, "wildcard should match in subdirectory"},
		{"*.go", "main.go", true, "wildcard should match go files"},
		{"*.go", "main.txt", false, "wildcard should not match different extension"},
		{"test*", "test.txt", true, "prefix wildcard should match"},
		{"test*", "testing.go", true, "prefix wildcard should match longer name"},
		{"**/*.txt", "a/b/c/file.txt", true, "double wildcard should match nested paths"},
		{"**/*.go", "src/main.go", true, "double wildcard should match"},
		{"dir/file.txt", "dir/file.txt", true, "exact path should match"},
		{"dir/file.txt", "other/file.txt", false, "exact path should not match different dir"},
		{"node_modules", "node_modules", true, "directory name should match"},
		{"node_modules", "src/node_modules", true, "directory name should match in subdirectory"},
		{"*.log", "app.log", true, "log files should match"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			regex, err := patternToRegexp(test.pattern)
			if err != nil {
				t.Errorf("Failed to compile pattern '%s': %v", test.pattern, err)
				return
			}

			matched := regex.MatchString(test.testPath)
			if matched != test.shouldMatch {
				t.Errorf("Pattern '%s' against path '%s': expected match=%v, got match=%v",
					test.pattern, test.testPath, test.shouldMatch, matched)
			}
		})
	}
}

func Test_ShouldIgnore_NoRulesFile(t *testing.T) {
	// Ensure no rules file exists
	os.Remove(".nexio.rules.yml")

	// Without a rules file, nothing should be ignored
	result := ShouldIgnore("test.txt")
	if result {
		t.Errorf("Expected file to not be ignored when no rules file exists")
	}
}

func Test_ShouldIgnore_WithRules(t *testing.T) {
	// Create a temporary rules file
	rulesContent := `ignore:
  - "*.log"
  - "node_modules"
  - "*.tmp"
allow:
  - "important.log"
`
	err := os.WriteFile(".nexio.rules.yml", []byte(rulesContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test rules file: %v", err)
	}
	defer os.Remove(".nexio.rules.yml")

	tests := []struct {
		path         string
		shouldIgnore bool
		description  string
	}{
		{"test.log", true, "log files should be ignored"},
		{"app.log", true, "log files should be ignored"},
		{"important.log", false, "allowed log should not be ignored"},
		{"node_modules", true, "node_modules should be ignored"},
		{"src/node_modules", true, "node_modules in subdirectory should be ignored"},
		{"test.txt", false, "txt files should not be ignored"},
		{"file.tmp", true, "tmp files should be ignored"},
		{"test.go", false, "go files should not be ignored"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			result := ShouldIgnore(test.path)
			if result != test.shouldIgnore {
				t.Errorf("Path '%s': expected ignore=%v, got ignore=%v",
					test.path, test.shouldIgnore, result)
			}
		})
	}
}

func Test_pathToRegexp_EmptyRules(t *testing.T) {
	// Create a rules file with empty rules
	emptyRules := `ignore: []
allow: []
`
	err := os.WriteFile(".nexio.rules.yml", []byte(emptyRules), 0644)
	if err != nil {
		t.Fatalf("Failed to create test rules file: %v", err)
	}
	defer os.Remove(".nexio.rules.yml")

	ignore, allow, err := pathToRegexp()
	if err != nil {
		t.Errorf("pathToRegexp should not fail with empty rules: %v", err)
	}

	if len(ignore) != 0 {
		t.Errorf("Expected 0 ignore patterns, got %d", len(ignore))
	}

	if len(allow) != 0 {
		t.Errorf("Expected 0 allow patterns, got %d", len(allow))
	}
}

func Test_readRules_ValidFile(t *testing.T) {
	// Create a valid rules file
	rulesContent := `ignore:
  - "*.log"
  - "node_modules"
allow:
  - "important.log"
`
	err := os.WriteFile(".nexio.rules.yml", []byte(rulesContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test rules file: %v", err)
	}
	defer os.Remove(".nexio.rules.yml")

	rules, err := readRules()
	if err != nil {
		t.Errorf("readRules should succeed with valid file: %v", err)
	}

	if rules == nil {
		t.Fatal("Expected non-nil rules")
	}

	if len(rules.Ignore) != 2 {
		t.Errorf("Expected 2 ignore patterns, got %d", len(rules.Ignore))
	}

	if len(rules.Allow) != 1 {
		t.Errorf("Expected 1 allow pattern, got %d", len(rules.Allow))
	}
}

func Test_readRules_NoFile(t *testing.T) {
	// Ensure no rules file exists
	os.Remove(".nexio.rules.yml")

	rules, err := readRules()
	if err == nil {
		t.Errorf("readRules should return error when file doesn't exist")
	}

	if rules != nil {
		t.Errorf("Expected nil rules when file doesn't exist")
	}
}

func Test_ShouldIgnore_EmptyRulesFile(t *testing.T) {
	// Create a rules file with empty rules
	emptyRules := `ignore: []
allow: []
`
	err := os.WriteFile(".nexio.rules.yml", []byte(emptyRules), 0644)
	if err != nil {
		t.Fatalf("Failed to create test rules file: %v", err)
	}
	defer os.Remove(".nexio.rules.yml")

	// With empty rules, nothing should be ignored
	result := ShouldIgnore("test.txt")
	if result {
		t.Errorf("Expected file to not be ignored when rules are empty")
	}
}

func Test_pathToRegexp_WithRegexPatterns(t *testing.T) {
	// Create a rules file with regex patterns (not glob patterns)
	rulesContent := `ignore:
  - "^test.*\\.log$"
allow:
  - "^important.*$"
`
	err := os.WriteFile(".nexio.rules.yml", []byte(rulesContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test rules file: %v", err)
	}
	defer os.Remove(".nexio.rules.yml")

	ignore, allow, err := pathToRegexp()
	if err != nil {
		t.Errorf("pathToRegexp should not fail: %v", err)
	}

	if len(ignore) != 1 {
		t.Errorf("Expected 1 ignore patterns, got %d", len(ignore))
	}

	if len(allow) != 1 {
		t.Errorf("Expected 1 allow pattern, got %d", len(allow))
	}
}
