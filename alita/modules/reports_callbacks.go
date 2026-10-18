package modules

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// reportCardNamespace is the callback namespace of the report card buttons:
// rpt|v1|a=<action>&i=<report id>.
const reportCardNamespace = "rpt"

type reportCardAction struct {
	buttonKey  string
	successKey string
	status     string
}

var reportCardActions = map[string]reportCardAction{
	"ban":     {buttonKey: "reports_button_ban", successKey: "reports_success_ban", status: models.ReportStatusBanned},
	"mute":    {buttonKey: "reports_button_mute", successKey: "reports_success_mute", status: models.ReportStatusMuted},
	"delete":  {buttonKey: "reports_button_delete", successKey: "reports_success_delete", status: models.ReportStatusDeleted},
	"dismiss": {buttonKey: "reports_button_dismiss", successKey: "reports_resolved_success", status: models.ReportStatusDismissed},
}

// reportCardHandler acts on a report card in the log channel. The card lives
// in the channel, but the presser is authorised against the group the report
// came from: an admin of the channel alone can do nothing.
func (moduleStruct) reportCardHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	answer := func(key string) error {
		text, _ := tr.GetString(key)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	decoded, ok := decodeCallbackData(query.Data, reportCardNamespace)
	if !ok || query.Message == nil {
		return answer("reports_invalid_action")
	}
	actionName, _ := decoded.Field("a")
	action, ok := reportCardActions[actionName]
	idRaw, _ := decoded.Field("i")
	reportID, err := strconv.ParseInt(idRaw, 10, 64)
	if !ok || err != nil || reportID <= 0 {
		log.Warnf("[Reports] Invalid report card callback: %s", query.Data)
		return answer("reports_invalid_action")
	}

	rep, err := reports.GetByID(reportID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[Reports] failed to load report %d: %v", reportID, err)
		}
		return answer("reports_invalid_action")
	}
	// Callback data can be forged: the report must belong to the card that
	// was pressed.
	card := query.Message
	if rep.AdminChatId == nil || rep.AdminMessageId == nil ||
		*rep.AdminChatId != card.GetChat().Id || *rep.AdminMessageId != card.GetMessageId() {
		log.Warnf("[Reports] report %d pressed on card %d in %d, which is not its card",
			reportID, card.GetMessageId(), card.GetChat().Id)
		return answer("reports_invalid_action")
	}
	// The channel has no language of its own: answer in the reported group's,
	// the language of the card's buttons.
	tr = i18n.MustNewTranslator(lang.GetGroupLanguage(tracing.UpdateContext(ctx), rep.ChatId))
	// Resolve closes every open report of the message, so a card whose own
	// report is closed must not close a newer one that has its own card.
	if rep.Status != models.ReportStatusOpen {
		return answer("reports_already_handled")
	}

	user := query.From
	source := &gotgbot.Chat{Id: rep.ChatId, Type: "supergroup"}
	if !reportCardAllowed(b, ctx, tr, source, user.Id, actionName) {
		return ext.EndGroups
	}

	closed, err := reports.Resolve(rep.ChatId, rep.ReportedMessageId, action.status, user.Id)
	if err != nil {
		log.Errorf("[Reports] failed to resolve report %d: %v", reportID, err)
		_ = answer("reports_action_failed")
		return err
	}
	if len(closed) == 0 {
		return answer("reports_already_handled")
	}

	if err := applyReportDecision(b, actionName, rep); err != nil {
		if reopenErr := reports.Reopen(closed, action.status); reopenErr != nil {
			log.Errorf("[Reports] failed to reopen reports %v after a failed %s: %v", closed, actionName, reopenErr)
		}
		log.Errorf("[Reports] %s on report %d failed: %v", actionName, reportID, err)
		_ = answer("reports_action_failed")
		return err
	}

	closeReportCard(b, card, action.status, user)
	return answer(action.successKey)
}

// reportCardAllowed checks the presser and the bot in the report's source
// group and answers the callback through tr when they lack a right.
func reportCardAllowed(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, source *gotgbot.Chat, userID int64, action string) bool {
	responder := chat_status.NewPermissionResponder(b)
	refuse := func(cmdKey, btnKey string) bool {
		responder.Respond(ctx, cmdKey, btnKey, chat_status.WithTranslator(tr))
		return false
	}
	if !chat_status.IsUserAdmin(b, source.Id, userID) {
		return refuse("chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error")
	}
	switch action {
	case "ban", "mute":
		if !chat_status.CanUserRestrict(b, ctx, source, userID) {
			return refuse("chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		}
		if !chat_status.CanBotRestrict(b, ctx, source) {
			return refuse("chat_status_bot_restrict_error", "chat_status_bot_restrict_error")
		}
	case "delete":
		if !chat_status.CanUserDelete(b, ctx, source, userID) {
			return refuse("chat_status_delete_cmd_error", "chat_status_delete_button_error")
		}
		if !chat_status.CanBotDelete(b, ctx, source) {
			return refuse("chat_status_bot_delete_error", "chat_status_bot_delete_error")
		}
	}
	return true
}

func applyReportDecision(b *gotgbot.Bot, action string, rep *models.Report) error {
	var err error
	switch action {
	case "ban":
		_, err = b.BanChatMember(rep.ChatId, rep.ReportedUserId, nil)
	case "mute":
		_, err = b.RestrictChatMember(rep.ChatId, rep.ReportedUserId, MutedPermissions, nil)
	case "delete":
		err = helpers.DeleteMessageWithErrorHandling(b, rep.ChatId, rep.ReportedMessageId)
	}
	return err
}

// closeReportCard appends the decision to the card and drops its action
// buttons, keeping the link to the reported message. Like the rest of the
// card, the decision line is English.
func closeReportCard(b *gotgbot.Bot, card gotgbot.MaybeInaccessibleMessage, status string, admin gotgbot.User) {
	var msg *gotgbot.Message
	switch m := card.(type) {
	case gotgbot.Message:
		msg = &m
	case *gotgbot.Message:
		msg = m
	}
	chatID, messageID := card.GetChat().Id, card.GetMessageId()
	if msg == nil || msg.Text == "" {
		// The card is too old to read: only drop its buttons.
		if _, _, err := b.EditMessageReplyMarkup(&gotgbot.EditMessageReplyMarkupOpts{ChatId: chatID, MessageId: messageID}); err != nil {
			log.Warnf("[Reports] failed to close report card %d in %d: %v", messageID, chatID, err)
		}
		return
	}

	links := gotgbot.InlineKeyboardMarkup{}
	if msg.ReplyMarkup != nil {
		for _, row := range msg.ReplyMarkup.InlineKeyboard {
			for _, button := range row {
				if button.Url != "" {
					links.InlineKeyboard = append(links.InlineKeyboard, []gotgbot.InlineKeyboardButton{button})
				}
			}
		}
	}
	// The appended line has no entities, so the card's own entities keep
	// their offsets.
	_, _, err := b.EditMessageText(&gotgbot.EditMessageTextOpts{
		ChatId:      chatID,
		MessageId:   messageID,
		Text:        fmt.Sprintf("%s\n\nDecision: %s by %s (%d)", msg.Text, status, admin.FirstName, admin.Id),
		Entities:    msg.Entities,
		ReplyMarkup: links,
	})
	if err != nil {
		log.Warnf("[Reports] failed to close report card %d in %d: %v", messageID, chatID, err)
	}
}
