package modules

import (
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

var digestModule = moduleStruct{
	moduleName: "Digest",
}

var digestDesc = helpers.CommandDescriptor{
	Name:        "digest",
	Disableable: true,
	RequiredChecks: []helpers.CheckFunc{
		helpers.CheckDisabled("digest"),
		helpers.RequireGroup(),
		helpers.RequireBotAdmin(),
		helpers.RequireUserAdmin(),
	},
}

// digestNow is the clock the preview reads. Tests replace it to pin the day
// and week boundaries; production never writes it after startup.
var digestNow = time.Now

// digest is /digest now: the current UTC day's counters and the totals since
// Monday 00:00 UTC. Counting is per chat and UTC-only by design.
func (moduleStruct) digest(c *helpers.CommandContext) error {
	if !config.AppConfig.EnableDigest {
		return digestReplyKey(c, "digest_globally_disabled", nil)
	}

	args := c.Ctx.Args()[1:]
	if len(args) != 1 || !strings.EqualFold(args[0], "now") {
		usage, _ := c.Tr.GetString("digest_usage")
		return replyCommandHelp(c.Bot, c.Msg, c.Tr, "digest", usage)
	}

	// Flush first, so the preview includes the last minute of counters and the
	// numbers it shows are already durable. A failed flush is not fatal: the
	// table still holds every earlier flush and the counters are retried.
	if err := stats.Flush(); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Warn("[Digest] could not flush daily counters before the preview")
	}

	now := digestNow().UTC()
	weekStart := digestWeekStart(now)
	rows, err := chatstats.GetRange(c.Chat.Id, stats.Day(weekStart), stats.Day(now))
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[Digest] could not read daily counters")
		return digestReplyKey(c, "error_generic", nil)
	}

	todayDay := stats.Day(now)
	var today, week models.ChatDailyStat
	for _, row := range rows {
		week.Messages += row.Messages
		week.Joins += row.Joins
		week.Leaves += row.Leaves
		if row.Day == todayDay {
			today = row
		}
	}

	_, err = c.Msg.Reply(c.Bot, digestPreviewText(c.Tr, today, week, now, weekStart), formatting.Shtml())
	return err
}

// digestWeekStart returns Monday 00:00 UTC of the week containing t.
func digestWeekStart(t time.Time) time.Time {
	t = t.UTC()
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(day.Weekday()) + 6) % 7 // Monday is 0
	return day.AddDate(0, 0, -offset)
}

func digestPreviewText(tr *i18n.Translator, today, week models.ChatDailyStat, now, weekStart time.Time) string {
	title, _ := tr.GetString("digest_preview_title")
	todayHeader, _ := tr.GetString("digest_preview_today", i18n.TranslationParams{"day": stats.Day(now)})
	weekHeader, _ := tr.GetString("digest_preview_week", i18n.TranslationParams{"day": stats.Day(weekStart)})

	lines := []string{title, "", todayHeader}
	lines = append(lines, digestCountLines(tr, today)...)
	lines = append(lines, "", weekHeader)
	lines = append(lines, digestCountLines(tr, week)...)
	return strings.Join(lines, "\n")
}

func digestCountLines(tr *i18n.Translator, row models.ChatDailyStat) []string {
	messages, _ := tr.GetString("digest_preview_messages", i18n.TranslationParams{"count": row.Messages})
	joins, _ := tr.GetString("digest_preview_joins", i18n.TranslationParams{"count": row.Joins})
	leaves, _ := tr.GetString("digest_preview_leaves", i18n.TranslationParams{"count": row.Leaves})
	return []string{messages, joins, leaves}
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
}

func init() {
	RegisterLegacyModule("Digest", 225, LoadDigest)
}
