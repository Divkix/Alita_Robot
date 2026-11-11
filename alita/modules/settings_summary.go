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
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
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

// Blacklist actions are command-only; shared settings get defaults from declarations.
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

// readChatSetting loads raw per-chat preferences for read-only summary values
// without creating a row.
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
		antifloodSettingsSection(ctx, chatID),
		antiraidSettingsSection(ctx, chatID),
		{Title: "AI Spam", Rows: []settingsRow{declaredBoolRow(ctx, "/aispam", chatsettings.AISpamEnabled, chatID)}},
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
		federationSettingsSection(ctx, chatID),
		chatSettingsSection(ctx, chatID),
	}
	return sections
}

func captchaSettingsSection(ctx context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Captcha", Rows: []settingsRow{
		declaredBoolRow(ctx, "/captcha", chatsettings.CaptchaEnabled, chatID),
		declaredBoolRow(ctx, "/captchajoin", chatsettings.CaptchaJoinRequests, chatID),
		declaredStringRow("/captchamode", chatsettings.CaptchaMode.ReadContext(ctx, chatID)),
		declaredSettingsRow("/captchatime", chatsettings.CaptchaTimeout.ReadContext(ctx, chatID), strconv.Itoa),
		declaredStringRow("/captchaaction", chatsettings.CaptchaAction.ReadContext(ctx, chatID)),
		declaredSettingsRow("/captchamaxattempts", chatsettings.CaptchaMaxAttempts.ReadContext(ctx, chatID), strconv.Itoa),
	}}
}

func declaredStringRow(command string, state chatsettings.State) settingsRow {
	return settingsRow{Command: command, Value: state.Value.(string), Changed: state.Changed}
}

func antifloodSettingsSection(ctx context.Context, chatID int64) settingsSection {
	f := chatsettings.ReadFloodContext(ctx, chatID)
	limit := declaredSettingsRow("/setflood", chatsettings.FloodLimit.ReadContext(ctx, chatID), func(value int) string {
		if value == 0 {
			return "off"
		}
		return strconv.Itoa(value)
	})
	return settingsSection{Title: "Antiflood", Rows: []settingsRow{
		limit,
		{Command: "/setfloodmode", Value: floodModeLabel(f), Changed: chatsettings.FloodMode.ReadContext(ctx, chatID).Changed},
		declaredBoolRow(ctx, "/delflood", chatsettings.FloodDelete, chatID),
	}}
}

func antiraidSettingsSection(ctx context.Context, chatID int64) settingsSection {
	st := getRaidState(chatID)
	active := st.Active && time.Now().Unix() <= st.ExpiresAt
	return settingsSection{Title: "AntiRaid", Rows: []settingsRow{
		boolRow("/antiraid", active, false),
		declaredSettingsRow("/raidtime", chatsettings.RaidTime.ReadContext(ctx, chatID), formatDuration),
		declaredSettingsRow("/raidactiontime", chatsettings.RaidActionTime.ReadContext(ctx, chatID), formatDuration),
		declaredSettingsRow("/autoantiraid", chatsettings.AutoAntiRaidThreshold.ReadContext(ctx, chatID), func(value int) string {
			if value == 0 {
				return "off"
			}
			return strconv.Itoa(value)
		}),
	}}
}

func declaredSettingsRow(command string, state chatsettings.State, format func(int) string) settingsRow {
	return settingsRow{Command: command, Value: format(state.Value.(int)), Changed: state.Changed}
}

