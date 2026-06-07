package modules

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var reportsModule = moduleStruct{
	moduleName:   "Reports",
	handlerGroup: 8,
}

// adminMentionRegex matches @admin and @admins mentions in messages.
// Pre-compiled once at init time to avoid per-message compilation overhead.
var adminMentionRegex = regexp.MustCompile(`(?i)(^|\s)@admins?(\s|$)`)

// report handles the /report command and @admin mentions to notify
// administrators about problematic messages with action buttons.
func (moduleStruct) report(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	sender := ctx.EffectiveSender
	if sender == nil || sender.User == nil {
		return ext.EndGroups
	}
	user := sender.User
	msg := ctx.EffectiveMessage

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "report") {
		return ext.EndGroups
	}

	if msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_reply_to_report")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	// Check if From is nil (channel posts, deleted users)
	if msg.ReplyToMessage.From == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_cannot_report_self")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	if replyMsg := msg.ReplyToMessage; replyMsg != nil {
		replyMsgId = replyMsg.MessageId
	} else {
		replyMsgId = msg.MessageId
	}
	reportprefs := reports.GetChatReportSettings(chat.Id)

	// don't let blocked users report
	if slices.Contains(reportprefs.BlockedList, user.Id) {
		if chat_status.CanBotDelete(b, ctx, nil) {
			_, err := msg.Delete(b, nil)
			if err != nil {
				log.Error(err)
			}

		}
		return ext.EndGroups
	}

	if user.Id == 1087968824 || user.Id == 777000 || user.Id == 136817688 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_expose_yourself")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
	if msg.ReplyToMessage.From.Id == 1087968824 || msg.ReplyToMessage.From.Id == 777000 || msg.ReplyToMessage.From.Id == 136817688 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_special_account")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	if chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_admin_report")
		_, err := msg.Reply(b, text, formatting.Shtml())
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
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_why_report_myself")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if slices.Contains(adminArray, reportedUser.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_why_report_admin")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	reportTemplate, _ := tr.GetString("reports_message_template")
	reported := fmt.Sprintf(
		reportTemplate,
		formatting.MentionHtml(user.Id, user.FirstName),
		formatting.MentionHtml(reportedUser.Id, reportedUser.FirstName),
	)
	var sb strings.Builder
	for _, adminUserId := range adminArray {
		if !reports.GetUserReportSettings(adminUserId).Status {
			continue
		}
		sb.WriteString(formatting.MentionHtml(adminUserId, "\u2063"))
	}
	reported += sb.String()

	_, err = msg.Reply(b,
		reported,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                replyMsgId,
				AllowSendingWithoutReply: true,
			},
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_message")
								return t
							}(),
							Url: chat_status.GetMessageLinkFromMessageId(chat, reportedMsgId),
						},
					},
					{
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_kick")
								return t
							}(),
							CallbackData: encodeCallbackData("report", map[string]string{
								"a": "kick",
								"u": fmt.Sprint(reportedUser.Id),
								"m": fmt.Sprint(reportedMsgId),
							}, fmt.Sprintf("report.kick=%d=%d", reportedUser.Id, reportedMsgId)),
						},
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_ban")
								return t
							}(),
							CallbackData: encodeCallbackData("report", map[string]string{
								"a": "ban",
								"u": fmt.Sprint(reportedUser.Id),
								"m": fmt.Sprint(reportedMsgId),
							}, fmt.Sprintf("report.ban=%d=%d", reportedUser.Id, reportedMsgId)),
						},
					},
					{
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_delete")
								return t
							}(),
							CallbackData: encodeCallbackData("report", map[string]string{
								"a": "delete",
								"u": fmt.Sprint(reportedUser.Id),
								"m": fmt.Sprint(reportedMsgId),
							}, fmt.Sprintf("report.delete=%d=%d", reportedUser.Id, reportedMsgId)),
						},
					},
					{
						{
							Text: func() string {
								tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
								t, _ := tr.GetString("reports_button_resolved")
								return t
							}(),
							CallbackData: encodeCallbackData("report", map[string]string{
								"a": "resolved",
								"u": fmt.Sprint(reportedUser.Id),
								"m": fmt.Sprint(reportedMsgId),
							}, fmt.Sprintf("report.resolved=%d=%d", reportedUser.Id, reportedMsgId)),
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
//
//nolint:dupl // reports has symmetric block/unblock logic
func (moduleStruct) reports(b *gotgbot.Bot, ctx *ext.Context) error {
	// connection status
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	sender := ctx.EffectiveSender
	if sender == nil || sender.User == nil {
		return ext.EndGroups
	}
	user := sender.User
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]

	var (
		err       error
		replyText string
	)

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}

	if len(args) >= 1 {
		action := strings.ToLower(args[0])
		switch action {
		case "on", "yes", "true":
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				err := reports.SetUserReportSettings(user.Id, true)
				if err != nil {
					replyText, _ = tr.GetString("common_settings_save_failed")
				} else {
					replyText, _ = tr.GetString("reports_turned_on_personal")
				}
			} else {
				err := reports.SetChatReportStatus(chat.Id, true)
				if err != nil {
					replyText, _ = tr.GetString("common_settings_save_failed")
				} else {
					replyText, _ = tr.GetString("reports_turned_on_group")
				}
			}
		case "off", "no", "false":
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				err := reports.SetUserReportSettings(user.Id, false)
				if err != nil {
					replyText, _ = tr.GetString("common_settings_save_failed")
				} else {
					replyText, _ = tr.GetString("reports_turned_off_personal")
				}
			} else {
				err := reports.SetChatReportStatus(chat.Id, false)
				if err != nil {
					replyText, _ = tr.GetString("common_settings_save_failed")
				} else {
					replyText, _ = tr.GetString("reports_turned_off_group")
				}
			}
		case "block":
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				replyText, _ = tr.GetString("reports_group_only")
			} else {
				if reply := msg.ReplyToMessage; reply != nil {
					// Check if From is nil (channel posts, deleted users, etc.)
					if reply.From == nil {
						replyText, _ = tr.GetString("reports_cannot_report_channel")
					} else {
						bUser := reply.From
						err := reports.BlockReportUser(chat.Id, bUser.Id)
						if err != nil {
							replyText, _ = tr.GetString("common_settings_save_failed")
						} else {
							replyText, _ = tr.GetString("reports_user_blocked", i18n.TranslationParams{
								"s": formatting.MentionHtml(bUser.Id, bUser.FirstName),
							})
						}
					}
				} else {
					replyText, _ = tr.GetString("reports_reply_to_block")
				}
			}
		case "unblock":
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				replyText, _ = tr.GetString("reports_group_only")
			} else {
				if reply := msg.ReplyToMessage; reply != nil {
					// Check if From is nil (channel posts, deleted users, etc.)
					if reply.From == nil {
						replyText, _ = tr.GetString("reports_cannot_report_channel")
					} else {
						bUser := reply.From
						err := reports.UnblockReportUser(chat.Id, bUser.Id)
						if err != nil {
							replyText, _ = tr.GetString("common_settings_save_failed")
						} else {
							replyText, _ = tr.GetString("reports_user_unblocked", i18n.TranslationParams{
								"s": formatting.MentionHtml(bUser.Id, bUser.FirstName),
							})
						}
					}
				} else {
					replyText, _ = tr.GetString("reports_reply_to_unblock")
				}
			}
		case "showblocklist":
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			if msg.Chat.Type == "private" {
				replyText, _ = tr.GetString("reports_group_only")
			} else {
				blockedUsers := reports.GetChatReportSettings(chat.Id).BlockedList
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
						builder.WriteString(formatting.MentionHtml(blockUserId, bUser.FirstName))
					}
					replyText = builder.String()
				}
			}
		default:
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			replyText, _ = tr.GetString("reports_invalid_input")
		}
	} else {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		if msg.Chat.Type == "private" {
			rStatus := reports.GetUserReportSettings(user.Id).Status
			if rStatus {
				replyText, _ = tr.GetString("reports_preference_enabled_private")
			} else {
				replyText, _ = tr.GetString("reports_preference_disabled_private")
			}
		} else {
			rStatus := reports.GetChatReportSettings(chat.Id).Status
			if rStatus {
				replyText, _ = tr.GetString("reports_status_enabled_group")
			} else {
				replyText, _ = tr.GetString("reports_status_disabled_group")
			}
		}
		hintText, _ := tr.GetString("reports_change_settings_hint")
		replyText += hintText
	}

	_, err = msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// markResolvedButtonHandler processes callback queries from report action buttons
