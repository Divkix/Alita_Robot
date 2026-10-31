package modules

import (
	"slices"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/probation"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const settingsTogglesPerRow = 2

// settingsToggle is a /settings summary row a button sets to its next value.
// It checks the rights its command checks and writes through the repository
// function the command uses, so a tap does what sending the command does.
type settingsToggle struct {
	key     string   // callback field k, short to fit the 64-byte cap
	command string   // the summary row it changes
	values  []string // the values a tap cycles through, as the row shows them
	// botAdmin is set when the command needs the bot to be an admin.
	botAdmin bool
	// right checks what the command needs beyond admin status and returns the
	// i18n key to refuse with, or "". Nil when admin status is enough.
	// It sends nothing; settingsTogglePress presents the refusal.
	right func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64) string
	// set receives the title for captcha writes; other repositories only need
	// the ID. It returns the i18n key to answer with when the write fails.
	set func(b *gotgbot.Bot, chatID int64, chatName, value string) string
}

// next is the value after current, or the first one when current is not one
// of the toggle's values.
func (t settingsToggle) next(current string) string {
	return t.values[(slices.Index(t.values, current)+1)%len(t.values)]
}

var settingsOnOff = []string{"off", "on"}

// boolToggle is an on/off row whose command writes it with set.
func boolToggle(key, command string, botAdmin bool, set func(chatID int64, on bool) error) settingsToggle {
	return settingsToggle{key: key, command: command, values: settingsOnOff, botAdmin: botAdmin,
		set: func(_ *gotgbot.Bot, chatID int64, _ string, value string) string {
			return settingsSaveFailed(command, chatID, set(chatID, value == "on"))
		}}
}

// valueToggle is a row of a few named values whose command writes it with set.
func valueToggle(key, command string, values []string, botAdmin bool, set func(chatID int64, value string) error) settingsToggle {
	return settingsToggle{key: key, command: command, values: values, botAdmin: botAdmin,
		set: func(_ *gotgbot.Bot, chatID int64, _ string, value string) string {
			return settingsSaveFailed(command, chatID, set(chatID, value))
		}}
}

// greetingSettingsToggle is a greetings on/off row, built from its command's
// config so the bot-admin check and the setter cannot drift from it. Each of
// these commands also needs the change-info right.
func greetingSettingsToggle(key, command string, cfg greetingToggleConfig) settingsToggle {
	return withRight(boolToggle(key, command, cfg.connectBotAdmin, cfg.setPref), canChangeInfoRight)
}

func settingsSaveFailed(command string, chatID int64, err error) string {
	if err == nil {
		return ""
	}
	log.Errorf("[Settings] toggle %s failed for chat %d: %v", command, chatID, err)
	return "common_settings_save_failed"
}

func withRight(t settingsToggle, right func(*gotgbot.Bot, *ext.Context, *gotgbot.Chat, int64) string) settingsToggle {
	t.right = right
	return t
}

// The rights the commands check beyond admin status.
func canChangeInfoRight(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64) string {
	if !chat_status.CanUserChangeInfo(b, ctx, chat, userID) {
		return "chat_status_change_info_button_error"
	}
	return ""
}

func restrictRight(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64) string {
	if !chat_status.CanUserRestrict(b, ctx, chat, userID) {
		return "chat_status_restrict_button_error"
	}
	return ""
}

func ownerRight(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64) string {
	if !chat_status.RequireUserOwner(b, ctx, chat, userID) {
		return "chat_status_owner_button_error"
	}
	return ""
}

func inFedRight(_ *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, _ int64) string {
	if federations.GetChatFedContext(tracing.UpdateContext(ctx), chat.Id) == nil {
		return "feds_not_in_fed"
	}
	return ""
}

