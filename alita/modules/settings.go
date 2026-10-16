package modules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const (
	settingsCandidateLimit = 50
	settingsMaxButtons     = 20
)

// settingsCandidateChats lists groups the user has posted in. It is a variable
// because the query needs Postgres jsonb, which the SQLite test database lacks.
var settingsCandidateChats = chats.ListChatsWithUser

// settings handles /settings. In PM it shows the group picker; in a group it
// links the admin to the summary in PM so nothing else is posted there.
func (m moduleStruct) settings(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if chat.Type == "private" {
		text, kb := m.settingsPicker(b, ctx, tr, user.Id)
		if _, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML, ReplyMarkup: kb}); err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	text, _ := tr.GetString("settings_open_in_pm")
	button, _ := tr.GetString("settings_open_in_pm_button")
	kb := gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
		Text: button,
		Url:  fmt.Sprintf("https://t.me/%s?start=settings_%d", b.Username, chat.Id),
	}}}}
	if _, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML, ReplyMarkup: kb}); err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// settingsPicker lists the connected group first, then recently active groups
// where the user is an admin.
func (moduleStruct) settingsPicker(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, userID int64) (string, gotgbot.InlineKeyboardMarkup) {
	var rows [][]gotgbot.InlineKeyboardButton
	seen := map[int64]bool{}
	addChat := func(chatID int64, title string, connected bool) {
		if strings.TrimSpace(title) == "" {
			title = strconv.FormatInt(chatID, 10)
		}
		if connected {
			title = "✅ " + title
		}
		seen[chatID] = true
		rows = append(rows, []gotgbot.InlineKeyboardButton{{
			Text:         title,
			CallbackData: encodeCallbackData("settings", map[string]string{"a": "p", "c": strconv.FormatInt(chatID, 10)}),
		}})
	}

	if conn := connections.Connection(userID); conn.Connected && conn.ChatId != 0 && chat_status.IsUserAdmin(b, conn.ChatId, userID) {
		title := ""
		if info, err := chats.GetChatBasicInfoCached(conn.ChatId); err == nil && info != nil {
			title = info.ChatName
		}
		addChat(conn.ChatId, title, true)
	}

	candidates, err := settingsCandidateChats(tracing.UpdateContext(ctx), userID, settingsCandidateLimit)
	if err != nil {
		log.Warnf("[Settings] candidate chats for user %d: %v", userID, err)
	}
	for _, candidate := range candidates {
		if len(rows) >= settingsMaxButtons {
			break
		}
		if seen[candidate.ChatId] || !chat_status.IsUserAdmin(b, candidate.ChatId, userID) {
			continue
		}
		addChat(candidate.ChatId, candidate.ChatName, false)
	}

	if len(rows) == 0 {
		text, _ := tr.GetString("settings_picker_empty")
		return text, gotgbot.InlineKeyboardMarkup{}
	}
	text, _ := tr.GetString("settings_picker_text")
	return text, gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func settingsSummaryView(ctx *ext.Context, tr *i18n.Translator, chat *gotgbot.Chat) (string, gotgbot.InlineKeyboardMarkup) {
	text := renderSettingsSummary(tr, chat.Title, collectChatSettings(tracing.UpdateContext(ctx), chat.Id))
	refresh, _ := tr.GetString("settings_btn_refresh")
	other, _ := tr.GetString("settings_btn_other_group")
	kb := gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
		{Text: refresh, CallbackData: encodeCallbackData("settings", map[string]string{"a": "r"})},
		{Text: other, CallbackData: encodeCallbackData("settings", map[string]string{"a": "l"})},
	}}}
	return text, kb
}

// connectForSettings re-checks that the user may manage chatID, connects them
// and returns the chat. On refusal it returns the i18n key to show instead.
func connectForSettings(b *gotgbot.Bot, chatID, userID int64) (*gotgbot.Chat, string) {
	if allowed, _ := canUserConnectToChat(b, chatID, userID); !allowed || !chat_status.IsUserAdmin(b, chatID, userID) {
		return nil, "connections_is_user_connected_user_not_admin"
	}
	full, err := b.GetChat(chatID, nil)
	if err != nil || full == nil {
		return nil, "helpers_chat_not_found"
	}
	if err := connections.ConnectId(userID, chatID); err != nil {
		return nil, "common_settings_save_failed"
	}
	chat := full.ToChat()
	return &chat, ""
}

func (m moduleStruct) settingsCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	msg := query.Message
	decoded, ok := decodeCallbackData(query.Data, "settings")
	if msg == nil || !ok {
		return answerInvalidCallback(b, ctx, query)
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	action, _ := decoded.Field("a")

	var (
		text string
		kb   gotgbot.InlineKeyboardMarkup
	)
	switch action {
	case "p":
		raw, _ := decoded.Field("c")
		chatID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return answerInvalidCallback(b, ctx, query)
		}
		chat, denyKey := connectForSettings(b, chatID, query.From.Id)
		if chat == nil {
			deny, _ := tr.GetString(denyKey)
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: deny, ShowAlert: true})
			return ext.EndGroups
		}
		text, kb = settingsSummaryView(ctx, tr, chat)
	case "r":
		chat := chat_status.IsUserConnected(b, ctx, true, false)
		if chat == nil {
			return ext.EndGroups
		}
		text, kb = settingsSummaryView(ctx, tr, chat)
	case "l":
		text, kb = m.settingsPicker(b, ctx, tr, query.From.Id)
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	if _, _, err := msg.EditText(b, &gotgbot.EditMessageTextOpts{Text: text, ParseMode: formatting.HTML, ReplyMarkup: kb}); err != nil {
		log.Error(err)
	}
	_, _ = query.Answer(b, nil)
	return ext.EndGroups
}

func settingsDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	chatID, err := strconv.ParseInt(strings.TrimPrefix(arg, "settings_"), 10, 64)
	if err != nil {
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}
	chat, denyKey := connectForSettings(b, chatID, user.Id)
	if chat == nil {
		text, _ := tr.GetString(denyKey)
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}
	text, kb := settingsSummaryView(ctx, tr, chat)
	if _, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML, ReplyMarkup: kb}); err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}
