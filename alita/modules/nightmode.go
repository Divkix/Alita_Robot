package modules

import (
	"fmt"
	"html"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/nightmode"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// nightModeNow is the clock night mode reads. Tests pin it; production never
// writes it after startup.
var nightModeNow = time.Now

// nightLockExcluded are lock types night mode cannot apply: "bots" bans added
// bots on the ChatMember path, not on the delete path night mode runs on.
var nightLockExcluded = map[string]bool{"bots": true}

// withNightLocks returns chatLocks with the chat's night lock types added
// while its night window is open in the chat's timezone, and chatLocks itself
// otherwise. It never writes to chatLocks, the cached map: the merge is a
// copy. A chat without night mode costs one cached read.
func withNightLocks(chatID int64, chatLocks map[string]bool) map[string]bool {
	settings, err := nightmode.Get(chatID)
	if err != nil {
		log.Errorf("[NightMode] read night mode: %v - %d", err, chatID)
		return chatLocks
	}
	if !settings.Enabled || !nightmode.ActiveAt(settings, nightModeNow().In(chattz.Location(chatID))) {
		return chatLocks
	}
	merged := make(map[string]bool, len(chatLocks)+1)
	maps.Copy(merged, chatLocks)
	for lockType := range strings.SplitSeq(settings.LockTypes, ",") {
		if !nightLockExcluded[lockType] {
			merged[lockType] = true
		}
	}
	return merged
}

// nightModeDesc is /nightmode. It asks what /lock asks (a group admin, with
// the bot an admin); from PM it takes the group as its first argument or from
// the connection.
var nightModeDesc = helpers.CommandDescriptor{
	Name:       "nightmode",
	PMGroupArg: helpers.PMGroupArgOwn,
	RequiredChecks: []helpers.CheckFunc{
		groupOnlyChatCheck,
		helpers.RequireGroup(),
		helpers.RequireBotAdmin(),
		helpers.RequireUserAdmin(),
	},
}

// nightMode handles /nightmode. Without arguments it shows the chat's night
// mode; "off" turns it off; "<start> <end> <types>" turns it on with those
// times in the chat's timezone and those lock types (comma- or
// space-separated, or "all").
func (m moduleStruct) nightMode(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		settings, err := nightmode.Get(c.Chat.Id)
		if err != nil {
			log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
				Error("[NightMode] could not read the chat's night mode")
			return nightModeReply(c, trS(c.Tr, "error_generic"))
		}
		if !settings.Enabled {
			return nightModeReply(c, trS(c.Tr, "nightmode_status_off"))
		}
		return nightModeReply(c, nightModeOnText(c, settings))
	}

	if len(args) == 1 && strings.EqualFold(args[0], "off") {
		if err := nightmode.Disable(c.Chat.Id); err != nil {
			log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
				Error("[NightMode] could not turn night mode off")
			return nightModeReply(c, trS(c.Tr, "common_settings_save_failed"))
		}
		return nightModeReply(c, trS(c.Tr, "nightmode_disabled"))
	}

	start, end, types, refusal := m.parseNightMode(c.Tr, args)
	if refusal != "" {
		if err := replyCommandHelp(c.Bot, c.Msg, c.Tr, nightModeDesc.Name, refusal); err != nil {
			log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
				Warn("[NightMode] could not send the usage reply")
		}
		return ext.EndGroups
	}
	if err := nightmode.Enable(c.Chat.Id, start, end, types); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[NightMode] could not save night mode")
		return nightModeReply(c, trS(c.Tr, "common_settings_save_failed"))
	}
	// types are sorted and unique, as Enable stores them.
	return nightModeReply(c, nightModeOnText(c, &models.NightModeSettings{
		ChatID: c.Chat.Id, Enabled: true, StartMinute: start, EndMinute: end, LockTypes: strings.Join(types, ","),
	}))
}

// parseNightMode reads "<start> <end> <types>", returning the types sorted
// and unique. On a bad call it returns the refusal to show above the
// command's help.
func (m moduleStruct) parseNightMode(tr *i18n.Translator, args []string) (start, end int, types []string, refusal string) {
	var ok bool
	for i, out := range []*int{&start, &end} {
		if i >= len(args) {
			return 0, 0, nil, trS(tr, "nightmode_no_types")
		}
		if *out, ok = parseNightClock(args[i]); !ok {
			text, _ := tr.GetString("nightmode_invalid_time", i18n.TranslationParams{"input": html.EscapeString(args[i])})
			return 0, 0, nil, text
		}
	}
	if start == end {
		return 0, 0, nil, trS(tr, "nightmode_same_times")
	}

	valid := m.getLockMapAsArray()
	// Args are already split on spaces; a word may still hold commas.
	for field := range strings.SplitSeq(strings.Join(args[2:], ","), ",") {
		if field == "" {
			continue
		}
		lockType := strings.ToLower(field)
		if !slices.Contains(valid, lockType) || nightLockExcluded[lockType] {
			text, _ := tr.GetString("nightmode_invalid_type", i18n.TranslationParams{"type": html.EscapeString(field)})
			return 0, 0, nil, text
		}
		types = append(types, lockType)
	}
	if len(types) == 0 {
		return 0, 0, nil, trS(tr, "nightmode_no_types")
	}
	slices.Sort(types)
	return start, end, slices.Compact(types), ""
}

// parseNightClock reads a 24-hour H:MM or HH:MM time as minutes after
// midnight.
func parseNightClock(s string) (int, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// nightWindowLabel is how replies and the settings summary show a night
// window ("23:00-07:00") and its lock types ("media, url").
func nightWindowLabel(settings *models.NightModeSettings) (start, end, types string) {
	clock := func(minute int) string { return fmt.Sprintf("%02d:%02d", minute/60, minute%60) }
	return clock(settings.StartMinute), clock(settings.EndMinute), strings.Join(settings.Types(), ", ")
}

// nightModeOnText describes an enabled night mode and whether it applies now
// in the chat's timezone.
func nightModeOnText(c *helpers.CommandContext, settings *models.NightModeSettings) string {
	loc := chattz.Location(c.Chat.Id)
	now := nightModeNow().In(loc)
	start, end, types := nightWindowLabel(settings)
	text, _ := c.Tr.GetString("nightmode_status_on", i18n.TranslationParams{
		"start":    start,
		"end":      end,
		"timezone": html.EscapeString(loc.String()),
		"types":    html.EscapeString(types),
	})
	nowKey := "nightmode_now_inactive"
	if nightmode.ActiveAt(settings, now) {
		nowKey = "nightmode_now_active"
	}
	nowText, _ := c.Tr.GetString(nowKey, i18n.TranslationParams{"time": now.Format("15:04")})
	return text + "\n" + nowText
}

// nightModeReply answers in the chat the command came from; from PM the reply
// also names the group it acted on.
func nightModeReply(c *helpers.CommandContext, text string) error {
	if _, err := c.Msg.Reply(c.Bot, withGroupLine(c.Tr, c.Msg, c.Chat, text), formatting.Shtml()); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Warn("[NightMode] could not reply")
		return err
	}
	return ext.EndGroups
}

// loadNightMode registers /nightmode as part of the Locks module (LoadLocks).
func loadNightMode(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, nightModeDesc, locksModule.nightMode)
	RegisterAnonymousAdminHandler(nightModeDesc.Name, anonCommandContextHandler(nightModeDesc, locksModule.nightMode))
}
