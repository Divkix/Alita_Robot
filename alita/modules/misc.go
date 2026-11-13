package modules

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/devs"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var (
	miscModule = moduleStruct{moduleName: "Misc"}
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}
)

func parseTranslateResponse(body []byte) (detectedLang, translatedText string, err error) {
	var payload []json.RawMessage
	if err = json.Unmarshal(body, &payload); err != nil {
		return "", "", err
	}

	if len(payload) == 0 {
		return "", "", fmt.Errorf("empty translate response")
	}

	var firstEntry []json.RawMessage
	if err = json.Unmarshal(payload[0], &firstEntry); err != nil || len(firstEntry) < 2 {
		return "", "", fmt.Errorf("unexpected translate response shape")
	}

	if err = json.Unmarshal(firstEntry[0], &translatedText); err != nil || translatedText == "" {
		return "", "", fmt.Errorf("missing translated text")
	}
	if err = json.Unmarshal(firstEntry[1], &detectedLang); err != nil || detectedLang == "" {
		return "", "", fmt.Errorf("missing detected language")
	}

	return detectedLang, translatedText, nil
}

func (moduleStruct) echomsg(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]

	replyMsg := msg.ReplyToMessage
	if replyMsg == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("misc_reply_to_someone")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	if len(args) > 0 {
		echoText := strings.Join(strings.Split(msg.OriginalHTML(), " ")[1:], " ")
		_, err := msg.Reply(b,
			echoText,
			&gotgbot.SendMessageOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId: replyMsg.MessageId,
				},
				ParseMode: formatting.Shtml().ParseMode,
			},
		)
		if err != nil {
			log.Error(err)
			return ext.EndGroups
		}
		if _, derr := msg.Delete(b, nil); derr != nil {
			log.Error(derr)
		}
	} else {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("misc_provide_content")
		_, _ = msg.Reply(b, text, nil)
	}

	return ext.EndGroups
}

