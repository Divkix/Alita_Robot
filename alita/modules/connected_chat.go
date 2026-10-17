package modules

import (
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
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

// callbackAdminChat returns the group a button sent in PM acts on: the one
// named by the callback's "c" field, which is the group the message showed,
// not whichever group the user is connected to when they press it. The user
// must be an admin there (and the bot too when requireBotAdmin); on refusal it
// has already answered the query and returns nil.
func callbackAdminChat(b *gotgbot.Bot, ctx *ext.Context, query *gotgbot.CallbackQuery, decoded *callbackcodec.Decoded, requireBotAdmin bool) *gotgbot.Chat {
	raw, _ := decoded.Field("c")
	chatID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		_ = answerInvalidCallback(b, ctx, query)
		return nil
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	deny := func(key string) *gotgbot.Chat {
		text, _ := tr.GetString(key)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text, ShowAlert: true})
		return nil
	}
	if !chat_status.IsUserAdmin(b, chatID, query.From.Id) {
		return deny("connections_is_user_connected_user_not_admin")
	}
	if requireBotAdmin && !chat_status.IsUserAdmin(b, chatID, b.Id) {
		return deny("connections_is_user_connected_bot_not_admin")
	}
	full, err := b.GetChat(chatID, nil)
	if err != nil || full == nil {
		return deny("helpers_chat_not_found")
	}
	chat := full.ToChat()
	return &chat
}
