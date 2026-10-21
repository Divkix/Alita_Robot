package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

func callbackQueryFromContext(ctx *ext.Context) (*gotgbot.CallbackQuery, bool) {
	return chat_status.CallbackQueryFromContext(ctx)
}
