package helpers

import (
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

// Global command tracking variables
var (
	AdminCmds   = make([]string, 0)
	UserCmds    = make([]string, 0)
	DisableCmds = make([]string, 0)
	cmdsMu      = &sync.Mutex{}
)

// MultiCommand registers multiple command aliases with the same handler function.
func MultiCommand(dispatcher *ext.Dispatcher, alias []string, r handlers.Response) {
	for _, cmd := range alias {
		dispatcher.AddHandler(handlers.NewCommand(cmd, r))
	}
}

// AddCmdToDisableable adds a command to the list of commands that can be disabled.
func AddCmdToDisableable(cmd string) {
	cmdsMu.Lock()
	DisableCmds = append(DisableCmds, cmd)
	cmdsMu.Unlock()
}