func (moduleStruct) getId(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	}
	var builder strings.Builder
	builder.Grow(512)

	if userId != 0 {
		if reply := extraction.UserReply(msg); reply != nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			temp, _ := tr.GetString("misc_chat_id")
			text := fmt.Sprintf(temp, msg.Chat.Id)
			builder.WriteString(text)
			builder.WriteString("\n")
			if msg.IsTopicMessage {
				temp2, _ := tr.GetString("misc_thread_id")
				text = fmt.Sprintf(temp2, msg.MessageThreadId)
				builder.WriteString(text)
				builder.WriteString("\n")
			}
			if reply.From != nil {
				originalId := reply.From.Id
				_, user1Name, _ := extraction.GetUserInfo(originalId)
				temp3, _ := tr.GetString("misc_user_id")
				text = fmt.Sprintf(temp3, html.EscapeString(user1Name), originalId)
				builder.WriteString(text)
				builder.WriteString("\n")
			}

			if rpm := reply; rpm != nil {
				if frpm := rpm.ForwardOrigin; frpm != nil {
					if frpm.GetDate() != 0 {
						fwdd := frpm.MergeMessageOrigin()

						if fwdc := fwdd.SenderUser; fwdc != nil {
							user1Id := fwdc.Id
							_, user1Name, _ := extraction.GetUserInfo(user1Id)
							temp4, _ := tr.GetString("misc_forwarded_from_user")
							text = fmt.Sprintf(temp4, html.EscapeString(user1Name), user1Id)
							builder.WriteString(text)
							builder.WriteString("\n")
						}

						if fwdc := fwdd.Chat; fwdc != nil {
							temp5, _ := tr.GetString("misc_forwarded_from_chat")
							text = fmt.Sprintf(temp5, html.EscapeString(fwdc.Title), fwdc.Id)
							builder.WriteString(text)
							builder.WriteString("\n")
						}
					}
				}
			}
			if reply.Animation != nil {
				temp6, _ := tr.GetString("misc_gif_id")
				text = fmt.Sprintf(temp6, reply.Animation.FileId)
				builder.WriteString(text)
				builder.WriteString("\n")
			}
			if reply.Sticker != nil {
				temp7, _ := tr.GetString("misc_sticker_id")
				text = fmt.Sprintf(temp7, reply.Sticker.FileId)
				builder.WriteString(text)
				builder.WriteString("\n")
			}
		} else {
			_, name, _ := extraction.GetUserInfo(userId)
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			temp, _ := tr.GetString("misc_user_id_is")
			text := fmt.Sprintf(temp, html.EscapeString(name), userId)
			builder.WriteString(text)
		}
	} else {
		chat := ctx.EffectiveChat
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		if ctx.Message.Chat.Type == "private" {
			temp, _ := tr.GetString("misc_your_id_private")
			text := fmt.Sprintf(temp, chat.Id)
			builder.WriteString(text)
		} else {
			if msg.From == nil {
				temp, _ := tr.GetString("common_anonymous_user_error")
				builder.WriteString(temp)
			} else {
				temp, _ := tr.GetString("misc_your_id_group")
				text := fmt.Sprintf(temp, msg.From.Id, chat.Id)
				builder.WriteString(text)
			}
		}
	}

	_, err := msg.Reply(b,
		builder.String(),
		formatting.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) ping(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	pingingText, _ := tr.GetString("misc_pinging")
	sendStart := time.Now()
	sentMsg, err := msg.Reply(b, pingingText, &gotgbot.SendMessageOpts{
		ParseMode: formatting.HTML,
	})
	sendLatency := time.Since(sendStart)
	if err != nil {
		log.WithError(err).Error("[Ping] Failed to send ping response")
		return err
	}

	getMeStart := time.Now()
	_, getMeErr := b.GetMe(nil)
	getMeLatency := time.Since(getMeStart)
	if getMeErr != nil {
		log.WithError(getMeErr).Error("[Ping] Failed to call getMe")
	}

	resultTemplate, _ := tr.GetString("misc_ping_result")
	text := fmt.Sprintf(
		resultTemplate,
		getMeLatency.Milliseconds(),
		sendLatency.Milliseconds(),
		(sendLatency - getMeLatency).Milliseconds(),
	)
	var userId int64
	if msg.From != nil {
		userId = msg.From.Id
	}
	_, _, err = sentMsg.EditText(b, &gotgbot.EditMessageTextOpts{Text: text, ParseMode: formatting.HTML})
	if err != nil {
		log.WithError(err).Error("[Ping] Failed to edit ping response")
		return err
	}

	log.WithFields(log.Fields{
		"user_id":       userId,
		"send_latency":  sendLatency.Milliseconds(),
		"getme_latency": getMeLatency.Milliseconds(),
	}).Debug("[Ping] Response sent")

	return ext.EndGroups
}

