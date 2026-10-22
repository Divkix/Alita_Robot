package modules

import (
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// reportAutoMuteDuration caps a report auto-mute. Telegram ends the mute at
// its until_date, so it never outlasts this even when the bot is down or no
// admin decides.
const reportAutoMuteDuration = 24 * time.Hour

// autoMuteReported mutes the author of a reported message once threshold
// different members have open reports against them in chat (/reports mute
// <N>). It never mutes the bot, an admin, an approved user, an anonymous or
// channel sender, or a member another restriction already applies to, and
// mutes a user once until an admin decides. Telegram's until_date ends the
// mute after reportAutoMuteDuration. Failures are logged.
func autoMuteReported(b *gotgbot.Bot, tr *i18n.Translator, chat *gotgbot.Chat, reported *gotgbot.Message, threshold int) {
	if threshold <= 0 {
		return
	}
	target := autoMuteTarget(b, chat.Id, reported)
	if target == nil {
		return
	}
	count, err := reports.CountOpenReporters(chat.Id, target.Id)
	if err != nil {
		log.Errorf("[Reports] failed to count reporters of %d in %d: %v", target.Id, chat.Id, err)
		return
	}
	if count < threshold {
		return
	}

	// Telegram takes until_date in whole seconds; the stored value must match
	// the one it reports back.
	until := time.Now().Add(reportAutoMuteDuration).Truncate(time.Second)
	claimed, err := reports.ClaimAutoMute(chat.Id, target.Id, until)
	if err != nil {
		log.Errorf("[Reports] failed to record auto-mute of %d in %d: %v", target.Id, chat.Id, err)
		return
	}
	if !claimed {
		return
	}
	release := func() {
		if _, err := reports.ReleaseAutoMute(chat.Id, target.Id); err != nil {
			log.Errorf("[Reports] failed to drop auto-mute of %d in %d: %v", target.Id, chat.Id, err)
		}
	}

	// Only a plain member: an admin is never muted, and a restriction set
	// by an admin must not be replaced by one that ends sooner.
	member, err := b.GetChatMember(chat.Id, target.Id, nil)
	if err != nil || member.GetStatus() != gotgbot.ChatMemberStatusMember {
		if err != nil {
			log.Debugf("[Reports] cannot check %d in %d before auto-mute: %v", target.Id, chat.Id, err)
		}
		release()
		return
	}
	if _, err := b.RestrictChatMember(chat.Id, target.Id, MutedPermissions, &gotgbot.RestrictChatMemberOpts{
		UntilDate: until.Unix(),
	}); err != nil {
		log.Debugf("[Reports] auto-mute of %d in %d failed: %v", target.Id, chat.Id, err)
		release()
		return
	}
	// An admin who decided while the mute was being applied found no
	// restriction to lift and dropped the record: lift it now.
	if current, err := reports.GetAutoMute(chat.Id, target.Id); err == nil && current == nil {
		liftReportAutoMute(b, chat.Id, target.Id, until)
		return
	}

	actionlog.Automated(b, chat, "REPORT_MUTE", target.Id, target.FirstName,
		actionlog.Field{Name: "Reports", Value: strconv.Itoa(count)})
	text, _ := tr.GetString("reports_automute_applied", i18n.TranslationParams{
		"user": formatting.MentionHtml(target.Id, target.FirstName),
	})
	if text == "" {
		return
	}
	if _, err := helpers.SendMessageWithErrorHandling(b, chat.Id, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML}); err != nil {
		log.Debugf("[Reports] failed to announce auto-mute of %d in %d: %v", target.Id, chat.Id, err)
	}
}

// autoMuteTarget returns the author of reported when the auto-mute may apply
// to them in chatID: a user who is not the bot, a system account, an
// anonymous or channel sender, or approved. Admins are left to the live
// member check.
func autoMuteTarget(b *gotgbot.Bot, chatID int64, reported *gotgbot.Message) *gotgbot.User {
	if reported == nil || reported.From == nil || reported.SenderChat != nil {
		return nil
	}
	target := reported.From
	if target.Id <= 0 || target.Id == b.Id || chat_status.IsSystemUser(target.Id) ||
		approvals.IsUserApproved(chatID, target.Id) {
		return nil
	}
	return target
}

