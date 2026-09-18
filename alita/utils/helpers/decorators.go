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

// IsDisableable reports whether cmd was registered via AddCmdToDisableable.
// Direct slices.Contains(DisableCmds) reads race with startup registration;
// keep every runtime read under the same mutex as the writes.
func IsDisableable(cmd string) bool {
	cmdsMu.Lock()
	defer cmdsMu.Unlock()
	for _, c := range DisableCmds {
		if c == cmd {
			return true
		}
	}
	return false
}

// DisableableCommands returns a snapshot copy for listing (/disableable).
func DisableableCommands() []string {
	cmdsMu.Lock()
	defer cmdsMu.Unlock()
	out := make([]string, len(DisableCmds))
	copy(out, DisableCmds)
	return out
}
