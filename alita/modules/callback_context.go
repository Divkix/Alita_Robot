package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func callbackQueryFromContext(ctx *ext.Context) (*gotgbot.CallbackQuery, bool) {
	if ctx == nil {
		return nil, false
	}
	update := ctx.Update
	if update == nil || update.CallbackQuery == nil {
		return nil, false
	}
	return update.CallbackQuery, true
}
