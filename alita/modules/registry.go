package modules

import (
	"sort"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
)

// Module defines the interface for bot modules that can be registered
// and loaded via the module registry.
type Module interface {
	Name() string
	Priority() int
	Load(dispatcher *ext.Dispatcher)
}

type legacyModule struct {
	name     string
	priority int
	load     func(*ext.Dispatcher)
}

func (m legacyModule) Name() string {
	return m.name
}

func (m legacyModule) Priority() int {
	return m.priority
}

func (m legacyModule) Load(dispatcher *ext.Dispatcher) {
	m.load(dispatcher)
}

// registry holds all registered modules in insertion order.
var registry []Module

// RegisterLegacyModule adapts an existing LoadXxx function to the registry.
// It lets modules migrate to registration-first loading without rewriting
// handlers that still use moduleStruct internally.
func RegisterLegacyModule(name string, priority int, load func(*ext.Dispatcher)) {
	RegisterModule(legacyModule{
		name:     name,
		priority: priority,
		load:     load,
	})
}

// RegisterModule adds a module to the global registry.
// Modules are sorted by priority at load time, not at registration.
// Duplicate registrations are silently ignored to prevent double-loading handlers.
func RegisterModule(m Module) {
	for _, existing := range registry {
		if existing.Name() == m.Name() {
			log.Debugf("Duplicate module registration ignored: %s", m.Name())
			return
		}
	}
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
