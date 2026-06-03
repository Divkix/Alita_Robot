package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// AnonymousAdminHandler processes an anonymous admin command.
type AnonymousAdminHandler func(b *gotgbot.Bot, ctx *ext.Context) error

var anonAdminRegistry = make(map[string]AnonymousAdminHandler)

// RegisterAnonymousAdminHandler registers a handler for an anonymous admin command.
func RegisterAnonymousAdminHandler(command string, handler AnonymousAdminHandler) {
	anonAdminRegistry[command] = handler
}

// HandleAnonymousAdmin routes an anonymous admin command to the appropriate handler.
func HandleAnonymousAdmin(b *gotgbot.Bot, ctx *ext.Context, command string) error {
	if handler, ok := anonAdminRegistry[command]; ok {
		return handler(b, ctx)
	}
	return fmt.Errorf("unknown anonymous admin command: %s", command)
}
