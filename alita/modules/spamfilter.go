package modules

import (
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/spamfilter"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var spamFilterModule = moduleStruct{moduleName: "SpamFilter"}

// spamFilterAdminChecks are the checks of the commands that read or change a
// chat's samples and switch. They are group-only: the samples belong to the
// group, and /spam needs a reply to a group message anyway.
func spamFilterAdminChecks() []helpers.CheckFunc {
	return []helpers.CheckFunc{helpers.RequireGroup(), helpers.RequireUserAdmin(), helpers.RequireBotAdmin()}
}

var (
	// spamDesc needs what /dban needs: /spam deletes the replied message and
	// bans its sender.
	spamDesc        = helpers.CommandDescriptor{Name: "spam", RequiredChecks: deleteRestrictChecks("spam")}
	spamFilterDesc  = helpers.CommandDescriptor{Name: "spamfilter", RequiredChecks: spamFilterAdminChecks()}
	spamSamplesDesc = helpers.CommandDescriptor{Name: "spamsamples", RequiredChecks: spamFilterAdminChecks()}
	forgetSpamDesc  = helpers.CommandDescriptor{Name: "forgetspam", RequiredChecks: spamFilterAdminChecks()}
)

// spamSamplesShown caps the samples /spamsamples lists, newest first.
const spamSamplesShown = 20

// spamSamplePreviewRunes bounds each sample /spamsamples shows.
const spamSamplePreviewRunes = 80

// spamFilterNow is the clock the dry run is measured with. Tests replace it.
var spamFilterNow = time.Now

//
// /spam
//

// spamCommand handles /spam: in reply to a message, it deletes the message,
// bans its sender like /dban, and stores the text as one of the chat's spam
// samples. The ban counts like /dban's whatever happens to the sample.
func (moduleStruct) spamCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	bans := bansModule
	return moderationSpam(&bans).run(b, ctx)
}

func moderationSpam(m *moduleStruct) *moderationCommand {
	// stored is the sample the cleanup step saved, for the reply.
	var stored *models.SpamSample
	var tooShort bool
	return &moderationCommand{
		command: "spam",
		module:  m,
		extract: func(c *moderationCtx) (target, error) {
			if c.Msg.ReplyToMessage == nil {
				text, _ := c.Tr.GetString("spamfilter_spam_no_reply")
				if err := c.replyCommandHelp(text); err != nil {
					log.Error(err)
					return target{}, err
				}
				return target{}, fmt.Errorf("no reply")
			}
			return extractFromReply(c)
		},
		validate: banTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			// A message another admin or a filter removed first is gone
			// already: the ban and the sample still stand.
			if err := helpers.DeleteMessageWithErrorHandling(c.Bot, c.Chat.Id, c.Msg.ReplyToMessage.MessageId); err != nil {
				return err
			}
			_, err := c.Chat.BanMember(c.Bot, t.userID, nil)
			return err
		},
		cleanup: func(c *moderationCtx, t *target) error {
			sample, err := storeSpamSample(c, t.userID)
			switch {
			case errors.Is(err, spamfilter.ErrTooShort):
				tooShort = true
			case err != nil:
				log.WithError(err).WithField("chat_id", c.Chat.Id).Error("[SpamFilter] could not store the /spam sample")
			default:
				stored = sample
			}
			return nil
		},
		reply: func(c *moderationCtx, t *target) error {
			return spamReply(c, t, stored, tooShort)
		},
		logAction:   "spam",
		eventAction: models.ModerationActionBan,
	}
}

// storeSpamSample stores the replied message as the chat's spam, with the
// chat's owner and human admins from the admin list the checks just loaded.
func storeSpamSample(c *moderationCtx, senderID int64) (*models.SpamSample, error) {
	ownerID, adminIDs := spamFilterChatAdmins(c.Bot, c.Chat.Id)
	sample, err := spamfilter.Add(spamfilter.NewSample{
		ChatID:   c.Chat.Id,
		Label:    models.SpamLabelSpam,
		Text:     aispamMessageText(c.Msg.ReplyToMessage),
		SenderID: senderID,
		AddedBy:  c.User.Id,
		OwnerID:  ownerID,
		AdminIDs: adminIDs,
	})
	if err == nil {
		spamCorpusForget(c.Chat.Id)
	}
	return sample, err
}

