package helpers

import (
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

var (
	DisableCmds = make([]string, 0)
	cmdsMu      = &sync.Mutex{}
)

func MultiCommand(dispatcher *ext.Dispatcher, alias []string, r handlers.Response) {
	for _, cmd := range alias {
		dispatcher.AddHandler(handlers.NewCommand(cmd, r))
	}
}

func AddCmdToDisableable(cmd string) {
	cmdsMu.Lock()
	DisableCmds = append(DisableCmds, cmd)
	cmdsMu.Unlock()
}
