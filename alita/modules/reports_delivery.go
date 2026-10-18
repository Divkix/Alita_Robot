package modules

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"

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
// Failures are logged.
//
// It reports whether the admins already have the report without the in-group
// one: its card is in the log channel, or they decided this member's report
// of the message before.
//
// When the report could not be stored (stored is false), the card is posted
// on its own: the open reports cannot be counted or linked to it, so it has
// no buttons. A repeat of a stored report (created is false) leaves an
// existing card alone.
func deliverReport(
	b *gotgbot.Bot,
	tr *i18n.Translator,
	chat *gotgbot.Chat,
	reporter *gotgbot.User,
	reported *gotgbot.Message,
	reason string,
	stored, created bool,
) bool {
	settings := logchannels.Get(chat.Id)
	if settings == nil || !logchannels.CategoryEnabled(settings, logchannels.CategoryReports) {
		return false
	}
	channelID := settings.LogChannelID

	if !stored {
		sendReportCard(b, channelID, chat, reported,
			reportCardText(chat, reporter, reported.From, 1, reason), reportCardMarkup(tr, chat, reported.MessageId, 0))
		return false
	}

	rows, err := reports.OpenForMessage(chat.Id, reported.MessageId)
	if err != nil {
		log.Errorf("[Reports] failed to load open reports for chat %d message %d: %v", chat.Id, reported.MessageId, err)
		return false
	}
	if len(rows) == 0 {
		// A new report was closed by an admin before its card was posted.
		// A repeat of a report the admins already decided needs no card.
		return !created
	}
	text := reportCardText(chat, reporter, reported.From, len(rows), reason)
	// The card acts on every open report of the message; the oldest one
	// identifies them.
	markup := reportCardMarkup(tr, chat, reported.MessageId, rows[0].ID)

	for _, row := range rows {
		if row.AdminChatId == nil || row.AdminMessageId == nil || *row.AdminChatId != channelID {
			continue
		}
		if !created {
			return true
		}
		_, _, err := b.EditMessageText(&gotgbot.EditMessageTextOpts{
			Text:        text,
			ChatId:      channelID,
			MessageId:   *row.AdminMessageId,
			ParseMode:   formatting.HTML,
			ReplyMarkup: markup,
		})
		if err != nil && !isReportCardKept(err) {
			// The card was deleted, or the channel no longer takes the bot's
			// messages: post a new card. If the channel is unusable, that
			// fails too and the report gets no card.
			log.Debugf("[Reports] cannot update report card %d in %d: %v", *row.AdminMessageId, channelID, err)
			break
		}
		if err != nil {
			// The card is still there (flood wait, network, Telegram
			// outage): keep it rather than post a duplicate; the next
			// report updates its count.
			log.Warnf("[Reports] failed to update report card %d in %d: %v", *row.AdminMessageId, channelID, err)
		}
		recordReportCard(chat.Id, reported.MessageId, channelID, *row.AdminMessageId)
		return true
	}

	cardID := sendReportCard(b, channelID, chat, reported, text, markup)
	if cardID == 0 {
		return false
	}
	recordReportCard(chat.Id, reported.MessageId, channelID, cardID)
	return true
}

// isReportCardKept reports whether the card is still usable after editing it
// failed with err. Only failures that say nothing about the card keep it:
// a request that never got an answer from the Bot API (network, timeout), a
// flood wait (429), a Bot API server error (5xx), or an edit that changed
// nothing. Any other API error (the card deleted, the bot removed from the
// channel, the channel gone, missing rights) means the card can no longer be
// updated.
func isReportCardKept(err error) bool {
	var tgErr *gotgbot.TelegramError
	if !errors.As(err, &tgErr) {
		return true
	}
	if tgErr.Code == http.StatusTooManyRequests || tgErr.Code >= http.StatusInternalServerError {
		return true
	}
	return strings.Contains(tgErr.Description, "message is not modified")
}

// sendReportCard posts the evidence and the card to the log channel and
// returns the card's message id, or 0.
func sendReportCard(
	b *gotgbot.Bot,
	channelID int64,
	chat *gotgbot.Chat,
	reported *gotgbot.Message,
	text string,
	markup gotgbot.InlineKeyboardMarkup,
) int64 {
	opts := &gotgbot.SendMessageOpts{ParseMode: formatting.HTML, ReplyMarkup: markup}
	if evidenceID := sendReportEvidence(b, channelID, chat.Id, reported); evidenceID != 0 {
		opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: evidenceID, AllowSendingWithoutReply: true}
	}
	card, err := helpers.SendMessageWithErrorHandling(b, channelID, text, opts)
	if err != nil || card == nil {
		log.Debugf("[Reports] failed to send report card to %d: %v", channelID, err)
		if opts.ReplyParameters != nil {
			// Without its card the evidence is noise, and a retry posts it
			// again.
			if err := helpers.DeleteMessageWithErrorHandling(b, channelID, opts.ReplyParameters.MessageId); err != nil {
				log.Debugf("[Reports] failed to remove report evidence from %d: %v", channelID, err)
			}
		}
		return 0
	}
	return card.MessageId
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

// reportCardMarkup links the reported message and, for a stored report
// (reportID != 0), adds the ban/mute/delete/dismiss buttons. Each button
// carries only the report id; reportCardHandler checks everything else.
func reportCardMarkup(tr *i18n.Translator, chat *gotgbot.Chat, messageID, reportID int64) gotgbot.InlineKeyboardMarkup {
	markup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{}}
	if link := chat_status.GetMessageLinkFromMessageId(chat, messageID); link != "" {
		label, _ := tr.GetString("reports_button_message")
		markup.InlineKeyboard = append(markup.InlineKeyboard, []gotgbot.InlineKeyboardButton{{Text: label, Url: link}})
	}
	if reportID == 0 {
		return markup
	}
	for _, pair := range [][2]string{{"ban", "mute"}, {"delete", "dismiss"}} {
		var row []gotgbot.InlineKeyboardButton
		for _, action := range pair {
			data := encodeCallbackData(reportCardNamespace, map[string]string{
				"a": action,
				"i": strconv.FormatInt(reportID, 10),
			})
			if data == "" {
				continue
			}
			label, _ := tr.GetString(reportCardActions[action].buttonKey)
			row = append(row, gotgbot.InlineKeyboardButton{Text: label, CallbackData: data})
		}
		if len(row) > 0 {
			markup.InlineKeyboard = append(markup.InlineKeyboard, row)
		}
	}
	return markup
}
