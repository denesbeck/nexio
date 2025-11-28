package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func WriteJson(fullPath string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		MustSucceed(err, "operation failed")
	}
	if _, err = os.Stat(fullPath); os.IsNotExist(err) {
		path, _ := ParsePath(fullPath)
		os.MkdirAll(path, 0700)
	}
	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		MustSucceed(err, "operation failed")
	}
}

func GenRandHex(length int) string {
	Rando := rand.Reader
	b := make([]byte, length)
	_, _ = Rando.Read(b)
	return hex.EncodeToString(b)
}

func ParsePath(fullPath string) (path string, fileName string) {
	// Handle empty path
	if fullPath == "" {
		return "", ""
	}

	// First, convert all backslashes to forward slashes for cross-platform compatibility
	// This handles Windows paths on Unix systems and vice versa
	normalizedPath := strings.ReplaceAll(fullPath, "\\", "/")

	// Clean the path (removes trailing slashes, resolves . and .., etc.)
	cleanPath := filepath.Clean(normalizedPath)

	// Convert back to forward slashes (filepath.Clean might use OS-specific separators)
	cleanPath = filepath.ToSlash(cleanPath)

	// Split by forward slash
	tmpArr := strings.Split(cleanPath, "/")

	// Safety check (shouldn't happen after clean, but defensive)
	if len(tmpArr) == 0 {
		return "", ""
	}

	file := tmpArr[len(tmpArr)-1]

	var dirs string
	if len(tmpArr) > 1 {
		dirs = strings.Join(tmpArr[:len(tmpArr)-1], "/")
		if dirs != "" {
			dirs = dirs + "/"
		}
	}

	return dirs, file
}

// ValidatePath ensures a file path doesn't escape the working directory
func ValidatePath(userPath string) error {
	// Get the working directory
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Clean and make the user path absolute
	cleanPath := filepath.Clean(userPath)
	var absPath string
	if filepath.IsAbs(cleanPath) {
		absPath = cleanPath
	} else {
		absPath = filepath.Join(workdir, cleanPath)
	}

	// Check if the absolute path is within the working directory
	relPath, err := filepath.Rel(workdir, absPath)
	if err != nil {
		return err
	}

	// If the relative path starts with "..", it's trying to escape
	if strings.HasPrefix(relPath, "..") {
		return errors.New("path traversal detected: path escapes working directory")
	}

	return nil
}

func GetTimestamp() string {
	return time.Now().Format(time.RFC3339)
}

func FindIndex(arr []string, val string) int {
	for i, v := range arr {
		if v == val {
			return i
		}
	}
	return -1
}

/*
* Checks if a branch name follows GitHub's naming conventions:
*
* - Can contain alphanumeric characters, hyphens (-), underscores (_), and forward slashes (/)
* - Cannot start with a hyphen, underscore, or forward slash
* - Cannot have consecutive hyphens, underscores, or forward slashes
* - Cannot end with a forward slash
* - Cannot contain control characters or spaces
* - Cannot be empty
 */
func IsValidBranchName(name string) bool {
	Debug("Validating branch name: %s", name)

	// Empty check
	if name == "" {
		Debug("Branch name cannot be empty")
		return false
	}

	// Check if starts with invalid characters
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "_") || strings.HasPrefix(name, "/") {
		Debug("Branch name cannot start with -, _, or /")
		return false
	}

	// Check for consecutive special characters
	if strings.Contains(name, "--") || strings.Contains(name, "__") || strings.Contains(name, "//") {
		Debug("Branch name cannot contain consecutive -, _, or /")
		return false
	}

	// Check if ends with forward slash
	if strings.HasSuffix(name, "/") {
		Debug("Branch name cannot end with /")
		return false
	}

	// Main regex pattern:
	// ^[a-zA-Z0-9] - Must start with alphanumeric
	// [a-zA-Z0-9\-_/]* - Can contain alphanumeric, hyphens, underscores, and forward slashes
	// $ - End of string
	pattern := `^[a-zA-Z0-9][a-zA-Z0-9\-_/]*$`
	matched, err := regexp.MatchString(pattern, name)
	if err != nil {
		Debug("Error validating branch name: %v", err)
		return false
	}

	Debug("Branch name validation result: %v", matched)
	return matched
}

func Capitalize(text string) string {
	return strings.ToUpper(text[:1]) + strings.ToLower(text[1:])
}

func FormatFileCount(count int) string {
	if count == 1 {
		return "(1)"
	}
	return fmt.Sprintf("(%d)", count)
}

// TimeAgo converts a timestamp to a human-readable "time ago" format
// Examples: "just now", "2 minutes ago", "3 hours ago", "4 days ago", "2 weeks ago"
func TimeAgo(timestamp string) string {
	// Parse the RFC3339 timestamp
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "N/A"
	}
	// Calculate the difference
	duration := time.Since(t)
	// Convert to different units
	seconds := int(duration.Seconds())
	minutes := int(duration.Minutes())
	hours := int(duration.Hours())
	days := hours / 24
	weeks := days / 7
	months := days / 30
	years := days / 365
	switch {
	case seconds < 60:
		return "just now"
	case minutes < 2:
		return "1 minute ago"
	case minutes < 60:
		return fmt.Sprintf("%d minutes ago", minutes)
	case hours < 2:
		return "1 hour ago"
	case hours < 24:
		return fmt.Sprintf("%d hours ago", hours)
	case days < 2:
		return "1 day ago"
	case days < 7:
		return fmt.Sprintf("%d days ago", days)
	case weeks < 2:
		return "1 week ago"
	case weeks < 4:
		return fmt.Sprintf("%d weeks ago", weeks)
	case months < 2:
		return "1 month ago"
	case months < 12:
		return fmt.Sprintf("%d months ago", months)
	case years < 2:
		return "1 year ago"
	default:
		return fmt.Sprintf("%d years ago", years)
	}
}

func GetRepositorySize() string {
	var totalSize int64

	err := filepath.Walk(dirs.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking even if there's an error
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return "N/A"
	}

	return formatSize(totalSize)
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
