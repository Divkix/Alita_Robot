package modules

import (
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// groupOnlyChat returns the group a group-only command acts on: the chat it
// was sent in or, in PM, the group the user named (withPMGroupArg or
// CommandDescriptor.PMGroupArg) or is connected to, which they must
// administer. In PM it points ctx.EffectiveChat at that group, so chat_status
// checks given a nil chat check it. nil means the call has been answered.
func groupOnlyChat(b *gotgbot.Bot, ctx *ext.Context) *gotgbot.Chat {
	if msg := ctx.EffectiveMessage; msg == nil || msg.Chat.Type != "private" {
		return ctx.EffectiveChat
	}
	return resolveAdminChat(b, ctx, false)
}

// groupOnlyChatCheck runs groupOnlyChat first among a WrapCommand command's
// RequiredChecks, so the checks after it (RequireGroup, RequireUserOwner, ...)
// see the group named or connected in PM.
func groupOnlyChatCheck(c *helpers.CommandContext) bool {
	chat := groupOnlyChat(c.Bot, c.Ctx)
	if chat == nil {
		return false
	}
	c.Chat = chat
	return true
}

// confirmText adds the group a reset acts on to its confirmation question
// when the question is asked in PM, where "this chat" names no group. The
// question is sent as plain text, so the title is not escaped.
func confirmText(ctx *ext.Context, tr *i18n.Translator, question string, chat *gotgbot.Chat) string {
	if msg := ctx.EffectiveMessage; msg == nil || msg.Chat.Type != "private" {
		return question
	}
	target, _ := tr.GetString("connections_pm_group_confirm_target", i18n.TranslationParams{"title": chat.Title, "id": chat.Id})
	return question + "\n\n" + target
}

// confirmData is the callback data of a reset confirmation for chat. "c"
// names the group, so a confirmation sent in PM acts on it.
func confirmData(namespace, action string, chat *gotgbot.Chat) string {
	return encodeCallbackData(namespace, map[string]string{"a": action, "c": strconv.FormatInt(chat.Id, 10)})
}

// ownerConfirmChat returns the chat a reset confirmation acts on, once the
// presser is shown to own it: the group its "c" field names or, on a button
// without one, the chat the button is in. On refusal it has answered the
// query and returns nil.
func ownerConfirmChat(b *gotgbot.Bot, ctx *ext.Context, query *gotgbot.CallbackQuery, decoded *callbackcodec.Decoded) *gotgbot.Chat {
	var chat *gotgbot.Chat
	if raw, ok := decoded.Field("c"); ok {
		chatID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			_ = answerInvalidCallback(b, ctx, query)
			return nil
		}
		chat = &gotgbot.Chat{Id: chatID}
	} else if query.Message != nil {
		shown := query.Message.GetChat()
		chat = &shown
	}
	if chat == nil || !chat_status.RequireUserOwner(b, ctx, chat, query.From.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return nil
	}
	return chat
}