// spamFilterChatAdmins returns the chat's owner (0 when unknown) and its
// human admins. Bots are left out: one bot admin in many chats does not make
// them related.
func spamFilterChatAdmins(b *gotgbot.Bot, chatID int64) (int64, []int64) {
	found, admins := cache.GetAdminCacheList(chatID)
	if !found || !admins.Cached {
		admins = cache.LoadAdminCache(b, chatID)
	}
	var ownerID int64
	ids := make([]int64, 0, len(admins.UserInfo))
	for _, member := range admins.UserInfo {
		if member.User.IsBot || member.User.Id <= 0 {
			continue
		}
		if member.Status == "creator" {
			ownerID = member.User.Id
		}
		ids = append(ids, member.User.Id)
	}
	return ownerID, ids
}

func spamReply(c *moderationCtx, t *target, stored *models.SpamSample, tooShort bool) error {
	name := ""
	if from := c.Msg.ReplyToMessage.From; from != nil {
		name = from.FirstName
	}
	text, _ := c.Tr.GetString("spamfilter_spam_banned", i18n.TranslationParams{
		"user": formatting.MentionHtml(t.userID, name),
	})
	switch {
	case stored != nil:
		line, _ := c.Tr.GetString("spamfilter_spam_saved", i18n.TranslationParams{"id": stored.ID})
		text += "\n" + line
	case tooShort:
		line, _ := c.Tr.GetString("spamfilter_spam_too_short")
		text += "\n" + line
	}
	_, err := c.Msg.Reply(c.Bot, text, &gotgbot.SendMessageOpts{
		ParseMode: formatting.HTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{keyboard.UnrestrictButton(c.Tr, "bans_unban_button", "unban", t.userID, 0)},
			},
		},
	})
	return err
}

// spamFilterUnbanHam turns the spam samples a ban of userID stored in the
// chat into ham: an admin who unbans the sender has said the message was not
// spam. It never affects the unban.
func spamFilterUnbanHam(chatID, userID, actorID int64) {
	if userID <= 0 {
		return
	}
	turned, err := spamfilter.RelabelSenderAsHam(chatID, userID, actorID)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"chat_id": chatID, "user_id": userID}).
			Error("[SpamFilter] could not turn the unbanned user's samples into ham")
		return
	}
	if turned > 0 {
		spamCorpusForget(chatID)
	}
}

//
// /spamfilter
//

func (moduleStruct) spamFilter(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	switch {
	case len(args) == 0:
		return spamFilterReplyStatus(c)
	case len(args) == 1 && spamfilter.ValidMode(strings.ToLower(args[0])):
		mode := strings.ToLower(args[0])
		if err := spamfilter.SetMode(c.Chat.Id, mode); err != nil {
			return spamFilterFailed(c, err)
		}
		actionlog.SettingChange(c.Bot, c.Chat, c.User, "SPAMFILTER", actionlog.Field{Name: "Mode", Value: mode})
		return spamFilterReplyStatus(c)
	case len(args) == 2 && strings.EqualFold(args[0], "threshold"):
		threshold, err := strconv.ParseFloat(strings.TrimSuffix(args[1], "%"), 64)
		if err == nil && strings.HasSuffix(args[1], "%") {
			threshold /= 100
		}
		if err != nil || !spamfilter.ValidThreshold(threshold) {
			usage, _ := c.Tr.GetString("spamfilter_threshold_invalid")
			return replyCommandHelp(c.Bot, c.Msg, c.Tr, "spamfilter", usage)
		}
		if err := spamfilter.SetThreshold(c.Chat.Id, threshold); err != nil {
			return spamFilterFailed(c, err)
		}
		actionlog.SettingChange(c.Bot, c.Chat, c.User, "SPAMFILTER",
			actionlog.Field{Name: "Threshold", Value: strconv.FormatFloat(threshold, 'f', 2, 64)})
		return spamFilterReplyStatus(c)
	default:
		usage, _ := c.Tr.GetString("spamfilter_usage")
		return replyCommandHelp(c.Bot, c.Msg, c.Tr, "spamfilter", usage)
	}
}

