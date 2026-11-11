package modules

import (
	"errors"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/i18n"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
	"github.com/divkix/Alita_Robot/alita/utils/spamdb"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var spamDBModule = moduleStruct{moduleName: "SpamDB"}

var spamDBDesc = helpers.CommandDescriptor{
	Name:           "spamdb",
	Disableable:    true,
	PMGroupArg:     helpers.PMGroupArgOwn,
	RequiredChecks: declaredSettingChecks(chatsettings.SpamDBEnabled),
}

// spamDBChecker answers the database lookups. LoadSpamDB builds it from the
// config.
var spamDBChecker *spamdb.Checker

// spamDBEntry ties the name admins type for a database to its source.
type spamDBEntry struct {
	name    string
	source  spamdb.Source
	setting *chatsettings.Declaration
	// service is where the user IDs go, as /spamdb names it. Shared sends
	// nothing anywhere, so it has none.
	service string
}

// spamDBDatabases lists the databases in the order /spamdb shows them.
var spamDBDatabases = []spamDBEntry{
	{spamdbsettings.DatabaseCAS, spamdb.SourceCAS, chatsettings.SpamDBCAS, "cas.chat"},
	{spamdbsettings.DatabaseLols, spamdb.SourceLols, chatsettings.SpamDBLols, "lols.bot"},
	{spamdbsettings.DatabaseShared, spamdb.SourceShared, chatsettings.SpamDBShared, ""},
}

// spamDBSources lists the databases from the declaration's normalized read,
// CAS first, then lols.bot and the shared network.
func spamDBSources(settings *models.SpamDBSettings) []spamdb.Source {
	var sources []spamdb.Source
	if settings.CheckCAS {
		sources = append(sources, spamdb.SourceCAS)
	}
	if settings.CheckLols {
		sources = append(sources, spamdb.SourceLols)
	}
	if settings.CheckShared {
		sources = append(sources, spamdb.SourceShared)
	}
	return sources
}

// spamDBPunishments maps the chat's spam database action to its punishment;
// ActionLog punishes nobody.
var spamDBPunishments = map[string]punishment.Mode{
	spamdbsettings.ActionBan:  punishment.Ban,
	spamdbsettings.ActionMute: punishment.Mute,
}

// spamDBStage is membership intake's spam database check: it looks the
// newcomer up in the chat's databases and, when one lists them, applies the
// chat's action and reports it to the log channel only, so a raid does not
// flood the group with notices. A ban removes the member and a mute holds
// them, so no captcha can lift it; a log-only match, a failed lookup or a
// failed ban lets them on.
func (moduleStruct) spamDBStage(b *gotgbot.Bot, ctx *ext.Context, member gotgbot.User) StageAnswer {
	chat := ctx.EffectiveChat
	if !config.AppConfig.EnableSpamDB || chat == nil || (chat.Type != "group" && chat.Type != "supergroup") {
		return StageNotApplicable
	}
	if member.IsBot || !chat_status.IsValidUserId(member.Id) {
		return StageNotApplicable
	}

	reqCtx := tracing.UpdateContext(ctx)
	settings, err := chatsettings.ReadSpamDBContext(reqCtx, chat.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chat.Id, "error": err}).Warn("[SpamDB] could not read the chat settings")
		return StageNotApplicable
	}
	if !settings.Enabled {
		return StageNotApplicable
	}
	if chat_status.IsApprovedForUpdate(b, ctx, chat.Id, member.Id) ||
		chat_status.IsUserAdminForUpdate(b, ctx, chat.Id, member.Id) {
		return StageNotApplicable
	}

	source, err := spamDBChecker.Check(reqCtx, member.Id, spamDBSources(settings))
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chat.Id, "user_id": member.Id, "error": err}).Debug("[SpamDB] lookup failed; letting the member in")
		return StageNotApplicable
	}
	if source == "" {
		return StageNotApplicable
	}

	answer, outcome := StageNotApplicable, settings.Action
	var logMarkup *gotgbot.InlineKeyboardMarkup
	if mode, punishes := spamDBPunishments[settings.Action]; punishes {
		result, err := punishment.Apply(reqCtx, b, punishment.Request{
			ChatID: chat.Id, Target: punishment.Target{ID: member.Id}, Mode: mode,
			Source: models.BanSourceSpamDB, Detail: source.Name(),
		})
		switch {
		case err != nil || !result.Applied:
			log.WithContext(reqCtx).WithFields(log.Fields{"chat_id": chat.Id, "user_id": member.Id, "error": result.Refusal}).Warnf("[SpamDB] %s failed", settings.Action)
			answer, outcome = StageFailed, outcome+" (failed)"
		case mode == punishment.Mute:
			answer, logMarkup = StageHeld, result.LogMarkup
		default:
			answer, logMarkup = StageRemoved, result.LogMarkup
		}
	}
	actionlog.AutomatedWithMarkup(b, chat, logMarkup, "SPAMDB", member.Id, member.FirstName,
		actionlog.Field{Name: "Listed in", Value: source.Name()},
		actionlog.Field{Name: "Action", Value: outcome},
	)
	return answer
}

