package modules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

var reportsModule = moduleStruct{
	moduleName:   "Reports",
	handlerGroup: 8,
}

// report handles the /report command and @admin mentions to notify
// administrators about problematic messages with action buttons.
func (moduleStruct) report(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "report") {
		return ext.EndGroups
	}

	if msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reports_reply_to_report")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	// Check if From is nil (channel posts, deleted users)
	if msg.ReplyToMessage.From == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reports_cannot_report_channel")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	var (
		replyMsgId int64
		adminArray []int64
		err        error
	)

	if msg.ReplyToMessage.From.Id == user.Id {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reports_cannot_report_self")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	if replyMsg := msg.ReplyToMessage; replyMsg != nil {
		replyMsgId = replyMsg.MessageId
	} else {
		replyMsgId = msg.MessageId
	}
	reportprefs := db.GetChatReportSettings(chat.Id)

	// don't let blocked users report
	if string_handling.FindInInt64Slice(reportprefs.BlockedList, user.Id) {
		if chat_status.CanBotDelete(b, ctx, nil, true) {
			_, err := msg.Delete(b, nil)
			if err != nil {
				log.Error(err)
			}

		}
		return ext.EndGroups
	}

	if user.Id == 1087968824 || user.Id == 777000 || user.Id == 136817688 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reports_expose_yourself")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
	if msg.ReplyToMessage.From.Id == 1087968824 || msg.ReplyToMessage.From.Id == 777000 || msg.ReplyToMessage.From.Id == 136817688 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reports_special_account")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	if chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reports_admin_report")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if !reportprefs.Status {
		return ext.EndGroups
	}

	adminsAvail, admins := cache.GetAdminCacheList(chat.Id)
	if !adminsAvail {
		admins = cache.LoadAdminCache(b, chat.Id)
	}

	for i := range admins.UserInfo {
		admin := &admins.UserInfo[i]
		adminArray = append(adminArray, admin.User.Id)
	}

	reportedUser := msg.ReplyToMessage.From
	reportedMsgId := msg.ReplyToMessage.MessageId

	if reportedUser.Id == b.Id {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reports_why_report_myself")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if string_handling.FindInInt64Slice(adminArray, reportedUser.Id) {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reports_why_report_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	reportTemplate, _ := tr.GetString("reports_message_template")
	reported := fmt.Sprintf(
		reportTemplate,
		helpers.MentionHtml(user.Id, user.FirstName),
		helpers.MentionHtml(reportedUser.Id, reportedUser.FirstName),
	)
	var sb strings.Builder
	for _, adminUserId := range adminArray {
		if !db.GetUserReportSettings(adminUserId).Status {
			continue
		}
		sb.WriteString(helpers.MentionHtml(adminUserId, "\u2063"))
	}
	reported += sb.String()

	callbackData := "report." + "%s=" + fmt.Sprint(user.Id) + "=" + fmt.Sprint(reportedMsgId)
	_, err = msg.Reply(b,
		reported,
		&gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                replyMsgId,
				AllowSendingWithoutReply: true,
			},
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_message")
								return t
							}(),
							Url: helpers.GetMessageLinkFromMessageId(chat, reportedMsgId),
						},
					},
					{
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_kick")
								return t
							}(),
							CallbackData: fmt.Sprintf(callbackData, "kick"),
						},
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_ban")
								return t
							}(),
							CallbackData: fmt.Sprintf(callbackData, "ban"),
						},
					},
					{
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_delete")
								return t
							}(),
							CallbackData: fmt.Sprintf(callbackData, "delete"),
						},
					},
					{
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_resolved")
								return t
							}(),
							CallbackData: fmt.Sprintf(callbackData, "resolved"),
						},
					},
				},
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// reports handles the /reports command to manage reporting settings
// for both users and chats, including blocking and status changes.
func (moduleStruct) reports(b *gotgbot.Bot, ctx *ext.Context) error {
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]

	var (
		err       error
		replyText string
	)

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	if len(args) >= 1 {
		action := strings.ToLower(args[0])
		switch action {
		case "on", "yes", "true":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				replyText, _ = tr.GetString("reports_turned_on_personal")
				go func() {
					defer error_handling.RecoverFromPanic("SetUserReportSettings", "reports")
					db.SetUserReportSettings(user.Id, true)
				}()
			} else {
				replyText, _ = tr.GetString("reports_turned_on_group")
				go func() {
					defer error_handling.RecoverFromPanic("SetChatReportStatus", "reports")
					db.SetChatReportStatus(chat.Id, true)
				}()
			}
		case "off", "no", "false":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				replyText, _ = tr.GetString("reports_turned_off_personal")
				go func() {
					defer error_handling.RecoverFromPanic("SetUserReportSettings", "reports")
					db.SetUserReportSettings(user.Id, false)
				}()
			} else {
				replyText, _ = tr.GetString("reports_turned_off_group")
				go func() {
					defer error_handling.RecoverFromPanic("SetChatReportStatus", "reports")
					db.SetChatReportStatus(chat.Id, false)
				}()
			}
		case "block":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				replyText, _ = tr.GetString("reports_group_only")
			} else {
				if reply := msg.ReplyToMessage; reply != nil {
					// Check if From is nil (channel posts, deleted users, etc.)
					if reply.From == nil {
						replyText, _ = tr.GetString("reports_cannot_report_channel")
					} else {
						bUser := reply.From
						go func() {
							defer error_handling.RecoverFromPanic("BlockReportUser", "reports")
							db.BlockReportUser(chat.Id, bUser.Id)
						}()
						replyText, _ = tr.GetString("reports_user_blocked", i18n.TranslationParams{
							"s": helpers.MentionHtml(bUser.Id, bUser.FirstName),
						})
					}
				} else {
					replyText, _ = tr.GetString("reports_reply_to_block")
				}
			}
		case "unblock":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				replyText, _ = tr.GetString("reports_group_only")
			} else {
				if reply := msg.ReplyToMessage; reply != nil {
					// Check if From is nil (channel posts, deleted users, etc.)
					if reply.From == nil {
						replyText, _ = tr.GetString("reports_cannot_report_channel")
					} else {
						bUser := reply.From
						go func() {
							defer error_handling.RecoverFromPanic("UnblockReportUser", "reports")
							db.UnblockReportUser(chat.Id, bUser.Id)
						}()
						replyText, _ = tr.GetString("reports_user_unblocked", i18n.TranslationParams{
							"s": helpers.MentionHtml(bUser.Id, bUser.FirstName),
						})
					}
				} else {
					replyText, _ = tr.GetString("reports_reply_to_unblock")
				}
			}
		case "showblocklist":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				replyText, _ = tr.GetString("reports_group_only")
			} else {
				blockedUsers := db.GetChatReportSettings(chat.Id).BlockedList
				if len(blockedUsers) == 0 {
					replyText, _ = tr.GetString("reports_no_blocked_users")
				} else {
					var builder strings.Builder
					builder.Grow(256) // Pre-allocate capacity
					headerText, _ := tr.GetString("reports_blocked_users_header")
					builder.WriteString(headerText)
					for _, blockUserId := range blockedUsers {
						bUser, err := b.GetChat(blockUserId, nil)
						if err != nil {
							log.Error(err)
							continue
						}
						builder.WriteString("\n - ")
						builder.WriteString(helpers.MentionHtml(blockUserId, bUser.FirstName))
					}
					replyText = builder.String()
				}
			}
		default:
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			replyText, _ = tr.GetString("reports_invalid_input")
		}
	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		if msg.Chat.Type == "private" {
			rStatus := db.GetUserReportSettings(chat.Id).Status
			if rStatus {
				replyText, _ = tr.GetString("reports_preference_enabled_private")
			} else {
				replyText, _ = tr.GetString("reports_preference_disabled_private")
			}
		} else {
			rStatus := db.GetChatReportSettings(chat.Id).Status
			if rStatus {
				replyText, _ = tr.GetString("reports_status_enabled_group")
			} else {
				replyText, _ = tr.GetString("reports_status_disabled_group")
			}
		}
		hintText, _ := tr.GetString("reports_change_settings_hint")
		replyText += hintText
	}

	_, err = msg.Reply(b, replyText, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// markResolvedButtonHandler processes callback queries from report action buttons
// to kick, ban, delete messages, or mark reports as resolved.
func (moduleStruct) markResolvedButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := query.Message
	var replyQuery, replyText string

	// permissions check
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	args := strings.Split(strings.Split(query.Data, ".")[1], "=")
	action := args[0]
	_userId, _ := strconv.Atoi(args[1])
	_msgId, _ := strconv.Atoi(args[2])

	userId := int64(_userId)
	msgId := int64(_msgId)

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	switch action {
	case "kick":
		replyQuery, _ = tr.GetString("reports_success_kick")
		kickedText, _ := tr.GetString("reports_user_kicked")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": helpers.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", kickedText, actionBy)
		_, err := chat.BanMember(b, userId, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		time.Sleep(1 * time.Second) // wait for sometime before unbanning

		_, err = chat.UnbanMember(b, userId, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	case "ban":
		replyQuery, _ = tr.GetString("reports_success_ban")
		bannedText, _ := tr.GetString("reports_user_banned")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": helpers.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", bannedText, actionBy)
		_, err := chat.BanMember(b, userId, nil)
		if err != nil {
			log.Error(err)
			return err
		}

	case "delete":
		replyQuery, _ = tr.GetString("reports_success_delete")
		deletedText, _ := tr.GetString("reports_message_deleted")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": helpers.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", deletedText, actionBy)
		_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msgId)
	default:
		replyQuery, _ = tr.GetString("reports_resolved_success")
		replyText, _ = tr.GetString("reports_resolved_by", i18n.TranslationParams{"s": helpers.MentionHtml(user.Id, user.FirstName)})

	}
	_, _, err := msg.EditText(
		b,
		replyText,
		&gotgbot.EditMessageTextOpts{
			ChatId:    chat.Id,
			ParseMode: helpers.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b,
		&gotgbot.AnswerCallbackQueryOpts{
			Text: replyQuery,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// LoadReports registers all reports module handlers with the dispatcher,
// including report commands and @admin mention monitoring.
func LoadReports(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(reportsModule.moduleName, true)

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				r, _ := regexp.Compile("(?i)@admin(s)?")
				return r.MatchString(msg.Text)
			},
			reportsModule.report,
		),
		reportsModule.handlerGroup,
	)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("report."), reportsModule.markResolvedButtonHandler))
	dispatcher.AddHandler(handlers.NewCommand("report", reportsModule.report))
	misc.AddCmdToDisableable("report")
	dispatcher.AddHandler(handlers.NewCommand("reports", reportsModule.reports))
}
