package modules

import (
	"html"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/bans"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// banRecordTimeFormat is how ban records show a time.
const banRecordTimeFormat = "2006-01-02 15:04 UTC"

// From PM, /baninfo takes the group as its first argument, then the user.
var banInfoDesc = helpers.CommandDescriptor{Name: "baninfo", PMGroupArg: helpers.PMGroupArgOwn}

// banRecordChecks are the checks of a command that reads the ban records: an
// admin who can restrict members, as for /ban. The bot needs no right of its
// own, since it only reads its records.
func banRecordChecks(cmd string) []helpers.CheckFunc {
	return []helpers.CheckFunc{
		helpers.CheckDisabled(cmd),
		pmTargetChatCheck,
		helpers.RequireGroup(),
		helpers.RequireUserAdmin(),
		helpers.CanUserRestrict(),
	}
}

func initBanRecordDescs() {
	banInfoDesc.RequiredChecks = banRecordChecks("baninfo")
	banListDesc.RequiredChecks = banRecordChecks("banlist")
}

// banRecordUser is the HTML label of a user or channel a record names: a
// mention with the name users holds now, or the ID when it holds none.
func banRecordUser(id int64) string {
	if id <= 0 {
		return "<code>" + strconv.FormatInt(id, 10) + "</code>"
	}
	name := strconv.FormatInt(id, 10)
	if u, err := user.GetUserBasicInfoCached(id); err == nil && u != nil {
		switch {
		case u.Name != "":
			name = u.Name
		case u.UserName != "":
			name = "@" + u.UserName
		}
	}
	return formatting.MentionHtml(id, name)
}

// banRecordActor is the HTML label of who banned or unbanned: the bot itself
// for 0, an anonymous admin for the group's anonymous bot, else the account.
func banRecordActor(tr *i18n.Translator, id int64) string {
	switch id {
	case 0:
		return html.EscapeString(trS(tr, "bans_record_by_bot"))
	case chat_status.GroupAnonymousBotID:
		return html.EscapeString(trS(tr, "bans_record_by_anonymous"))
	}
	return banRecordUser(id)
}

// banRecordSource is the HTML label of a record's source with its detail.
func banRecordSource(r *models.BanRecord) string {
	source := "<code>" + html.EscapeString(r.Source) + "</code>"
	if r.SourceDetail != "" {
		source += " (<code>" + html.EscapeString(r.SourceDetail) + "</code>)"
	}
	return source
}

// banRecordText renders one record for /baninfo, as of now.
func banRecordText(tr *i18n.Translator, r *models.BanRecord, now time.Time) string {
	var status string
	switch {
	case r.LiftedAt != nil:
		liftedBy := int64(0)
		if r.LiftedBy != nil {
			liftedBy = *r.LiftedBy
		}
		status, _ = tr.GetString("bans_baninfo_status_lifted", i18n.TranslationParams{
			"admin": banRecordActor(tr, liftedBy),
			"date":  r.LiftedAt.UTC().Format(banRecordTimeFormat),
		})
	case !r.IsOpen(now):
		status = html.EscapeString(trS(tr, "bans_baninfo_status_expired"))
	default:
		status = html.EscapeString(trS(tr, "bans_baninfo_status_banned"))
	}
	until := html.EscapeString(trS(tr, "bans_baninfo_permanent"))
	if r.Until != nil {
		until = r.Until.UTC().Format(banRecordTimeFormat)
	}
	text, _ := tr.GetString("bans_baninfo_record", i18n.TranslationParams{
		"user":   banRecordUser(r.UserId),
		"id":     r.UserId,
		"status": status,
		"admin":  banRecordActor(tr, r.BannedBy),
		"date":   r.CreatedAt.UTC().Format(banRecordTimeFormat),
		"source": banRecordSource(r),
		"until":  until,
	})
	if r.Reason != "" {
		reason, _ := tr.GetString("bans_baninfo_reason", i18n.TranslationParams{"reason": html.EscapeString(r.Reason)})
		text += reason
	}
	return text
}

// banInfo is /baninfo: the latest ban the bot recorded for a user in the
// group, open, expired or lifted.
func (moduleStruct) banInfo(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	userID := extraction.ExtractUser(b, ctx)
	switch userID {
	case -1:
		// Extraction has already said why.
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("common_no_user_specified")
		if err := replyCommandHelp(b, msg, tr, "baninfo", text); err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	record, err := bans.Latest(chat.Id, userID)
	var text string
	switch {
	case err != nil:
		log.WithError(err).WithField("chat_id", chat.Id).Error("[Bans] Failed to read the ban record")
		text, _ = tr.GetString("bans_record_read_failed")
	case record == nil:
		text, _ = tr.GetString("bans_baninfo_none", i18n.TranslationParams{"user": banRecordUser(userID)})
	default:
		text = banRecordText(tr, record, time.Now())
	}
	if _, err := msg.Reply(b, withGroupLine(tr, msg, chat, text), formatting.Shtml()); err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}
