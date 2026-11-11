package modules

import (
	"errors"
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
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// nightModeNow is the clock night mode reads. Tests pin it; production never
// writes it after startup.
var nightModeNow = time.Now

func init() {
	types := slices.Clone(locksModule.getLockMapAsArray())
	types = slices.DeleteFunc(types, func(lockType string) bool { return lockType == "bots" })
	chatsettings.AttachNightModeLockTypes(types)
}

// withNightLocks returns chatLocks with the chat's night lock types added
// while its night window is open in the chat's timezone, and chatLocks itself
// otherwise. It never writes to chatLocks, the cached map: the merge is a
// copy. A chat without night mode costs one cached read.
func withNightLocks(chatID int64, chatLocks map[string]bool) map[string]bool {
	settings, err := chatsettings.ReadNightMode(chatID)
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
		merged[lockType] = true
	}
	return merged
}

// nightModeDesc is /nightmode. It asks what /lock asks (a group admin, with
// the bot an admin); from PM it takes the group as its first argument or from
// the connection.
var nightModeDesc = helpers.CommandDescriptor{
	Name:       "nightmode",
	PMGroupArg: helpers.PMGroupArgOwn,
	RequiredChecks: append([]helpers.CheckFunc{
		groupOnlyChatCheck,
		helpers.RequireGroup(),
	}, declaredGroupSettingChecks(chatsettings.NightModeEnabled)...),
}

// nightMode handles /nightmode. Without arguments it shows the chat's night
// mode; "off" turns it off; "<start> <end> <types>" turns it on with those
// times in the chat's timezone and those lock types (comma- or
// space-separated, or "all").
func (moduleStruct) nightMode(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		settings, err := chatsettings.ReadNightMode(c.Chat.Id)
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
		if err := chatsettings.NightModeEnabled.Write(chatsettings.WriteContext{ChatID: c.Chat.Id}, false); err != nil {
			log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
				Error("[NightMode] could not turn night mode off")
			return nightModeReply(c, trS(c.Tr, "common_settings_save_failed"))
		}
		return nightModeReply(c, trS(c.Tr, "nightmode_disabled"))
	}

	changes := map[string]any{chatsettings.NightModeEnabled.Key: true}
	for i, field := range []*chatsettings.Declaration{chatsettings.NightModeStart, chatsettings.NightModeEnd} {
		if i >= len(args) {
			return nightModeRefused(c, chatsettings.Refusal("nightmode_no_types"))
		}
		if err := field.Validate(args[i]); err != nil {
			return nightModeRefused(c, err)
		}
		changes[field.Key] = args[i]
	}
	changes[chatsettings.NightModeLockTypes.Key] = strings.Join(args[2:], ",")
	if err := chatsettings.NightModeEnabled.WriteChanges(chatsettings.WriteContext{ChatID: c.Chat.Id}, changes); err != nil {
		if chatsettings.RefusalKey(err) != "" {
			return nightModeRefused(c, err)
		}
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[NightMode] could not save night mode")
		return nightModeReply(c, trS(c.Tr, "common_settings_save_failed"))
	}
	settings, err := chatsettings.ReadNightMode(c.Chat.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Error("[NightMode] could not read the saved night mode")
		return nightModeReply(c, trS(c.Tr, "error_generic"))
	}
	return nightModeReply(c, nightModeOnText(c, settings))
}

func nightModeRefused(c *helpers.CommandContext, err error) error {
	params := i18n.TranslationParams{}
	var inputError *chatsettings.NightModeInputError
	if errors.As(err, &inputError) {
		params["input"] = html.EscapeString(inputError.Input)
		params["type"] = html.EscapeString(inputError.LockType)
	}
	refusal, _ := c.Tr.GetString(chatsettings.RefusalKey(err), params)
	if err := replyCommandHelp(c.Bot, c.Msg, c.Tr, nightModeDesc.Name, refusal); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
			Warn("[NightMode] could not send the usage reply")
	}
	return ext.EndGroups
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

}
