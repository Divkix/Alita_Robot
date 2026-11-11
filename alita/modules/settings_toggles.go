package modules

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/i18n"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const settingsTogglesPerRow = 2

// settingsToggle is a /settings row a button sets to its next value. Declared
// rows derive their values, rights and write from their declaration.
type settingsToggle struct {
	key         string // callback field k, short to fit the 64-byte cap
	command     string // the summary row it changes
	declaration *chatsettings.Declaration
	values      []string // the values a tap cycles through, as the row shows them
	// botAdmin is set when the command needs the bot to be an admin.
	botAdmin bool
	// right checks what the command needs beyond admin status and returns the
	// i18n key to refuse with, or "". Nil when admin status is enough.
	// It sends nothing; settingsTogglePress presents the refusal.
	right func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64) string
	// set receives the title for captcha writes; other repositories only need
	// the ID. It returns the i18n key to answer with when the write fails.
	set func(ctx context.Context, b *gotgbot.Bot, chatID int64, chatName, value string) string
}

// next is the value after current, or the first one when current is not one
// of the toggle's values.
func (t settingsToggle) next(current string) string {
	return t.values[(slices.Index(t.values, current)+1)%len(t.values)]
}

var settingsOnOff = []string{"off", "on"}

func declaredSettingsToggle(key, command string, declaration *chatsettings.Declaration) settingsToggle {
	toggle := settingsToggle{
		key: key, command: command, declaration: declaration, values: declaration.ToggleValues(),
		set: func(ctx context.Context, b *gotgbot.Bot, chatID int64, title string, value string) string {
			var parsed any = value
			if declaration.Type == chatsettings.Switch {
				parsed = value == "on"
			}
			err := declaration.Write(chatsettings.WriteContext{Context: ctx, ChatID: chatID, Title: title, Bot: b}, parsed)
			if key := chatsettings.RefusalKey(err); key != "" {
				return key
			}
			if err != nil && declaration.FailureKey != nil {
				return declaration.FailureKey(parsed)
			}
			return settingsSaveFailed(command, chatID, err)
		},
	}
	switch declaration.Right {
	case chatsettings.Admin:
	case chatsettings.AdminBot:
		toggle.botAdmin = true
	case chatsettings.ChangeInfo:
		toggle.right = canChangeInfoRight
	case chatsettings.ChangeInfoBot:
		toggle.botAdmin, toggle.right = true, canChangeInfoRight
	case chatsettings.Restrict:
		toggle.botAdmin, toggle.right = true, restrictRight
	case chatsettings.RestrictUser:
		toggle.botAdmin, toggle.right = true, userRestrictRight
	case chatsettings.OwnerBot:
		toggle.botAdmin, toggle.right = true, ownerRight
	case chatsettings.FederationAdmin:
		toggle.right = inFedRight
	default:
		panic("unmapped setting right")
	}
	return toggle
}

func settingsSaveFailed(command string, chatID int64, err error) string {
	if err == nil {
		return ""
	}
	log.Errorf("[Settings] toggle %s failed for chat %d: %v", command, chatID, err)
	return "common_settings_save_failed"
}

// The rights the commands check beyond admin status.
func canChangeInfoRight(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64) string {
	if !chat_status.CanUserChangeInfo(b, ctx, chat, userID) {
		return "chat_status_change_info_cmd_error"
	}
	return ""
}

func userRestrictRight(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64) string {
	if !chat_status.CanUserRestrict(b, ctx, chat, userID) {
		return "chat_status_restrict_cmd_error"
	}
	return ""
}

func restrictRight(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64) string {
	if key := userRestrictRight(b, ctx, chat, userID); key != "" {
		return key
	}
	if !chat_status.BotHasPermissionForUpdate(b, ctx, chat, func(member *gotgbot.MergedChatMember) bool { return member.CanRestrictMembers }) {
		return "chat_status_bot_restrict_group_error"
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
// Declared rows carry only their callback key, command and declaration.
var settingsToggles = []settingsToggle{
	declaredSettingsToggle("cp", "/captcha", chatsettings.CaptchaEnabled),
	declaredSettingsToggle("cj", "/captchajoin", chatsettings.CaptchaJoinRequests),
	declaredSettingsToggle("cm", "/captchamode", chatsettings.CaptchaMode),
	declaredSettingsToggle("ca", "/captchaaction", chatsettings.CaptchaAction),
	declaredSettingsToggle("fm", "/setfloodmode", chatsettings.FloodMode),
	declaredSettingsToggle("fd", "/delflood", chatsettings.FloodDelete),
	{key: "ar", command: "/antiraid", values: settingsOnOff, botAdmin: true, set: setAntiRaidToggle},
	declaredSettingsToggle("ai", "/aispam", chatsettings.AISpamEnabled),
	declaredSettingsToggle("pb", "/probation", chatsettings.ProbationEnabled),
	declaredSettingsToggle("pv", "/probation notice", chatsettings.ProbationNotice),
	declaredSettingsToggle("we", "/welcome", chatsettings.Welcome),
	declaredSettingsToggle("cw", "/cleanwelcome", chatsettings.CleanWelcome),
	declaredSettingsToggle("gb", "/goodbye", chatsettings.Goodbye),
	declaredSettingsToggle("cg", "/cleangoodbye", chatsettings.CleanGoodbye),
	declaredSettingsToggle("cs", "/cleanservice", chatsettings.CleanService),
	declaredSettingsToggle("aa", "/autoapprove", chatsettings.AutoApprove),
	declaredSettingsToggle("rp", "/reports", chatsettings.ReportsEnabled),
	declaredSettingsToggle("pr", "/privaterules", chatsettings.PrivateRules),
	declaredSettingsToggle("pn", "/privnote", chatsettings.PrivateNotes),
	declaredSettingsToggle("ap", "/antichannelpin", chatsettings.AntiChannelPin),
	declaredSettingsToggle("cl", "/cleanlinked", chatsettings.CleanLinked),
	declaredSettingsToggle("dd", "/disabledel", chatsettings.DeleteDisabledCommands),
	declaredSettingsToggle("qf", "/quietfed", chatsettings.QuietFed),
	declaredSettingsToggle("ac", "/allowconnect", chatsettings.AllowConnect),
	declaredSettingsToggle("an", "/anonadmin", chatsettings.AnonAdmin),
	declaredSettingsToggle("ep", "/ephemeral", chatsettings.EphemeralReplies),
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

// setAntiRaidToggle is /antiraid on|off: on starts raid mode for the chat's
// raid time. A raid already in the state asked for counts as done.
func setAntiRaidToggle(_ context.Context, _ *gotgbot.Bot, chatID int64, _ string, value string) string {
	var err error
	if value == "on" {
		_, err = antiRaidModule.enableRaid(chatID, chatsettings.ReadAntiRaid(chatID).RaidTime)
	} else {
		_, err = antiRaidModule.disableRaid(chatID)
	}
	if err != nil {
		log.Errorf("[Settings] toggle /antiraid failed for chat %d: %v", chatID, err)
		return "error_generic"
	}
	return ""
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
	failKey := toggle.set(tracing.UpdateContext(ctx), b, chat.Id, chat.Title, value)
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
