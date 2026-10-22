package modules

import (
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/digestsettings"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/digest"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/panel"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

var digestModule = moduleStruct{
	moduleName: "Digest",
}

var digestDesc = helpers.CommandDescriptor{
	Name: "digest",
	// Every /digest form is admin-only, so a chat cannot silence it: /digest
	// off already is the switch. A stored disable row must not block that.
	Disableable: false,
	RequiredChecks: []helpers.CheckFunc{
		helpers.RequireGroup(),
		helpers.RequireBotAdmin(),
		helpers.RequireUserAdmin(),
	},
}

// digestNow is the clock the command reads. Tests replace it to pin the day
// and week boundaries; production never writes it after startup.
var digestNow = time.Now

// digest handles /digest. A bare call shows the chat's digest state; on, off,
// here and log change it; now renders the current partial week with the same
// builder the weekly scheduler uses. Counting and sending are UTC-only.
func (moduleStruct) digest(c *helpers.CommandContext) error {
	if !config.AppConfig.EnableDigest {
		return digestReplyKey(c, "digest_globally_disabled", nil)
	}

	settings, err := digestsettings.Get(c.Chat.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[Digest] could not read the chat's digest settings")
		return digestReplyKey(c, "error_generic", nil)
	}

	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		return digestReplyStatus(c, settings)
	}
	if len(args) > 1 {
		return digestReplyUsage(c)
	}

	switch strings.ToLower(args[0]) {
	case "now":
		return digestPreview(c)
	case "on":
		return digestSetEnabled(c, true)
	case "off":
		return digestSetEnabled(c, false)
	case "here":
		return digestSetDestination(c, digestsettings.DestinationGroup)
	case "log":
		if digestLogChannelID(c.Chat.Id) == 0 {
			return digestReplyKey(c, "digest_log_unavailable", nil)
		}
		return digestSetDestination(c, digestsettings.DestinationLog)
	default:
		return digestReplyUsage(c)
	}
}

// digestPreview renders this week so far through the weekly builder and
// replies with it. It flushes first so the numbers include the last minute of
// counters; a failed flush is not fatal, earlier flushes are still stored.
func digestPreview(c *helpers.CommandContext) error {
	if err := stats.Flush(); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Warn("[Digest] could not flush daily counters before the preview")
	}

	now := digestNow().UTC()
	weekStart := digest.WeekStart(now)
	current, err := digest.ReadStats(c.Chat.Id, weekStart, now)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[Digest] could not read activity for the preview")
		return digestReplyKey(c, "error_generic", nil)
	}
	previous, err := digest.ReadStats(c.Chat.Id, weekStart.AddDate(0, 0, -7), weekStart)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[Digest] could not read last week's activity for the preview")
		return digestReplyKey(c, "error_generic", nil)
	}

	text, keyboard := digest.Build(c.Tr, current, previous, weekStart, digestDashboardURL(c.Bot.Username, c.Chat.Id))
	opts := formatting.Shtml()
	if keyboard != nil {
		opts.ReplyMarkup = keyboard
	}
	_, err = c.Msg.Reply(c.Bot, text, opts)
	return err
}

// digestSetEnabled stores the switch and answers with the new state.
func digestSetEnabled(c *helpers.CommandContext, enabled bool) error {
	if err := digestsettings.SetEnabled(c.Chat.Id, enabled); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[Digest] could not save the digest switch")
		return digestReplyKey(c, "error_generic", nil)
	}
	return digestReplyFreshStatus(c)
}

// digestSetDestination stores the admin's choice and answers with the new
// state, including whether the destination is still the default.
func digestSetDestination(c *helpers.CommandContext, destination string) error {
	if err := digestsettings.SetDestination(c.Chat.Id, destination); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[Digest] could not save the digest destination")
		return digestReplyKey(c, "error_generic", nil)
	}
	return digestReplyFreshStatus(c)
}

// digestReplyFreshStatus re-reads the settings after a write (the setter
// invalidated the cache) and replies with them.
func digestReplyFreshStatus(c *helpers.CommandContext) error {
	settings, err := digestsettings.Get(c.Chat.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[Digest] could not read the saved digest settings")
		return digestReplyKey(c, "error_generic", nil)
	}
	return digestReplyStatus(c, settings)
}

func digestReplyStatus(c *helpers.CommandContext, settings *models.DigestSettings) error {
	text := digestStatusText(c.Tr, settings, digestLogChannelID(c.Chat.Id) != 0)
	_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
	return err
}

// digestStatusText describes the chat's digest: whether it is on, where it
// goes and whether that destination is the default or an admin's choice.
func digestStatusText(tr *i18n.Translator, settings *models.DigestSettings, hasLogChannel bool) string {
	statusKey := "digest_status_disabled"
	if settings.Enabled {
		statusKey = "digest_status_enabled"
	}

	destinationName := "digest_destination_group"
	if digest.ResolveDestination(settings.Destination, hasLogChannel) == digestsettings.DestinationLog {
		destinationName = "digest_destination_log"
	}
	destinationLineKey := "digest_status_destination_default"
	if settings.Destination != digestsettings.DestinationAuto {
		destinationLineKey = "digest_status_destination_set"
	}

	lines := []string{
		digestString(tr, "digest_status_title"),
		"",
		digestString(tr, statusKey),
		digestString(tr, destinationLineKey, i18n.TranslationParams{
			"destination": digestString(tr, destinationName),
		}),
		"",
		digestString(tr, "digest_status_hint"),
	}
	return strings.Join(lines, "\n")
}

func digestReplyUsage(c *helpers.CommandContext) error {
	usage, _ := c.Tr.GetString("digest_usage")
	return replyCommandHelp(c.Bot, c.Msg, c.Tr, "digest", usage)
}

// digestLogChannelID returns the chat's log channel, or 0 when it has none.
func digestLogChannelID(chatID int64) int64 {
	logChannel := logchannels.Get(chatID)
	if logChannel == nil {
		return 0
	}
	return logChannel.LogChannelID
}

// digestDashboardURL returns the panel button target, or "" when the panel is
// disabled so the digest renders without a dead button.
func digestDashboardURL(botUsername string, chatID int64) string {
	if !config.AppConfig.PanelEnabled {
		return ""
	}
	return panel.EntryURL(botUsername, config.AppConfig.PanelMiniAppShortName, chatID)
}

func digestString(tr *i18n.Translator, key string, params ...i18n.TranslationParams) string {
	text, _ := tr.GetString(key, params...)
	return text
}

func digestReplyKey(c *helpers.CommandContext, key string, params i18n.TranslationParams) error {
	var text string
	if params == nil {
		text, _ = c.Tr.GetString(key)
	} else {
		text, _ = c.Tr.GetString(key, params)
	}
	_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
	return err
}

func LoadDigest(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(digestModule.moduleName, true)

	helpers.WrapCommand(dispatcher, digestDesc, digestModule.digest)
	RegisterAnonymousAdminHandler("digest", anonCommandContextHandler(digestDesc, digestModule.digest))
	startDigestStatsRetention()
	startActivityStatsRetention()
}

func init() {
	RegisterLegacyModule("Digest", 225, LoadDigest)
}