// spamdb is the /spamdb command: a bare call reports the settings, on/off
// toggles the check, "db cas|lols|shared on|off" picks the databases and
// "action ban|mute|log" picks what happens to a listed member. Enabling
// refuses when the operator has switched the feature off, and the shared
// database refuses when the operator has not enabled the network.
func (moduleStruct) spamdb(c *helpers.CommandContext) error {
	chat := c.Chat
	args := c.Ctx.Args()[1:]

	settings, err := chatsettings.ReadSpamDBContext(tracing.UpdateContext(c.Ctx), chat.Id)
	if err != nil {
		return spamDBSaveFailed(c, err)
	}
	if len(args) == 0 {
		return spamDBReply(c, spamDBStatusText(c.Tr, settings))
	}

	if strings.EqualFold(args[0], "db") {
		return spamDBDatabase(c, args[1:])
	}

	if strings.EqualFold(args[0], "action") {
		if len(args) < 2 || chatsettings.SpamDBAction.Validate(strings.ToLower(args[1])) != nil {
			return spamDBReplyKey(c, "spamdb_invalid_action", nil)
		}
		action := strings.ToLower(args[1])
		if err := chatsettings.SpamDBAction.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(c.Ctx), ChatID: chat.Id}, action); err != nil {
			return spamDBSaveFailed(c, err)
		}
		return spamDBReplyKey(c, "spamdb_action_success", i18n.TranslationParams{"action": action})
	}

	want, ok := parseToggleArg(args[0])
	if !ok {
		return spamDBReplyKey(c, "spamdb_invalid_option", nil)
	}
	if err := chatsettings.SpamDBEnabled.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(c.Ctx), ChatID: chat.Id}, want); err != nil {
		if key := chatsettings.RefusalKey(err); key != "" {
			return spamDBReplyKey(c, key, nil)
		}
		return spamDBSaveFailed(c, err)
	}
	if want {
		return spamDBReplyKey(c, "spamdb_enabled_success", nil)
	}
	return spamDBReplyKey(c, "spamdb_disabled_success", nil)
}

