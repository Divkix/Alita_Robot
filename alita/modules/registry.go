package modules

import (
	"cmp"
	"math"
	"slices"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// PriorityFinal sorts a module after every numbered priority; the help module
// uses it so /help, /start and /commands register last.
const PriorityFinal = math.MaxInt

type registeredModule struct {
	name     string
	priority int
	load     func(*ext.Dispatcher)
	// noHelp keeps the module out of the /help menu: watcher-only modules and
	// Dev declare it instead of skipping an enable call inside the loader.
	noHelp bool
}

// RegisterLegacyModuleOptions declares how a module deviates from the
// default: listed in /help and enabled when its loader runs.
type RegisterLegacyModuleOptions struct {
	NoHelp bool
}

// registry holds all registered modules in insertion order.
var registry []registeredModule

// loadModule runs one module's loader with the command manifest pointed at
// it, so every WrapCommand inside records name as the owning module, and
// marks the module enabled in the help registry unless it declared NoHelp.
func loadModule(m registeredModule, dispatcher *ext.Dispatcher) {
	previous := helpers.CurrentCommandModule()
	helpers.SetCommandModule(m.name)
	defer helpers.SetCommandModule(previous)
	if !m.noHelp {
		defaultHelpRegistry.setEnabled(m.name, true)
	}
	if m.load != nil {
		m.load(dispatcher)
	}
}

// RegisterLegacyModule registers a module loader.
// Duplicate registrations are silently ignored to prevent double-loading handlers.
// init-only, not safe for concurrent Register.
func RegisterLegacyModule(name string, priority int, load func(*ext.Dispatcher), opts ...RegisterLegacyModuleOptions) {
	for _, existing := range registry {
		if existing.name == name {
			log.Debugf("Duplicate module registration ignored: %s", name)
			return
		}
	}
	var opt RegisterLegacyModuleOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	registry = append(registry, registeredModule{name: name, priority: priority, load: load, noHelp: opt.NoHelp})
	log.Debugf("Registered module: %s (priority=%d)", name, priority)
}

// LoadAllModules sorts registered modules by ascending priority, then by
// name so equal priorities have a total order, and calls each loader,
// returning the loaded module names in load order.
func LoadAllModules(dispatcher *ext.Dispatcher) []string {
	mods := append([]registeredModule(nil), registry...)

	// Subtracting priorities overflows for PriorityFinal (math.MaxInt) against
	// a negative priority: cmp.Compare keeps the comparison total.
	slices.SortFunc(mods, func(a, b registeredModule) int {
		return cmp.Or(cmp.Compare(a.priority, b.priority), cmp.Compare(a.name, b.name))
	})
	loaded := make([]string, 0, len(mods))
	for _, m := range mods {
		log.Debugf("Loading module: %s (priority=%d)", m.name, m.priority)
		loadModule(m, dispatcher)
		loaded = append(loaded, m.name)
	}
	return loaded
}
