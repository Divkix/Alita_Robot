package modules

import (
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Module defines the interface for bot modules that can be registered
// and loaded via the module registry.
type Module interface {
	Name() string
	Priority() int
	Load(dispatcher *ext.Dispatcher)
}

// registry holds all registered modules in insertion order.
var registry []Module

// RegisterModule adds a module to the global registry.
// Modules are sorted by priority at load time, not at registration.
func RegisterModule(m Module) {
	registry = append(registry, m)
	log.Debugf("Registered module: %s (priority=%d)", m.Name(), m.Priority())
}

// LoadAllModules sorts registered modules by ascending priority and
// calls Load on each one.
func LoadAllModules(dispatcher *ext.Dispatcher) {
	mods := make([]Module, len(registry))
	copy(mods, registry)

	sort.SliceStable(mods, func(i, j int) bool {
		return mods[i].Priority() < mods[j].Priority()
	})

	for _, m := range mods {
		log.Debugf("Loading module: %s (priority=%d)", m.Name(), m.Priority())
		m.Load(dispatcher)
	}
}