// spamDBDatabase is "/spamdb db <cas|lols|shared> <on|off>". The last database
// left cannot be switched off, so a chat with the check on always asks one.
// Switching lols.bot on warns about its false positives; switching the shared
// network on refuses while the operator has it disabled.
func spamDBDatabase(c *helpers.CommandContext, args []string) error {
	if len(args) < 2 {
		return spamDBReplyKey(c, "spamdb_db_usage", nil)
	}
	name := strings.ToLower(args[0])
	idx := slices.IndexFunc(spamDBDatabases, func(d spamDBEntry) bool { return d.name == name })
	want, ok := parseToggleArg(args[1])
	if idx < 0 || !ok {
		return spamDBReplyKey(c, "spamdb_db_usage", nil)
	}
	database := spamDBDatabases[idx]
	params := i18n.TranslationParams{"database": database.source.Name()}

	err := database.setting.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(c.Ctx), ChatID: c.Chat.Id}, want)
	if key := chatsettings.RefusalKey(err); key != "" {
		return spamDBReplyKey(c, key, params)
	}
	if errors.Is(err, chatsettings.ErrLastSpamDatabase) {
		return spamDBReplyKey(c, "spamdb_db_last", params)
	}
	if err != nil {
		return spamDBSaveFailed(c, err)
	}
	if !want {
		return spamDBReplyKey(c, "spamdb_db_disabled", params)
	}
	// Members are looked up, and their IDs sent, only while the check runs, so
	// the reply must not say otherwise when it does not.
	enabledKey, warningKey := "spamdb_db_enabled_idle", "spamdb_db_lols_warning_idle"
	if spamDBCheckRuns(c) {
		enabledKey, warningKey = "spamdb_db_enabled", "spamdb_db_lols_warning"
	}
	text, _ := c.Tr.GetString(enabledKey, params)
	if database.source == spamdb.SourceLols {
		warning, _ := c.Tr.GetString(warningKey)
		text += "\n\n" + warning
	}
	return spamDBReply(c, text)
}

// spamDBCheckRuns reports whether new members of the chat are looked up now:
// the operator allows the feature and the chat has the check on. An unreadable
// setting counts as not running, so no reply claims a lookup that may not happen.
func spamDBCheckRuns(c *helpers.CommandContext) bool {
	if !config.AppConfig.EnableSpamDB {
		return false
	}
	settings, err := chatsettings.ReadSpamDBContext(tracing.UpdateContext(c.Ctx), c.Chat.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Warn("[SpamDB] could not read the chat settings")
		return false
	}
	return settings.Enabled
}

// spamDBReputationEnabled reports whether the operator lets chats use the
// shared reputation network.
func spamDBReputationEnabled() bool {
	return config.AppConfig != nil && config.AppConfig.EnableReputation
}

func spamDBStatusText(tr *i18n.Translator, settings *models.SpamDBSettings) string {
	lineKey := "spamdb_status_disabled"
	if settings.Enabled {
		lineKey = "spamdb_status_enabled"
	}
	line, _ := tr.GetString(lineKey, i18n.TranslationParams{"action": settings.Action})

	sources := spamDBSources(settings)
	var entries, services []string
	for _, d := range spamDBDatabases {
		on := slices.Contains(sources, d.source)
		stateKey := "spamdb_status_db_off"
		if on {
			stateKey = "spamdb_status_db_on"
			// Shared bans is selected but does nothing while the operator's
			// switch is off, so the line must not read as a running lookup.
			if d.source == spamdb.SourceShared && !spamDBReputationEnabled() {
				stateKey = "spamdb_status_db_on_inactive"
			}
			if d.service != "" {
				services = append(services, d.service)
			}
		}
		entry, _ := tr.GetString(stateKey, i18n.TranslationParams{"database": d.source.Name()})
		entries = append(entries, entry)
	}
	databases, _ := tr.GetString("spamdb_status_databases", i18n.TranslationParams{"databases": strings.Join(entries, ", ")})

	parts := []string{line, databases}
	if !config.AppConfig.EnableSpamDB {
		warning, _ := tr.GetString("spamdb_status_globally_disabled")
		parts = append(parts, warning)
	}
	if settings.CheckShared {
		sharedNotice, _ := tr.GetString("spamdb_shared_data_notice")
		parts = append(parts, sharedNotice)
	}
	if len(services) > 0 {
		notice, _ := tr.GetString("spamdb_data_notice", i18n.TranslationParams{"services": strings.Join(services, ", ")})
		parts = append(parts, notice)
	}

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
}

func init() {
	RegisterLegacyModule("SpamDB", 13, LoadSpamDB)
}
