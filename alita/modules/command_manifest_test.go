//go:build testtools

package modules

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// The command manifest is the single source the help module, the /commands
// menu and each command's admin-only flag derive from. This invariant keeps
// it honest: every registered command names its module, every
// en.yml <module>_extend_msg command is registered under the module the
// extend list documents, the menu holds exactly the extend-listed commands,
// and every command whose wrong call answers with its own help is a menu
// command of the same module.
func TestCommandManifestInvariant(t *testing.T) {
	useRealLocales(t)
	withCommandManifest(t)

	manifest := helpers.CommandManifest()
	if len(manifest) == 0 {
		t.Fatal("command manifest is empty")
	}
	for cmd, entry := range manifest {
		if entry.Module == "" {
			t.Errorf("/%s has no module in the command manifest", cmd)
		}
	}

	menu := menuCommands()
	listed := map[string]bool{}
	for module, commands := range extendCommands(t) {
		for _, cmd := range commands {
			listed[cmd] = true
			entry, ok := menu[cmd]
			if !ok {
				t.Errorf("/%s is in en.yml %s_extend_msg but not in the menu", cmd, module)
				continue
			}
			if entry.Module != module {
				t.Errorf("menu puts /%s in %q; en.yml lists it in %s_extend_msg", cmd, entry.Module, module)
			}
		}
	}
	for cmd := range menu {
		if !listed[cmd] {
			t.Errorf("menu has /%s, but no en.yml <module>_extend_msg lists it", cmd)
		}
	}

	for cmd, module := range commandHelpModules() {
		entry, ok := menu[cmd]
		if !ok {
			t.Errorf("/%s answers a wrong call with its own help but is not a menu command", cmd)
			continue
		}
		if entry.Module != module {
			t.Errorf("help puts /%s in %q, menu in %q", cmd, module, entry.Module)
		}
	}
}
