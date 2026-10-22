package modules

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/reactions"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// settingsRow is one line of the /settings summary. Command is the command
// that changes the value, so the row doubles as a usage hint and needs no
// translation.
type settingsRow struct {
	Command string
	Value   string
	Changed bool
}

type settingsSection struct {
	Title string
	Rows  []settingsRow
}

// Effective defaults of a group that never touched a setting. They follow what
// the bot actually does, not the Go fallbacks: GORM drops zero values that have
// a default tag, so goodbye and allow_connect end up true once their row exists.
const (
	settingsDefaultCaptchaMode     = "math"
	settingsDefaultCaptchaTimeout  = 2
	settingsDefaultCaptchaAction   = "kick"
	settingsDefaultCaptchaAttempts = 3
	settingsDefaultFloodAction     = "mute"
	settingsDefaultRaidTime        = 21600
	settingsDefaultRaidActionTime  = 3600
	settingsDefaultWarnLimit       = 3
	settingsDefaultWarnMode        = "mute"
	settingsDefaultBlacklistAction = "warn"
	settingsDefaultLanguage        = "en"
	settingsMaxListedItems         = 15
)

func boolRow(command string, value, def bool) settingsRow {
	text := "off"
	if value {
		text = "on"
	}
	return settingsRow{Command: command, Value: text, Changed: value != def}
}

func intRow(command string, value, def int) settingsRow {
	return settingsRow{Command: command, Value: strconv.Itoa(value), Changed: value != def}
}

func stringRow(command, value, def string) settingsRow {
	return settingsRow{Command: command, Value: value, Changed: value != def}
}

func countRow(command string, n int) settingsRow {
	return settingsRow{Command: command, Value: strconv.Itoa(n), Changed: n != 0}
}

func listRow(command string, items []string) settingsRow {
	if len(items) == 0 {
		return settingsRow{Command: command, Value: "—"}
	}
	slices.Sort(items)
	shown := items
	suffix := ""
	if len(shown) > settingsMaxListedItems {
		shown = shown[:settingsMaxListedItems]
		suffix = fmt.Sprintf(" … (+%d)", len(items)-settingsMaxListedItems)
	}
	return settingsRow{Command: command, Value: strings.Join(shown, ", ") + suffix, Changed: true}
}

func customTextRow(command, text, def string) settingsRow {
	if text == "" || text == def {
		return settingsRow{Command: command, Value: "default"}
	}
	return settingsRow{Command: command, Value: "custom", Changed: true}
}

// readChatSetting loads a per-chat settings row without creating it. Several
// repository getters insert a row on a miss, and greetings would insert one
// with goodbye enabled, so the summary must not call them.
func readChatSetting[T any](ctx context.Context, chatID int64) (*T, bool) {
	var setting T
	err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).Take(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false
	}
	if err != nil {
		log.Warnf("[Settings] read %T for chat %d: %v", setting, chatID, err)
		return nil, false
	}
	return &setting, true
}

func collectChatSettings(ctx context.Context, chatID int64) []settingsSection {
	sections := []settingsSection{
		captchaSettingsSection(ctx, chatID),
		antifloodSettingsSection(chatID),
		antiraidSettingsSection(chatID),
		{Title: "AI Spam", Rows: []settingsRow{boolRow("/aispam", aispam.IsAISpamEnabled(chatID), false)}},
		warnsSettingsSection(ctx, chatID),
		locksSettingsSection(chatID),
		blacklistSettingsSection(chatID),
		greetingsSettingsSection(ctx, chatID),
		reportsSettingsSection(ctx, chatID),
		rulesSettingsSection(ctx, chatID),
		notesSettingsSection(ctx, chatID),
		pinsSettingsSection(ctx, chatID),
		disablingSettingsSection(ctx, chatID),
		{Title: "Approvals & Reactions", Rows: []settingsRow{
			countRow("/approved", len(approvals.GetApprovedUsers(chatID))),
			countRow("/reactions", len(reactions.GetReactions(chatID))),
		}},
		logChannelSettingsSection(chatID),
		federationSettingsSection(chatID),
		chatSettingsSection(ctx, chatID),
	}
	return sections
}

