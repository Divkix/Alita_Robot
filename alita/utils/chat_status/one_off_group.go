package chat_status

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// OneOffGroupDataKey is the ext.Context.Data key holding the group a PM
// command named as its first argument. IsUserConnected uses it in place of
// the saved connection for that one update.
const OneOffGroupDataKey = "pmOneOffGroup"

// SetOneOffGroup makes chat the group IsUserConnected returns for this update.
// The caller must have checked the user against chat as /connect does and
// looked them up in it live once (a cached admin list does not show that an
// admin left): IsUserConnected checks only admin rights on it, not membership.
func SetOneOffGroup(ctx *ext.Context, chat *gotgbot.Chat) {
	if ctx.Data == nil {
		ctx.Data = make(map[string]any)
	}
	ctx.Data[OneOffGroupDataKey] = chat
}

// OneOffGroup returns the group set by SetOneOffGroup, or nil.
func OneOffGroup(ctx *ext.Context) *gotgbot.Chat {
	if ctx == nil {
		return nil
	}
	chat, _ := ctx.Data[OneOffGroupDataKey].(*gotgbot.Chat)
	return chat
}
