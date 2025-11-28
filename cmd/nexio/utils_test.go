package main

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
	"time"
)

func Test_ParsePath(t *testing.T) {
	tests := []struct {
		fullPath         string
		expectedPath     string
		expectedFileName string
	}{
		// Existing tests
		{"/path/to/file.txt", "/path/to/", "file.txt"},
		{"file.txt", "", "file.txt"},
		{"/path/to/another/file.txt", "/path/to/another/", "file.txt"},

		// Edge case: empty path (prevents panic)
		{"", "", ""},

		// Trailing slashes
		{"path/to/file.txt/", "path/to/", "file.txt"},
		{"/path/to/file.txt/", "/path/to/", "file.txt"},
		{"file.txt/", "", "file.txt"},

		// Windows paths (backslash separators)
		{"path\\to\\file.txt", "path/to/", "file.txt"},
		{"C:\\path\\to\\file.txt", "C:/path/to/", "file.txt"},
		{"\\path\\to\\file.txt", "/path/to/", "file.txt"},

		// Mixed separators
		{"path/to\\file.txt", "path/to/", "file.txt"},

		// Relative paths
		{"./file.txt", "", "file.txt"},
		{"./path/to/file.txt", "path/to/", "file.txt"},

		// Multiple trailing slashes
		{"path/to/file.txt///", "path/to/", "file.txt"},
	}
	for _, test := range tests {
		path, fileName := ParsePath(test.fullPath)
		if path != test.expectedPath {
			t.Errorf("Input: '%s' - Expected path '%s', but got '%s'", test.fullPath, test.expectedPath, path)
		}
		if fileName != test.expectedFileName {
			t.Errorf("Input: '%s' - Expected file name '%s', but got '%s'", test.fullPath, test.expectedFileName, fileName)
		}
	}
}

func Test_FindIndex(t *testing.T) {
	tests := []struct {
		arr      []string
		val      string
		expected int
	}{
		{[]string{"a", "b", "c"}, "b", 1},
		{[]string{"x", "y", "z"}, "a", -1},
		{[]string{"1", "2", "3"}, "3", 2},
	}

	for _, test := range tests {
		index := FindIndex(test.arr, test.val)
		if index != test.expected {
			t.Errorf("Expected index %d, but got %d", test.expected, index)
		}
	}
}

func Test_IsValidBranchName(t *testing.T) {
	tests := []struct {
		branchName string
		expected   bool
	}{
		{"feature/awesome-feature", true},
		{"bugfix-issue-123", true},
		{"hotfix_urgent_fix", true},
		{"invalid branch name", false},
		{"", false},
		{"-invalid-start", false},
		{"_invalid_start", false},
		{"invalid--name--", false},
		{"invalid__name__", false},
		{"invalid//name", false},
		{"invalid@%#&", false},
	}

	for _, test := range tests {
		result := IsValidBranchName(test.branchName)
		if result != test.expected {
			t.Errorf("Expected %v for branch name '%s', but got %v", test.expected, test.branchName, result)
		}
	}
}

func Test_Capitalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"WORLD", "World"},
		{"gO", "Go"},
		{"test", "Test"},
		{"USERNAME", "Username"},
		{"email", "Email"},
		{"a", "A"},
		{"Z", "Z"},
		{"mIxEd", "Mixed"},
	}
	for _, test := range tests {
		result := Capitalize(test.input)
		if result != test.expected {
			t.Errorf("Expected '%s' for input '%s', but got '%s'", test.expected, test.input, result)
		}
	}
}