func spamFilterFailed(c *helpers.CommandContext, err error) error {
	log.WithError(err).WithField("chat_id", c.Chat.Id).Error("[SpamFilter] could not save the chat's spam filter")
	text, _ := c.Tr.GetString("error_generic")
	_, replyErr := c.Msg.Reply(c.Bot, text, formatting.Shtml())
	return replyErr
}

func spamFilterReplyStatus(c *helpers.CommandContext) error {
	reqCtx := tracing.UpdateContext(c.Ctx)
	settings, err := spamfilter.GetSettings(reqCtx, c.Chat.Id)
	if err != nil {
		return spamFilterFailed(c, err)
	}
	samples, err := spamfilter.List(reqCtx, c.Chat.Id)
	if err != nil {
		return spamFilterFailed(c, err)
	}
	spam, ham := spamSampleCounts(samples)
	hasLog := logchannels.Get(c.Chat.Id) != nil
	_, err = c.Msg.Reply(c.Bot, spamFilterStatusText(c.Tr, settings, spam, ham, hasLog, spamFilterNow()), formatting.Shtml())
	return err
}

// spamFilterStatusText describes the chat's filter: its mode, when the dry
// run ends, the threshold and the samples, and warns when matches have no log
// channel to go to.
func spamFilterStatusText(tr *i18n.Translator, settings *models.SpamFilterSettings, spam, ham int, hasLog bool, now time.Time) string {
	str := func(key string, params ...i18n.TranslationParams) string {
		text, _ := tr.GetString(key, params...)
		return text
	}
	var mode string
	switch {
	case settings.Mode == models.SpamFilterDryRun:
		mode = str("spamfilter_status_dryrun")
	case settings.Mode == models.SpamFilterOn && !spamfilter.Acting(settings, now):
		mode = str("spamfilter_status_on_pending", i18n.TranslationParams{
			"date": spamfilter.DryRunEnds(settings).UTC().Format("2006-01-02 15:04 UTC"),
		})
	case settings.Mode == models.SpamFilterOn:
		mode = str("spamfilter_status_on")
	default:
		mode = str("spamfilter_status_off")
	}
	lines := []string{
		mode,
		str("spamfilter_status_threshold", i18n.TranslationParams{"threshold": strconv.FormatFloat(settings.Threshold, 'f', 2, 64)}),
		str("spamfilter_status_samples", i18n.TranslationParams{"spam": spam, "ham": ham}),
	}
	if settings.Mode != models.SpamFilterOff && !hasLog {
		lines = append(lines, "", str("spamfilter_status_no_log"))
	}
	return strings.Join(lines, "\n")
}

func spamSampleCounts(samples []models.SpamSample) (spam, ham int) {
	for _, s := range samples {
		if s.Label == models.SpamLabelHam {
			ham++
		} else {
			spam++
		}
	}
	return spam, ham
}

//
// /spamsamples and /forgetspam
//

