package modules

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/bans"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// banLogUnbanNamespace is the callback namespace of the Unban button under a
// ban's log channel entry: banlogunban|v1|r=<ban record id>.
const banLogUnbanNamespace = "banlogunban"

// unbanLogMarkup is the Unban button for the log channel entry of the ban
// with record recordID in chatID, labelled in the group's language, or nil
// when the ban has no record (its write failed, or the action was no ban).
func unbanLogMarkup(chatID, recordID int64) *gotgbot.InlineKeyboardMarkup {
	if recordID == 0 {
		return nil
	}
	data := encodeCallbackData(banLogUnbanNamespace, map[string]string{"r": strconv.FormatInt(recordID, 10)})
	if data == "" {
		return nil
	}
	tr := i18n.MustNewTranslator(lang.GetGroupLanguage(context.Background(), chatID))
	label, _ := tr.GetString("bans_log_unban_button")
	return &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: label, CallbackData: data}}}}
}

// banLogUnbanCallback lifts a ban from the Unban button of its log channel
// entry. The entry lives in the channel, but the presser is authorised
// against the group the ban is in, like the report card buttons: an admin of
// the channel alone can do nothing. The first press claims the record; any
// later one, or one after the ban was lifted or ended, is told so.
func (moduleStruct) banLogUnbanCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	recordID, readable, ok := logUnbanTarget(query)
	if !ok {
		return answerInvalidCallback(b, ctx, query)
	}
	entry := query.Message

	rec, err := bans.GetByID(recordID)
	// The channel has no language of its own: answer in the group's, the
	// language of the button.
	language := lang.GetLanguage(ctx)
	if rec != nil {
		language = lang.GetGroupLanguage(tracing.UpdateContext(ctx), rec.ChatId)
	}
	tr := i18n.MustNewTranslator(language)
	answer := func(key string) error {
		text, _ := tr.GetString(key)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}
	if err != nil {
		log.WithError(err).WithField("record_id", recordID).Error("[Bans] Failed to read the ban record of a log entry")
		return answer("bans_log_unban_failed")
	}
	// The button must also be pressed in the log channel of the record's
	// group, so an entry copied elsewhere cannot act.
	if rec != nil {
		if settings := logchannels.Get(rec.ChatId); settings == nil || settings.LogChannelID != entry.GetChat().Id {
			log.Warnf("[Bans] Unban button of record %d pressed in %d, which is not its group's log channel", recordID, entry.GetChat().Id)
			return answerInvalidCallback(b, ctx, query)
		}
	}
	// A removed record (the retention sweep keeps none of a ban in force)
	// was lifted long ago. The button of a ban lifted or ended elsewhere is
	// dropped.
	if rec == nil || !rec.IsOpen(time.Now()) {
		if readable != nil {
			dropStaleUnban(b, entry)
		}
		return answer("bans_log_unban_already")
	}

	admin := query.From
	source := &gotgbot.Chat{Id: rec.ChatId, Type: "supergroup"}
	if !reportCardAllowed(b, ctx, tr, source, admin.Id, "ban") {
		return ext.EndGroups
	}

	lifted, err := bans.LiftByID(rec.ID, admin.Id)
	if err != nil {
		log.WithError(err).WithField("record_id", rec.ID).Error("[Bans] Failed to claim the ban record")
		return answer("bans_log_unban_failed")
	}
	if !lifted {
		return answer("bans_log_unban_already")
	}
	if err := unbanRecorded(b, rec); err != nil {
		if reopenErr := bans.Reopen(rec.ID, admin.Id); reopenErr != nil {
			log.WithError(reopenErr).WithField("record_id", rec.ID).Error("[Bans] Failed to reopen the ban record after a failed unban")
		}
		log.WithError(err).WithFields(log.Fields{"chat_id": rec.ChatId, "user_id": rec.UserId}).Warn("[Bans] Unban from the log channel failed")
		return answer("bans_log_unban_failed")
	}

	// The same trail as /unban: the moderation event and the #unban entry.
	recordModerationEvent(rec.ChatId, admin.Id, rec.UserId, models.ModerationActionUnban, "")
	actionlog.Admin(b, source, &admin, "unban", rec.UserId, "")

	// Like the rest of the entry, the appended line is English.
	if readable != nil {
		if err := closeLogEntry(b, readable, fmt.Sprintf("Unbanned by %s (%d)", admin.FirstName, admin.Id)); err != nil {
			log.Warnf("[Bans] failed to close log entry %d in %d: %v", entry.GetMessageId(), entry.GetChat().Id, err)
		}
	}
	return answer("bans_log_unban_done")
}

// logUnbanTarget reads the record ID of an Unban press and the entry it was
// pressed on (nil when too old to read). Callback data can be forged: it
// must be the data of a button on the message it was pressed on, or a press
// on another message of the channel (a report card) would close that
// message. An entry too old to read cannot be checked: the press may still
// unban, but never edits it.
func logUnbanTarget(query *gotgbot.CallbackQuery) (int64, *gotgbot.Message, bool) {
	decoded, ok := decodeCallbackData(query.Data, banLogUnbanNamespace)
	if !ok || query.Message == nil {
		return 0, nil, false
	}
	rawID, _ := decoded.Field("r")
	recordID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || recordID <= 0 {
		return 0, nil, false
	}
	entry := query.Message
	readable := readableMessage(entry)
	if readable != nil && !carriesCallback(readable, query.Data) {
		log.Warnf("[Bans] Unban data %q pressed on message %d in %d, which has no such button", query.Data, entry.GetMessageId(), entry.GetChat().Id)
		return 0, nil, false
	}
	return recordID, readable, true
}

// dropStaleUnban removes the Unban button of an entry whose ban was lifted
// or ended elsewhere: it can do nothing more.
func dropStaleUnban(b *gotgbot.Bot, entry gotgbot.MaybeInaccessibleMessage) {
	if _, _, err := b.EditMessageReplyMarkup(&gotgbot.EditMessageReplyMarkupOpts{
		ChatId: entry.GetChat().Id, MessageId: entry.GetMessageId(),
	}); err != nil {
		log.Debugf("[Bans] failed to drop the stale Unban button of entry %d in %d: %v", entry.GetMessageId(), entry.GetChat().Id, err)
	}
}

// carriesCallback reports whether msg has an inline button with data.
func carriesCallback(msg *gotgbot.Message, data string) bool {
	if msg.ReplyMarkup == nil {
		return false
	}
	for _, row := range msg.ReplyMarkup.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == data {
				return true
			}
		}
	}
	return false
}

// unbanRecorded lifts rec's ban in Telegram the way /unban does: a channel
// through the sender-chat endpoint, a user only if still banned.
func unbanRecorded(b *gotgbot.Bot, rec *models.BanRecord) error {
	if chat_status.IsChannelId(rec.UserId) {
		_, err := b.UnbanChatSenderChat(rec.ChatId, rec.UserId, nil)
		return err
	}
	_, err := b.UnbanChatMember(rec.ChatId, rec.UserId, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: true})
	return err
}
