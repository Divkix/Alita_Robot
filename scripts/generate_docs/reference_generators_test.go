package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateLockTypesReferenceWritesPermissionAndRestrictionSections(t *testing.T) {
	tmpDir := t.TempDir()
	previousConfig := config
	config.DryRun = false
	t.Cleanup(func() {
		config = previousConfig
	})

	locks := []LockType{
		{Name: "media", Description: "Blocks media uploads.", Category: "restriction"},
		{Name: "photo", Description: "Blocks photos.", Category: "permission"},
		{Name: "forward", Description: "Blocks forwards.", Category: "permission"},
		{Name: "bots", Description: "Blocks unauthorized bots.", Category: "restriction"},
		{Name: "all", Description: "Blocks all messages.", Category: "restriction"},
	}
	if err := generateLockTypesReference(locks, tmpDir); err != nil {
		t.Fatalf("generateLockTypesReference() error = %v", err)
	}

	content := readGeneratedDoc(t, tmpDir, "api-reference", "lock-types.md")
	for _, want := range []string{
		"Total Lock Types**: 5",
		"Restriction Locks**: 3",
		"| `media` | Blocks media uploads. |",
		"| `photo` | Blocks photos. |",
		"**`forward`**: Blocks forwards.",
		"Blocks unauthorized bots.",
		"Blocks all messages.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("lock types reference missing %q\n%s", want, content)
		}
	}
}

func TestBuildModulesIncludesTranslationAndExtendedDocsOnlyModules(t *testing.T) {
	modules := buildModules(
		map[string]string{
			"admin_help_msg": "Admin help.",
		},
		map[string]ExtendedDocs{
			"admin":   {Features: "Admin features."},
			"filters": {Extended: "Filters docs."},
		},
		[]Command{
			{Name: "promote", Module: "Admin"},
			{Name: "filter", Module: "filters"},
		},
		map[string][]string{
			"Admin": {"admins"},
		},
	)

	if len(modules) != 2 {
		t.Fatalf("buildModules() returned %d modules, want 2", len(modules))
	}
	if modules[0].Name != "admin" || modules[0].DisplayName != "Admin" {
		t.Fatalf("first module = %#v, want sorted admin module", modules[0])
	}
	if len(modules[0].Commands) != 1 || modules[0].Commands[0].Name != "promote" {
		t.Fatalf("admin commands = %#v, want promote", modules[0].Commands)
	}
	if len(modules[0].Aliases) != 1 || modules[0].Aliases[0] != "admins" {
		t.Fatalf("admin aliases = %#v, want admins", modules[0].Aliases)
	}
	if modules[1].Name != "filters" || modules[1].HelpText != "" {
		t.Fatalf("second module = %#v, want extended-docs-only filters module", modules[1])
	}
}

func TestGeneratorSmallHelpers(t *testing.T) {
	if got := truncateString("abcdef", 3); got != "abc..." {
		t.Fatalf("truncateString() = %q, want abc...", got)
	}
	if got := truncateString("abc", 3); got != "abc" {
		t.Fatalf("truncateString(short) = %q, want abc", got)
	}
	if got := countLocksByCategory([]LockType{{Category: "permission"}, {Category: "restriction"}}, "permission"); got != 1 {
		t.Fatalf("countLocksByCategory() = %d, want 1", got)
	}
}

func readGeneratedDoc(t *testing.T, root string, pathParts ...string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(append([]string{root}, pathParts...)...))
	if err != nil {
		t.Fatalf("read generated doc: %v", err)
	}
	return string(content)
}
