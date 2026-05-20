package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCallbacksReferenceWritesGroupedDocs(t *testing.T) {
	tmpDir := t.TempDir()
	previousConfig := config
	config.DryRun = false
	t.Cleanup(func() {
		config = previousConfig
	})

	callbacks := []Callback{
		{Prefix: "restrict.", Handler: "restrictButtonHandler", Module: "bans", SourceFile: "bans.go"},
		{Prefix: "warns.", Handler: "warnsButtonHandler", Module: "warns", SourceFile: "warns.go"},
	}
	if err := generateCallbacksReference(callbacks, tmpDir); err != nil {
		t.Fatalf("generateCallbacksReference() error = %v", err)
	}

	content := readGeneratedDoc(t, tmpDir, "api-reference", "callbacks.md")
	for _, want := range []string{
		"Total Callbacks**: 2",
		"| bans | `restrict.` | restrictButtonHandler |",
		"### Bans",
		"#### `warns.`",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("callbacks reference missing %q\n%s", want, content)
		}
	}
}

func TestGeneratePermissionsReferenceWritesCategoriesAndFallbackDescription(t *testing.T) {
	tmpDir := t.TempDir()
	previousConfig := config
	config.DryRun = false
	t.Cleanup(func() {
		config = previousConfig
	})

	permissions := []PermissionFunc{
		{
			Name:        "IsValidUserId",
			Signature:   "func IsValidUserId(userId int64) bool",
			Parameters:  []string{"userId int64"},
			ReturnType:  "bool",
			Category:    "ID Validation",
			Description: "Checks whether a Telegram user ID can be targeted safely.",
		},
		{
			Name:       "CanBotDelete",
			Signature:  "func CanBotDelete(bot *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, silent bool) bool",
			ReturnType: "bool",
			Category:   "Bot Permission Checks",
		},
	}
	if err := generatePermissionsReference(permissions, tmpDir); err != nil {
		t.Fatalf("generatePermissionsReference() error = %v", err)
	}

	content := readGeneratedDoc(t, tmpDir, "api-reference", "permissions.md")
	for _, want := range []string{
		"Total Functions**: 2",
		"| `CanBotDelete` | `bool` |",
		"ID Validation",
		"`userId int64`",
		"Bot Permission Checks",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("permissions reference missing %q\n%s", want, content)
		}
	}
}

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
	if boolToYesNo(true) != "Yes" || boolToYesNo(false) != "No" {
		t.Fatal("boolToYesNo() did not map booleans to Yes/No")
	}
	if got := truncateString("abcdef", 3); got != "abc..." {
		t.Fatalf("truncateString() = %q, want abc...", got)
	}
	if got := truncateString("abc", 3); got != "abc" {
		t.Fatalf("truncateString(short) = %q, want abc", got)
	}
	if got := toTitleCase("bot permission checks"); got != "Bot Permission Checks" {
		t.Fatalf("toTitleCase() = %q", got)
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
