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
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/reactions"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/settings"
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
	settingsDefaultBlacklistAction = "warn"
	settingsMaxListedItems         = 15
)

func boolRow(command string, value, def bool) settingsRow {
	text := "off"
	if value {
		text = "on"
	}
	return settingsRow{Command: command, Value: text, Changed: value != def}
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
		{Title: "AI Spam", Rows: []settingsRow{declaredBoolRow("/aispam", settings.AISpamEnabled, chatID)}},
		probationSettingsSection(ctx, chatID),
		warnsSettingsSection(ctx, chatID),
		locksSettingsSection(ctx, chatID),
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

func captchaSettingsSection(_ context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Captcha", Rows: []settingsRow{
		declaredBoolRow("/captcha", settings.CaptchaEnabled, chatID),
		declaredBoolRow("/captchajoin", settings.CaptchaJoinRequests, chatID),
		declaredStringRow("/captchamode", settings.CaptchaMode.Read(chatID)),
		declaredSettingsRow("/captchatime", settings.CaptchaTimeout.Read(chatID), strconv.Itoa),
		declaredStringRow("/captchaaction", settings.CaptchaAction.Read(chatID)),
		declaredSettingsRow("/captchamaxattempts", settings.CaptchaMaxAttempts.Read(chatID), strconv.Itoa),
	}}
}

func declaredStringRow(command string, state settings.State) settingsRow {
	return settingsRow{Command: command, Value: state.Value.(string), Changed: state.Changed}
}

func antifloodSettingsSection(chatID int64) settingsSection {
	f := settings.ReadFlood(chatID)
	limit := declaredSettingsRow("/setflood", settings.FloodLimit.Read(chatID), func(value int) string {
		if value == 0 {
			return "off"
		}
		return strconv.Itoa(value)
	})
	return settingsSection{Title: "Antiflood", Rows: []settingsRow{
		limit,
		{Command: "/setfloodmode", Value: floodModeLabel(f), Changed: settings.FloodMode.Read(chatID).Changed},
		declaredBoolRow("/delflood", settings.FloodDelete, chatID),
	}}
}

func antiraidSettingsSection(chatID int64) settingsSection {
	st := getRaidState(chatID)
	active := st.Active && time.Now().Unix() <= st.ExpiresAt
	return settingsSection{Title: "AntiRaid", Rows: []settingsRow{
		boolRow("/antiraid", active, false),
		declaredSettingsRow("/raidtime", settings.RaidTime.Read(chatID), formatDuration),
		declaredSettingsRow("/raidactiontime", settings.RaidActionTime.Read(chatID), formatDuration),
		declaredSettingsRow("/autoantiraid", settings.AutoAntiRaidThreshold.Read(chatID), func(value int) string {
			if value == 0 {
				return "off"
			}
			return strconv.Itoa(value)
		}),
	}}
}

func declaredSettingsRow(command string, state settings.State, format func(int) string) settingsRow {
	return settingsRow{Command: command, Value: format(state.Value.(int)), Changed: state.Changed}
}

func probationSettingsSection(_ context.Context, chatID int64) settingsSection {
	links := settings.ProbationBlockLinks.Read(chatID)
	forwards := settings.ProbationBlockForwards.Read(chatID)
	media := settings.ProbationBlockMedia.Read(chatID)
	blocked := &models.ProbationSettings{BlockLinks: links.Value.(bool), BlockForwards: forwards.Value.(bool), BlockMedia: media.Value.(bool)}
	return settingsSection{Title: "Probation", Rows: []settingsRow{
		declaredBoolRow("/probation", settings.ProbationEnabled, chatID),
		declaredSettingsRow("/probation time", settings.ProbationTime.Read(chatID), formatDuration),
		declaredSettingsRow("/probation messages", settings.ProbationMessages.Read(chatID), strconv.Itoa),
		declaredSettingsRow("/probation minlen", settings.ProbationMinLength.Read(chatID), strconv.Itoa),
		{Command: "/probation block", Value: probationSettingSummary(blocked), Changed: links.Changed || forwards.Changed || media.Changed},
		declaredBoolRow("/probation notice", settings.ProbationNotice, chatID),
	}}
}

// probationSettingSummary is the value of the /probation block summary row.
func probationSettingSummary(settings *models.ProbationSettings) string {
	kinds := probationBlockedKinds(settings)
	if len(kinds) == 0 {
		return "—"
	}
	return strings.Join(kinds, ", ")
}

