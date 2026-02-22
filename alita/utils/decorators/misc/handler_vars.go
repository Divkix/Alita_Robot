package misc

import "sync"

var (
	AdminCmds   = make([]string, 0)
	UserCmds    = make([]string, 0)
	DisableCmds = make([]string, 0)
	mu          = &sync.Mutex{}
)

// addToArray appends multiple strings to a string slice.
// Callers must hold mu before calling this function.
func addToArray(arr []string, val ...string) []string {
	return append(arr, val...)
}

// AddCmdToDisableable adds a command to the list of commands that can be disabled in chats.
// Administrators can use this to control which commands are available to regular users.
func AddCmdToDisableable(cmd string) {
	mu.Lock()
	DisableCmds = addToArray(DisableCmds, cmd)
	mu.Unlock()
}
