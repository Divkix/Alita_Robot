package modules

import (
	"fmt"
	"html"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// reportExcerptRunes caps the reported text quoted when the message itself
// cannot be copied to the log channel.
const reportExcerptRunes = 300

// deliverReport posts the report card for a reported message to the group's
// log channel, or updates the card already posted for it. One card per
// message: it replies to a copy of the reported message (kept as evidence
// even if the original is deleted) and shows how many members reported it.
// Delivery failures are logged and never block the in-group reply.
func deliverReport(
	b *gotgbot.Bot,
	tr *i18n.Translator,
	chat *gotgbot.Chat,
	reporter *gotgbot.User,
	reported *gotgbot.Message,
	reason string,
) {
	settings := logchannels.Get(chat.Id)
	if settings == nil || !logchannels.CategoryEnabled(settings, logchannels.CategoryReports) {
		return
	}
	channelID := settings.LogChannelID

	rows, err := reports.OpenForMessage(chat.Id, reported.MessageId)
	if err != nil {
		log.Errorf("[Reports] failed to load open reports for chat %d message %d: %v", chat.Id, reported.MessageId, err)
		return
	}
	text := reportCardText(chat, reporter, reported.From, len(rows), reason)
	markup := reportCardMarkup(tr, chat, reported.MessageId)

	for _, row := range rows {
		if row.AdminChatId == nil || row.AdminMessageId == nil || *row.AdminChatId != channelID {
			continue
		}
		_, _, err := b.EditMessageText(&gotgbot.EditMessageTextOpts{
			Text:        text,
			ChatId:      channelID,
			MessageId:   *row.AdminMessageId,
			ParseMode:   formatting.HTML,
			ReplyMarkup: markup,
		})
		if err == nil {
			recordReportCard(chat.Id, reported.MessageId, channelID, *row.AdminMessageId)
			return
		}
		// The card is gone (deleted by an admin) or uneditable: post a new one.
		log.Debugf("[Reports] failed to update report card in %d: %v", channelID, err)
		break
	}

	opts := &gotgbot.SendMessageOpts{ParseMode: formatting.HTML, ReplyMarkup: markup}
	if evidenceID := sendReportEvidence(b, channelID, chat.Id, reported); evidenceID != 0 {
		opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: evidenceID, AllowSendingWithoutReply: true}
	}
	card, err := helpers.SendMessageWithErrorHandling(b, channelID, text, opts)
	if err != nil || card == nil {
		log.Debugf("[Reports] failed to send report card to %d: %v", channelID, err)
		return
	}
	recordReportCard(chat.Id, reported.MessageId, channelID, card.MessageId)
}

func recordReportCard(chatID, messageID, channelID, cardID int64) {
	if err := reports.SetAdminMessage(chatID, messageID, channelID, cardID); err != nil {
		log.Errorf("[Reports] failed to record report card for chat %d message %d: %v", chatID, messageID, err)
	}
}

// sendReportEvidence copies the reported message to the log channel. Messages
// that cannot be copied (service messages, protected content) are quoted as
// escaped text instead. It returns the id of the posted message, or 0.
func sendReportEvidence(b *gotgbot.Bot, channelID, chatID int64, reported *gotgbot.Message) int64 {
	copied, err := b.CopyMessage(channelID, chatID, reported.MessageId, &gotgbot.CopyMessageOpts{DisableNotification: true})
	if err == nil && copied != nil {
		return copied.MessageId
	}
	log.Debugf("[Reports] failed to copy reported message %d from %d: %v", reported.MessageId, chatID, err)

	quoted := reported.Text
	if quoted == "" {
		quoted = reported.Caption
	}
	if r := []rune(quoted); len(r) > reportExcerptRunes {
		quoted = string(r[:reportExcerptRunes]) + "…"
	}
	text := "<i>The reported message could not be copied.</i>"
	if quoted != "" {
		text += "\n" + html.EscapeString(quoted)
	}
	excerpt, err := helpers.SendMessageWithErrorHandling(b, channelID, text, &gotgbot.SendMessageOpts{
		ParseMode:           formatting.HTML,
		DisableNotification: true,
	})
	if err != nil || excerpt == nil {
		log.Debugf("[Reports] failed to send report excerpt to %d: %v", channelID, err)
		return 0
	}
	return excerpt.MessageId
}

// reportCardText follows the actionlog layout of the other log channel
// entries, which are English-only.
func reportCardText(chat *gotgbot.Chat, reporter, target *gotgbot.User, reportCount int, reason string) string {
	text := fmt.Sprintf(
		"<b>%s</b> (<code>%d</code>)\n#REPORT\nTarget: %s (<code>%d</code>)\nReports: %d\nLast reporter: %s",
		html.EscapeString(chat.Title),
		chat.Id,
		formatting.MentionHtml(target.Id, target.FirstName),
		target.Id,
		reportCount,
		formatting.MentionHtml(reporter.Id, reporter.FirstName),
	)
	if reason != "" {
		text += "\nReason: " + html.EscapeString(reason)
	}
	return text
}

func reportCardMarkup(tr *i18n.Translator, chat *gotgbot.Chat, messageID int64) gotgbot.InlineKeyboardMarkup {
	markup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{}}
	if link := chat_status.GetMessageLinkFromMessageId(chat, messageID); link != "" {
		label, _ := tr.GetString("reports_button_message")
		markup.InlineKeyboard = append(markup.InlineKeyboard, []gotgbot.InlineKeyboardButton{{Text: label, Url: link}})
	}
	return markup
}
