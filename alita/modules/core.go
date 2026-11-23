package modules

import (
	"slices"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// moduleStruct is the per-module state every feature module embeds: the name
// help keys and the command manifest derive from, plus the gotgbot handler
// group the module's handlers register into. Help-menu state (which modules
// are enabled, per-module help keyboards) lives in helpRegistry instead.
type moduleStruct struct {
	moduleName        string
	handlerGroup      int
	permHandlerGroup  int
	restrHandlerGroup int
	defaultRulesBtn   string
}

// helpRegistry owns the /help menu's view of the loaded modules: which modules
// appear in it and the per-module help keyboards. The module registry marks a
// module enabled when its loader runs; a module registered with NoHelp stays
// out. It has its own lock because loaders write it while /help reads it.
type helpRegistry struct {
	mu             sync.RWMutex
	enabled        map[string]bool
	helpKbBuilders map[string]func(lang string) [][]gotgbot.InlineKeyboardButton
}

func newHelpRegistry() *helpRegistry {
	return &helpRegistry{
		enabled:        make(map[string]bool),
		helpKbBuilders: make(map[string]func(lang string) [][]gotgbot.InlineKeyboardButton),
	}
}

func DefaultHelpRegistry() *helpRegistry {
	return defaultHelpRegistry
}

var defaultHelpRegistry = newHelpRegistry()

// reset clears the enabled set and help builders, e.g. before a fresh load.
func (r *helpRegistry) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = make(map[string]bool)
	r.helpKbBuilders = make(map[string]func(lang string) [][]gotgbot.InlineKeyboardButton)
}

// setEnabled records whether name appears in the /help menu.
func (r *helpRegistry) setEnabled(name string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if enabled {
		r.enabled[name] = true
		return
	}
	delete(r.enabled, name)
}

// isEnabled reports whether name appears in the /help menu.
func (r *helpRegistry) isEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled[name]
}

// enabledModules lists the enabled module names in sorted order.
func (r *helpRegistry) enabledModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.enabled))
	for name, enabled := range r.enabled {
		if enabled {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// helpKbBuilder returns name's help-keyboard builder, or nil.
func (r *helpRegistry) helpKbBuilder(name string) func(lang string) [][]gotgbot.InlineKeyboardButton {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.helpKbBuilders[name]
}

// ResetHelpRegistry clears the help registry before modules load.
func ResetHelpRegistry() {
	defaultHelpRegistry.reset()
}

// EnabledModules lists the modules the /help menu shows, sorted.
func EnabledModules() []string {
	return defaultHelpRegistry.enabledModules()
}

// SetModuleHelpBuilder records help buttons whose labels follow the viewer's
// language.
func SetModuleHelpBuilder(name string, build func(lang string) [][]gotgbot.InlineKeyboardButton) {
	defaultHelpRegistry.mu.Lock()
	defer defaultHelpRegistry.mu.Unlock()
	defaultHelpRegistry.helpKbBuilders[name] = build
}
