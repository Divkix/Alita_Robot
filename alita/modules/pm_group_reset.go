package modules

import (
	"html"
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

// groupOnlyMemberChat is groupOnlyChat for a command that checks the user's
// admin status itself, live, such as /admincache: in PM it does not ask the
// cached admin list, which may not list an admin promoted since.
func groupOnlyMemberChat(b *gotgbot.Bot, ctx *ext.Context) *gotgbot.Chat {
	if msg := ctx.EffectiveMessage; msg == nil || msg.Chat.Type != "private" {
		return ctx.EffectiveChat
	}
	chat := chat_status.IsUserConnected(b, ctx, false, false)
	if chat == nil {
		return nil
	}
	ctx.EffectiveChat = chat
	return chat
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

// pmGroupLine is what a reply to msg adds to name chat, the group it acts on,
// when msg came from PM, where the reply names no group: a line after a blank
// one, with the title HTML-escaped when escape is set. Elsewhere it is "".
func pmGroupLine(tr *i18n.Translator, msg *gotgbot.Message, chat *gotgbot.Chat, escape bool) string {
	if !inPM(msg) {
		return ""
	}
	title := chat.Title
	if escape {
		title = html.EscapeString(title)
	}
	line, _ := tr.GetString("connections_pm_group_confirm_target", i18n.TranslationParams{"title": title, "id": chat.Id})
	return "\n\n" + line
}

// confirmText adds the group a reset acts on to its confirmation question
// (pmGroupLine). The question is sent as plain text, so the title is not
// escaped.
func confirmText(ctx *ext.Context, tr *i18n.Translator, question string, chat *gotgbot.Chat) string {
	return question + pmGroupLine(tr, ctx.EffectiveMessage, chat, false)
}

// confirmData is the callback data of a reset confirmation for chat. "c"
// names the group, so a confirmation sent in PM acts on it.
func confirmData(namespace, action string, chat *gotgbot.Chat) string {
	return encodeCallbackData(namespace, map[string]string{"a": action, "c": strconv.FormatInt(chat.Id, 10)})
}

// buttonChat returns the group a button sent by a bans or reset command acts
// on: the one its "c" field names or, on a button without one, the chat the
// button is in (nil when Telegram did not include its message). ok is false
// when "c" is malformed.
func buttonChat(ctx *ext.Context, decoded *callbackcodec.Decoded) (chat *gotgbot.Chat, ok bool) {
	if decoded != nil {
		if raw, named := decoded.Field("c"); named {
			chatID, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return nil, false
			}
			return &gotgbot.Chat{Id: chatID}, true
		}
	}
	if query := ctx.CallbackQuery; query != nil && query.Message != nil {
		shown := query.Message.GetChat()
		return &shown, true
	}
	return nil, true
}

// ownerConfirmChat returns the chat a reset confirmation acts on (buttonChat),
// once the presser is shown to own it. On refusal it has answered the query
// and returns nil.
func ownerConfirmChat(b *gotgbot.Bot, ctx *ext.Context, query *gotgbot.CallbackQuery, decoded *callbackcodec.Decoded) *gotgbot.Chat {
	chat, ok := buttonChat(ctx, decoded)
	if !ok {
		_ = answerInvalidCallback(b, ctx, query)
		return nil
	}
	if chat == nil || !chat_status.RequireUserOwner(b, ctx, chat, query.From.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return nil
	}
	return chat
}
