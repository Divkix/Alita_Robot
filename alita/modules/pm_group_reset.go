package modules

import (
	"html"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// groupOnlyChat returns the group a group-only command acts on: the chat it
// was sent in or, in PM, the group resolved by CommandDescriptor.PMGroupArg
// or the saved connection, which they must administer. In PM it points
// ctx.EffectiveChat at that group, so chat_status checks given a nil chat
// check it. nil means the call has been answered.
func groupOnlyChat(b *gotgbot.Bot, ctx *ext.Context) *gotgbot.Chat {
	if !inPM(ctx.EffectiveMessage) {
		return ctx.EffectiveChat
	}
	return resolveAdminChat(b, ctx, false)
}

// groupOnlyChatCheck runs groupOnlyChat first among a WrapCommand command's
// RequiredChecks, so the checks after it (RequireGroup, RequireUserOwner, ...)
// see the group named or connected in PM.
var groupOnlyChatCheck = helpers.NewCheck(helpers.CheckKindAny, func(c *helpers.CommandContext) bool {
	chat := groupOnlyChat(c.Bot, c.Ctx)
	if chat == nil {
		return false
	}
	c.Chat = chat
	return true
})

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
	line := tr.String("connections_pm_group_confirm_target", i18n.TranslationParams{"title": title, "id": chat.Id})
	return "\n\n" + line
}

// confirmText adds the group a reset acts on to its confirmation question, or
// the group /warns reads to its answer (pmGroupLine). The text is sent as
// plain text, so the title is not escaped.
func confirmText(ctx *ext.Context, tr *i18n.Translator, question string, chat *gotgbot.Chat) string {
	return question + pmGroupLine(tr, ctx.EffectiveMessage, chat, false)
}

// confirmData is the callback data of a reset confirmation for chat. "c"
// names the group, so a confirmation sent in PM acts on it.
// Confirmations always include "c", even when sent in a group, and their
// handlers must resolve and authorize that target through ownerConfirmChat.
func confirmData(namespace, action string, chat *gotgbot.Chat) string {
	return encodeCallbackData(namespace, map[string]string{"a": action, "c": strconv.FormatInt(chat.Id, 10)})
}

// confirmKeyboard is the yes/no keyboard of a reset confirmation for chat.
func confirmKeyboard(tr *i18n.Translator, namespace string, chat *gotgbot.Chat) gotgbot.InlineKeyboardMarkup {
	yesText := tr.String("button_yes")
	noText := tr.String("button_no")
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
			{Text: yesText, CallbackData: confirmData(namespace, "yes", chat)},
			{Text: noText, CallbackData: confirmData(namespace, "no", chat)},
		}},
	}
}

// buttonChat returns the group a button sent by a bans or reset command acts
// on: the one its "c" field names or, on a button without one, the chat the
// button is in (nil when Telegram did not include its message). ok is false
// when "c" is malformed.
// This fallback also supports buttons sent before PM support. Bans and
// unrestrict handlers must check the presser's rights against the returned chat,
// never against the PM where the button was pressed.
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

// ownerConfirmKeys are the locale keys of a reset confirmation's answers:
// the text that replaces the prompt once the owner confirms or cancels, and
// the text when the commit fails.
type ownerConfirmKeys struct {
	Confirmed string
	Cancelled string
	Failed    string
}

// ownerConfirmSpec is everything a reset-all command supplies to
// registerOwnerConfirm. AskKey is the locale key of the yes/no question.
// ask replies to the command: it answers an empty state or a refusal itself
// and returns false; true means the skeleton sends the prompt for chat. A
// nil ask always prompts. commit runs the reset for the owner-confirmed
// chat; its key and params localize the replacement text (an empty key
// falls back to Keys.Confirmed).
type ownerConfirmSpec struct {
	Namespace string
	AskKey    string
	Ask       func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, tr *i18n.Translator) (bool, error)
	Commit    func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, tr *i18n.Translator, actor *gotgbot.User) (key string, params i18n.TranslationParams, err error)
	Keys      ownerConfirmKeys
}

// ownerConfirmKey translates key with params, falling back to the generic
// error when the key is empty or missing from the locale.
func ownerConfirmKey(tr *i18n.Translator, key string, params i18n.TranslationParams) string {
	if key == "" {
		key = "error_generic"
	}
	return tr.String(key, params)
}

// registerOwnerConfirm owns the reset-everything lifecycle: the command asks
// through spec.Ask, the callback decodes, authorizes the presser as the
// chat's owner and runs spec.Commit on yes. It returns the message handler
// for WrapCommand and the callback handler for the namespace's dispatcher
// registration.
func registerOwnerConfirm(spec ownerConfirmSpec) (message, callback handlers.Response) {
	message = func(b *gotgbot.Bot, ctx *ext.Context) error {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		send := true
		var err error
		if spec.Ask != nil {
			send, err = spec.Ask(b, ctx, ctx.EffectiveChat, tr)
			if err != nil {
				return err
			}
		}
		if !send {
			return ext.EndGroups
		}
		text := tr.String(spec.AskKey)
		_, err = ctx.EffectiveMessage.Reply(b, confirmText(ctx, tr, text, ctx.EffectiveChat),
			&gotgbot.SendMessageOpts{ReplyMarkup: confirmKeyboard(tr, spec.Namespace, ctx.EffectiveChat)})
		if err != nil {
			return err
		}
		return ext.EndGroups
	}
	callback = func(b *gotgbot.Bot, ctx *ext.Context) (err error) {
		defer error_handling.RecoverFromPanicContext(tracing.UpdateContext(ctx), "ownerConfirm", spec.Namespace)
		query, ok := callbackQueryFromContext(ctx)
		if !ok {
			return ext.EndGroups
		}
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		decoded, _ := decodeCallbackData(query.Data, spec.Namespace)
		chat := ownerConfirmChat(b, ctx, query, decoded)
		if chat == nil {
			return ext.EndGroups
		}
		if query.Message == nil {
			// The confirmation is gone (deleted or inline): answer, never commit.
			text := tr.String("common_callback_message_unavailable")
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		action, _ := decoded.Field("a")
		var key string
		var params i18n.TranslationParams
		switch action {
		case "yes":
			key, params, err = spec.Commit(b, ctx, chat, tr, &query.From)
			if err != nil {
				key, params = spec.Keys.Failed, nil
			} else if key == "" {
				key = spec.Keys.Confirmed
			}
		case "no":
			key = spec.Keys.Cancelled
		default:
			return answerInvalidCallback(b, ctx, query)
		}
		text := ownerConfirmKey(tr, key, params)
		_, _, _ = query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: text})
		_, err = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		if err != nil {
			return err
		}
		return ext.EndGroups
	}
	return message, callback
}
