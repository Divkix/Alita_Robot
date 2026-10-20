package modules

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
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

var adminMentionRegex = regexp.MustCompile(`(?i)(^|\s)@admins?(\s|$)`)

// maxReportReasonRunes caps the stored reason so one report cannot store an
// arbitrarily long message.
const maxReportReasonRunes = 500

// reportReason is the text after /report, or the whole @admin message.
func reportReason(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "/") {
		if i := strings.IndexFunc(text, unicode.IsSpace); i >= 0 {
			text = strings.TrimSpace(text[i:])
		} else {
			text = ""
		}
	}
	if r := []rune(text); len(r) > maxReportReasonRunes {
		text = string(r[:maxReportReasonRunes])
	}
	return text
}

// reportSnippet is the start of the reported message's text or caption. The
// log channel keeps the copy; the stored row only needs to recognise it.
func reportSnippet(m *gotgbot.Message) string {
	text := m.Text
	if text == "" {
		text = m.Caption
	}
	if r := []rune(text); len(r) > models.MaxReportSnippetRunes {
		text = string(r[:models.MaxReportSnippetRunes])
	}
	return text
}

func (moduleStruct) report(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	sender := ctx.EffectiveSender
	if sender == nil || sender.User == nil {
		return ext.EndGroups
	}
	user := sender.User
	msg := ctx.EffectiveMessage

	if chat_status.CheckDisabledCmd(b, msg, "report") {
		return ext.EndGroups
	}

	reportprefs := reports.GetChatReportSettings(chat.Id)
	if !reportprefs.Status {
		return ext.EndGroups
	}

	if msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_reply_to_report")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

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

	reason := reportReason(msg.GetText())
	created, storeErr := reports.Create(&models.Report{
		ChatId:            chat.Id,
		ReportedUserId:    reportedUser.Id,
		ReportedMessageId: reportedMsgId,
		ReporterId:        user.Id,
		Reason:            reason,
		Snippet:           reportSnippet(msg.ReplyToMessage),
	})
	if storeErr != nil {
		log.Errorf("[Reports] failed to store report in chat %d: %v", chat.Id, storeErr)
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if storeErr == nil && !created && reporterDecided(chat.Id, reportedMsgId, user.Id) {
		// The admins decided this member's report of the message: repeating
		// it neither pings them again nor counts on a card.
		text, _ := tr.GetString("reports_already_reviewed")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
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
							}),
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
							}),
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
							}),
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
							}),
						},
					},
				},
			},
		},
	)
	if err != nil {
		log.Error(err)
	}
	// The group gets the report with its admin mentions and buttons whether
	// or not the card is posted; the card goes second so that a slow log
	// channel does not hold up the group reply, and still goes when that
	// reply failed.
	deliverReport(b, tr, chat, user, msg.ReplyToMessage, reason, storeErr == nil, created)
	if err != nil {
		return err
	}
	return ext.EndGroups
}

// reporterDecided reports whether a member's stored report of a message was
// decided. On a lookup failure it says no, so the report goes out as usual.
func reporterDecided(chatID, messageID, reporterID int64) bool {
	rep, err := reports.GetByReporter(chatID, messageID, reporterID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[Reports] failed to load report of %d in chat %d: %v", reporterID, chatID, err)
		}
		return false
	}
	return rep.Status != models.ReportStatusOpen
}

// reportWatcher serves the group-8 @admin mention watcher. report() ends
// iteration for the /report command, but a watcher must let reactions (8),
// filters (9), pins (10) and log-channel (11) still see the message.
// report() logs its own errors before returning them.
func (m moduleStruct) reportWatcher(b *gotgbot.Bot, ctx *ext.Context) error {
	_ = m.report(b, ctx)
	return ext.ContinueGroups
}

