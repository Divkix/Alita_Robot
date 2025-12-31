package modules

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var (
	miscModule = moduleStruct{moduleName: "Misc"}
	// HTTP client with timeout and connection pooling for external requests
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}
)

// echomsg handles the /tell command to make the bot echo a message
// as a reply to another message, requiring admin permissions.
func (moduleStruct) echomsg(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]

	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.IsUserAdmin(b, msg.Chat.Id, msg.From.Id) {
		return ext.EndGroups
	}

	replyMsg := msg.ReplyToMessage
	if replyMsg == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("misc_reply_to_someone")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	if len(args) > 0 {
		_, _ = msg.Delete(b, nil)
		_, err := msg.Reply(b,
			strings.Join(
				strings.Split(msg.OriginalHTML(), " ")[1:], " ",
			),
			&gotgbot.SendMessageOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId: replyMsg.MessageId,
				},
				ParseMode: helpers.Shtml().ParseMode,
			},
		)
		if err != nil {
			log.Error(err)
		}
	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("misc_provide_content")
		_, _ = msg.Reply(b, text, nil)
	}

	return ext.EndGroups
}

// getId handles the /id command to display IDs of users, chats,
// files, and forwarded messages with detailed information.
func (moduleStruct) getId(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	}
	var builder strings.Builder
	builder.Grow(512) // Pre-allocate capacity for better performance

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "id") {
		return ext.EndGroups
	}

	if userId != 0 {
		if msg.ReplyToMessage != nil {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("misc_chat_id")
			text := fmt.Sprintf(temp, msg.Chat.Id)
			builder.WriteString(text + "\n")
			if msg.IsTopicMessage {
				temp2, _ := tr.GetString("misc_thread_id")
				text = fmt.Sprintf(temp2, msg.MessageThreadId)
				builder.WriteString(text + "\n")
			}
			if msg.ReplyToMessage.From != nil {
				originalId := msg.ReplyToMessage.From.Id
				_, user1Name, _ := extraction.GetUserInfo(originalId)
				temp3, _ := tr.GetString("misc_user_id")
				text = fmt.Sprintf(temp3, user1Name, originalId)
				builder.WriteString(text + "\n")
			}

			if rpm := msg.ReplyToMessage; rpm != nil {
				if frpm := rpm.ForwardOrigin; frpm != nil {
					if frpm.GetDate() != 0 {
						fwdd := frpm.MergeMessageOrigin()

						if fwdc := fwdd.SenderUser; fwdc != nil {
							user1Id := fwdc.Id
							_, user1Name, _ := extraction.GetUserInfo(user1Id)
							temp4, _ := tr.GetString("misc_forwarded_from_user")
							text = fmt.Sprintf(temp4, user1Name, user1Id)
							builder.WriteString(text + "\n")
						}

						if fwdc := fwdd.Chat; fwdc != nil {
							temp5, _ := tr.GetString("misc_forwarded_from_chat")
							text = fmt.Sprintf(temp5, fwdc.Title, fwdc.Id)
							builder.WriteString(text + "\n")
						}
					}
				}
			}
			if msg.ReplyToMessage.Animation != nil {
				temp6, _ := tr.GetString("misc_gif_id")
				text = fmt.Sprintf(temp6, msg.ReplyToMessage.Animation.FileId)
				builder.WriteString(text + "\n")
			}
			if msg.ReplyToMessage.Sticker != nil {
				temp7, _ := tr.GetString("misc_sticker_id")
				text = fmt.Sprintf(temp7, msg.ReplyToMessage.Sticker.FileId)
				builder.WriteString(text + "\n")
			}
		} else {
			_, name, _ := extraction.GetUserInfo(userId)
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("misc_user_id_is")
			text := fmt.Sprintf(temp, name, userId)
			builder.WriteString(text)
		}
	} else {
		chat := ctx.EffectiveChat
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		if ctx.Message.Chat.Type == "private" {
			temp, _ := tr.GetString("misc_your_id_private")
			text := fmt.Sprintf(temp, chat.Id)
			builder.WriteString(text)
		} else {
			temp, _ := tr.GetString("misc_your_id_group")
			text := fmt.Sprintf(temp, msg.From.Id, chat.Id)
			builder.WriteString(text)
		}
	}

	_, err := msg.Reply(b,
		builder.String(),
		helpers.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// ping handles the /ping command to measure and display response time
func (moduleStruct) ping(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if command is disabled
	if chat_status.CheckDisabledCmd(b, msg, "ping") {
		return ext.EndGroups
	}

	// Calculate time since user sent the message
	userSentTime := time.Unix(int64(msg.Date), 0)
	latency := time.Since(userSentTime)

	// Simple response with latency
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	pingTemplate, _ := tr.GetString("misc_ping_response")
	text := fmt.Sprintf(pingTemplate, latency.Milliseconds())

	_, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode: helpers.HTML,
	})
	if err != nil {
		log.WithError(err).Error("[Ping] Failed to send ping response")
		return err
	}

	// Log latency for monitoring
	log.WithFields(log.Fields{
		"user_id": msg.From.Id,
		"latency": latency.Milliseconds(),
	}).Debug("[Ping] Response sent")

	return ext.EndGroups
}

