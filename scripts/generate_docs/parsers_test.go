package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestParseCommands_MultiCommand(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "blacklists.go")
	content := `package modules

var blacklistsModule = moduleStruct{moduleName: "blacklists"}

func LoadBlacklists(dispatcher) {
	cmdDecorator.MultiCommand(dispatcher, []string{"remallbl", "rmallbl"}, blacklistsModule.rmAllBlacklists)
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

func TestParseCommands_MultiCommand_AllSites(t *testing.T) {
	tmpDir := t.TempDir()

	// Blacklists module
	blFile := filepath.Join(tmpDir, "blacklists.go")
	blContent := `package modules

var blacklistsModule = moduleStruct{moduleName: "blacklists"}

func LoadBlacklists(dispatcher) {
	cmdDecorator.MultiCommand(dispatcher, []string{"remallbl", "rmallbl"}, blacklistsModule.rmAllBlacklists)
}
`
	if err := os.WriteFile(blFile, []byte(blContent), 0644); err != nil {
		t.Fatalf("Failed to write blacklists file: %v", err)
	}

	// Formatting module
	fmtFile := filepath.Join(tmpDir, "formatting.go")
	fmtContent := `package modules

var formattingModule = moduleStruct{moduleName: "formatting"}

func LoadFormatting(dispatcher) {
	cmdDecorator.MultiCommand(dispatcher, []string{"markdownhelp", "formatting"}, formattingModule.markdownHelp)
}
`
	if err := os.WriteFile(fmtFile, []byte(fmtContent), 0644); err != nil {
		t.Fatalf("Failed to write formatting file: %v", err)
	}

	// Notes module
	notesFile := filepath.Join(tmpDir, "notes.go")
	notesContent := `package modules

var notesModule = moduleStruct{moduleName: "notes"}

func LoadNotes(dispatcher) {
	cmdDecorator.MultiCommand(dispatcher, []string{"privnote", "privatenotes"}, notesModule.privNote)
}
`
	if err := os.WriteFile(notesFile, []byte(notesContent), 0644); err != nil {
		t.Fatalf("Failed to write notes file: %v", err)
	}

	// Rules module
	rulesFile := filepath.Join(tmpDir, "rules.go")
	rulesContent := `package modules

var rulesModule = moduleStruct{moduleName: "rules"}

func LoadRules(dispatcher) {
	cmdDecorator.MultiCommand(dispatcher, []string{"resetrules", "clearrules"}, rulesModule.clearRules)
}
`
	if err := os.WriteFile(rulesFile, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("Failed to write rules file: %v", err)
	}

	commands, err := parseCommands(tmpDir)
	if err != nil {
		t.Fatalf("parseCommands returned error: %v", err)
	}

	expectedAliases := []string{
		"remallbl", "rmallbl",
		"markdownhelp", "formatting",
		"privnote", "privatenotes",
		"resetrules", "clearrules",
	}

	foundCmds := make(map[string]bool)
	for _, cmd := range commands {
		foundCmds[cmd.Name] = true
	}

	for _, alias := range expectedAliases {
		if !foundCmds[alias] {
			t.Errorf("Expected command '%s' not found in parsed commands", alias)
		}
	}

	if len(commands) < 8 {
		t.Errorf("Expected at least 8 commands, got %d", len(commands))
	}
}

func TestParseCommands_Mixed(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "bans.go")
	content := `package modules

var bansModule = moduleStruct{moduleName: "bans"}