// settingsToggles are the summary rows with a button.
// Each matches its summary row's Command and cycles exactly the values that row shows.
// Keep botAdmin, right, repository writes and command side effects in sync with the command;
// /captcha off also calls DisableCaptchaForChat. Reuse greetingToggleConfig and pinToggleConfig
// setters where available so the panel and command do not acquire different behavior.
var settingsToggles = []settingsToggle{
	{key: "cp", command: "/captcha", values: settingsOnOff, botAdmin: true, set: setCaptchaToggle},
	{key: "cm", command: "/captchamode", values: []string{"math", "text"},
		set: func(_ *gotgbot.Bot, chatID int64, chatName, value string) string {
			return settingsSaveFailed("/captchamode", chatID, captcha.SetCaptchaMode(chatID, chatName, value))
		}},
	{key: "ca", command: "/captchaaction", values: []string{"kick", "ban", "mute"},
		set: func(_ *gotgbot.Bot, chatID int64, chatName, value string) string {
			return settingsSaveFailed("/captchaaction", chatID, captcha.SetCaptchaFailureAction(chatID, chatName, value))
		}},
	valueToggle("fm", "/setfloodmode", []string{"mute", "ban", "kick"}, true, antiflood.SetFloodMode),
	boolToggle("fd", "/delflood", true, antiflood.SetFloodMsgDel),
	{key: "ar", command: "/antiraid", values: settingsOnOff, botAdmin: true, set: setAntiRaidToggle},
	{key: "ai", command: "/aispam", values: settingsOnOff, set: setAISpamToggle},
	withRight(boolToggle("pb", "/probation", true, probation.SetEnabled), restrictRight),
	withRight(boolToggle("pv", "/probation notice", true, probation.SetNotice), restrictRight),
	boolToggle("we", "/welcome", false, greetings.SetWelcomeToggle),
	greetingSettingsToggle("cw", "/cleanwelcome", cleanWelcomeToggleConfig),
	boolToggle("gb", "/goodbye", false, greetings.SetGoodbyeToggle),
	greetingSettingsToggle("cg", "/cleangoodbye", cleanGoodbyeToggleConfig),
	greetingSettingsToggle("cs", "/cleanservice", delJoinedToggleConfig),
	greetingSettingsToggle("aa", "/autoapprove", autoApproveToggleConfig),
	boolToggle("rp", "/reports", true, reports.SetChatReportStatus),
	boolToggle("pr", "/privaterules", true, rules.SetPrivateRules),
	boolToggle("pn", "/privnote", false, notes.TooglePrivateNote),
	boolToggle("ap", "/antichannelpin", true, antiChannelPinToggle.set),
	boolToggle("cl", "/cleanlinked", true, cleanLinkedToggle.set),
	boolToggle("dd", "/disabledel", true, disabling.ToggleDel),
	withRight(boolToggle("qf", "/quietfed", false, federations.SetQuietFed), inFedRight),
	boolToggle("ac", "/allowconnect", false, connections.ToggleAllowConnect),
	withRight(boolToggle("an", "/anonadmin", true, admin.SetAnonAdminMode), ownerRight),
}

// settingsTogglesByKey and settingsTogglesByCommand index settingsToggles for
// presses and for drawing the buttons.
var settingsTogglesByKey, settingsTogglesByCommand = func() (byKey, byCommand map[string]settingsToggle) {
	byKey = make(map[string]settingsToggle, len(settingsToggles))
	byCommand = make(map[string]settingsToggle, len(settingsToggles))
	for _, t := range settingsToggles {
		byKey[t.key] = t
		byCommand[t.command] = t
	}
	return byKey, byCommand
}()

// setCaptchaToggle is /captcha on|off: turning it off also releases the
// users a pending captcha holds.
func setCaptchaToggle(b *gotgbot.Bot, chatID int64, chatName, value string) string {
	on := value == "on"
	failKey := "captcha_disable_failed"
	if on {
		failKey = "captcha_enable_failed"
	}
	if err := captcha.SetCaptchaEnabled(chatID, chatName, on); err != nil {
		log.Errorf("[Settings] toggle /captcha failed for chat %d: %v", chatID, err)
		return failKey
	}
	if on {
		return ""
	}
	if err := DisableCaptchaForChat(b, chatID); err != nil {
		log.Errorf("[Settings] captcha cleanup failed for chat %d: %v", chatID, err)
		return failKey
	}
	return ""
}

