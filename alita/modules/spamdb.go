package modules

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/spamdb"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// spamDBHandlerGroup runs the join check after fed-bans (-6) and antiraid (-5)
// and before greetings and captcha (0).
const spamDBHandlerGroup = -4

var spamDBModule = moduleStruct{
	moduleName:   "SpamDB",
	handlerGroup: spamDBHandlerGroup,
}

var spamDBDesc = helpers.CommandDescriptor{
	Name:        "spamdb",
	Disableable: true,
	RequiredChecks: []helpers.CheckFunc{
		helpers.CheckDisabled("spamdb"),
	},
}

// spamDBChecker answers the CAS lookups. LoadSpamDB builds it from the config.
var spamDBChecker *spamdb.Checker

// checkSpamDB looks every new member up in CAS and, when listed, applies the
// chat's action and reports it to the log channel only, so a raid does not
// flood the group with notices. A failed lookup lets the member in.
func (moduleStruct) checkSpamDB(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	if !config.AppConfig.EnableSpamDB || chat == nil || msg == nil {
		return ext.ContinueGroups
	}
	if chat.Type != "group" && chat.Type != "supergroup" {
		return ext.ContinueGroups
	}

	reqCtx := tracing.UpdateContext(ctx)
	settings, err := spamdbsettings.GetSpamDBSettingsContext(reqCtx, chat.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chat.Id, "error": err}).Warn("[SpamDB] could not read the chat settings")
		return ext.ContinueGroups
	}
	if !settings.Enabled {
		return ext.ContinueGroups
	}

	for _, member := range msg.NewChatMembers {
		if member.IsBot || !chat_status.IsValidUserId(member.Id) {
			continue
		}
		if chat_status.IsApprovedForUpdate(b, ctx, chat.Id, member.Id) ||
			chat_status.IsUserAdminForUpdate(b, ctx, chat.Id, member.Id) {
			continue
		}

		listed, err := spamDBChecker.Check(reqCtx, member.Id)
		if err != nil {
			log.WithFields(log.Fields{"chat_id": chat.Id, "user_id": member.Id, "error": err}).Debug("[SpamDB] lookup failed; letting the member in")
			continue
		}
		if !listed {
			continue
		}

		outcome := settings.Action
		if err := applySpamDBAction(b, chat.Id, member.Id, settings.Action); err != nil {
			log.WithFields(log.Fields{"chat_id": chat.Id, "user_id": member.Id, "error": err}).Warnf("[SpamDB] %s failed", settings.Action)
			outcome += " (failed)"
		} else if settings.Action != spamdbsettings.ActionLog {
			// Take the join so greetings send no welcome or captcha: a captcha
			// would lift the mute and its timeout kick would undo the ban.
			claimRecentJoinProcessing(chat.Id, member.Id)
		}
		actionlog.Log(b, chat, logchannels.CategoryAutomated, fmt.Sprintf(
			"#SPAMDB\nUser: %s (<code>%d</code>)\nListed in: CAS\nAction: %s",
			formatting.MentionHtml(member.Id, member.FirstName), member.Id, outcome,
		))
	}
	return ext.ContinueGroups
}

func applySpamDBAction(b *gotgbot.Bot, chatID, userID int64, action string) error {
	switch action {
	case spamdbsettings.ActionBan:
		_, err := b.BanChatMember(chatID, userID, nil)
		return err
	case spamdbsettings.ActionMute:
		_, err := b.RestrictChatMember(chatID, userID, MutedPermissions, nil)
		return err
	default:
		return nil
	}
}

// spamdb is the /spamdb command: a bare call reports the settings, on/off
// toggles the check and "action ban|mute|log" picks what happens to a listed
// member. Enabling refuses when the operator has switched the feature off.
func (moduleStruct) spamdb(c *helpers.CommandContext) error {
	// Admin check runs here, against the connected group when used from PM.
	chat := resolveAdminChat(c.Bot, c.Ctx, false)
	if chat == nil {
		return ext.EndGroups
	}
	c.Chat = chat
	args := c.Ctx.Args()[1:]

	settings, err := spamdbsettings.GetSpamDBSettingsContext(tracing.UpdateContext(c.Ctx), chat.Id)
	if err != nil {
		return spamDBSaveFailed(c, err)
	}
	if len(args) == 0 {
		return spamDBReply(c, spamDBStatusText(c.Tr, settings.Enabled, settings.Action))
	}

	if strings.EqualFold(args[0], "action") {
		if len(args) < 2 || !spamdbsettings.ValidAction(strings.ToLower(args[1])) {
			return spamDBReplyKey(c, "spamdb_invalid_action", nil)
		}
		action := strings.ToLower(args[1])
		if err := spamdbsettings.SetSpamDBAction(chat.Id, action); err != nil {
			return spamDBSaveFailed(c, err)
		}
		return spamDBReplyKey(c, "spamdb_action_success", i18n.TranslationParams{"action": action})
	}

	want, ok := parseToggleArg(args[0])
	if !ok {
		return spamDBReplyKey(c, "spamdb_invalid_option", nil)
	}
	if want && !config.AppConfig.EnableSpamDB {
		return spamDBReplyKey(c, "spamdb_globally_disabled", nil)
	}
	if err := spamdbsettings.SetSpamDBEnabled(chat.Id, want); err != nil {
		return spamDBSaveFailed(c, err)
	}
	if want {
		return spamDBReplyKey(c, "spamdb_enabled_success", nil)
	}
	return spamDBReplyKey(c, "spamdb_disabled_success", nil)
}

func spamDBStatusText(tr *i18n.Translator, enabled bool, action string) string {
	lineKey := "spamdb_status_disabled"
	if enabled {
		lineKey = "spamdb_status_enabled"
	}
	line, _ := tr.GetString(lineKey, i18n.TranslationParams{"action": action})

	parts := []string{line}
	if !config.AppConfig.EnableSpamDB {
		warning, _ := tr.GetString("spamdb_status_globally_disabled")
		parts = append(parts, warning)
	}
	notice, _ := tr.GetString("spamdb_data_notice")
	parts = append(parts, notice)

	return strings.Join(parts, "\n\n")
}

func spamDBSaveFailed(c *helpers.CommandContext, err error) error {
	log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Error("[SpamDB] failed to save the setting")
	if replyErr := spamDBReplyKey(c, "common_settings_save_failed", nil); replyErr != nil {
		return replyErr
	}
	return err
}

func spamDBReplyKey(c *helpers.CommandContext, key string, params i18n.TranslationParams) error {
	var text string
	if params == nil {
		text, _ = c.Tr.GetString(key)
	} else {
		text, _ = c.Tr.GetString(key, params)
	}
	return spamDBReply(c, text)
}

func spamDBReply(c *helpers.CommandContext, text string) error {
	_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
	return err
}

func LoadSpamDB(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(spamDBModule.moduleName, true)
	spamDBChecker = spamdb.New(spamdb.Options{
		CASAPIKey: config.AppConfig.CASAPIKey,
		Redis:     cache.GetRedisClient,
	})

	helpers.WrapCommand(dispatcher, spamDBDesc, spamDBModule.spamdb)

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(func(msg *gotgbot.Message) bool { return msg.NewChatMembers != nil }, spamDBModule.checkSpamDB),
		spamDBModule.handlerGroup,
	)
}

func init() {
	RegisterLegacyModule("SpamDB", 13, LoadSpamDB)
}