func LoadBans(dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("ban", bansModule.ban))
	dispatcher.AddHandler(handlers.NewCommand("unban", bansModule.unban))
	cmdDecorator.MultiCommand(dispatcher, []string{"remallbl", "rmallbl"}, bansModule.rmAllBlacklists)
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	commands, err := parseCommands(tmpDir)
	if err != nil {
		t.Fatalf("parseCommands returned error: %v", err)
	}

	foundCmds := make(map[string]bool)
	for _, cmd := range commands {
		foundCmds[cmd.Name] = true
	}

	// NewCommand registrations
	if !foundCmds["ban"] {
		t.Error("Expected NewCommand 'ban' not found")
	}
	if !foundCmds["unban"] {
		t.Error("Expected NewCommand 'unban' not found")
	}

	// MultiCommand registrations
	if !foundCmds["remallbl"] {
		t.Error("Expected MultiCommand 'remallbl' not found")
	}
	if !foundCmds["rmallbl"] {
		t.Error("Expected MultiCommand 'rmallbl' not found")
	}

	if len(commands) < 4 {
		t.Errorf("Expected at least 4 commands (2 NewCommand + 2 MultiCommand), got %d", len(commands))
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

func TestParseMessageWatchers_NegativeHandlerGroup(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "antiflood.go")
	content := `package modules

var antifloodModule = moduleStruct{moduleName: "antiflood"}

func LoadAntiflood(dispatcher) {
	dispatcher.AddHandlerToGroup(handlers.NewMessage(anyFilter, antifloodModule.checkFlood), -1)
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

	if watchers[0].HandlerGroup != -1 {
		t.Errorf("Expected handler group -1, got %d", watchers[0].HandlerGroup)
	}
}

func TestParseMessageWatchers_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "antispam.go")
	content1 := `package modules

var antispamModule = moduleStruct{moduleName: "antispam"}

func LoadAntispam(dispatcher) {
	dispatcher.AddHandlerToGroup(handlers.NewMessage(anyFilter, antispamModule.checkSpam), 5)
}
`
	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	file2 := filepath.Join(tmpDir, "antiflood.go")
	content2 := `package modules

var antifloodModule = moduleStruct{moduleName: "antiflood"}

func LoadAntiflood(dispatcher) {
	dispatcher.AddHandlerToGroup(handlers.NewMessage(anyFilter, antifloodModule.checkFlood), -1)
}
`
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	watchers, err := parseMessageWatchers(tmpDir)
	if err != nil {
		t.Fatalf("parseMessageWatchers returned error: %v", err)
	}

	if len(watchers) != 2 {
		t.Fatalf("Expected 2 watchers from 2 files, got %d", len(watchers))
	}

	// Sort by handler name for deterministic comparison
	sort.Slice(watchers, func(i, j int) bool {
		return watchers[i].Handler < watchers[j].Handler
	})

	if watchers[0].Handler != "checkFlood" {
		t.Errorf("Expected first watcher handler 'checkFlood', got '%s'", watchers[0].Handler)
	}
	if watchers[1].Handler != "checkSpam" {
		t.Errorf("Expected second watcher handler 'checkSpam', got '%s'", watchers[1].Handler)
	}
}

func TestParseMessageWatchers_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	watchers, err := parseMessageWatchers(tmpDir)
	if err != nil {
		t.Fatalf("parseMessageWatchers returned error for empty dir: %v", err)
	}

	if len(watchers) != 0 {
		t.Errorf("Expected 0 watchers for empty dir, got %d", len(watchers))
	}
}

func TestCamelToScreamingSnake(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BotToken", "BOT_TOKEN"},
		{"DatabaseURL", "DATABASE_URL"},
		{"DBMaxIdleConns", "DB_MAX_IDLE_CONNS"},
		{"DBConnMaxIdleTimeMin", "DB_CONN_MAX_IDLE_TIME_MIN"},
		{"BatchRequestTimeoutMS", "BATCH_REQUEST_TIMEOUT_MS"},
		{"EnableHTTPConnectionPooling", "ENABLE_HTTP_CONNECTION_POOLING"},
		{"HTTPPort", "HTTP_PORT"},
		{"EnablePPROF", "ENABLE_PPROF"},
		{"RedisAddress", "REDIS_ADDRESS"},
		{"UseWebhooks", "USE_WEBHOOKS"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := camelToScreamingSnake(tt.input)
			if got != tt.expected {
				t.Errorf("camelToScreamingSnake(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseMessageWatchers_NoWatchers(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "bans.go")
	content := `package modules

var bansModule = moduleStruct{moduleName: "bans"}

func LoadBans(dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("ban", bansModule.ban))
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	watchers, err := parseMessageWatchers(tmpDir)
	if err != nil {
		t.Fatalf("parseMessageWatchers returned error: %v", err)
	}

	if len(watchers) != 0 {
		t.Errorf("Expected 0 watchers (file has commands, not watchers), got %d", len(watchers))
	}
}
