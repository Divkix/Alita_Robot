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

/*
	Check the status of connection of a user in their PM

User can check if they are connected to a chat and can also bring up the keyboard for it.
Normal use will have just one option with 'User Commands' and admin will have "Admin Commands" along the earlier as
well.
*/
// connection handles the /connection command to check user's connection status.
// Shows current connected chat and provides keyboard with available commands.
func (m moduleStruct) connection(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// permission checks
	if !chat_status.RequirePrivate(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_pm_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}

	chatId := m.isConnected(b, ctx, user.Id)
	if chatId == 0 {
		return ext.EndGroups
	}

	chat, err := b.GetChat(chatId, nil)
	if err != nil {
		connections.DisconnectId(user.Id)
		log.Error(err)
		return err
	}
	temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connected")
	_text := fmt.Sprintf(temp, chat.Title)
	connKeyboard := keyboard.InitButtons(b, chat.Id, user.Id)
	_, err = msg.Reply(b,
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

/*
	Allow users to connect to your chat

You can give a word such as on/off/yes/no to toggle options

Also, if no word is given, you will get your current setting.
*/
// allowConnect handles the /allowconnect command to toggle connection permissions.
// Admins can enable/disable whether users can connect to their chat remotely.
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

	// permission checks
	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.EndGroups
	}

	if len(args) >= 2 {
		toogleOption := args[1]
		switch toogleOption {
		case "on", "true", "yes":
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_turned_on")
			connections.ToggleAllowConnect(chat.Id, true)
		case "off", "false", "no":
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_turned_off")
			connections.ToggleAllowConnect(chat.Id, false)
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

/*
	Connect to a chat

Use this command to connect to your chat!

Admins and Users both can use this.
*/
// connect handles the /connect command to establish connection to a chat.
// Allows users and admins to remotely manage chats through private messages.
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
		} else {
			connections.ConnectId(user.Id, chat.Id)
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

// Handler for Connection buttons
// connectionButtons handles inline keyboard callbacks for connection management.
// Processes admin and user command list requests from connection interface.
func (m moduleStruct) connectionButtons(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	msg := query.Message
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	userType := ""
	if decoded, ok := decodeCallbackData(query.Data, "connbtns"); ok {
		userType, _ = decoded.Field("t")
	}
	if userType == "" {
		log.Warnf("[Connections] Invalid callback data format: %s", query.Data)
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
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

	chatStat := m.isConnected(b, ctx, user.Id)
	if chatStat == 0 {
		return ext.EndGroups
	}

	switch userType {
	case "Admin":
		replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_connections_btns_admin_conn_cmds")
	case "User":
		replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_connections_btns_user_conn_cmds")
	case "Main":
		chatId := m.isConnected(b, ctx, user.Id)
		if chatId == 0 {
			return ext.EndGroups
		}
		pchat, err := b.GetChat(chatId, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connected")
		replyText = fmt.Sprintf(temp, pchat.Title)
		replyKb = keyboard.InitButtons(b, pchat.Id, user.Id)
	}

	_, _, err := msg.EditText(b,
		replyText,
		&gotgbot.EditMessageTextOpts{
			ReplyMarkup: replyKb,
			ParseMode:   formatting.HTML,
		},
	)
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

/*
	Disconnect from a chat

Used to disconnect from currently connected chat
*/
// disconnect handles the /disconnect command to end current chat connection.
// Removes the user's connection to allow connecting to different chats.
func (m moduleStruct) disconnect(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var text string

	if ctx.Message.Chat.Type == "private" {
		chatId := m.isConnected(b, ctx, user.Id)
		if chatId == 0 {
			return ext.EndGroups
		}

		connections.DisconnectId(user.Id)

		text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_disconnect_disconnected")
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

/*
	Function used to check if user is connected to a chat or not

If user is connected, chatId is returned else 0
*/
// isConnected checks if a user has an active connection to any chat.
// Returns the connected chat ID or 0 if no connection exists.
func (m moduleStruct) isConnected(b *gotgbot.Bot, ctx *ext.Context, userId int64) int64 {
	conn := connections.Connection(userId)
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if conn.Connected {
		return conn.ChatId
	}

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_not_connected")
	if query, ok := callbackQueryFromContext(ctx); ok && query != nil {
		_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		if err != nil {
			log.Error(err)
		}
		return 0
	}
	if ctx == nil || ctx.EffectiveMessage == nil {
		return 0
	}
	_, err := ctx.EffectiveMessage.Reply(b, text, nil)
	if err != nil {
		log.Error(err)
	}

	return 0
}

/*
	Used to reconnect to last chat connected by user

Both user and admin can use this command to connect to the previous chat
*/
// reconnect handles the /reconnect command to restore previous connection.
// Reconnects users to their last connected chat if they're still a member.
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
		chatId := connections.ReconnectId(user.Id)

		if chatId != 0 {
			gchat, err := b.GetChat(chatId, nil)
			if err != nil {
				log.Error(err)
				return err
			}

			// need to convert to chat type
			_chat := gchat.ToChat()

			if !chat_status.IsUserInChat(b, &_chat, user.Id) {
				return ext.EndGroups
			}

			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_reconnect_reconnected")
			text = fmt.Sprintf(temp, gchat.Title)
			connKeyboard = keyboard.InitButtons(b, gchat.Id, user.Id)
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

// LoadConnections registers all connection module handlers with the dispatcher.
// Sets up commands for managing remote chat connections and their callbacks.
func LoadConnections(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap.Store(ConnectionsModule.moduleName, true)

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

	chatID, err := strconv.Atoi(parts[1])
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	cochat, err := b.GetChat(int64(chatID), nil)
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

	// Synchronous DB write before user confirmation - fixes issue #694
	connections.ConnectId(user.Id, cochat.Id)

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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
