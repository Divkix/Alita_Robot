package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateModuleDocs_SkipsManuallyMaintainedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	moduleDir := filepath.Join(tmpDir, "commands", "testmodule")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("Failed to create module dir: %v", err)
	}

	originalContent := "---\ntitle: Test\n---\n" + manualMaintenanceSentinel + "\n\n# Hand-crafted content"
	moduleFile := filepath.Join(moduleDir, "index.md")
	if err := os.WriteFile(moduleFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to write sentinel file: %v", err)
	}

	modules := []Module{{Name: "testmodule", DisplayName: "Test", HelpText: "some help"}}
	if err := generateModuleDocs(modules, tmpDir); err != nil {
		t.Fatalf("generateModuleDocs returned error: %v", err)
	}

	got, err := os.ReadFile(moduleFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(got) != originalContent {
		t.Errorf("Sentinel-protected file was overwritten.\nExpected: %q\nGot: %q", originalContent, string(got))
	}
}

func TestGenerateModuleDocs_WritesNonSentinelFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modules := []Module{{Name: "newmodule", DisplayName: "New Module", HelpText: "help text"}}
	if err := generateModuleDocs(modules, tmpDir); err != nil {
		t.Fatalf("generateModuleDocs returned error: %v", err)
	}

	moduleFile := filepath.Join(tmpDir, "commands", "newmodule", "index.md")
	if _, err := os.Stat(moduleFile); os.IsNotExist(err) {
		t.Error("Expected module file to be created but it was not")
	}
}

func TestGenerateModuleDocs_MixedSentinelAndNonSentinel(t *testing.T) {
	tmpDir := t.TempDir()

	// Protected module — has sentinel
	protectedDir := filepath.Join(tmpDir, "commands", "protected")
	if err := os.MkdirAll(protectedDir, 0755); err != nil {
		t.Fatalf("Failed to create protected dir: %v", err)
	}

	protectedContent := "---\ntitle: Protected\n---\n" + manualMaintenanceSentinel + "\n\n# Do not overwrite"
	protectedFile := filepath.Join(protectedDir, "index.md")
	if err := os.WriteFile(protectedFile, []byte(protectedContent), 0644); err != nil {
		t.Fatalf("Failed to write protected file: %v", err)
	}

	// Two modules: one protected, one not
	modules := []Module{
		{Name: "protected", DisplayName: "Protected", HelpText: "protected help"},
		{Name: "unprotected", DisplayName: "Unprotected", HelpText: "unprotected help"},
	}

	if err := generateModuleDocs(modules, tmpDir); err != nil {
		t.Fatalf("generateModuleDocs returned error: %v", err)
	}

	// Protected file must be unchanged
	got, err := os.ReadFile(protectedFile)
	if err != nil {
		t.Fatalf("Failed to read protected file: %v", err)
	}
	if string(got) != protectedContent {
		t.Errorf("Protected file was overwritten")
	}

	// Unprotected file must be created
	unprotectedFile := filepath.Join(tmpDir, "commands", "unprotected", "index.md")
	if _, err := os.Stat(unprotectedFile); os.IsNotExist(err) {
		t.Error("Unprotected file was NOT created — likely used return nil instead of continue")
	}
}
