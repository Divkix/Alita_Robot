package alita

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/modules"
)

func resetHelpRegistryForTest(t *testing.T) {
	t.Helper()

	registry := modules.DefaultHelpRegistry()
	registry.AbleMap.Init()
	registry.AltHelpOptions = make(map[string][]string)
	t.Cleanup(func() {
		registry.AbleMap.Init()
		registry.AltHelpOptions = make(map[string][]string)
	})
}

func TestListModulesSortsEnabledModuleNames(t *testing.T) {
	resetHelpRegistryForTest(t)

	registry := modules.DefaultHelpRegistry()

	registry.AbleMap.Store("Warns", true)
	registry.AbleMap.Store("Admin", true)
	registry.AbleMap.Store("Filters", true)

	if got, want := ListModules(), "[Admin, Filters, Warns]"; got != want {
		t.Fatalf("ListModules() = %q, want %q", got, want)
	}
}

func TestLoadModulesLoadsRegistryAndHelp(t *testing.T) {
	resetHelpRegistryForTest(t)

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadModules(dispatcher)

	for _, moduleName := range []string{"Admin", "Captcha", "Filters", "Greetings", "Warns"} {
		_, enabled := modules.DefaultHelpRegistry().AbleMap.Load(moduleName)
		if !enabled {
			t.Fatalf("%s was not enabled after LoadModules", moduleName)
		}
	}
}
