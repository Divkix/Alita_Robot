package modules

import (
	"sort"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
)

type registeredModule struct {
	name     string
	priority int
	load     func(*ext.Dispatcher)
}

// registry holds all registered modules in insertion order.
var registry []registeredModule

// RegisterLegacyModule registers a module loader.
// Duplicate registrations are silently ignored to prevent double-loading handlers.
func RegisterLegacyModule(name string, priority int, load func(*ext.Dispatcher)) {
	for _, existing := range registry {
		if existing.name == name {
			log.Debugf("Duplicate module registration ignored: %s", name)
			return
		}
	}
	registry = append(registry, registeredModule{name: name, priority: priority, load: load})
	log.Debugf("Registered module: %s (priority=%d)", name, priority)
}

// LoadAllModules sorts registered modules by ascending priority and
// calls each loader.
func LoadAllModules(dispatcher *ext.Dispatcher) {
	mods := append([]registeredModule(nil), registry...)

	sort.SliceStable(mods, func(i, j int) bool {
		return mods[i].priority < mods[j].priority
	})

	for _, m := range mods {
		log.Debugf("Loading module: %s (priority=%d)", m.name, m.priority)
		if m.load != nil {
			m.load(dispatcher)
		}
	}
}
