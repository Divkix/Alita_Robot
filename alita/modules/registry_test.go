package modules

import (
	"reflect"
	"slices"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type testRegistryModule struct {
	name     string
	priority int
	load     func()
}

func (m testRegistryModule) Name() string {
	return m.name
}

func (m testRegistryModule) Priority() int {
	return m.priority
}

func (m testRegistryModule) Load(_ *ext.Dispatcher) {
	if m.load != nil {
		m.load()
	}
}

func withIsolatedRegistry(t *testing.T) {
	t.Helper()

	original := registry
	registry = nil
	t.Cleanup(func() {
		registry = original
	})
}

func TestLoadAllModulesUsesPriorityOrder(t *testing.T) {
	withIsolatedRegistry(t)

	var loaded []string
	RegisterModule(testRegistryModule{name: "third", priority: 30, load: func() {
		loaded = append(loaded, "third")
	}})
	RegisterModule(testRegistryModule{name: "first", priority: 10, load: func() {
		loaded = append(loaded, "first")
	}})
	RegisterModule(testRegistryModule{name: "second", priority: 20, load: func() {
		loaded = append(loaded, "second")
	}})

	LoadAllModules(nil)

	want := []string{"first", "second", "third"}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("LoadAllModules load order = %v, want %v", loaded, want)
	}
}

func TestRegisterModuleIgnoresDuplicateNames(t *testing.T) {
	withIsolatedRegistry(t)

	var loaded []string
	RegisterModule(testRegistryModule{name: "duplicate", priority: 20, load: func() {
		loaded = append(loaded, "first")
	}})
	RegisterModule(testRegistryModule{name: "duplicate", priority: 10, load: func() {
		loaded = append(loaded, "second")
	}})

	LoadAllModules(nil)

	want := []string{"first"}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("LoadAllModules after duplicate registration = %v, want %v", loaded, want)
	}
}

func TestRegisterLegacyModuleAdaptsLoader(t *testing.T) {
	withIsolatedRegistry(t)

	called := false
	RegisterLegacyModule("legacy", 1, func(_ *ext.Dispatcher) {
		called = true
	})

	LoadAllModules(nil)

	if !called {
		t.Fatal("RegisterLegacyModule did not adapt and load the legacy function")
	}
}

type recordingHandler struct {
	name   string
	record func(string)
}

func (h recordingHandler) CheckUpdate(_ *gotgbot.Bot, _ *ext.Context) bool {
	return true
}

func (h recordingHandler) HandleUpdate(_ *gotgbot.Bot, _ *ext.Context) error {
	h.record(h.name)
	return nil
}

func (h recordingHandler) Name() string {
	return h.name
}

func TestRegisteredModulesInstallGotgbotHandlerGroups(t *testing.T) {
	withIsolatedRegistry(t)

	var handled []string
	record := func(name string) {
		handled = append(handled, name)
	}

	RegisterLegacyModule("late", 20, func(dispatcher *ext.Dispatcher) {
		dispatcher.AddHandlerToGroup(recordingHandler{name: "group-20", record: record}, 20)
	})
	RegisterLegacyModule("early", 10, func(dispatcher *ext.Dispatcher) {
		dispatcher.AddHandlerToGroup(recordingHandler{name: "group-10", record: record}, 10)
	})

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAllModules(dispatcher)

	if err := dispatcher.ProcessUpdate(&gotgbot.Bot{}, &gotgbot.Update{}, nil); err != nil {
		t.Fatalf("ProcessUpdate returned error: %v", err)
	}

	want := []string{"group-10", "group-20"}
	if !reflect.DeepEqual(handled, want) {
		t.Fatalf("gotgbot handler execution order = %v, want %v", handled, want)
	}
}

func TestDefaultRegistryIncludesEveryRuntimeModule(t *testing.T) {
	got := make([]string, 0, len(registry))
	for _, module := range registry {
		got = append(got, module.Name())
	}
	slices.Sort(got)

	want := []string{
		"Admin",
		"AntiRaid",
		"Antiflood",
		"Antispam",
		"Approvals",
		"Backup",
		"Bans",
		"Blacklists",
		"BotUpdates",
		"Captcha",
		"Connections",
		"Dev",
		"Disabling",
		"Filters",
		"Formatting",
		"Greetings",
		"Languages",
		"Locks",
		"Misc",
		"Mutes",
		"Notes",
		"Pins",
		"Purges",
		"Reactions",
		"Reports",
		"Rules",
		"Users",
		"Warns",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default registry modules = %v, want %v", got, want)
	}
}

func TestDefaultRegistryLoadsRuntimeModules(t *testing.T) {
	originalHelpRegistry := defaultHelpRegistry
	defaultHelpRegistry = NewHelpRegistry()
	defaultHelpRegistry.AbleMap.Init()
	t.Cleanup(func() {
		defaultHelpRegistry = originalHelpRegistry
	})

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAllModules(dispatcher)

	loadedModules := defaultHelpRegistry.AbleMap.LoadModules()
	slices.Sort(loadedModules)
	want := []string{
		"Admin",
		"AntiRaid",
		"Antiflood",
		"Approvals",
		"Backup",
		"Bans",
		"Blacklists",
		"Captcha",
		"Connections",
		"Disabling",
		"Filters",
		"Formatting",
		"Greetings",
		"Languages",
		"Locks",
		"Misc",
		"Mutes",
		"Notes",
		"Pins",
		"Purges",
		"Reactions",
		"Reports",
		"Rules",
		"Warns",
	}
	if !reflect.DeepEqual(loadedModules, want) {
		t.Fatalf("loaded modules = %v, want %v", loadedModules, want)
	}
}
