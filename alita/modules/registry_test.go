package modules

import (
	"reflect"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func withIsolatedRegistry(t *testing.T) {
	t.Helper()

	original := registry
	registry = nil
	t.Cleanup(func() {
		registry = original
	})
}

func TestLoadAllModulesUsesStablePriorityOrder(t *testing.T) {
	withIsolatedRegistry(t)

	var loaded []string
	RegisterLegacyModule("third", 30, func(_ *ext.Dispatcher) {
		loaded = append(loaded, "third")
	})
	RegisterLegacyModule("first", 10, func(_ *ext.Dispatcher) {
		loaded = append(loaded, "first")
	})
	RegisterLegacyModule("second", 10, func(_ *ext.Dispatcher) {
		loaded = append(loaded, "second")
	})

	LoadAllModules(nil)

	want := []string{"first", "second", "third"}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("LoadAllModules load order = %v, want %v", loaded, want)
	}
}

func TestRegisterLegacyModuleIgnoresDuplicateNames(t *testing.T) {
	withIsolatedRegistry(t)

	var loaded []string
	RegisterLegacyModule("duplicate", 20, func(_ *ext.Dispatcher) {
		loaded = append(loaded, "first")
	})
	RegisterLegacyModule("duplicate", 10, func(_ *ext.Dispatcher) {
		loaded = append(loaded, "second")
	})

	LoadAllModules(nil)

	want := []string{"first"}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("LoadAllModules after duplicate registration = %v, want %v", loaded, want)
	}
}

func TestRegisterLegacyModuleLoadsLoader(t *testing.T) {
	withIsolatedRegistry(t)

	called := false
	RegisterLegacyModule("legacy", 1, func(_ *ext.Dispatcher) {
		called = true
	})

	LoadAllModules(nil)

	if !called {
		t.Fatal("RegisterLegacyModule did not call loader")
	}
}

type recordingHandler struct {
	name   string
	err    error
	record func(string)
}

func (h recordingHandler) CheckUpdate(_ *gotgbot.Bot, _ *ext.Context) bool {
	return true
}