func captchaSettingsSection(ctx context.Context, chatID int64) settingsSection {
	s, err := captcha.GetCaptchaSettingsContext(ctx, chatID)
	if err != nil || s == nil {
		s = &models.CaptchaSettings{CaptchaMode: settingsDefaultCaptchaMode, Timeout: settingsDefaultCaptchaTimeout,
			FailureAction: settingsDefaultCaptchaAction, MaxAttempts: settingsDefaultCaptchaAttempts}
	}
	return settingsSection{Title: "Captcha", Rows: []settingsRow{
		boolRow("/captcha", s.Enabled, false),
		stringRow("/captchamode", s.CaptchaMode, settingsDefaultCaptchaMode),
		intRow("/captchatime", s.Timeout, settingsDefaultCaptchaTimeout),
		stringRow("/captchaaction", s.FailureAction, settingsDefaultCaptchaAction),
		intRow("/captchamaxattempts", s.MaxAttempts, settingsDefaultCaptchaAttempts),
	}}
}

func antifloodSettingsSection(chatID int64) settingsSection {
	f := antiflood.GetFlood(chatID)
	limit := settingsRow{Command: "/setflood", Value: "off"}
	if f.Limit > 0 {
		limit = settingsRow{Command: "/setflood", Value: strconv.Itoa(f.Limit), Changed: true}
	}
	return settingsSection{Title: "Antiflood", Rows: []settingsRow{
		limit,
		stringRow("/setfloodmode", f.Action, settingsDefaultFloodAction),
		boolRow("/delflood", f.DeleteAntifloodMessage, false),
	}}
}

func antiraidSettingsSection(chatID int64) settingsSection {
	s := antiraid.GetAntiRaidSettings(chatID)
	st := getRaidState(chatID)
	active := st.Active && time.Now().Unix() <= st.ExpiresAt
	auto := settingsRow{Command: "/autoantiraid", Value: "off"}
	if s.AutoAntiRaidThreshold > 0 {
		auto = settingsRow{Command: "/autoantiraid", Value: strconv.Itoa(s.AutoAntiRaidThreshold), Changed: true}
	}
	return settingsSection{Title: "AntiRaid", Rows: []settingsRow{
		boolRow("/antiraid", active, false),
		{Command: "/raidtime", Value: formatDuration(s.RaidTime), Changed: s.RaidTime != settingsDefaultRaidTime},
		{Command: "/raidactiontime", Value: formatDuration(s.RaidActionTime), Changed: s.RaidActionTime != settingsDefaultRaidActionTime},
		auto,
	}}
}

func warnsSettingsSection(ctx context.Context, chatID int64) settingsSection {
	limit, mode, expiry := settingsDefaultWarnLimit, settingsDefaultWarnMode, "off"
	if w, ok := readChatSetting[models.WarnSettings](ctx, chatID); ok {
		if w.WarnLimit > 0 {
			limit = w.WarnLimit
		}
		// warnUser takes no action at the limit for an empty mode.
		mode = warnModeLabel(w)
		if mode == "" {
			mode = "none"
		}
		if w.WarnExpiry > 0 {
			expiry = formatDuration(w.WarnExpiry)
		}
	}
	return settingsSection{Title: "Warns", Rows: []settingsRow{
		intRow("/setwarnlimit", limit, settingsDefaultWarnLimit),
		stringRow("/setwarnmode", mode, settingsDefaultWarnMode),
		stringRow("/setwarntime", expiry, "off"),
	}}
}

func locksSettingsSection(chatID int64) settingsSection {
	var locked []string
	for perm, on := range locks.GetChatLocks(chatID) {
		if on {
			locked = append(locked, perm)
		}
	}
	return settingsSection{Title: "Locks", Rows: []settingsRow{listRow("/locks", locked)}}
}