func (moduleStruct) spamSamples(c *helpers.CommandContext) error {
	samples, err := spamfilter.List(tracing.UpdateContext(c.Ctx), c.Chat.Id)
	if err != nil {
		return spamFilterFailed(c, err)
	}
	spam, ham := spamSampleCounts(samples)
	if spam == 0 {
		text, _ := c.Tr.GetString("spamfilter_samples_empty", i18n.TranslationParams{"ham": ham})
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		return err
	}
	header, _ := c.Tr.GetString("spamfilter_samples_header", i18n.TranslationParams{"spam": spam, "ham": ham})
	lines := []string{header, ""}
	shown := 0
	for _, s := range samples {
		if s.Label != models.SpamLabelSpam {
			continue
		}
		if shown == spamSamplesShown {
			break
		}
		shown++
		lines = append(lines, fmt.Sprintf("<code>#%d</code> · %s · %s",
			s.ID, s.CreatedAt.UTC().Format("2006-01-02"), html.EscapeString(spamSamplePreview(s.Text))))
	}
	footer, _ := c.Tr.GetString("spamfilter_samples_footer")
	lines = append(lines, "", footer)
	_, err = c.Msg.Reply(c.Bot, strings.Join(lines, "\n"), formatting.Shtml())
	return err
}

// spamSamplePreview is one line of a sample: its first runes, newlines
// folded.
func spamSamplePreview(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); len(runes) > spamSamplePreviewRunes {
		return string(runes[:spamSamplePreviewRunes]) + "…"
	}
	return text
}

func (moduleStruct) forgetSpam(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	var id int64
	var err error
	if len(args) == 1 {
		id, err = strconv.ParseInt(strings.TrimPrefix(args[0], "#"), 10, 64)
	}
	if len(args) != 1 || err != nil || id <= 0 {
		usage, _ := c.Tr.GetString("spamfilter_forget_usage")
		return replyCommandHelp(c.Bot, c.Msg, c.Tr, "forgetspam", usage)
	}
	removed, err := spamfilter.Delete(c.Chat.Id, id)
	if err != nil {
		return spamFilterFailed(c, err)
	}
	key := "spamfilter_forget_not_found"
	if removed {
		key = "spamfilter_forget_done"
		spamCorpusForget(c.Chat.Id)
	}
	text, _ := c.Tr.GetString(key, i18n.TranslationParams{"id": id})
	_, err = c.Msg.Reply(c.Bot, text, formatting.Shtml())
	return err
}

//
// Retention and loading
//

var spamSampleRetentionOnce sync.Once

// startSpamSampleRetention deletes samples past their 90 days every hour.
func startSpamSampleRetention() {
	startRetentionSweeper(&spamSampleRetentionOnce, "spamSampleRetention", time.Hour, func(now time.Time) {
		removed, err := spamfilter.DeleteExpiredBefore(now.Add(-spamfilter.Retention))
		if err != nil {
			log.WithError(err).Error("[SpamFilter] retention sweep failed")
			return
		}
		if removed > 0 {
			spamCorpusForgetAll()
			log.WithField("removed", removed).Debug("[SpamFilter] removed expired spam samples")
		}
	})
}

func LoadSpamFilter(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(spamFilterModule.moduleName, true)

	helpers.WrapCommand(dispatcher, spamDesc, pipelineHandler(spamFilterModule.spamCommand))
	helpers.WrapCommand(dispatcher, spamFilterDesc, spamFilterModule.spamFilter)
	helpers.WrapCommand(dispatcher, spamSamplesDesc, spamFilterModule.spamSamples)
	helpers.WrapCommand(dispatcher, forgetSpamDesc, spamFilterModule.forgetSpam)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(spamHamNamespace+"|"), spamFilterModule.spamHamButton))

	startSpamSampleRetention()
}

func init() {
	RegisterLegacyModule("SpamFilter", 226, LoadSpamFilter)
	RegisterAnonymousAdminHandler("spam", anonPipelineHandler(spamDesc, spamFilterModule.spamCommand))
	RegisterAnonymousAdminHandler("spamfilter", anonCommandContextHandler(spamFilterDesc, spamFilterModule.spamFilter))
	RegisterAnonymousAdminHandler("spamsamples", anonCommandContextHandler(spamSamplesDesc, spamFilterModule.spamSamples))
	RegisterAnonymousAdminHandler("forgetspam", anonCommandContextHandler(forgetSpamDesc, spamFilterModule.forgetSpam))
}