func (h recordingHandler) HandleUpdate(_ *gotgbot.Bot, _ *ext.Context) error {
	h.record(h.name)
	return h.err
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

func TestRegisteredModulesRespectGotgbotDefaultGroupSemantics(t *testing.T) {
	withIsolatedRegistry(t)

	var handled []string
	record := func(name string) {
		handled = append(handled, name)
	}

	RegisterLegacyModule("handlers", 10, func(dispatcher *ext.Dispatcher) {
		dispatcher.AddHandlerToGroup(recordingHandler{
			name:   "early",
			err:    ext.ContinueGroups,
			record: record,
		}, -1)
		dispatcher.AddHandlerToGroup(recordingHandler{name: "early-second", record: record}, -1)
		dispatcher.AddHandler(recordingHandler{name: "default", record: record})
		dispatcher.AddHandlerToGroup(recordingHandler{name: "late", record: record}, 1)
	})

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAllModules(dispatcher)

	if err := dispatcher.ProcessUpdate(&gotgbot.Bot{}, &gotgbot.Update{}, nil); err != nil {
		t.Fatalf("ProcessUpdate returned error: %v", err)
	}

	want := []string{"early", "early-second", "default", "late"}
	if !reflect.DeepEqual(handled, want) {
		t.Fatalf("gotgbot default group semantics = %v, want %v", handled, want)
	}
}

func withResetHelpRegistry(t *testing.T) {
	t.Helper()

	defaultHelpRegistry.reset()
	t.Cleanup(defaultHelpRegistry.reset)
}

// The registry owns enabled-state: loading a module marks it in the help
// registry unless it declared NoHelp, and nothing else writes the set.
func TestLoadAllModulesMarksEachHelpModuleEnabled(t *testing.T) {
	withIsolatedRegistry(t)
	withResetHelpRegistry(t)

	RegisterLegacyModule("watched", 10, func(_ *ext.Dispatcher) {}, RegisterLegacyModuleOptions{NoHelp: true})
	RegisterLegacyModule("listed", 20, func(_ *ext.Dispatcher) {})
	RegisterLegacyModule("also-listed", 30, func(_ *ext.Dispatcher) {})

	loaded := LoadAllModules(nil)

	if !reflect.DeepEqual(loaded, []string{"watched", "listed", "also-listed"}) {
		t.Fatalf("LoadAllModules load order = %v", loaded)
	}
	for _, name := range loaded {
		seen := 0
		for _, other := range loaded {
			if other == name {
				seen++
			}
		}
		if seen != 1 {
			t.Fatalf("module %q loaded %d times", name, seen)
		}
	}

	enabled := defaultHelpRegistry.enabledModules()
	if !reflect.DeepEqual(enabled, []string{"also-listed", "listed"}) {
		t.Fatalf("enabled modules = %v, want the two help-listed modules", enabled)
	}
}

// PriorityFinal is the declared way to load after every numbered module; the
// help module uses it.
func TestLoadAllModulesLoadsFinalModuleLast(t *testing.T) {
	withIsolatedRegistry(t)
	withResetHelpRegistry(t)

	var order []string
	RegisterLegacyModule("first", 10, func(_ *ext.Dispatcher) { order = append(order, "first") })
	RegisterLegacyModule("last", PriorityFinal, func(_ *ext.Dispatcher) { order = append(order, "last") }, RegisterLegacyModuleOptions{NoHelp: true})

	loaded := LoadAllModules(nil)

	if !reflect.DeepEqual(order, []string{"first", "last"}) {
		t.Fatalf("load order = %v, want first then last", order)
	}
	if loaded[len(loaded)-1] != "last" {
		t.Fatalf("LoadAllModules last = %q, want last", loaded[len(loaded)-1])
	}
}

// A priority subtraction overflows when PriorityFinal meets a negative
// priority; the comparator must still place the final module last.
func TestLoadAllModulesFinalBeatsNegativePriority(t *testing.T) {
	withIsolatedRegistry(t)

	var order []string
	RegisterLegacyModule("negative", -10, func(_ *ext.Dispatcher) { order = append(order, "negative") })
	RegisterLegacyModule("final", PriorityFinal, func(_ *ext.Dispatcher) { order = append(order, "final") }, RegisterLegacyModuleOptions{NoHelp: true})

	LoadAllModules(nil)

	if !reflect.DeepEqual(order, []string{"negative", "final"}) {
		t.Fatalf("load order = %v, want negative then final", order)
	}
}

// Equal priorities get a deterministic order by name, not init file order.
func TestLoadAllModulesTieBreaksEqualPriorityByName(t *testing.T) {
	withIsolatedRegistry(t)

	var order []string
	RegisterLegacyModule("zeta", 226, func(_ *ext.Dispatcher) { order = append(order, "zeta") })
	RegisterLegacyModule("alpha", 226, func(_ *ext.Dispatcher) { order = append(order, "alpha") })

	LoadAllModules(nil)

	if !reflect.DeepEqual(order, []string{"alpha", "zeta"}) {
		t.Fatalf("load order = %v, want alpha then zeta", order)
	}
}

// The real registry: every module that loads and did not declare NoHelp is
// enabled exactly once, and Help — registered at PriorityFinal — loads last.
func TestLoadAllModulesEnabledSetMatchesLoadedModules(t *testing.T) {
	withResetHelpRegistry(t)

	loaded := LoadAllModules(ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}))

	noHelp := map[string]bool{}
	for _, m := range registry {
		noHelp[m.name] = m.noHelp
	}
	want := map[string]bool{}
	for _, name := range loaded {
		if _, dup := want[name]; dup {
			t.Errorf("module %q loaded more than once", name)
		}
		want[name] = !noHelp[name]
	}
	got := map[string]bool{}
	for _, name := range defaultHelpRegistry.enabledModules() {
		got[name] = true
	}
	for name, enabled := range want {
		if enabled && !got[name] {
			t.Errorf("module %q loaded but not enabled in the help registry", name)
		}
		if !enabled && got[name] {
			t.Errorf("module %q declared NoHelp but is enabled", name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("module %q enabled but never loaded", name)
		}
	}
	if loaded[len(loaded)-1] != "Help" {
		t.Fatalf("last loaded module = %q, want Help", loaded[len(loaded)-1])
	}
}
