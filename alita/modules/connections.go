package modules

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var ConnectionsModule = moduleStruct{moduleName: "Connections"}

func (m moduleStruct) connection(c *helpers.CommandContext) error {
	b, msg, chat, user, tr := c.Bot, c.Msg, c.Chat, c.User, c.Tr

	temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connected")
	_text := fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))
	connKeyboard := keyboard.InitButtons(b, tr, chat.Id, user.Id)
	_, err := msg.Reply(b,
		_text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: connKeyboard,
			ParseMode:   formatting.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) allowConnect(c *helpers.CommandContext) error {
	b, ctx, msg, chat, tr := c.Bot, c.Ctx, c.Msg, c.Chat, c.Tr
	args := ctx.Args()

	var text string

	if len(args) >= 2 {
		toogleOption := args[1]
		switch toogleOption {
		case "on", "true", "yes":
			if err := settings.AllowConnect.Write(settings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, true); err != nil {
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_turned_on")
			}
		case "off", "false", "no":
			if err := settings.AllowConnect.Write(settings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, false); err != nil {
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_turned_off")
			}
		default:
			text, _ = tr.GetString("connections_invalid_option")
		}
	} else {
		currSetting := settings.AllowConnect.ReadContext(tracing.UpdateContext(ctx), chat.Id).Value.(bool)
		if currSetting {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_currently_on")
		} else {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_currently_off")
		}
	}

	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) connect(c *helpers.CommandContext) error {
	b, ctx, chat, msg, user, tr := c.Bot, c.Ctx, c.Chat, c.Msg, c.User, c.Tr
	var text string
	var replyMarkup gotgbot.ReplyMarkup

	if ctx.Message.Chat.Type == "private" {
		if len(ctx.Args()) < 2 {
			text, _ = tr.GetString("extraction_need_chat_id")
			if err := replyPMGroupHelp(b, ctx, tr, "connect", formatting.HtmlEscape(text)); err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		chat := extraction.ExtractChat(b, ctx)
		if chat == nil {
			return ext.EndGroups
		}

		if !connectionTargetCheck(chat.Id)(c) {
			return ext.EndGroups
		}
		if err := connections.ConnectId(user.Id, chat.Id); err != nil {
			text, _ = tr.GetString("common_settings_save_failed")
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connect_connected")
			text = fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))
			replyMarkup = keyboard.InitButtons(b, tr, chat.Id, user.Id)
		}
	} else {
		if !connectionTargetCheck(chat.Id)(c) {
			return ext.EndGroups
		}
		text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_connect_tap_btn_connect")
		replyMarkup = gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text: trS(ctxTr(ctx), "connections_button_connect"),
						Url:  fmt.Sprintf("https://t.me/%s?start=connect_%d", b.Username, chat.Id),
					},
				},
			},
		}
	}

	_, err := msg.Reply(b,
		text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: replyMarkup,
			ParseMode:   formatting.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) connectionButtons(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	msg := query.Message
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if msg == nil {
		return answerInvalidCallback(b, ctx, query)
	}

	userType := ""
	if decoded, ok := decodeCallbackData(query.Data, "connbtns"); ok {
		userType, _ = decoded.Field("t")
	}
	switch userType {
	case "Admin", "User", "Main":
	default:
		log.Warnf("[Connections] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}

	backText, _ := tr.GetString("button_back")
	var (
		replyText string
		replyKb   = gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         backText,
						CallbackData: encodeCallbackData("connbtns", map[string]string{"t": "Main"}),
					},
				},
			},
		}
	)

	if conn := connections.Connection(user.Id); !conn.Connected || conn.ChatId == 0 {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_not_connected")
		if _, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text}); err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	chat := chat_status.IsUserConnected(b, ctx, false, false)
	if chat == nil {
		return ext.EndGroups
	}

	switch userType {
	case "Admin":
		replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_connections_btns_admin_conn_cmds")
	case "User":
		replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_connections_btns_user_conn_cmds")
	case "Main":
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connected")
		replyText = fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))
		replyKb = keyboard.InitButtons(b, tr, chat.Id, user.Id)
	}

	_, _, err := msg.EditText(b, &gotgbot.EditMessageTextOpts{
		Text: replyText, ReplyMarkup: replyKb,
		ParseMode: formatting.HTML,
	})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) disconnect(c *helpers.CommandContext) error {
	b, msg, user, tr := c.Bot, c.Msg, c.User, c.Tr
	var text string

	if connections.Connection(user.Id).Connected {
		if err := connections.DisconnectId(user.Id); err != nil {
			text, _ = tr.GetString("common_settings_save_failed")
		} else {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_disconnect_disconnected")
		}
	} else {
		text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_not_connected")
	}

	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) reconnect(c *helpers.CommandContext) error {
	b, msg, user, tr := c.Bot, c.Msg, c.User, c.Tr
	var (
		connKeyboard gotgbot.InlineKeyboardMarkup
		text         string
	)
	chatId := connections.Connection(user.Id).ChatId

	if chatId != 0 {
		gchat, err := b.GetChat(chatId, nil)
		if err != nil || gchat == nil {
			log.WithFields(log.Fields{"userId": user.Id, "chatId": chatId, "error": err}).Warn("[Reconnect] Last chat lookup failed")
			text, _ = tr.GetString("error_generic")
		} else {
			chat := gchat.ToChat()
			if !helpers.RunChecks(c, []helpers.CheckFunc{reconnectTargetCheck(&chat, chatId)}) {
				return ext.EndGroups
			}
			if err := connections.ConnectId(user.Id, chatId); err != nil {
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_reconnect_reconnected")
				text = fmt.Sprintf(temp, formatting.HtmlEscape(gchat.Title))
				connKeyboard = keyboard.InitButtons(b, tr, gchat.Id, user.Id)
			}
		}
	} else {
		text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_reconnect_no_last_chat")
	}
	_, err := msg.Reply(b, text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: connKeyboard,
			ParseMode:   formatting.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// connectionPrivateCheck keeps /connection's generic PM refusal before its
// saved-connection and membership checks.
func connectionPrivateCheck(c *helpers.CommandContext) bool {
	if chat_status.RequirePrivate(c.Bot, c.Ctx, nil) {
		return true
	}
	chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_pm_only_error", "", chat_status.WithReply())
	return false
}

func connectionSavedCheck(c *helpers.CommandContext) bool {
	if conn := connections.Connection(c.User.Id); conn.Connected && conn.ChatId != 0 {
		return true
	}
	sendHTMLReply(c.Bot, c.Msg, trS(c.Tr, "connections_not_connected"))
	return false
}

// connectionPMOnlyCheck preserves disconnect/reconnect's distinct group replies.
func connectionPMOnlyCheck(key string) helpers.CheckFunc {
	return func(c *helpers.CommandContext) bool {
		if c.Msg.Chat.Type == "private" {
			return true
		}
		sendHTMLReply(c.Bot, c.Msg, trS(c.Tr, key))
		return false
	}
}

// allowConnectAdminCheck keeps group non-admins silent, without anonymous proof.
func allowConnectAdminCheck(c *helpers.CommandContext) bool {
	if c.Msg.Chat.Type != "private" && !chat_status.IsUserAdmin(c.Bot, c.Chat.Id, c.User.Id) {
		return false
	}
	return helpers.RunChecks(c, declaredSettingChecks(settings.AllowConnect))
}

// connectionTargetCheck applies the shared policy only after the command has
// read its actual target; a saved connection is not /connect's target.
func connectionTargetCheck(chatID int64) helpers.CheckFunc {
	return func(c *helpers.CommandContext) bool {
		allowed, key := canUserConnectToChat(c.Bot, chatID, c.User.Id)
		if allowed {
			return true
		}
		sendHTMLReply(c.Bot, c.Msg, trS(c.Tr, key))
		return false
	}
}

// reconnectTargetCheck requires live membership before the allow-connect policy.
// Lookup failures preserve the saved connection; only a former member is detached.
func reconnectTargetCheck(chat *gotgbot.Chat, chatID int64) helpers.CheckFunc {
	return func(c *helpers.CommandContext) bool {
		isMember, err := chat_status.IsUserInChatWithError(c.Bot, chat, c.User.Id)
		var key string
		if err != nil {
			log.WithFields(log.Fields{"userId": c.User.Id, "chatId": chatID, "error": err}).Warn("[Reconnect] Membership check failed")
			key = "error_generic"
		} else if !isMember {
			if err := connections.DisconnectId(c.User.Id); err != nil {
				key = "common_settings_save_failed"
			} else {
				key = "connections_stale_connection"
			}
		} else {
			return connectionTargetCheck(chatID)(c)
		}
		sendHTMLReply(c.Bot, c.Msg, trS(c.Tr, key))
		return false
	}
}

func LoadConnections(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(ConnectionsModule.moduleName, true)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "connect", RequiredChecks: []helpers.CheckFunc{helpers.RequireIdentifiedUser()},
	}, ConnectionsModule.connect)
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "disconnect",
		RequiredChecks: []helpers.CheckFunc{connectionPMOnlyCheck("connections_disconnect_need_pm")},
	}, ConnectionsModule.disconnect)
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "connection",
		RequiredChecks: []helpers.CheckFunc{
			connectionPrivateCheck,
			connectionSavedCheck,
			helpers.RequireConnectedMember(helpers.BotAdminNotRequired),
		},
	}, ConnectionsModule.connection)
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "reconnect",
		RequiredChecks: []helpers.CheckFunc{connectionPMOnlyCheck("connections_reconnect_need_pm")},
	}, ConnectionsModule.reconnect)
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "allowconnect",
		PMGroupArg:     helpers.PMGroupArgOwn,
		RequiredChecks: []helpers.CheckFunc{allowConnectAdminCheck},
	}, ConnectionsModule.allowConnect)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("connbtns"), ConnectionsModule.connectionButtons))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "settings", RequiredChecks: []helpers.CheckFunc{checkInPMOrGroup(nil, helpers.RequireUserAdmin())},
	}, pipelineHandler(ConnectionsModule.settings))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("settings"), ConnectionsModule.settingsCallback))
	dispatcher.AddHandlerToGroup(connectedChatGuard{}, connectedChatGuardGroup)
}

func init() {
	RegisterLegacyModule("Connections", 170, LoadConnections)
	RegisterDeepLinkHandler("connect_", connectDeepLinkHandler)
	RegisterDeepLinkHandler("settings_", settingsDeepLinkHandler)
}

func connectDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage

	cochat, err := parseChatInfoFromDeepLink(b, ctx, arg)
	if err != nil {
		return err
	}

	if allowed, denyKey := canUserConnectToChat(b, cochat.Id, user.Id); !allowed {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString(denyKey)
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if err := connections.ConnectId(user.Id, cochat.Id); err != nil {
		text, _ := tr.GetString("common_settings_save_failed")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	Text, _ := tr.GetString("helpers_connected_to_chat", i18n.TranslationParams{"s": cochat.Title})
	connKeyboard := keyboard.InitButtons(b, tr, cochat.Id, user.Id)

	_, err = msg.Reply(b, Text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: connKeyboard,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}