func Test_TimeAgo(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		timestamp string
		expected  string
	}{
		// Just now (< 60 seconds)
		{
			name:      "just now - 30 seconds ago",
			timestamp: now.Add(-30 * time.Second).Format(time.RFC3339),
			expected:  "just now",
		},
		{
			name:      "just now - 59 seconds ago",
			timestamp: now.Add(-59 * time.Second).Format(time.RFC3339),
			expected:  "just now",
		},

		// Minutes
		{
			name:      "1 minute ago",
			timestamp: now.Add(-1 * time.Minute).Format(time.RFC3339),
			expected:  "1 minute ago",
		},
		{
			name:      "5 minutes ago",
			timestamp: now.Add(-5 * time.Minute).Format(time.RFC3339),
			expected:  "5 minutes ago",
		},
		{
			name:      "45 minutes ago",
			timestamp: now.Add(-45 * time.Minute).Format(time.RFC3339),
			expected:  "45 minutes ago",
		},

		// Hours
		{
			name:      "1 hour ago",
			timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339),
			expected:  "1 hour ago",
		},
		{
			name:      "3 hours ago",
			timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339),
			expected:  "3 hours ago",
		},
		{
			name:      "12 hours ago",
			timestamp: now.Add(-12 * time.Hour).Format(time.RFC3339),
			expected:  "12 hours ago",
		},

		// Days
		{
			name:      "1 day ago",
			timestamp: now.Add(-24 * time.Hour).Format(time.RFC3339),
			expected:  "1 day ago",
		},
		{
			name:      "4 days ago",
			timestamp: now.Add(-4 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "4 days ago",
		},
		{
			name:      "6 days ago",
			timestamp: now.Add(-6 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "6 days ago",
		},

		// Weeks
		{
			name:      "1 week ago",
			timestamp: now.Add(-7 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "1 week ago",
		},
		{
			name:      "2 weeks ago",
			timestamp: now.Add(-14 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "2 weeks ago",
		},
		{
			name:      "3 weeks ago",
			timestamp: now.Add(-21 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "3 weeks ago",
		},

		// Months
		{
			name:      "1 month ago",
			timestamp: now.Add(-30 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "1 month ago",
		},
		{
			name:      "2 months ago",
			timestamp: now.Add(-60 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "2 months ago",
		},
		{
			name:      "6 months ago",
			timestamp: now.Add(-180 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "6 months ago",
		},

		// Years
		{
			name:      "1 year ago",
			timestamp: now.Add(-365 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "1 year ago",
		},
		{
			name:      "2 years ago",
			timestamp: now.Add(-730 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "2 years ago",
		},

		// Invalid timestamp
		{
			name:      "invalid timestamp",
			timestamp: "invalid-timestamp",
			expected:  "N/A",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := TimeAgo(test.timestamp)
			if result != test.expected {
				t.Errorf("Expected '%s', but got '%s'", test.expected, result)
			}
		})
	}
}

func Test_FormatFileCount(t *testing.T) {
	tests := []struct {
		count    int
		expected string
	}{
		{0, "(0)"},
		{1, "(1)"},
		{2, "(2)"},
		{10, "(10)"},
		{100, "(100)"},
	}
	for _, test := range tests {
		result := FormatFileCount(test.count)
		if result != test.expected {
			t.Errorf("Expected '%s' for count %d, but got '%s'", test.expected, test.count, result)
		}
	}
}

func Test_WriteJson(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := namespace + "test_json"
	os.RemoveAll(tempDir)
	defer os.RemoveAll(tempDir)

	// Test writing JSON to a new file
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": []string{"a", "b", "c"},
	}

	filePath := tempDir + "/test.json"
	WriteJson(filePath, testData)

	// Verify the file exists
	if !FileExists(filePath) {
		t.Errorf("Expected file to exist at %s", filePath)
	}

	// Read and verify the content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}

	// Check if content is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		t.Errorf("Failed to unmarshal JSON: %v", err)
	}
}

func Test_GenRandHex(t *testing.T) {
	// Test that it generates hex strings of correct length
	tests := []int{10, 20, 32}

	for _, length := range tests {
		hex := GenRandHex(length)
		expectedStrLen := length * 2 // Each byte becomes 2 hex characters
		if len(hex) != expectedStrLen {
			t.Errorf("Expected hex string of length %d, but got %d", expectedStrLen, len(hex))
		}

		// Verify it's a valid hex string
		matched, _ := regexp.MatchString("^[a-f0-9]+$", hex)
		if !matched {
			t.Errorf("Generated string is not valid hex: %s", hex)
		}
	}

	// Test uniqueness - generate multiple and ensure they're different
	hex1 := GenRandHex(20)
	hex2 := GenRandHex(20)
	if hex1 == hex2 {
		t.Errorf("Generated hex strings should be unique, but got duplicate: %s", hex1)
	}
}

func Test_GetTimestamp(t *testing.T) {
	timestamp := GetTimestamp()

	// Verify it's a valid RFC3339 timestamp
	_, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		t.Errorf("GetTimestamp returned invalid RFC3339 timestamp: %s, error: %v", timestamp, err)
	}

	// Verify timestamp is recent (within last 5 seconds)
	parsedTime, _ := time.Parse(time.RFC3339, timestamp)
	now := time.Now()
	diff := now.Sub(parsedTime)
	if diff > 5*time.Second || diff < 0 {
		t.Errorf("Timestamp is not recent. Difference: %v", diff)
	}
}

func Test_ValidatePath(t *testing.T) {
	// Get the current working directory for testing
	workdir, _ := os.Getwd()

	tests := []struct {
		name        string
		path        string
		shouldError bool
	}{
		{"valid relative path", "test.txt", false},
		{"valid nested path", "path/to/file.txt", false},
		{"valid absolute path in workdir", workdir + "/test.txt", false},
		{"path traversal attempt", "../../../etc/passwd", true},
		{"path traversal with dots", "test/../../../etc/passwd", true},
		{"current directory", ".", false},
		{"current directory explicit", "./test.txt", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePath(test.path)
			if test.shouldError && err == nil {
				t.Errorf("Expected error for path '%s', but got none", test.path)
			}
			if !test.shouldError && err != nil {
				t.Errorf("Expected no error for path '%s', but got: %v", test.path, err)
			}
		})
	}
}

func Test_GetRepositorySize(t *testing.T) {
	os.RemoveAll(namespace)
	runInitCommand()

	// Create some test files to measure
	testFile := namespace + "test_size.txt"
	os.WriteFile(testFile, []byte("test content for size measurement"), 0644)

	// Test GetRepositorySize
	size := GetRepositorySize()

	// Should return a formatted size string
	if size == "" || size == "N/A" {
		t.Errorf("Expected valid size string, got '%s'", size)
	}

	// Size should contain a unit (B, KB, MB, or GB)
	validUnits := []string{"B", "KB", "MB", "GB"}
	hasValidUnit := false
	for _, unit := range validUnits {
		if len(size) >= len(unit) && size[len(size)-len(unit):] == unit {
			hasValidUnit = true
			break
		}
	}
	if !hasValidUnit {
		t.Errorf("Size '%s' should end with a valid unit (B, KB, MB, GB)", size)
	}

	os.RemoveAll(namespace)
}

func Test_formatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "small bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "exactly 1 KB",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "kilobytes",
			bytes:    2048,
			expected: "2.0 KB",
		},
		{
			name:     "fractional kilobytes",
			bytes:    1536,
			expected: "1.5 KB",
		},
		{
			name:     "exactly 1 MB",
			bytes:    1024 * 1024,
			expected: "1.0 MB",
		},
		{
			name:     "megabytes",
			bytes:    5 * 1024 * 1024,
			expected: "5.0 MB",
		},
		{
			name:     "exactly 1 GB",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GB",
		},
		{
			name:     "gigabytes",
			bytes:    2 * 1024 * 1024 * 1024,
			expected: "2.0 GB",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := formatSize(test.bytes)
			if result != test.expected {
				t.Errorf("Expected '%s' for %d bytes, got '%s'", test.expected, test.bytes, result)
			}
		})
	}
}

func Test_IsValidBranchName_EdgeCases(t *testing.T) {
	tests := []struct {
		branchName string
		expected   bool
	}{
		{"/invalid-start", false},     // starts with /
		{"valid/nested/branch", true}, // valid nested
		{"a", true},                   // single char
		{"123", true},                 // numeric
		{"feature-123", true},         // with numbers
		{"feature_123", true},         // underscore with numbers
		{"UPPERCASE", true},           // uppercase
		{"Mixed-Case_Name", true},     // mixed case
		{"ends-with-slash/", false},   // ends with /
	}

	for _, test := range tests {
		result := IsValidBranchName(test.branchName)
		if result != test.expected {
			t.Errorf("Expected %v for branch name '%s', but got %v", test.expected, test.branchName, result)
		}
	}
}
