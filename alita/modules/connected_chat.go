package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

// resolveAdminChat returns the chat an admin command acts on: the current
// group, or in PM the group the user is connected to. The user must be an
// admin there (and the bot too when requireBotAdmin); on refusal it has
// already replied and returns nil. It also points ctx.EffectiveChat at the
// resolved chat. Pass the returned chat to later permission checks: the
// chat_status Require* helpers read the update's chat, not EffectiveChat.
func resolveAdminChat(b *gotgbot.Bot, ctx *ext.Context, requireBotAdmin bool) *gotgbot.Chat {
	chat := chat_status.IsUserConnected(b, ctx, true, requireBotAdmin)
	if chat == nil {
		return nil
	}
	ctx.EffectiveChat = chat
	return chat
}
