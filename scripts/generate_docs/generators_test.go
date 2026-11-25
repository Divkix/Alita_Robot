package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateModuleDocs_KeepsHandWrittenPage(t *testing.T) {
	tmpDir := t.TempDir()
	moduleDir := filepath.Join(tmpDir, "commands", "admin")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("Failed to create module dir: %v", err)
	}

	originalContent := "---\ntitle: Test\n---\n\n# Hand-crafted content"
	moduleFile := filepath.Join(moduleDir, "index.md")
	if err := os.WriteFile(moduleFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to write hand-written file: %v", err)
	}

	modules := []Module{{Name: "admin", DisplayName: "Admin", HelpText: "some help"}}
	if err := generateModuleDocs(modules, tmpDir); err != nil {
		t.Fatalf("generateModuleDocs returned error: %v", err)
	}

	got, err := os.ReadFile(moduleFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(got) != originalContent {
		t.Errorf("Allowlisted file was overwritten.\nExpected: %q\nGot: %q", originalContent, string(got))
	}
}

func TestGenerateModuleDocs_OverwritesPageNotAllowlisted(t *testing.T) {
	tmpDir := t.TempDir()
	moduleDir := filepath.Join(tmpDir, "commands", "users")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("Failed to create module dir: %v", err)
	}

	// The old opt-out comment must not freeze a page that is not in handWrittenPages.
	originalContent := "---\ntitle: Users\n---\n<!-- MANUALLY MAINTAINED: do not regenerate -->\n\n# Stale content"
	moduleFile := filepath.Join(moduleDir, "index.md")
	if err := os.WriteFile(moduleFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to write stale file: %v", err)
	}

	modules := []Module{{Name: "users", DisplayName: "Users", HelpText: "some help"}}
	if err := generateModuleDocs(modules, tmpDir); err != nil {
		t.Fatalf("generateModuleDocs returned error: %v", err)
	}

	got, err := os.ReadFile(moduleFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(got) == originalContent || strings.Contains(string(got), "MANUALLY MAINTAINED") {
		t.Errorf("Non-allowlisted file was not regenerated:\n%s", string(got))
	}
}

func TestGenerateModuleDocs_SkipsOnlyAllowlistedPages(t *testing.T) {
	tmpDir := t.TempDir()

	protectedDir := filepath.Join(tmpDir, "commands", "admin")
	if err := os.MkdirAll(protectedDir, 0755); err != nil {
		t.Fatalf("Failed to create protected dir: %v", err)
	}

	protectedContent := "---\ntitle: Protected\n---\n\n# Do not overwrite"
	protectedFile := filepath.Join(protectedDir, "index.md")
	if err := os.WriteFile(protectedFile, []byte(protectedContent), 0644); err != nil {
		t.Fatalf("Failed to write protected file: %v", err)
	}

	// Allowlisted first: the skip must continue rather than abort generation.
	modules := []Module{
		{Name: "admin", DisplayName: "Admin", HelpText: "admin help"},
		{Name: "users", DisplayName: "Users", HelpText: "users help"},
	}

	if err := generateModuleDocs(modules, tmpDir); err != nil {
		t.Fatalf("generateModuleDocs returned error: %v", err)
	}

	got, err := os.ReadFile(protectedFile)
	if err != nil {
		t.Fatalf("Failed to read protected file: %v", err)
	}
	if string(got) != protectedContent {
		t.Errorf("Protected file was overwritten")
	}

	unprotectedFile := filepath.Join(tmpDir, "commands", "users", "index.md")
	if _, err := os.Stat(unprotectedFile); os.IsNotExist(err) {
		t.Error("Unprotected file was NOT created — likely used return nil instead of continue")
	}
}

func writeCommandPage(t *testing.T, root, module, content string) {
	t.Helper()
	dir := filepath.Join(root, "commands", module)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write page: %v", err)
	}
}

func TestVerifyHandWrittenCommandCoverage_Passes(t *testing.T) {
	root := t.TempDir()
	writeCommandPage(t, root, "demo", "# Demo\n\n| Command | Description | Disableable |\n|---------|-------------|-------------|\n| `/demo` | Does a thing (alias `/demoalias`) | ✅ |\n")
	commands := []Command{{Name: "demo", Aliases: []string{"demoalias"}, Module: "demo"}}
	pages := map[string]string{"commands/demo/index.md": "covered by test"}

	if err := verifyHandWrittenCommandCoverage(root, commands, pages); err != nil {
		t.Fatalf("verifyHandWrittenCommandCoverage returned error: %v", err)
	}
}