func (moduleStruct) info(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender
	userId := extraction.ExtractUser(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		if sender == nil {
			return ext.EndGroups
		}
		userId = sender.Id()
	}

	username, name, found := extraction.GetUserInfo(userId)
	var text string

	if !found {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ = tr.GetString("misc_user_not_found")
	} else {

		user := &gotgbot.User{
			Id:        userId,
			Username:  username,
			FirstName: name,
		}

		if chat_status.IsChannelId(userId) {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			textTemplate, _ := tr.GetString("misc_channel_info_header")
			text = fmt.Sprintf(textTemplate, userId, html.EscapeString(user.FirstName))

			if user.Username != "" {
				usernameTemplate, _ := tr.GetString("misc_username")
				text += fmt.Sprintf("\n"+usernameTemplate, user.Username)
				linkTemplate, _ := tr.GetString("misc_channel_link")
				text += fmt.Sprintf("\n"+linkTemplate, user.Username)
			}
		} else {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			textTemplate, _ := tr.GetString("misc_user_info_header")
			text = fmt.Sprintf(textTemplate, userId, html.EscapeString(user.FirstName))
			if user.Username != "" {
				usernameTemplate, _ := tr.GetString("misc_username")
				text += fmt.Sprintf("\n"+usernameTemplate, user.Username)
			}
			linkTemplate, _ := tr.GetString("misc_user_link")
			text += fmt.Sprintf("\n"+linkTemplate, formatting.MentionHtml(user.Id, "link"))
			if user.Id == config.AppConfig.OwnerId {
				ownerText, _ := tr.GetString("misc_owner_info")
				text += "\n" + ownerText
			}
			if devs.GetTeamMemInfo(user.Id).IsDev {
				devText, _ := tr.GetString("misc_dev_info")
				text += "\n" + devText
			}
		}
	}

	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) translate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]

	var (
		origText string
		toLang   string
	)

	reply := extraction.UserReply(msg)
	if len(args) == 0 && reply == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("misc_need_text_and_lang")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if reply != nil {
		if reply.Text != "" {
			origText = reply.Text
		} else if reply.Caption != "" {
			origText = reply.Caption
		} else {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("misc_no_text_to_translate")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}
		if len(args) == 0 {
			toLang = "en"
		} else {
			toLang = args[0]
		}
	} else {
		if len(args[1:]) < 1 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("misc_provide_text_translate")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}
		toLang = args[0]
		origText = strings.Join(args[1:], " ")
	}
	req, err := httpClient.Get(fmt.Sprintf("https://clients5.google.com/translate_a/t?client=dict-chrome-ex&sl=auto&tl=%s&q=%s", toLang, url.QueryEscape(strings.TrimSpace(origText))))
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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
	all, err := io.ReadAll(io.LimitReader(req.Body, 1*1024*1024))
	if err != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("misc_translate_read_error")
		_, _ = msg.Reply(b, text+": "+err.Error(), nil)
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	detectedLang, translatedText, parseErr := parseTranslateResponse(all)
	if parseErr != nil {
		log.WithFields(log.Fields{
			"error":       parseErr,
			"target_lang": toLang,
			"response":    string(all),
		}).Warn("[Misc] Failed to parse translation response")
		text, _ := tr.GetString("misc_translate_parse_error")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}
	textTemplate, _ := tr.GetString("misc_translate_result")
	text := fmt.Sprintf(textTemplate, html.EscapeString(detectedLang), html.EscapeString(translatedText))
	if _, err := msg.Reply(b, text, formatting.Shtml()); err != nil {
		log.WithError(err).Warn("[Misc] failed to send translation")
	}
	return ext.EndGroups
}

func (moduleStruct) removeBotKeyboard(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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

	time.AfterFunc(1*time.Second, func() {
		defer error_handling.RecoverFromPanic("removeBotKeyboard", "misc")
		_ = helpers.DeleteMessageWithErrorHandling(b, rMsg.Chat.Id, rMsg.MessageId)
	})

	return ext.EndGroups
}

func (moduleStruct) stat(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	textTemplate, _ := tr.GetString("misc_total_messages")
	text := fmt.Sprintf(textTemplate, msg.Chat.Title, msg.MessageId+1)
	_, err := msg.Reply(b, text, nil)
	if err != nil {
		log.Error(err)
	}
	return ext.EndGroups
}

func LoadMisc(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(miscModule.moduleName, true)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "stat", Disableable: true,
		RequiredChecks: []helpers.CheckFunc{helpers.RequireGroup()},
	}, pipelineHandler(miscModule.stat))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "id", Disableable: true}, pipelineHandler(miscModule.getId))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "tell",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			func(c *helpers.CommandContext) bool {
				return c.User != nil && chat_status.IsUserAdmin(c.Bot, c.Chat.Id, c.User.Id)
			},
		},
	}, pipelineHandler(miscModule.echomsg))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "ping", Disableable: true}, pipelineHandler(miscModule.ping))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "info", Disableable: true}, pipelineHandler(miscModule.info))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "tr", Disableable: true}, pipelineHandler(miscModule.translate))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "removebotkeyboard"}, pipelineHandler(miscModule.removeBotKeyboard))
	registerMiscExtras(dispatcher)
}

func init() {
	RegisterLegacyModule("Misc", 60, LoadMisc)
}
