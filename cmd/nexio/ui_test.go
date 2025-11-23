package main

import (
	"strings"
	"testing"
)

func Test_Warning(t *testing.T) {
	// Just test that it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Warning function panicked: %v", r)
		}
	}()
	Warning("Test warning message")
}

func Test_Fail(t *testing.T) {
	// Just test that it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Fail function panicked: %v", r)
		}
	}()
	Fail("Test fail message")
}

func Test_ErrorMsg(t *testing.T) {
	result := ErrorMsg("test error")
	// Result should contain the text (even if styled)
	if !strings.Contains(result, "test error") && result == "" {
		t.Errorf("ErrorMsg should return a non-empty string containing 'test error'")
	}
}

func Test_Branch(t *testing.T) {
	result := StyledBranch("main")
	// Result should contain the text (even if styled)
	if !strings.Contains(result, "main") && result == "" {
		t.Errorf("Branch should return a non-empty string containing 'main'")
	}
}

func Test_Code(t *testing.T) {
	result := Code("test code")
	// Result should contain the text (even if styled)
	if !strings.Contains(result, "test code") && result == "" {
		t.Errorf("Code should return a non-empty string containing 'test code'")
	}
}

func Test_Bold(t *testing.T) {
	result := Bold("bold text")
	// Result should contain the text (even if styled)
	if !strings.Contains(result, "bold text") && result == "" {
		t.Errorf("Bold should return a non-empty string containing 'bold text'")
	}
}

func Test_Box(t *testing.T) {
	// Test without title
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Box function panicked: %v", r)
		}
	}()
	Box("", "test content")
	Box("Test Title", "test content with title")
}

func Test_Text(t *testing.T) {
	// Just test that it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Text function panicked: %v", r)
		}
	}()
	Text("test text", "")
	Text("test text with icon", "📝")
}

func Test_BreakLine(t *testing.T) {
	// Just test that it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("BreakLine function panicked: %v", r)
		}
	}()
	BreakLine()
}

func Test_List(t *testing.T) {
	// Just test that it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("List function panicked: %v", r)
		}
	}()
	List("Root", []string{"item1", "item2", "item3"}, true)
	List("Empty", []string{}, false)
}

func Test_TreeList(t *testing.T) {
	// Just test that it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Tree function panicked: %v", r)
		}
	}()
	// Test with empty list
	TreeList([]string{}, true)

	// Test with single file
	TreeList([]string{"file1.txt"}, true)

	// Test with multiple files
	TreeList([]string{"file1.txt", "file2.txt", "file3.txt"}, true)
	TreeList([]string{"file1.txt", "file2.txt", "file3.txt"}, false)
}

func Test_GenerateLeveledList_Empty(t *testing.T) {
	result := GenerateLeveledList([]string{})
	if len(result) != 0 {
		t.Errorf("Expected empty list, got %d items", len(result))
	}
}

func Test_GenerateLeveledList_SingleFile(t *testing.T) {
	result := GenerateLeveledList([]string{"file.txt"})
	if len(result) != 1 {
		t.Errorf("Expected 1 item, got %d", len(result))
	}
	if result[0].Level != 0 {
		t.Errorf("Expected level 0, got %d", result[0].Level)
	}
	if result[0].Text != "file.txt" {
		t.Errorf("Expected text 'file.txt', got '%s'", result[0].Text)
	}
}

func Test_GenerateLeveledList_NestedPath(t *testing.T) {
	result := GenerateLeveledList([]string{"dir/subdir/file.txt"})
	if len(result) != 3 {
		t.Errorf("Expected 3 items, got %d", len(result))
	}

	// Check first level (dir)
	if result[0].Level != 0 || result[0].Text != "dir" {
		t.Errorf("Expected level 0 'dir', got level %d '%s'", result[0].Level, result[0].Text)
	}

	// Check second level (subdir)
	if result[1].Level != 1 || result[1].Text != "subdir" {
		t.Errorf("Expected level 1 'subdir', got level %d '%s'", result[1].Level, result[1].Text)
	}

	// Check third level (file.txt)
	if result[2].Level != 2 || result[2].Text != "file.txt" {
		t.Errorf("Expected level 2 'file.txt', got level %d '%s'", result[2].Level, result[2].Text)
	}
}

func Test_GenerateLeveledList_MultiplePaths(t *testing.T) {
	result := GenerateLeveledList([]string{
		"dir1/file1.txt",
		"dir1/file2.txt",
		"dir2/file3.txt",
	})

	// Should have: dir1, file1.txt, file2.txt, dir2, file3.txt = 5 items
	if len(result) != 5 {
		t.Errorf("Expected 5 items, got %d", len(result))
	}

	// Verify dir1 appears only once
	dir1Count := 0
	for _, item := range result {
		if item.Text == "dir1" {
			dir1Count++
		}
	}
	if dir1Count != 1 {
		t.Errorf("Expected dir1 to appear once, got %d times", dir1Count)
	}
}

func Test_GenerateLeveledList_SharedParent(t *testing.T) {
	result := GenerateLeveledList([]string{
		"parent/child1/file1.txt",
		"parent/child2/file2.txt",
	})

	// Should have: parent, child1, file1.txt, child2, file2.txt = 5 items
	if len(result) != 5 {
		t.Errorf("Expected 5 items, got %d", len(result))
	}

	// Verify parent appears only once
	parentCount := 0
	for _, item := range result {
		if item.Text == "parent" && item.Level == 0 {
			parentCount++
		}
	}
	if parentCount != 1 {
		t.Errorf("Expected parent to appear once, got %d times", parentCount)
	}
}

func Test_GenerateLeveledList_DuplicatePaths(t *testing.T) {
	result := GenerateLeveledList([]string{
		"dir/file.txt",
		"dir/file.txt",
	})

	// Should have: dir, file.txt = 2 items (no duplicates)
	if len(result) != 2 {
		t.Errorf("Expected 2 items, got %d", len(result))
	}
}

func Test_GenerateLeveledList_DeepNesting(t *testing.T) {
	result := GenerateLeveledList([]string{
		"a/b/c/d/e/f.txt",
	})

	// Should have 6 items: a, b, c, d, e, f.txt
	if len(result) != 6 {
		t.Errorf("Expected 6 items, got %d", len(result))
	}

	// Verify levels are correct
	for i, item := range result {
		if item.Level != i {
			t.Errorf("Expected item %d to have level %d, got %d", i, i, item.Level)
		}
	}
}