func probationSettingsSection(ctx context.Context, chatID int64) settingsSection {
	links := chatsettings.ProbationBlockLinks.ReadContext(ctx, chatID)
	forwards := chatsettings.ProbationBlockForwards.ReadContext(ctx, chatID)
	media := chatsettings.ProbationBlockMedia.ReadContext(ctx, chatID)
	blocked := &models.ProbationSettings{BlockLinks: links.Value.(bool), BlockForwards: forwards.Value.(bool), BlockMedia: media.Value.(bool)}
	return settingsSection{Title: "Probation", Rows: []settingsRow{
		declaredBoolRow(ctx, "/probation", chatsettings.ProbationEnabled, chatID),
		declaredSettingsRow("/probation time", chatsettings.ProbationTime.ReadContext(ctx, chatID), formatDuration),
		declaredSettingsRow("/probation messages", chatsettings.ProbationMessages.ReadContext(ctx, chatID), strconv.Itoa),
		declaredSettingsRow("/probation minlen", chatsettings.ProbationMinLength.ReadContext(ctx, chatID), strconv.Itoa),
		{Command: "/probation block", Value: probationSettingSummary(blocked), Changed: links.Changed || forwards.Changed || media.Changed},
		declaredBoolRow(ctx, "/probation notice", chatsettings.ProbationNotice, chatID),
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
	w := chatsettings.ReadWarnsContext(ctx, chatID)
	mode := warnModeLabel(w)
	return settingsSection{Title: "Warns", Rows: []settingsRow{
		declaredSettingsRow("/setwarnlimit", chatsettings.WarnLimit.ReadContext(ctx, chatID), strconv.Itoa),
		{Command: "/setwarnmode", Value: mode, Changed: chatsettings.WarnMode.ReadContext(ctx, chatID).Changed},
		declaredSettingsRow("/setwarntime", chatsettings.WarnExpiry.ReadContext(ctx, chatID), func(value int) string {
			if value == 0 {
				return "off"
			}
			return formatDuration(value)
		}),
	}}
}

func locksSettingsSection(ctx context.Context, chatID int64) settingsSection {
	var locked []string
	for perm, on := range locks.GetChatLocks(chatID) {
		if on {
			locked = append(locked, perm)
		}
	}
	night := "off"
	if n, err := chatsettings.ReadNightModeContext(ctx, chatID); err != nil {
		log.Warnf("[Settings] read night mode for chat %d: %v", chatID, err)
	} else if n.Enabled {
		start, end, types := nightWindowLabel(n)
		night = start + "-" + end + " " + types
	}
	return settingsSection{Title: "Locks", Rows: []settingsRow{
		listRow("/locks", locked),
		{Command: "/nightmode", Value: night, Changed: chatsettings.NightModeEnabled.ReadContext(ctx, chatID).Changed},
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

func declaredBoolRow(ctx context.Context, command string, declaration *chatsettings.Declaration, chatID int64) settingsRow {
	state := declaration.ReadContext(ctx, chatID)
	value := "off"
	if state.Value.(bool) {
		value = "on"
	}
	return settingsRow{Command: command, Value: value, Changed: state.Changed}
}

func greetingsSettingsSection(ctx context.Context, chatID int64) settingsSection {
	g := chatsettings.ReadGreetings(chatID)
	return settingsSection{Title: "Greetings", Rows: []settingsRow{
		declaredBoolRow(ctx, "/welcome", chatsettings.Welcome, chatID),
		customTextRow("/setwelcome", g.WelcomeSettings.WelcomeText, db.DefaultWelcome),
		declaredBoolRow(ctx, "/cleanwelcome", chatsettings.CleanWelcome, chatID),
		declaredBoolRow(ctx, "/goodbye", chatsettings.Goodbye, chatID),
		customTextRow("/setgoodbye", g.GoodbyeSettings.GoodbyeText, db.DefaultGoodbye),
		declaredBoolRow(ctx, "/cleangoodbye", chatsettings.CleanGoodbye, chatID),
		declaredBoolRow(ctx, "/cleanservice", chatsettings.CleanService, chatID),
		declaredBoolRow(ctx, "/autoapprove", chatsettings.AutoApprove, chatID),
	}}
}

func reportsSettingsSection(ctx context.Context, chatID int64) settingsSection {
	row := chatsettings.ReadReports(chatID)
	mute := declaredSettingsRow("/reports mute", chatsettings.ReportMuteThreshold.ReadContext(ctx, chatID), func(value int) string {
		if value == 0 {
			return "off"
		}
		return strconv.Itoa(value)
	})
	return settingsSection{Title: "Reports", Rows: []settingsRow{
		declaredBoolRow(ctx, "/reports", chatsettings.ReportsEnabled, chatID),
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
		declaredBoolRow(ctx, "/privaterules", chatsettings.PrivateRules, chatID),
	}}
}

func notesSettingsSection(ctx context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Notes & Filters", Rows: []settingsRow{
		countRow("/notes", len(notes.GetNotesList(chatID, true))),
		declaredBoolRow(ctx, "/privnote", chatsettings.PrivateNotes, chatID),
		countRow("/filters", int(filters.CountFilters(chatID))),
	}}
}

func pinsSettingsSection(ctx context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Pins", Rows: []settingsRow{
		declaredBoolRow(ctx, "/antichannelpin", chatsettings.AntiChannelPin, chatID),
		declaredBoolRow(ctx, "/cleanlinked", chatsettings.CleanLinked, chatID),
	}}
}

func disablingSettingsSection(ctx context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Disabling", Rows: []settingsRow{
		listRow("/disabled", slices.Clone(disabling.GetChatDisabledCMDsCached(chatID))),
		declaredBoolRow(ctx, "/disabledel", chatsettings.DeleteDisabledCommands, chatID),
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

func federationSettingsSection(ctx context.Context, chatID int64) settingsSection {
	fc := federations.GetChatFed(chatID)
	if fc == nil {
		return settingsSection{Title: "Federation", Rows: []settingsRow{{Command: "/chatfed", Value: "—"}}}
	}
	return settingsSection{Title: "Federation", Rows: []settingsRow{
		{Command: "/chatfed", Value: fc.FedID, Changed: true},
		declaredBoolRow(ctx, "/quietfed", chatsettings.QuietFed, chatID),
	}}
}

func chatSettingsSection(ctx context.Context, chatID int64) settingsSection {
	return settingsSection{Title: "Chat", Rows: []settingsRow{
		declaredStringRow("/lang group", chatsettings.Language.ReadContext(ctx, chatID)),
		declaredStringRow("/timezone", chatsettings.Timezone.ReadContext(ctx, chatID)),
		declaredBoolRow(ctx, "/allowconnect", chatsettings.AllowConnect, chatID),
		declaredBoolRow(ctx, "/anonadmin", chatsettings.AnonAdmin, chatID),
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