// info handles the /info command to display detailed information
// about a user or channel including ID, name, and special roles.
func (moduleStruct) info(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender
	userId := extraction.ExtractUser(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		// 0 id is for self
		userId = sender.Id()
	}

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "info") {
		return ext.EndGroups
	}

	username, name, found := extraction.GetUserInfo(userId)
	var text string

	if !found {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ = tr.GetString("misc_user_not_found")
	} else {

		user := &gotgbot.User{
			Id:        userId,
			Username:  username,
			FirstName: name,
		}

		// If channel then this info
		if helpers.IsChannelID(userId) {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			textTemplate, _ := tr.GetString("misc_channel_info_header")
			text = fmt.Sprintf(textTemplate, userId, html.EscapeString(user.FirstName))

			if user.Username != "" {
				usernameTemplate, _ := tr.GetString("misc_username")
				text += fmt.Sprintf("\n"+usernameTemplate, user.Username)
				linkTemplate, _ := tr.GetString("misc_channel_link")
				text += fmt.Sprintf("\n"+linkTemplate, user.Username)
			}
		} else {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			textTemplate, _ := tr.GetString("misc_user_info_header")
			text = fmt.Sprintf(textTemplate, userId, html.EscapeString(user.FirstName))
			if user.Username != "" {
				usernameTemplate, _ := tr.GetString("misc_username")
				text += fmt.Sprintf("\n"+usernameTemplate, user.Username)
			}
			linkTemplate, _ := tr.GetString("misc_user_link")
			text += fmt.Sprintf("\n"+linkTemplate, helpers.MentionHtml(user.Id, "link"))
			if user.Id == config.AppConfig.OwnerId {
				ownerText, _ := tr.GetString("misc_owner_info")
				text += "\n" + ownerText
			}
			if db.GetTeamMemInfo(user.Id).IsDev {
				devText, _ := tr.GetString("misc_dev_info")
				text += "\n" + devText
			}
		}
	}

	_, err := msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// translate handles the /tr command to translate text using
// Google Translate API with automatic language detection.
func (moduleStruct) translate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "tr") {
		return ext.EndGroups
	}

	var (
		origText string
		toLang   string
	)

	if len(args) == 0 && msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("misc_need_text_and_lang")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if reply := msg.ReplyToMessage; reply != nil {
		if reply.Text != "" {
			origText = reply.Text
		} else if reply.Caption != "" {
			origText = reply.Caption
		} else {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("misc_no_text_to_translate")
			_, _ = msg.Reply(b, text, helpers.Shtml())
			return ext.EndGroups
		}
		if len(args) == 0 {
			toLang = "en"
		} else {
			toLang = args[0]
		}
	} else {
		// args[1:] leaves the language code and takes rest of the text
		if len(args[1:]) < 1 {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("misc_provide_text_translate")
			_, _ = msg.Reply(b, text, helpers.Shtml())
			return ext.EndGroups
		}
		// args[0] is the language code
		toLang = args[0]
		origText = strings.Join(args[1:], " ")
	}
	req, err := httpClient.Get(fmt.Sprintf("https://clients5.google.com/translate_a/t?client=dict-chrome-ex&sl=auto&tl=%s&q=%s", toLang, url.QueryEscape(strings.TrimSpace(origText))))
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("misc_translation_error")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Error(err)
		}
	}(req.Body)
	// Limit response size to 1MB to prevent memory exhaustion from malicious responses
	all, err := io.ReadAll(io.LimitReader(req.Body, 1*1024*1024))
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("misc_translate_read_error")
		_, _ = msg.Reply(b, text+": "+err.Error(), nil)
		return ext.EndGroups
	}
	data := strings.Split(strings.Trim(string(all), `"][`), `","`)
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	textTemplate, _ := tr.GetString("misc_translate_result")
	text := fmt.Sprintf(textTemplate, data[1], data[0])
	_, _ = msg.Reply(b, text, helpers.Shtml())
	return ext.EndGroups
}

// removeBotKeyboard handles the /removebotkeyboard command to
// remove stuck bot keyboards from the chat interface.
func (moduleStruct) removeBotKeyboard(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("misc_removing_keyboard")
	rMsg, err := msg.Reply(b,
		text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: &gotgbot.ReplyKeyboardRemove{
				RemoveKeyboard: true,
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	time.Sleep(1 * time.Second)
	_, err = rMsg.Delete(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// stat handles the /stat command to display the total number
// of messages in the current group chat.
func (moduleStruct) stat(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if !chat_status.RequireGroup(b, ctx, chat, false) {
		return ext.EndGroups
	}
	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "stat") {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	textTemplate, _ := tr.GetString("misc_total_messages")
	text := fmt.Sprintf(textTemplate, msg.Chat.Title, msg.MessageId+1)
	_, err := msg.Reply(b, text, nil)
	if err != nil {
		log.Error(err)
	}
	return ext.EndGroups
}

// LoadMisc registers all miscellaneous module handlers with the dispatcher,
// including utility commands for IDs, ping, translation, and stats.
func LoadMisc(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(miscModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("stat", miscModule.stat))
	misc.AddCmdToDisableable("stat")
	dispatcher.AddHandler(handlers.NewCommand("id", miscModule.getId))
	misc.AddCmdToDisableable("id")
	dispatcher.AddHandler(handlers.NewCommand("tell", miscModule.echomsg))
	dispatcher.AddHandler(handlers.NewCommand("ping", miscModule.ping))
	misc.AddCmdToDisableable("ping")
	dispatcher.AddHandler(handlers.NewCommand("info", miscModule.info))
	misc.AddCmdToDisableable("info")
	dispatcher.AddHandler(handlers.NewCommand("tr", miscModule.translate))
	misc.AddCmdToDisableable("tr")
	dispatcher.AddHandler(handlers.NewCommand("removebotkeyboard", miscModule.removeBotKeyboard))
}