//nolint:dupl // reports has symmetric block/unblock logic
func (moduleStruct) reports(b *gotgbot.Bot, ctx *ext.Context) error {
	connectedChat := resolveAdminChat(b, ctx, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
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
					builder.Grow(256)
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

// legacyReportStatus maps the in-group report buttons to report decisions.
var legacyReportStatus = map[string]string{
	"kick":     models.ReportStatusKicked,
	"ban":      models.ReportStatusBanned,
	"delete":   models.ReportStatusDeleted,
	"resolved": models.ReportStatusDismissed,
}

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
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if msg == nil {
		return answerInvalidCallback(b, ctx, query)
	}
	var replyQuery, replyText string

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	invalidActionText, _ := tr.GetString("reports_invalid_action")

	action := ""
	userIDRaw := ""
	msgIDRaw := ""
	if decoded, ok := decodeCallbackData(query.Data, "report"); ok {
		action, _ = decoded.Field("a")
		userIDRaw, _ = decoded.Field("u")
		msgIDRaw, _ = decoded.Field("m")
	}
	if action == "" || userIDRaw == "" || msgIDRaw == "" {
		log.Warnf("[Reports] Invalid callback data format: %s", query.Data)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: invalidActionText})
		return ext.EndGroups
	}
	switch action {
	case "kick", "ban", "delete", "resolved":
	default:
		log.Warnf("[Reports] Invalid callback action: %s", action)
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
	case "kick", "ban":
		if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
			return ext.EndGroups
		}
		if !chat_status.CanBotRestrict(b, ctx, chat) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_error", "chat_status_bot_restrict_error")
			return ext.EndGroups
		}
	case "delete":
		if !chat_status.CanUserDelete(b, ctx, chat, user.Id) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_delete_cmd_error", "chat_status_delete_button_error")
			return ext.EndGroups
		}
		if !chat_status.CanBotDelete(b, ctx, chat) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_delete_error", "chat_status_bot_delete_error")
			return ext.EndGroups
		}
	}

	// A message with stored reports is decided once: they are closed before
	// acting, so a decision already taken on the card or by another admin
	// stops this press, and opened again if the action fails. A message
	// reported before reports were stored has none and acts as before.
	status := legacyReportStatus[action]
	closed, err := reports.Resolve(chat.Id, msgId, status, user.Id)
	legacy := false
	if err == nil && len(closed) == 0 {
		var stored bool
		stored, err = reports.HasReports(chat.Id, msgId)
		if err == nil && stored {
			handledText, _ := tr.GetString("reports_already_handled")
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: handledText})
			return ext.EndGroups
		}
		legacy = true
	}
	if err != nil {
		log.Errorf("[Reports] failed to close reports of message %d in %d for %s: %v", msgId, chat.Id, action, err)
		failedText, _ := tr.GetString("reports_action_failed")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: failedText})
		return err
	}
	reopen := func() {
		if reopenErr := reports.Reopen(closed, status); reopenErr != nil {
			log.Errorf("[Reports] failed to reopen reports %v after a failed %s: %v", closed, action, reopenErr)
		}
	}

	switch action {
	case "kick":
		replyQuery, _ = tr.GetString("reports_success_kick")
		kickedText, _ := tr.GetString("reports_user_kicked")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", kickedText, actionBy)
		if err := kickMember(b, chat.Id, userId); err != nil {
			reopen()
			log.Error(err)
			return err
		}
	case "ban":
		replyQuery, _ = tr.GetString("reports_success_ban")
		bannedText, _ := tr.GetString("reports_user_banned")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", bannedText, actionBy)
		_, err := chat.BanMember(b, userId, nil)
		if err != nil {
			reopen()
			log.Error(err)
			return err
		}

	case "delete":
		replyQuery, _ = tr.GetString("reports_success_delete")
		deletedText, _ := tr.GetString("reports_message_deleted")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", deletedText, actionBy)
		if err := helpers.DeleteMessageWithErrorHandling(b, chat.Id, msgId); err != nil {
			reopen()
			log.Error(err)
			return err
		}
	case "resolved":
		replyQuery, _ = tr.GetString("reports_resolved_success")
		replyText, _ = tr.GetString("reports_resolved_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
	}
	if legacy {
		// Close any report stored while the action ran, so its card stops
		// acting.
		if _, err := reports.Resolve(chat.Id, msgId, status, user.Id); err != nil {
			log.Errorf("[Reports] failed to close reports of message %d in %d after %s: %v", msgId, chat.Id, action, err)
		}
	}
	_, _, err = msg.EditText(b, &gotgbot.EditMessageTextOpts{Text: replyText, ChatId: chat.Id,
		ParseMode: formatting.HTML})
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

func LoadReports(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(reportsModule.moduleName, true)

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				return adminMentionRegex.MatchString(msg.Text)
			},
			reportsModule.reportWatcher,
		),
		reportsModule.handlerGroup,
	)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("report"), reportsModule.markResolvedButtonHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(reportCardNamespace+"|"), reportsModule.reportCardHandler))
	dispatcher.AddHandler(handlers.NewCommand("report", reportsModule.report))
	helpers.AddCmdToDisableable("report")
	dispatcher.AddHandler(handlers.NewCommand("reports", reportsModule.reports))
	startReportRetentionSweeper()
}

func init() {
	RegisterLegacyModule("Reports", 110, LoadReports)
}
