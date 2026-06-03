package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
)

// module struct for all modules
type moduleStruct struct {
	moduleName        string
	handlerGroup      int
	permHandlerGroup  int
	restrHandlerGroup int
	defaultRulesBtn   string
	AbleMap           moduleEnabled
	AltHelpOptions    map[string][]string
	helpableKb        map[string][][]gotgbot.InlineKeyboardButton
}

// moduleEnabled tracks which modules are enabled per chat.
type moduleEnabled struct {
	modules map[string]bool
}

// Init initializes the moduleEnabled map.
func (m *moduleEnabled) Init() {
	m.modules = make(map[string]bool)
}

// Store enables or disables a module.
func (m *moduleEnabled) Store(module string, enabled bool) {
	m.modules[module] = enabled
}

// Load returns the module name and whether it is enabled.
func (m *moduleEnabled) Load(module string) (string, bool) {
	return module, m.modules[module]
}

// LoadModules returns a slice of all enabled module names.
func (m *moduleEnabled) LoadModules() []string {
	var modules []string
	for module, enabled := range m.modules {
		if enabled {
			modules = append(modules, module)
		}
	}
	return modules
}

// NewHelpRegistry creates a new help registry.
func NewHelpRegistry() *moduleStruct {
	return &moduleStruct{
		moduleName:     "Help",
		AbleMap:        moduleEnabled{modules: make(map[string]bool)},
		AltHelpOptions: make(map[string][]string),
		helpableKb:     make(map[string][][]gotgbot.InlineKeyboardButton),
	}
}

// DefaultHelpRegistry returns the default help registry.
func DefaultHelpRegistry() *moduleStruct {
	return defaultHelpRegistry
}

var defaultHelpRegistry = NewHelpRegistry()