func TestVerifyHandWrittenCommandCoverage_MissingCommandAndAlias(t *testing.T) {
	root := t.TempDir()
	writeCommandPage(t, root, "demo", "# Demo\n\nNo command table yet.\n")
	commands := []Command{{Name: "demo", Aliases: []string{"demoalias"}, Module: "demo"}}
	pages := map[string]string{"commands/demo/index.md": "covered by test"}

	err := verifyHandWrittenCommandCoverage(root, commands, pages)
	if err == nil {
		t.Fatal("expected an error for a page missing its commands")
	}
	for _, want := range []string{"/demo", "/demoalias"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestVerifyHandWrittenCommandCoverage_FlagsRemovedCommand(t *testing.T) {
	root := t.TempDir()
	writeCommandPage(t, root, "demo", "# Demo\n\nUse /demo or /demoalias.\n\n| Command | Description |\n|---------|-------------|\n| `/removed` | No module registers this |\n")
	commands := []Command{{Name: "demo", Aliases: []string{"demoalias"}, Module: "demo"}}
	pages := map[string]string{"commands/demo/index.md": "covered by test"}

	err := verifyHandWrittenCommandCoverage(root, commands, pages)
	if err == nil {
		t.Fatal("expected an error for a command row that no longer exists")
	}
	if !strings.Contains(err.Error(), "/removed") {
		t.Errorf("error %q does not mention /removed", err)
	}
}

func TestVerifyHandWrittenCommandCoverage_FlagsMissingAllowlistedPage(t *testing.T) {
	pages := map[string]string{"commands/gone/index.md": "missing on disk"}
	if err := verifyHandWrittenCommandCoverage(t.TempDir(), nil, pages); err == nil {
		t.Fatal("expected an error for an allowlisted page that does not exist")
	}
}

func TestVerifyHandWrittenCommandCoverage_AllowsCommandlessModule(t *testing.T) {
	root := t.TempDir()
	writeCommandPage(t, root, "bot_updates", "# Bot Updates\n\nNo user-facing commands.\n")
	pages := map[string]string{"commands/bot_updates/index.md": "no registered commands"}

	if err := verifyHandWrittenCommandCoverage(root, nil, pages); err != nil {
		t.Fatalf("verifyHandWrittenCommandCoverage returned error: %v", err)
	}
}

func TestGeneratePermissionsSection_DigestIsAdminOnly(t *testing.T) {
	got := generatePermissionsSection(Module{Name: "digest"})
	if strings.Contains(got, "available to all users") {
		t.Fatalf("digest permissions = %q, want no all-users claim", got)
	}
	if !strings.Contains(got, "admin permissions") {
		t.Fatalf("digest permissions = %q, want the admin requirement", got)
	}
}

func TestGeneratePermissionsSection_RecurringIsAdminOnly(t *testing.T) {
	got := generatePermissionsSection(Module{Name: "recurring"})
	if strings.Contains(got, "available to all users") {
		t.Fatalf("recurring permissions = %q, want no all-users claim", got)
	}
	if !strings.Contains(got, "admin permissions") {
		t.Fatalf("recurring permissions = %q, want the admin requirement", got)
	}
}

func TestExtractCommandDescription_FalsePositivePrefix(t *testing.T) {
	helpText := "• /banall is a command, or use /ban - Ban a user"
	got := extractCommandDescription("ban", helpText)
	want := "Ban a user"
	if got != want {
		t.Errorf("extractCommandDescription(\"ban\", ...) = %q, want %q", got, want)
	}
}

func TestExtractCommandDescription_DashSeparator(t *testing.T) {
	helpText := "• /export - Export all group settings to a JSON file\n• /import - Restore settings from a backup file"
	got := extractCommandDescription("export", helpText)
	want := "Export all group settings to a JSON file"
	if got != want {
		t.Errorf("extractCommandDescription(\"export\", ...) = %q, want %q", got, want)
	}
}