// to kick, ban, delete messages, or mark reports as resolved.
func (moduleStruct) markResolvedButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := query.Message
	var replyQuery, replyText string

	// permissions check
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	invalidActionText, _ := tr.GetString("reports_invalid_action")

	action := ""
	userIDRaw := ""
	msgIDRaw := ""
	if decoded, ok := decodeCallbackData(query.Data, "report"); ok {
		action, _ = decoded.Field("a")
		userIDRaw, _ = decoded.Field("u")
		msgIDRaw, _ = decoded.Field("m")
	} else {
		parts := strings.Split(query.Data, ".")
		if len(parts) >= 2 {
			args := strings.Split(parts[1], "=")
			if len(args) >= 3 {
				action = args[0]
				userIDRaw = args[1]
				msgIDRaw = args[2]
			}
		}
	}
	if action == "" || userIDRaw == "" || msgIDRaw == "" {
		log.Warnf("[Reports] Invalid callback data format: %s", query.Data)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: invalidActionText})
		return ext.EndGroups
	}
	userId, err := strconv.ParseInt(userIDRaw, 10, 64)
	if err != nil {
		log.Warnf("[Reports] Invalid user ID in callback: %s", userIDRaw)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: invalidActionText})
		return ext.EndGroups
	}
	msgId, err := strconv.ParseInt(msgIDRaw, 10, 64)
	if err != nil {
		log.Warnf("[Reports] Invalid message ID in callback: %s", msgIDRaw)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: invalidActionText})
		return ext.EndGroups
	}
	switch action {
	case "kick":
		replyQuery, _ = tr.GetString("reports_success_kick")
		kickedText, _ := tr.GetString("reports_user_kicked")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", kickedText, actionBy)
		_, err := chat.BanMember(b, userId, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		// Unban after 1s in a background goroutine to avoid blocking a dispatcher worker.
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("[Reports] panic in delayed unban: %v", r)
				}
			}()
			timer := time.NewTimer(1 * time.Second)
			defer timer.Stop()
			<-timer.C
			if _, unbanErr := chat.UnbanMember(b, userId, nil); unbanErr != nil {
				log.Errorf("[Reports] Failed to unban %d after kick: %v", userId, unbanErr)
			}
		}()
	case "ban":
		replyQuery, _ = tr.GetString("reports_success_ban")
		bannedText, _ := tr.GetString("reports_user_banned")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", bannedText, actionBy)
		_, err := chat.BanMember(b, userId, nil)
		if err != nil {
			log.Error(err)
			return err
		}

	case "delete":
		replyQuery, _ = tr.GetString("reports_success_delete")
		deletedText, _ := tr.GetString("reports_message_deleted")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", deletedText, actionBy)
		_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msgId)
	default:
		replyQuery, _ = tr.GetString("reports_resolved_success")
		replyText, _ = tr.GetString("reports_resolved_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})

	}
	_, _, err = msg.EditText(
		b,
		replyText,
		&gotgbot.EditMessageTextOpts{
			ChatId:    chat.Id,
			ParseMode: formatting.HTML,
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
	DefaultHelpRegistry().AbleMap.Store(reportsModule.moduleName, true)

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				return adminMentionRegex.MatchString(msg.Text)
			},
			reportsModule.report,
		),
		reportsModule.handlerGroup,
	)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("report"), reportsModule.markResolvedButtonHandler))
	dispatcher.AddHandler(handlers.NewCommand("report", reportsModule.report))
	helpers.AddCmdToDisableable("report")
	dispatcher.AddHandler(handlers.NewCommand("reports", reportsModule.reports))
}

func init() {
	RegisterLegacyModule("Reports", 110, LoadReports)
}
