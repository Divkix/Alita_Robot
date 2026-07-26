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
	AbleMap           map[string]bool
	AltHelpOptions    map[string][]string
	helpableKb        map[string][][]gotgbot.InlineKeyboardButton
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