// setAntiRaidToggle is /antiraid on|off: on starts raid mode for the chat's
// raid time. A raid already in the state asked for counts as done.
func setAntiRaidToggle(_ *gotgbot.Bot, chatID int64, _ string, value string) string {
	var err error
	if value == "on" {
		_, err = antiRaidModule.enableRaid(chatID, antiraid.GetAntiRaidSettings(chatID).RaidTime)
	} else {
		_, err = antiRaidModule.disableRaid(chatID)
	}
	if err != nil {
		log.Errorf("[Settings] toggle /antiraid failed for chat %d: %v", chatID, err)
		return "error_generic"
	}
	return ""
}

// setAISpamToggle is /aispam on|off, which refuses to turn the filter on
// when the operator has not set it up.
func setAISpamToggle(_ *gotgbot.Bot, chatID int64, _ string, value string) string {
	on := value == "on"
	if refusal := aispamEnableRefusalKey(); on && refusal != "" {
		return refusal
	}
	return settingsSaveFailed("/aispam", chatID, aispam.SetAISpamEnabled(chatID, on))
}

// settingsToggleRows are the buttons for the toggles among sections' rows.
// Each carries the value it sets, so a tap on a stale summary sets what its
// label says rather than flipping a value someone else changed since.
func settingsToggleRows(chatID int64, sections []settingsSection) [][]gotgbot.InlineKeyboardButton {
	var buttons []gotgbot.InlineKeyboardButton
	for _, section := range sections {
		for _, row := range section.Rows {
			t, ok := settingsTogglesByCommand[row.Command]
			if !ok {
				continue
			}
			next := t.next(row.Value)
			data := encodeCallbackData("settings", map[string]string{
				"a": "t", "c": strconv.FormatInt(chatID, 10), "k": t.key, "v": next,
			})
			if data == "" {
				continue
			}
			buttons = append(buttons, gotgbot.InlineKeyboardButton{
				Text:         strings.TrimPrefix(t.command, "/") + " → " + next,
				CallbackData: data,
			})
		}
	}
	return slices.Collect(slices.Chunk(buttons, settingsTogglesPerRow))
}

// settingsTogglePress handles a toggle button: it checks the presser's rights
// in the group the button names, writes the value and shows the summary again.
func settingsTogglePress(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, query *gotgbot.CallbackQuery, decoded *callbackcodec.Decoded) error {
	key, _ := decoded.Field("k")
	value, _ := decoded.Field("v")
	toggle, ok := settingsTogglesByKey[key]
	if !ok || !slices.Contains(toggle.values, value) {
		return answerInvalidCallback(b, ctx, query)
	}
	chat := callbackAdminChat(b, ctx, query, decoded, toggle.botAdmin)
	if chat == nil {
		return ext.EndGroups
	}
	if toggle.right != nil {
		if deny := toggle.right(b, ctx, chat, query.From.Id); deny != "" {
			text, _ := tr.GetString(deny)
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text, ShowAlert: true})
			return ext.EndGroups
		}
	}
	// The summary is drawn again even when the write fails: one that fails
	// half way (captcha off, then the release fails) has changed the row.
	failKey := toggle.set(b, chat.Id, chat.Title, value)
	text, kb := settingsSummaryView(ctx, tr, chat)
	editSettingsMessage(b, query.Message, text, kb)
	answer := &gotgbot.AnswerCallbackQueryOpts{}
	if failKey != "" {
		answer.Text, _ = tr.GetString(failKey)
		answer.ShowAlert = true
	} else {
		answer.Text, _ = tr.GetString("settings_toggle_saved", i18n.TranslationParams{"command": toggle.command, "value": value})
	}
	_, _ = query.Answer(b, answer)
	return ext.EndGroups
}
