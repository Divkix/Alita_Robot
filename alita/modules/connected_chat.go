package modules

import (
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// resolveAdminChat returns the chat an admin command acts on: the current
// group, or in PM the group the user is connected to. The user must be an
// admin there (and the bot too when requireBotAdmin); on refusal it has
// already replied and returns nil. It also points ctx.EffectiveChat at the
// resolved chat, which chat_status helpers given a nil chat then check.
// lang.GetLanguage(ctx) still returns the PM's language afterwards, as
// replies go to the user's PM; read the group's language from the returned
// chat.
func resolveAdminChat(b *gotgbot.Bot, ctx *ext.Context, requireBotAdmin bool) *gotgbot.Chat {
	return resolveConnectedChat(b, ctx, true, requireBotAdmin)
}

// resolveMemberChat is resolveAdminChat for commands any member may run.
func resolveMemberChat(b *gotgbot.Bot, ctx *ext.Context, requireBotAdmin bool) *gotgbot.Chat {
	return resolveConnectedChat(b, ctx, false, requireBotAdmin)
}

func resolveConnectedChat(b *gotgbot.Bot, ctx *ext.Context, requireUserAdmin, requireBotAdmin bool) *gotgbot.Chat {
	chat := chat_status.IsUserConnected(b, ctx, requireUserAdmin, requireBotAdmin)
	if chat == nil {
		return nil
	}
	ctx.EffectiveChat = chat
	return chat
}

// parseChatInfoFromDeepLink resolves the chat named by a "<prefix>_<chatID>"
// deep-link payload, replying and returning ext.EndGroups when it is malformed
// or the chat cannot be fetched.
func parseChatInfoFromDeepLink(b *gotgbot.Bot, ctx *ext.Context, arg string) (*gotgbot.ChatFullInfo, error) {
	nArgs := strings.SplitN(arg, "_", 3)
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if len(nArgs) < 2 {
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return nil, ext.EndGroups
	}

	chatID, err := strconv.ParseInt(nArgs[1], 10, 64)
	if err != nil {
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return nil, ext.EndGroups
	}

	chatinfo, err := b.GetChat(chatID, nil)
	if err != nil || chatinfo == nil {
		text, _ := tr.GetString("helpers_chat_not_found")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return nil, ext.EndGroups
	}
	return chatinfo, nil
}

// connectedChatGuardGroup runs right after the group-0 command handlers.
const connectedChatGuardGroup = 1

// connectedChatGuard ends handler-group iteration once a handler has pointed
// ctx.EffectiveChat at a connected group. gotgbot passes the same ctx to every
// later group, so their watchers (filters, reactions, locks, ...) would
// otherwise act on the group for a message sent in PM, whatever the command
// returned. gotgbot sets EffectiveChat to the effective message's chat, so the
// ids differ only after such a substitution: messages sent in the group pass.
type connectedChatGuard struct{}

func (connectedChatGuard) CheckUpdate(_ *gotgbot.Bot, ctx *ext.Context) bool {
	return ctx.EffectiveMessage != nil && ctx.EffectiveChat != nil &&
		ctx.EffectiveChat.Id != ctx.EffectiveMessage.Chat.Id
}

func (connectedChatGuard) HandleUpdate(*gotgbot.Bot, *ext.Context) error {
	return ext.EndGroups
}

func (connectedChatGuard) Name() string {
	return "connected_chat_guard"
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