func blacklistSettingsSection(chatID int64) settingsSection {
	bl := blacklists.GetBlacklistSettings(chatID)
	action := settingsDefaultBlacklistAction
	if len(bl) > 0 && bl.Action() != "" {
		action = bl.Action()
	}
	return settingsSection{Title: "Blacklists", Rows: []settingsRow{
		countRow("/blacklists", len(bl)),
		stringRow("/blaction", action, settingsDefaultBlacklistAction),
	}}
}

func greetingsSettingsSection(ctx context.Context, chatID int64) settingsSection {
	g, ok := readChatSetting[models.GreetingSettings](ctx, chatID)
	if !ok || g.WelcomeSettings == nil || g.GoodbyeSettings == nil {
		g = &models.GreetingSettings{
			WelcomeSettings: &models.WelcomeSettings{ShouldWelcome: true},
			GoodbyeSettings: &models.GoodbyeSettings{ShouldGoodbye: true},
		}
	}
	return settingsSection{Title: "Greetings", Rows: []settingsRow{
		boolRow("/welcome", g.WelcomeSettings.ShouldWelcome, true),
		customTextRow("/setwelcome", g.WelcomeSettings.WelcomeText, db.DefaultWelcome),
		boolRow("/cleanwelcome", g.WelcomeSettings.CleanWelcome, false),
		boolRow("/goodbye", g.GoodbyeSettings.ShouldGoodbye, true),
		customTextRow("/setgoodbye", g.GoodbyeSettings.GoodbyeText, db.DefaultGoodbye),
		boolRow("/cleangoodbye", g.GoodbyeSettings.CleanGoodbye, false),
		boolRow("/cleanservice", g.ShouldCleanService, false),
		boolRow("/autoapprove", g.ShouldAutoApprove, false),
	}}
}

func reportsSettingsSection(ctx context.Context, chatID int64) settingsSection {
	status, blocked, muteThreshold := true, 0, 0
	if r, ok := readChatSetting[models.ReportChatSettings](ctx, chatID); ok {
		status, blocked, muteThreshold = r.Status, len(r.BlockedList), r.MuteThreshold
	}
	mute := settingsRow{Command: "/reports mute", Value: "off"}
	if muteThreshold > 0 {
		mute = settingsRow{Command: "/reports mute", Value: strconv.Itoa(muteThreshold), Changed: true}
	}
	return settingsSection{Title: "Reports", Rows: []settingsRow{
		boolRow("/reports", status, true),
		countRow("/reports showblocklist", blocked),
		mute,
	}}
}

func rulesSettingsSection(ctx context.Context, chatID int64) settingsSection {
	r, ok := readChatSetting[models.RulesSettings](ctx, chatID)
	if !ok {
		r = &models.RulesSettings{}
	}
	return settingsSection{Title: "Rules", Rows: []settingsRow{
		customTextRow("/setrules", r.Rules, ""),
		customTextRow("/rulesbutton", r.RulesBtn, ""),
		boolRow("/privaterules", r.Private, false),
	}}
}

func notesSettingsSection(ctx context.Context, chatID int64) settingsSection {
	private := false
	if n, ok := readChatSetting[models.NotesSettings](ctx, chatID); ok {
		private = n.Private
	}
	return settingsSection{Title: "Notes & Filters", Rows: []settingsRow{
		countRow("/notes", len(notes.GetNotesList(chatID, true))),
		boolRow("/privnote", private, false),
		countRow("/filters", int(filters.CountFilters(chatID))),
	}}
}

func pinsSettingsSection(ctx context.Context, chatID int64) settingsSection {
	p, ok := readChatSetting[models.PinSettings](ctx, chatID)
	if !ok {
		p = &models.PinSettings{}
	}
	return settingsSection{Title: "Pins", Rows: []settingsRow{
		boolRow("/antichannelpin", p.AntiChannelPin, false),
		boolRow("/cleanlinked", p.CleanLinked, false),
	}}
}

