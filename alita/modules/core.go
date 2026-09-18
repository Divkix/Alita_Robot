package modules

import (
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

var ableMapMu sync.RWMutex

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

func GetAbleMap() map[string]bool {
	ableMapMu.RLock()
	defer ableMapMu.RUnlock()
	out := make(map[string]bool, len(defaultHelpRegistry.AbleMap))
	for k, v := range defaultHelpRegistry.AbleMap {
		out[k] = v
	}
	return out
}

func ResetHelpRegistry() {
	ableMapMu.Lock()
	defer ableMapMu.Unlock()
	defaultHelpRegistry.AbleMap = make(map[string]bool)
	defaultHelpRegistry.helpableKb = make(map[string][][]gotgbot.InlineKeyboardButton)
	defaultHelpRegistry.AltHelpOptions = make(map[string][]string)
}

// SetModuleEnabled records a module under the registry lock: direct
// DefaultHelpRegistry().AbleMap writes bypass ableMapMu and race readers.
func SetModuleEnabled(name string, enabled bool) {
	ableMapMu.Lock()
	defer ableMapMu.Unlock()
	defaultHelpRegistry.AbleMap[name] = enabled
}

// SetModuleHelp records help buttons under the registry lock.
func SetModuleHelp(name string, kb [][]gotgbot.InlineKeyboardButton) {
	ableMapMu.Lock()
	defer ableMapMu.Unlock()
	defaultHelpRegistry.helpableKb[name] = kb
}

func newHelpRegistry() *moduleStruct {
	return &moduleStruct{
		moduleName:     "Help",
		AbleMap:        make(map[string]bool),
		AltHelpOptions: make(map[string][]string),
		helpableKb:     make(map[string][][]gotgbot.InlineKeyboardButton),
	}
}

func DefaultHelpRegistry() *moduleStruct {
	return defaultHelpRegistry
}

var defaultHelpRegistry = newHelpRegistry()