// settleReportAutoMute ends the report auto-mute of userID in chatID after an
// admin decided on reports against them. A dismissal or a deleted message
// lifts the mute, unless the user's other open reports still reach the
// threshold. A ban, kick or mute is the admins' own action: the record is
// dropped and the member left as the action put them. Only a mute this
// feature applied is ever lifted: never one an admin set or changed since.
func settleReportAutoMute(b *gotgbot.Bot, chatID, userID int64, status string) {
	switch status {
	case models.ReportStatusDismissed, models.ReportStatusDeleted:
	default:
		if _, err := reports.ReleaseAutoMute(chatID, userID); err != nil {
			log.Errorf("[Reports] failed to drop auto-mute of %d in %d: %v", userID, chatID, err)
		}
		return
	}

	muted, err := reports.GetAutoMute(chatID, userID)
	if err != nil {
		log.Errorf("[Reports] failed to load auto-mute of %d in %d: %v", userID, chatID, err)
		return
	}
	if muted == nil {
		return
	}
	if threshold := reports.GetChatReportSettings(chatID).MuteThreshold; threshold > 0 {
		count, err := reports.CountOpenReporters(chatID, userID)
		if err != nil {
			log.Errorf("[Reports] failed to count reporters of %d in %d: %v", userID, chatID, err)
			return
		}
		if count >= threshold {
			return
		}
	}
	muted, err = reports.ReleaseAutoMute(chatID, userID)
	if err != nil {
		log.Errorf("[Reports] failed to drop auto-mute of %d in %d: %v", userID, chatID, err)
		return
	}
	if muted == nil || !muted.MutedUntil.After(time.Now()) {
		return
	}
	liftReportAutoMute(b, chatID, userID, muted.MutedUntil)
}

// liftReportAutoMute restores the chat's default permissions to userID if
// the member still carries the auto-mute that ends at until.
func liftReportAutoMute(b *gotgbot.Bot, chatID, userID int64, until time.Time) {
	member, err := b.GetChatMember(chatID, userID, nil)
	if err != nil {
		log.Warnf("[Reports] cannot check %d in %d to lift the auto-mute: %v", userID, chatID, err)
		return
	}
	m := member.MergeChatMember()
	if m.Status != gotgbot.ChatMemberStatusRestricted || m.UntilDate != until.Unix() || m.CanSendMessages {
		// An admin unmuted, muted again or changed the restriction since.
		return
	}
	// unmuteCaptchaUser restores the chat's default permissions.
	if err := unmuteCaptchaUser(b, chatID, userID); err != nil {
		log.Warnf("[Reports] failed to lift the auto-mute of %d in %d: %v", userID, chatID, err)
	}
}

// reportsMuteSetting serves /reports mute [N|off] in a group.
func reportsMuteSetting(tr *i18n.Translator, chatID int64, args []string) string {
	usage, _ := tr.GetString("reports_automute_usage", i18n.TranslationParams{
		"min": models.ReportMuteThresholdMin,
		"max": models.ReportMuteThresholdMax,
	})
	if len(args) == 0 {
		return reportsMuteStatus(tr, reports.GetChatReportSettings(chatID).MuteThreshold) + "\n\n" + usage
	}
	if len(args) > 1 {
		return usage
	}
	var threshold int
	if on, ok := parseToggleArg(args[0]); !ok || on {
		n, err := strconv.Atoi(args[0])
		if err != nil || n < models.ReportMuteThresholdMin || n > models.ReportMuteThresholdMax {
			return usage
		}
		threshold = n
	}
	if err := reports.SetChatReportMuteThreshold(chatID, threshold); err != nil {
		log.Errorf("[Reports] %v", err)
		text, _ := tr.GetString("common_settings_save_failed")
		return text
	}
	if threshold == 0 {
		text, _ := tr.GetString("reports_automute_disabled")
		return text
	}
	text, _ := tr.GetString("reports_automute_enabled", i18n.TranslationParams{"count": threshold})
	return text
}

// reportsMuteStatus is the auto-mute line of /reports in a group.
func reportsMuteStatus(tr *i18n.Translator, threshold int) string {
	if threshold == 0 {
		text, _ := tr.GetString("reports_automute_status_off")
		return text
	}
	text, _ := tr.GetString("reports_automute_status_on", i18n.TranslationParams{"count": threshold})
	return text
}
