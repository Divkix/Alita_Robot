package modules

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
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
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permission checks
	if !chat_status.RequirePrivate(b, ctx, nil, false) {
		return ext.EndGroups
	}

	chatId := m.isConnected(b, ctx, user.Id)
	if chatId == 0 {
		return ext.EndGroups
	}

	chat, err := b.GetChat(chatId, nil)
	if err != nil {
		go func() {
			defer error_handling.RecoverFromPanic("DisconnectId", "connections")
			db.DisconnectId(user.Id)
		}()
		log.Error(err)
		return err
	}
	temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connected")
	_text := fmt.Sprintf(temp, chat.Title)
	connKeyboard := helpers.InitButtons(b, chat.Id, user.Id)
	_, err = msg.Reply(b,
		_text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: connKeyboard,
			ParseMode:   helpers.HTML,
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
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

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
			go func() {
				defer error_handling.RecoverFromPanic("ToggleAllowConnect", "connections")
				db.ToggleAllowConnect(chat.Id, true)
			}()
		case "off", "false", "no":
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_turned_off")
			go func() {
				defer error_handling.RecoverFromPanic("ToggleAllowConnect", "connections")
				db.ToggleAllowConnect(chat.Id, false)
			}()
		default:
			text, _ = tr.GetString("connections_invalid_option")
		}
	} else {
		currSetting := db.GetChatConnectionSetting(chat.Id).AllowConnect
		if currSetting {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_currently_on")
		} else {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_allow_connect_currently_off")
		}
	}

	_, err := msg.Reply(b, text, helpers.Shtml())
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
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
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
			go func() {
				defer error_handling.RecoverFromPanic("ConnectId", "connections")
				db.ConnectId(user.Id, chat.Id)
			}()
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connect_connected")
			text = fmt.Sprintf(temp, chat.Title)
			replyMarkup = helpers.InitButtons(b, chat.Id, user.Id)
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
								tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
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
			ParseMode:   helpers.HTML,
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
	query := ctx.CallbackQuery
	user := query.From
	msg := query.Message
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	userType := ""
	if decoded, ok := decodeCallbackData(query.Data, "connbtns"); ok {
		userType, _ = decoded.Field("t")
	} else {
		args := strings.Split(query.Data, ".")
		if len(args) >= 2 {
			userType = args[1]
		}
	}
	if userType == "" {
		log.Warnf("[Connections] Invalid callback data format: %s", query.Data)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid request."})
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
						CallbackData: encodeCallbackData("connbtns", map[string]string{"t": "Main"}, "connbtns.Main"),
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
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connections_btns_admin_conn_cmds")
		replyText = fmt.Sprintf(temp, m.adminCmdConnString())
	case "User":
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_connections_btns_user_conn_cmds")
		replyText = fmt.Sprintf(temp, m.userCmdConnString())
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
		replyKb = helpers.InitButtons(b, pchat.Id, user.Id)
	}

	_, _, err := msg.EditText(b,
		replyText,
		&gotgbot.EditMessageTextOpts{
			ReplyMarkup: replyKb,
			ParseMode:   helpers.HTML,
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
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	var text string

	if ctx.Message.Chat.Type == "private" {
		chatId := m.isConnected(b, ctx, user.Id)
		if chatId == 0 {
			return ext.EndGroups
		}

		go func() {
			defer error_handling.RecoverFromPanic("DisconnectId", "connections")
			db.DisconnectId(user.Id)
		}()

		text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_disconnect_disconnected")
	} else {
		text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_disconnect_need_pm")
	}

	_, err := msg.Reply(b, text, helpers.Shtml())
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
	conn := db.Connection(userId)
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	if conn.Connected {
		return conn.ChatId
	}

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_not_connected")
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
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	var (
		connKeyboard gotgbot.InlineKeyboardMarkup
		text         string
	)

	if ctx.Message.Chat.Type == "private" {
		user := chat_status.RequireUser(b, ctx, false)
		if user == nil {
			return ext.EndGroups
		}
		chatId := db.ReconnectId(user.Id)

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
			connKeyboard = helpers.InitButtons(b, gchat.Id, user.Id)
		} else {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_reconnect_no_last_chat")
		}
		_, err := msg.Reply(b, text,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: connKeyboard,
				ParseMode:   helpers.HTML,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}

	} else {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_reconnect_need_pm")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// adminCmdConnString returns a formatted list of admin commands available via connections.
// Used for displaying available commands in connection interface.
func (moduleStruct) adminCmdConnString() string {
	return "\n - /" + strings.Join(misc.AdminCmds, "\n - /")
}

// userCmdConnString returns a formatted list of user commands available via connections.
// Used for displaying available commands in connection interface.
func (moduleStruct) userCmdConnString() string {
	return "\n - /" + strings.Join(misc.UserCmds, "\n - /")
}

// LoadConnections registers all connection module handlers with the dispatcher.
// Sets up commands for managing remote chat connections and their callbacks.
func LoadConnections(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(ConnectionsModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("connect", ConnectionsModule.connect))
	dispatcher.AddHandler(handlers.NewCommand("disconnect", ConnectionsModule.disconnect))
	dispatcher.AddHandler(handlers.NewCommand("connection", ConnectionsModule.connection))
	dispatcher.AddHandler(handlers.NewCommand("reconnect", ConnectionsModule.reconnect))
	dispatcher.AddHandler(handlers.NewCommand("allowconnect", ConnectionsModule.allowConnect))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("connbtns"), ConnectionsModule.connectionButtons))
}