func warnsSettingsSection(ctx context.Context, chatID int64) settingsSection {
	w := settings.ReadWarnsContext(ctx, chatID)
	mode := warnModeLabel(w)
	return settingsSection{Title: "Warns", Rows: []settingsRow{
		declaredSettingsRow("/setwarnlimit", settings.WarnLimit.Read(chatID), strconv.Itoa),
		{Command: "/setwarnmode", Value: mode, Changed: settings.WarnMode.Read(chatID).Changed},
		declaredSettingsRow("/setwarntime", settings.WarnExpiry.Read(chatID), func(value int) string {
			if value == 0 {
				return "off"
			}
			return formatDuration(value)
		}),
	}}
}

func locksSettingsSection(_ context.Context, chatID int64) settingsSection {
	var locked []string
	for perm, on := range locks.GetChatLocks(chatID) {
		if on {
			locked = append(locked, perm)
		}
	}
	night := "off"
	if n, err := settings.ReadNightMode(chatID); err != nil {
		log.Warnf("[Settings] read night mode for chat %d: %v", chatID, err)
	} else if n.Enabled {
		start, end, types := nightWindowLabel(n)
		night = start + "-" + end + " " + types
	}
	return settingsSection{Title: "Locks", Rows: []settingsRow{
		listRow("/locks", locked),
		stringRow("/nightmode", night, "off"),
	}}
}

func blacklistSettingsSection(chatID int64) settingsSection {
	bl := blacklists.GetBlacklistSettings(chatID)
	action := settingsDefaultBlacklistAction
	if len(bl) > 0 && bl.Action() != "" {
		action = blacklistModeLabel(bl)
	}
	return settingsSection{Title: "Blacklists", Rows: []settingsRow{
		countRow("/blacklists", len(bl)),
		stringRow("/blaction", action, settingsDefaultBlacklistAction),
	}}
}

func declaredBoolRow(command string, declaration *settings.Declaration, chatID int64) settingsRow {
	state := declaration.Read(chatID)
	value := "off"
	if state.Value.(bool) {
		value = "on"
	}
	return settingsRow{Command: command, Value: value, Changed: state.Changed}
}

func greetingsSettingsSection(_ context.Context, chatID int64) settingsSection {
	g := settings.ReadGreetings(chatID)
	return settingsSection{Title: "Greetings", Rows: []settingsRow{
		declaredBoolRow("/welcome", settings.Welcome, chatID),
		customTextRow("/setwelcome", g.WelcomeSettings.WelcomeText, db.DefaultWelcome),
		declaredBoolRow("/cleanwelcome", settings.CleanWelcome, chatID),
		declaredBoolRow("/goodbye", settings.Goodbye, chatID),
		customTextRow("/setgoodbye", g.GoodbyeSettings.GoodbyeText, db.DefaultGoodbye),
		declaredBoolRow("/cleangoodbye", settings.CleanGoodbye, chatID),
		declaredBoolRow("/cleanservice", settings.CleanService, chatID),
		declaredBoolRow("/autoapprove", settings.AutoApprove, chatID),
	}}
}

func reportsSettingsSection(_ context.Context, chatID int64) settingsSection {
	row := settings.ReadReports(chatID)
	mute := declaredSettingsRow("/reports mute", settings.ReportMuteThreshold.Read(chatID), func(value int) string {
		if value == 0 {
			return "off"
		}
		return strconv.Itoa(value)
	})
	return settingsSection{Title: "Reports", Rows: []settingsRow{
		declaredBoolRow("/reports", settings.ReportsEnabled, chatID),
		countRow("/reports showblocklist", len(row.BlockedList)),
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
		declaredBoolRow("/privaterules", settings.PrivateRules, chatID),
	}}
}

func notesSettingsSection(_ context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Notes & Filters", Rows: []settingsRow{
		countRow("/notes", len(notes.GetNotesList(chatID, true))),
		declaredBoolRow("/privnote", settings.PrivateNotes, chatID),
		countRow("/filters", int(filters.CountFilters(chatID))),
	}}
}

func pinsSettingsSection(_ context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Pins", Rows: []settingsRow{
		declaredBoolRow("/antichannelpin", settings.AntiChannelPin, chatID),
		declaredBoolRow("/cleanlinked", settings.CleanLinked, chatID),
	}}
}

func disablingSettingsSection(_ context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Disabling", Rows: []settingsRow{
		listRow("/disabled", slices.Clone(disabling.GetChatDisabledCMDsCached(chatID))),
		declaredBoolRow("/disabledel", settings.DeleteDisabledCommands, chatID),
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
		declaredBoolRow("/quietfed", settings.QuietFed, chatID),
	}}
}

func chatSettingsSection(_ context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Chat", Rows: []settingsRow{
		declaredStringRow("/lang group", settings.Language.Read(chatID)),
		declaredStringRow("/timezone", settings.Timezone.Read(chatID)),
		declaredBoolRow("/allowconnect", settings.AllowConnect, chatID),
		declaredBoolRow("/anonadmin", settings.AnonAdmin, chatID),
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
