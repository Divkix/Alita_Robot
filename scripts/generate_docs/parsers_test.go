package main

import (
	"os"
	"path/filepath"
	"slices"
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

// A command wrapped by withPMGroupArg is still registered under its own
// handler and module.
func TestParseCommands_PMGroupArgWrapper(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "logchannels.go")
	content := `package modules

var logChannelsModule = moduleStruct{moduleName: "LogChannels"}

func LoadLogChannels(dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("setlog", withPMGroupArg("setlog", helpers.PMGroupArgOwn, logChannelsModule.setLog)).SetAllowChannel(true))
	dispatcher.AddHandler(handlers.NewCommand("unsetlog", withPMGroupArg("unsetlog", helpers.PMGroupArgNone, logChannelsModule.unsetLog)))
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	commands, err := parseCommands(tmpDir)
	if err != nil {
		t.Fatalf("parseCommands returned error: %v", err)
	}

	handlers := map[string]string{}
	for _, cmd := range commands {
		if cmd.Module != "logchannels" {
			t.Errorf("command %q has module %q, want logchannels", cmd.Name, cmd.Module)
		}
		handlers[cmd.Name] = cmd.Handler
	}
	if handlers["setlog"] != "setLog" || handlers["unsetlog"] != "unsetLog" {
		t.Fatalf("handlers = %v, want setlog→setLog and unsetlog→unsetLog", handlers)
	}
}

// The other forms that register withPMGroupArg commands: the wrapper inside
// helpers.MultiCommand, a loop over names, each registered with
// handlers.NewCommand(cmd, ...), and one handlers.NewCommand per alias, each
// wrapped with the name of the command it stands for. Every name is a command
// of the module, with the names registered with it as aliases.
func TestParseCommands_PMGroupArgMultiAndLoop(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "notes.go")
	content := `package modules

var notesModule = moduleStruct{moduleName: "Notes"}

func LoadNotes(dispatcher) {
	for _, cmd := range []string{"notes", "saved"} {
		dispatcher.AddHandler(handlers.NewCommand(cmd, withPMGroupArg(cmd, helpers.PMGroupArgNone, notesModule.notesList)))
	}
	helpers.MultiCommand(dispatcher, []string{"privnote", "privatenotes"}, withPMGroupArg("privnote", helpers.PMGroupArgOwn, notesModule.privNote))
	dispatcher.AddHandler(handlers.NewCommand("clearall", withPMGroupArg("clearall", helpers.PMGroupArgNone, notesModule.rmAllNotes)))
	dispatcher.AddHandler(handlers.NewCommand("resetnotes", withPMGroupArg("clearall", helpers.PMGroupArgNone, notesModule.rmAllNotes)))
	dispatcher.AddHandler(handlers.NewCommand("get", withPMGroupArg("get", helpers.PMGroupArgNone, notesModule.rmAllNotes)))
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	commands, err := parseCommands(tmpDir)
	if err != nil {
		t.Fatalf("parseCommands returned error: %v", err)
	}

	got := map[string]Command{}
	for _, cmd := range commands {
		got[cmd.Name] = cmd
	}
	for name, want := range map[string]struct {
		handler string
		aliases []string
	}{
		"notes":        {"notesList", []string{"notes", "saved"}},
		"saved":        {"notesList", []string{"notes", "saved"}},
		"privnote":     {"privNote", []string{"privnote", "privatenotes"}},
		"privatenotes": {"privNote", []string{"privnote", "privatenotes"}},
		"clearall":     {"rmAllNotes", []string{"clearall", "resetnotes"}},
		"resetnotes":   {"rmAllNotes", []string{"clearall", "resetnotes"}},
		// Another command on the same handler is not an alias.
		"get": {"rmAllNotes", nil},
	} {
		cmd, ok := got[name]
		if !ok {
			t.Errorf("command %q not found; parsed %v", name, commands)
			continue
		}
		if cmd.Module != "notes" || cmd.Handler != want.handler || !slices.Equal(cmd.Aliases, want.aliases) {
			t.Errorf("command %q = %+v, want module notes, handler %s, aliases %v", name, cmd, want.handler, want.aliases)
		}
	}
	if len(commands) != 7 {
		t.Errorf("parsed %d commands, want 7: %v", len(commands), commands)
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

func TestParseCommands_WrapCommandForms(t *testing.T) {
	tmpDir := t.TempDir()
	content := `package modules

var bansModule = moduleStruct{moduleName: "Bans"}

var (
	// Nested braces in RequiredChecks must not end the descriptor early.
	banDesc = helpers.CommandDescriptor{
		RequiredChecks: []helpers.CheckFunc{
			helpers.CheckDisabled("ban"),
			func(c *helpers.CommandContext) bool { return c != nil }, // don't stop here
		},
		Name:    "ban",
		Aliases: []string{"bean"},
	}
	kickmeDesc = helpers.CommandDescriptor{
		Name:        "kickme",
		Disableable: true,
	}
	listDesc = helpers.CommandDescriptor{Name: "banlist"}
)

func LoadBans(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, banDesc, pipelineHandler(bansModule.ban))
	helpers.WrapCommand(dispatcher, kickmeDesc, pipelineHandler(bansModule.kickme))
	helpers.WrapCommandRaw(dispatcher, listDesc, bansModule.banList)
	for _, cfg := range []struct {
		name    string
		handler func(*gotgbot.Bot, *ext.Context) error
	}{
		{"setban", bansModule.setBan},
		{"resetban", bansModule.resetBan},
	} {
		desc := helpers.CommandDescriptor{
			Name:           cfg.name,
			RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled(cfg.name)},
		}
		helpers.WrapCommand(dispatcher, desc, pipelineHandler(cfg.handler))
	}
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "bansettings",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
		},
	}, pipelineHandler(bansModule.banSettings))
	helpers.AddCmdToDisableable("resetban")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "bans.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	commands, err := parseCommands(tmpDir)
	if err != nil {
		t.Fatalf("parseCommands returned error: %v", err)
	}
	got := make(map[string]Command, len(commands))
	for _, cmd := range commands {
		got[cmd.Name] = cmd
	}

	for _, want := range []struct {
		name, handler string
		disableable   bool
		aliases       []string
	}{
		{name: "ban", handler: "ban", aliases: []string{"ban", "bean"}},
		{name: "bean", handler: "ban", aliases: []string{"ban", "bean"}},
		{name: "kickme", handler: "kickme", disableable: true},
		{name: "banlist", handler: "banList"},
		{name: "setban", handler: "setBan"},
		{name: "resetban", handler: "resetBan", disableable: true},
		{name: "bansettings", handler: "banSettings"},
	} {
		cmd, ok := got[want.name]
		if !ok {
			t.Errorf("command %q not parsed", want.name)
			continue
		}
		if cmd.Handler != want.handler || cmd.Module != "bans" || cmd.Disableable != want.disableable {
			t.Errorf("command %q = %+v, want handler %q, module bans, disableable %v", want.name, cmd, want.handler, want.disableable)
		}
		if !slices.Equal(cmd.Aliases, want.aliases) {
			t.Errorf("command %q aliases = %v, want %v", want.name, cmd.Aliases, want.aliases)
		}
	}
	if len(commands) != 7 {
		t.Errorf("parsed %d commands, want 7: %+v", len(commands), commands)
	}
}
