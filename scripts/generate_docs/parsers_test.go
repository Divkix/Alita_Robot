package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

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
			helpers.RequireGroup(),
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
			RequiredChecks: []helpers.CheckFunc{helpers.RequireGroup()},
			Disableable:    true,
		}
		helpers.WrapCommand(dispatcher, desc, pipelineHandler(cfg.handler))
	}
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "bansettings",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
		},
	}, pipelineHandler(bansModule.banSettings))
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
		{name: "setban", handler: "setBan", disableable: true},
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

func TestParseCommands_ChannelAndPipelineShareOneCommand(t *testing.T) {
	dir := t.TempDir()
	content := `package modules
var logChannelsModule = moduleStruct{moduleName: "LogChannels"}
func LoadLogChannels(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(setLogChannelCommand{handlers.NewCommand("setlog", logChannelsModule.setLogChannel).SetAllowChannel(true)})
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "setlog", PMGroupArg: helpers.PMGroupArgOwn,
	}, pipelineHandler(logChannelsModule.setLog))
}`
	if err := os.WriteFile(filepath.Join(dir, "logchannels.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	commands, err := parseCommands(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].Name != "setlog" || commands[0].Module != "logchannels" || commands[0].Handler != "setLog" || commands[0].Disableable {
		t.Fatalf("commands = %+v, want one non-disableable /setlog from its descriptor", commands)
	}
}

func TestParseCommands_IgnoresLegacyRegistrationsAndManualDisabling(t *testing.T) {
	dir := t.TempDir()
	content := `package modules
var exampleModule = moduleStruct{moduleName: "Example"}
func LoadExample(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "current", Aliases: []string{"alias"},
	}, pipelineHandler(exampleModule.current))
	handlers.NewCommand("direct", exampleModule.direct)
	handlers.NewCommand("wrapped", withPMGroupArg("wrapped", helpers.PMGroupArgOwn, exampleModule.wrapped))
	helpers.MultiCommand(dispatcher, []string{"multi", "multiAlias"}, exampleModule.multi)
	helpers.MultiCommand(dispatcher, []string{"old", "oldAlias"}, withPMGroupArg("old", helpers.PMGroupArgOwn, exampleModule.old))
	for _, name := range []string{"loop", "loopAlias"} {
		handlers.NewCommand(name, withPMGroupArg(name, helpers.PMGroupArgNone, exampleModule.loop))
	}
	helpers.AddCmdToDisableable("current")
	misc.AddCmdToDisableable("alias")
	helpers.AddCmdToDisableable("orphan")
}`
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture := `package modules
func TestFixture() {
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "fixture"}, exampleModule.fixture)
}`
	if err := os.WriteFile(filepath.Join(dir, "example_test.go"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	commands, err := parseCommands(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 {
		t.Fatalf("commands = %+v, want only the descriptor and its alias", commands)
	}
	for i, name := range []string{"alias", "current"} {
		cmd := commands[i]
		if cmd.Name != name || cmd.Handler != "current" || cmd.Module != "example" || cmd.Disableable || !slices.Equal(cmd.Aliases, []string{"current", "alias"}) {
			t.Errorf("command = %+v, want non-disableable %q from the descriptor", cmd, name)
		}
	}
}
