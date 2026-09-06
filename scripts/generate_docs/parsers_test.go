package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCommands_MultiCommand(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "blacklists.go")
	content := `package modules

var blacklistsModule = moduleStruct{moduleName: "blacklists"}

func LoadBlacklists(dispatcher) {
	helpers.MultiCommand(dispatcher, []string{"remallbl", "rmallbl"}, blacklistsModule.rmAllBlacklists)
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	commands, err := parseCommands(tmpDir)
	if err != nil {
		t.Fatalf("parseCommands returned error: %v", err)
	}

	if len(commands) < 2 {
		t.Fatalf("Expected at least 2 commands (remallbl, rmallbl), got %d", len(commands))
	}

	foundCmds := make(map[string]bool)
	for _, cmd := range commands {
		foundCmds[cmd.Name] = true
		if cmd.Module != "blacklists" {
			t.Errorf("Expected module 'blacklists', got '%s' for command '%s'", cmd.Module, cmd.Name)
		}
	}

	if !foundCmds["remallbl"] {
		t.Error("Expected command 'remallbl' not found")
	}
	if !foundCmds["rmallbl"] {
		t.Error("Expected command 'rmallbl' not found")
	}
}

func TestParseMessageWatchers_Basic(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "antispam.go")
	content := `package modules

var antispamModule = moduleStruct{moduleName: "antispam"}

func LoadAntispam(dispatcher) {
	dispatcher.AddHandlerToGroup(handlers.NewMessage(anyFilter, antispamModule.checkSpam), 5)
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	watchers, err := parseMessageWatchers(tmpDir)
	if err != nil {
		t.Fatalf("parseMessageWatchers returned error: %v", err)
	}

	if len(watchers) != 1 {
		t.Fatalf("Expected 1 watcher, got %d", len(watchers))
	}

	w := watchers[0]
	if w.Handler != "checkSpam" {
		t.Errorf("Expected handler 'checkSpam', got '%s'", w.Handler)
	}
	if w.Module != "antispam" {
		t.Errorf("Expected module 'antispam', got '%s'", w.Module)
	}
	if w.HandlerGroup != 5 {
		t.Errorf("Expected handler group 5, got %d", w.HandlerGroup)
	}
}
