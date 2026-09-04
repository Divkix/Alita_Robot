package modules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
)

var ConnectionsModule = moduleStruct{moduleName: "Connections"}

func (m moduleStruct) connection(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequirePrivate(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_pm_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}

	chat := chat_status.IsUserConnected(b, ctx, false, false)
	if chat == nil {
		return ext.EndGroups
	}

	temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connected")
	_text := fmt.Sprintf(temp, chat.Title)
	connKeyboard := keyboard.InitButtons(b, chat.Id, user.Id)
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

func (m moduleStruct) allowConnect(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var text string

	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.EndGroups
	}

	if len(args) >= 2 {
		toogleOption := args[1]
		switch toogleOption {
		case "on", "true", "yes":
			if err := connections.ToggleAllowConnect(chat.Id, true); err != nil {
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_turned_on")
			}
		case "off", "false", "no":
			if err := connections.ToggleAllowConnect(chat.Id, false); err != nil {
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_turned_off")
			}
		default:
			text, _ = tr.GetString("connections_invalid_option")
		}
	} else {
		currSetting := connections.GetChatConnectionSetting(chat.Id).AllowConnect
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

func (m moduleStruct) connect(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var text string
	var replyMarkup gotgbot.ReplyMarkup

	if ctx.Message.Chat.Type == "private" {
		chat := extraction.ExtractChat(b, ctx)
		if chat == nil {
			return ext.EndGroups
		}

		if allowed, denyKey := canUserConnectToChat(b, chat.Id, user.Id); !allowed {
			text, _ = tr.GetString(denyKey)
		} else if err := connections.ConnectId(user.Id, chat.Id); err != nil {
			text, _ = tr.GetString("common_settings_save_failed")
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connect_connected")
			text = fmt.Sprintf(temp, chat.Title)
			replyMarkup = keyboard.InitButtons(b, chat.Id, user.Id)
		}
	} else {
		if allowed, denyKey := canUserConnectToChat(b, chat.Id, user.Id); !allowed {
			text, _ = tr.GetString(denyKey)
		} else {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_connect_tap_btn_connect")
			replyMarkup = gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
								t, _ := tr.GetString("connections_button_connect")
								return t
							}(),
							Url: fmt.Sprintf("https://t.me/%s?start=connect_%d", b.Username, chat.Id),
						},
					},
				},
			}
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
		replyText = fmt.Sprintf(temp, chat.Title)
		replyKb = keyboard.InitButtons(b, chat.Id, user.Id)
	}

	_, _, err := msg.EditText(b, &gotgbot.EditMessageTextOpts{Text: replyText, ReplyMarkup: replyKb,
		ParseMode: formatting.HTML})
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

func (m moduleStruct) disconnect(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var text string

	if ctx.Message.Chat.Type == "private" {
		if connections.Connection(user.Id).Connected {
			if err := connections.DisconnectId(user.Id); err != nil {
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_disconnect_disconnected")
			}
		} else {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_not_connected")
		}
	} else {
		text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_disconnect_need_pm")
	}

	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) reconnect(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var (
		connKeyboard gotgbot.InlineKeyboardMarkup
		text         string
	)

	if ctx.Message.Chat.Type == "private" {
		user := chat_status.RequireUser(b, ctx)
		if user == nil {
			return ext.EndGroups
		}
		chatId := connections.Connection(user.Id).ChatId

		if chatId != 0 {
			gchat, err := b.GetChat(chatId, nil)
			if err != nil {
				log.Error(err)
				return err
			}

			_chat := gchat.ToChat()

			isMember, err := chat_status.IsUserInChatWithError(b, &_chat, user.Id)
			if err != nil {
				log.Error(err)
				return err
			}
			if !isMember {
				if err := connections.DisconnectId(user.Id); err != nil {
					text, _ = tr.GetString("common_settings_save_failed")
				} else {
					text, _ = tr.GetString("connections_stale_connection")
				}
			} else if allowed, denyKey := canUserConnectToChat(b, chatId, user.Id); !allowed {
				text, _ = tr.GetString(denyKey)
			} else if err := connections.ConnectId(user.Id, chatId); err != nil {
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_reconnect_reconnected")
				text = fmt.Sprintf(temp, gchat.Title)
				connKeyboard = keyboard.InitButtons(b, gchat.Id, user.Id)
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

	} else {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_reconnect_need_pm")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

func LoadConnections(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[ConnectionsModule.moduleName] = true

	dispatcher.AddHandler(handlers.NewCommand("connect", ConnectionsModule.connect))
	dispatcher.AddHandler(handlers.NewCommand("disconnect", ConnectionsModule.disconnect))
	dispatcher.AddHandler(handlers.NewCommand("connection", ConnectionsModule.connection))
	dispatcher.AddHandler(handlers.NewCommand("reconnect", ConnectionsModule.reconnect))
	dispatcher.AddHandler(handlers.NewCommand("allowconnect", ConnectionsModule.allowConnect))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("connbtns"), ConnectionsModule.connectionButtons))
}

func init() {
	RegisterLegacyModule("Connections", 170, LoadConnections)
	RegisterDeepLinkHandler("connect_", connectDeepLinkHandler)
}

func connectDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage

	parts := strings.Split(arg, "_")
	if len(parts) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	chatID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	cochat, err := b.GetChat(chatID, nil)
	if err != nil || cochat == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_chat_not_found")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
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
	connKeyboard := keyboard.InitButtons(b, cochat.Id, user.Id)

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
