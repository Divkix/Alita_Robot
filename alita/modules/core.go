package modules

import (
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// ableMapMu protects AbleMap/helpableKb/AltHelpOptions on the singleton help registry.
// All writes happen during init-only LoadModules; readers in help.go use RLock.
var ableMapMu sync.RWMutex

// module struct for all modules
type moduleStruct struct {
	moduleName        string
	handlerGroup      int
	permHandlerGroup  int
	restrHandlerGroup int
	defaultRulesBtn   string
	AbleMap           map[string]bool
	AltHelpOptions    map[string][]string
	helpableKb        map[string][][]gotgbot.InlineKeyboardButton
}

// SetAbleMap marks a module as enabled under lock.
func SetAbleMap(name string) {
	ableMapMu.Lock()
	defer ableMapMu.Unlock()
	defaultHelpRegistry.AbleMap[name] = true
}

// GetAbleMap returns a snapshot copy of AbleMap under RLock.
func GetAbleMap() map[string]bool {
	ableMapMu.RLock()
	defer ableMapMu.RUnlock()
	out := make(map[string]bool, len(defaultHelpRegistry.AbleMap))
	for k, v := range defaultHelpRegistry.AbleMap {
		out[k] = v
	}
	return out
}

// ResetHelpRegistry clears AbleMap, helpableKb and AltHelpOptions under lock.
// Call only during single-threaded startup (LoadModules).
func ResetHelpRegistry() {
	ableMapMu.Lock()
	defer ableMapMu.Unlock()
	defaultHelpRegistry.AbleMap = make(map[string]bool)
	defaultHelpRegistry.helpableKb = make(map[string][][]gotgbot.InlineKeyboardButton)
	defaultHelpRegistry.AltHelpOptions = make(map[string][]string)
}

func newHelpRegistry() *moduleStruct {
	return &moduleStruct{
		moduleName:     "Help",
		AbleMap:        make(map[string]bool),
		AltHelpOptions: make(map[string][]string),
		helpableKb:     make(map[string][][]gotgbot.InlineKeyboardButton),
	}
}

// DefaultHelpRegistry returns the default help registry.
func DefaultHelpRegistry() *moduleStruct {
	return defaultHelpRegistry
}

var defaultHelpRegistry = newHelpRegistry()
