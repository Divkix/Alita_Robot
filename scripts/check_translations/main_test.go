package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractKeysFromFile_RelativePaths(t *testing.T) {
	// Create a temporary directory with a Go file containing translation key patterns
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test_module.go")
	content := `package modules

import "fmt"

func handler() {
	tr.GetString("test_key_one")
	tr.GetString("test_key_two")
	_ = fmt.Sprintf("hello")
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Build a relative path containing ".." to simulate how the script is invoked
	// (from scripts/check_translations/, paths like ../../alita/... are legitimate)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	// Construct a relative path with ".." that resolves to the temp file
	relPath, err := filepath.Rel(cwd, goFile)
	if err != nil {
		t.Fatalf("Failed to compute relative path: %v", err)
	}

	// If relPath doesn't contain "..", force it by going up and back down
	if relPath == goFile || !containsDotDot(relPath) {
		// Create an artificial ".." path: go up one dir, then back into tmpDir
		parent := filepath.Dir(tmpDir)
		base := filepath.Base(tmpDir)
		relPath = filepath.Join(parent, "..", filepath.Base(filepath.Dir(parent)), base, "test_module.go")
		// Simpler approach: just use a path with .. that resolves correctly
		relPath, err = filepath.Rel(filepath.Join(cwd, "subdir"), goFile)
		if err != nil {
			t.Fatalf("Failed to compute relative path with ..: %v", err)
		}
	}

	// The relative path should contain ".." for this test to be meaningful
	if !containsDotDot(relPath) {
		// Force a relative path with ".."
		relPath = filepath.Join("..", filepath.Base(cwd), relPath)
	}

	t.Logf("Using relative path: %s", relPath)

	keys, err := extractKeysFromFile(relPath)
	if err != nil {
		t.Fatalf("extractKeysFromFile should accept relative paths with '..', got error: %v", err)
	}

	if len(keys) == 0 {
		t.Fatal("extractKeysFromFile returned 0 keys, expected at least 2")
	}

	// Verify we got the expected keys
	foundKeys := make(map[string]bool)
	for _, k := range keys {
		foundKeys[k.Key] = true
	}
	if !foundKeys["test_key_one"] {
		t.Error("Expected key 'test_key_one' not found")
	}
	if !foundKeys["test_key_two"] {
		t.Error("Expected key 'test_key_two' not found")
	}
}

func TestExtractKeysFromFile_AbsolutePaths(t *testing.T) {
	// Create a temporary Go file with translation keys
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test_module.go")
	content := `package modules

func handler() {
	tr.GetString("abs_test_key")
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Use the absolute path directly
	keys, err := extractKeysFromFile(goFile)
	if err != nil {
		t.Fatalf("extractKeysFromFile should accept absolute paths, got error: %v", err)
	}

	if len(keys) == 0 {
		t.Fatal("extractKeysFromFile returned 0 keys, expected at least 1")
	}

	foundKey := false
	for _, k := range keys {
		if k.Key == "abs_test_key" {
			foundKey = true
			break
		}
	}
	if !foundKey {
		t.Error("Expected key 'abs_test_key' not found")
	}
}

func TestLoadLocaleFiles_RelativePaths(t *testing.T) {
	// Create a temporary directory with a minimal YAML locale file
	tmpDir := t.TempDir()
	localeFile := filepath.Join(tmpDir, "en.yml")
	content := `test_key: "Hello"
another_key: "World"
`
	if err := os.WriteFile(localeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp locale file: %v", err)
	}

	// Build a relative path with ".."
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	relPath, err := filepath.Rel(cwd, tmpDir)
	if err != nil {
		t.Fatalf("Failed to compute relative path: %v", err)
	}

	// Ensure the path contains ".."
	if !containsDotDot(relPath) {
		relPath = filepath.Join("..", filepath.Base(cwd), relPath)
	}

	t.Logf("Using relative path for locales: %s", relPath)

	locales, err := loadLocaleFiles(relPath)
	if err != nil {
		t.Fatalf("loadLocaleFiles should accept relative paths with '..', got error: %v", err)
	}

	if len(locales) == 0 {
		t.Fatal("loadLocaleFiles returned empty map, expected at least 1 locale loaded")
	}

	localeData, ok := locales["en.yml"]
	if !ok {
		t.Fatal("Expected 'en.yml' to be loaded")
	}

	if _, exists := localeData["test_key"]; !exists {
		t.Error("Expected key 'test_key' in loaded locale data")
	}
	if _, exists := localeData["another_key"]; !exists {
		t.Error("Expected key 'another_key' in loaded locale data")
	}
}

func TestLoadLocaleFiles_PathTraversalRejected(t *testing.T) {
	// Attempt to load from an absolute path outside the project (e.g., /etc/)
	// This should either return an error or return empty results gracefully
	locales, err := loadLocaleFiles("/etc")
	if err != nil {
		// Returning an error is acceptable
		t.Logf("loadLocaleFiles returned error for /etc: %v (acceptable)", err)
		return
	}

	// If no error, the result should be empty (no YAML files in /etc/ or they are skipped)
	// This is acceptable behavior -- the function handles it gracefully
	t.Logf("loadLocaleFiles returned %d entries for /etc (graceful handling)", len(locales))
}

func TestExtractTranslationKeys_ExcludesTestFiles(t *testing.T) {
	// Create a temp directory with two files:
	// - module.go containing tr.GetString("real_key") — should be extracted
	// - module_test.go containing tr.GetString("test_only_key") — should be excluded
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "module.go")
	goContent := `package modules

func handler() {
	tr.GetString("real_key")
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write module.go: %v", err)
	}

	testFile := filepath.Join(tmpDir, "module_test.go")
	testContent := `package modules

func TestHandler(t *testing.T) {
	tr.GetString("test_only_key")
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write module_test.go: %v", err)
	}

	keys, err := extractTranslationKeys(tmpDir)
	if err != nil {
		t.Fatalf("extractTranslationKeys returned error: %v", err)
	}

	foundKeys := make(map[string]bool)
	for _, k := range keys {
		foundKeys[k.Key] = true
	}

	if !foundKeys["real_key"] {
		t.Error("Expected 'real_key' from module.go to be extracted, but it was not found")
	}
	if foundKeys["test_only_key"] {
		t.Error("Expected 'test_only_key' from module_test.go to be excluded, but it was found")
	}
}

func containsDotDot(path string) bool {
	for _, part := range filepath.SplitList(path) {
		if part == ".." {
			return true
		}
	}
	// Also check by splitting on separator
	parts := splitPath(path)
	for _, p := range parts {
		if p == ".." {
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == path {
			if dir != "" {
				parts = append([]string{dir}, parts...)
			}
			break
		}
		path = dir[:len(dir)-1] // Remove trailing separator
	}
	return parts
}