func disablingSettingsSection(ctx context.Context, chatID int64) settingsSection {
	del := false
	if d, ok := readChatSetting[models.DisableChatSettings](ctx, chatID); ok {
		del = d.DeleteCommands
	}
	return settingsSection{Title: "Disabling", Rows: []settingsRow{
		listRow("/disabled", slices.Clone(disabling.GetChatDisabledCMDsCached(chatID))),
		boolRow("/disabledel", del, false),
	}}
}

func logChannelSettingsSection(chatID int64) settingsSection {
	lc := logchannels.Get(chatID)
	if lc == nil {
		return settingsSection{Title: "Log Channel", Rows: []settingsRow{{Command: "/logchannel", Value: "—"}}}
	}
	// "all" is every category on by default; an opt-in category shows only
	// once it is turned on, so a chat that never did reads as before.
	var off, optIn []string
	for _, name := range logchannels.AllCategories {
		on := logchannels.CategoryEnabled(lc, name)
		switch {
		case slices.Contains(logchannels.OptInCategories, name):
			if on {
				optIn = append(optIn, name)
			}
		case !on:
			off = append(off, name)
		}
	}
	categories := settingsRow{Command: "/logcategories", Value: "all"}
	if len(off) > 0 {
		categories = listRow("/logcategories", off)
		categories.Value = "off: " + categories.Value
	}
	if len(optIn) > 0 {
		categories.Value += " + " + listRow("/logcategories", optIn).Value
		categories.Changed = true
	}
	return settingsSection{Title: "Log Channel", Rows: []settingsRow{
		{Command: "/logchannel", Value: strconv.FormatInt(lc.LogChannelID, 10), Changed: true},
		categories,
	}}
}

func federationSettingsSection(chatID int64) settingsSection {
	fc := federations.GetChatFed(chatID)
	if fc == nil {
		return settingsSection{Title: "Federation", Rows: []settingsRow{{Command: "/chatfed", Value: "—"}}}
	}
	return settingsSection{Title: "Federation", Rows: []settingsRow{
		{Command: "/chatfed", Value: fc.FedID, Changed: true},
		boolRow("/quietfed", fc.Quiet, false),
	}}
}

func chatSettingsSection(ctx context.Context, chatID int64) settingsSection {
	language := settingsDefaultLanguage
	if chat, err := chats.GetChatBasicInfoCachedContext(ctx, chatID); err == nil && chat != nil && chat.Language != "" {
		language = chat.Language
	}
	allowConnect := true
	if c, ok := readChatSetting[models.ConnectionChatSettings](ctx, chatID); ok {
		allowConnect = c.AllowConnect
	}
	anonAdmin := false
	if a, ok := readChatSetting[models.AdminSettings](ctx, chatID); ok {
		anonAdmin = a.AnonAdmin
	}
	return settingsSection{Title: "Chat", Rows: []settingsRow{
		stringRow("/lang group", language, settingsDefaultLanguage),
		boolRow("/allowconnect", allowConnect, true),
		boolRow("/anonadmin", anonAdmin, false),
	}}
}

func renderSettingsSummary(tr *i18n.Translator, chatTitle string, sections []settingsSection) string {
	legend, _ := tr.GetString("settings_summary_legend")
	hint, _ := tr.GetString("settings_summary_toggle_hint")
	var b strings.Builder
	b.WriteString("⚙️ <b>")
	b.WriteString(formatting.HtmlEscape(chatTitle))
	b.WriteString("</b>\n")
	b.WriteString(legend)
	b.WriteString("\n")
	b.WriteString(hint)
	for _, section := range sections {
		b.WriteString("\n\n<b>")
		b.WriteString(formatting.HtmlEscape(section.Title))
		b.WriteString("</b>")
		for _, row := range section.Rows {
			b.WriteString("\n")
			if row.Changed {
				b.WriteString("• ")
			}
			b.WriteString("<code>")
			b.WriteString(formatting.HtmlEscape(row.Command))
			b.WriteString("</code> — ")
			b.WriteString(formatting.HtmlEscape(row.Value))
		}
	}
	return b.String()
}
