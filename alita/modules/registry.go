package modules

import (
	"slices"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

type registeredModule struct {
	name     string
	priority int
	load     func(*ext.Dispatcher)
}

// registry holds all registered modules in insertion order.
var registry []registeredModule

// loadModule runs one module's loader with the command manifest pointed at
// it, so every WrapCommand inside records name as the owning module.
func loadModule(name string, dispatcher *ext.Dispatcher, load func(*ext.Dispatcher)) {
	previous := helpers.CurrentCommandModule()
	helpers.SetCommandModule(name)
	defer helpers.SetCommandModule(previous)
	load(dispatcher)
}

// RegisterLegacyModule registers a module loader.
// Duplicate registrations are silently ignored to prevent double-loading handlers.
// init-only, not safe for concurrent Register.
func RegisterLegacyModule(name string, priority int, load func(*ext.Dispatcher)) {
	for _, existing := range registry {
		if existing.name == name {
			log.Debugf("Duplicate module registration ignored: %s", name)
			return
		}
	}
	registry = append(registry, registeredModule{name: name, priority: priority, load: func(d *ext.Dispatcher) { loadModule(name, d, load) }})
	log.Debugf("Registered module: %s (priority=%d)", name, priority)
}

// LoadAllModules sorts registered modules by ascending priority and
// calls each loader.
func LoadAllModules(dispatcher *ext.Dispatcher) {
	mods := append([]registeredModule(nil), registry...)

	slices.SortStableFunc(mods, func(a, b registeredModule) int { return a.priority - b.priority })
	for _, m := range mods {
		log.Debugf("Loading module: %s (priority=%d)", m.name, m.priority)
		if m.load != nil {
			m.load(dispatcher)
		}
	}
}
