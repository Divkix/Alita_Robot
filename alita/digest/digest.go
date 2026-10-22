// Package digest renders and delivers the weekly group health digest.
//
// The message is counts only: messages, joins, leaves and moderation actions
// for one week, each with the change from the week before. No message content
// is read or stored, and no Redis is required.
package digest

import (
	"math"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/db/digestsettings"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

const dayLayout = "2006-01-02"

// Stats is one week's totals as the digest reports them.
type Stats struct {
	Messages   int64
	Joins      int64
	Leaves     int64
	Moderation int64
	// TopAction is the most frequent moderation action, empty without one.
	TopAction string
}

// Add folds one stored day into s.
func (s *Stats) Add(row models.ChatDailyStat) {
	s.Messages += row.Messages
	s.Joins += row.Joins
	s.Leaves += row.Leaves
}

// Empty reports whether the week had nothing to report.
func (s Stats) Empty() bool {
	return s.Messages == 0 && s.Joins == 0 && s.Leaves == 0 && s.Moderation == 0
}

// WeekStart returns Monday 00:00 UTC of the week containing t.
func WeekStart(t time.Time) time.Time {
	t = t.UTC()
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(day.Weekday()) + 6) % 7 // Monday is 0
	return day.AddDate(0, 0, -offset)
}

// ResolveDestination returns where a chat's digest goes. An explicit "group"
// always stays in the group; anything else uses the log channel when the chat
// has one and the group otherwise, so a removed log channel never loses a
// digest.
func ResolveDestination(configured string, hasLogChannel bool) string {
	if configured == digestsettings.DestinationGroup || !hasLogChannel {
		return digestsettings.DestinationGroup
	}
	return digestsettings.DestinationLog
}

// ReadStats sums a chat's stored counters for [from, to) and joins in the
// moderation events the same window recorded. Both reads use UTC bounds; the
// day keys come from utils/stats so the digest and the flusher agree.
func ReadStats(chatID int64, from, to time.Time) (Stats, error) {
	rows, err := chatstats.GetRange(chatID, stats.Day(from), stats.Day(to.Add(-time.Nanosecond)))
	if err != nil {
		return Stats{}, err
	}
	var total Stats
	for _, row := range rows {
		total.Add(row)
	}

	count, topAction, err := moderation.CountForChat(chatID, from, to)
	if err != nil {
		return Stats{}, err
	}
	total.Moderation = count
	total.TopAction = topAction
	return total, nil
}

// Build renders the digest for one week: current's totals against previous,
// localized through tr. A week without activity renders the short empty
// message instead of zero lines. dashboardURL, when not empty, becomes a single
// URL button; an empty URL renders no keyboard. Nothing in the input is user
// content, so every value can be formatted as HTML.
func Build(tr *i18n.Translator, current, previous Stats, weekStart time.Time, dashboardURL string) (string, *gotgbot.InlineKeyboardMarkup) {
	if current.Empty() {
		return digestLine(tr, "digest_message_empty"), dashboardKeyboard(tr, dashboardURL)
	}

	lines := []string{
		digestLine(tr, "digest_message_title"),
		digestLine(tr, "digest_message_period", i18n.TranslationParams{"date": weekStart.UTC().Format(dayLayout)}),
		"",
		digestCountLine(tr, "digest_message_messages", current.Messages, previous.Messages),
		digestCountLine(tr, "digest_message_joins", current.Joins, previous.Joins),
		digestCountLine(tr, "digest_message_leaves", current.Leaves, previous.Leaves),
		digestCountLine(tr, "digest_message_moderation", current.Moderation, previous.Moderation),
	}
	if name := actionName(tr, current.TopAction); name != "" {
		lines = append(lines, "", digestLine(tr, "digest_message_top_action", i18n.TranslationParams{"action": name}))
	}
	return strings.Join(lines, "\n"), dashboardKeyboard(tr, dashboardURL)
}

// digestCountLine renders one "count (change)" line.
func digestCountLine(tr *i18n.Translator, key string, current, previous int64) string {
	return digestLine(tr, key, i18n.TranslationParams{
		"count":  current,
		"change": changeText(tr, current, previous),
	})
}

// changeText names the change against the previous week. A previous week of
// zero has no percentage: it reads "new" when this week has some activity and
// "—" when it has none, so the line never shows NaN or infinity.
func changeText(tr *i18n.Translator, current, previous int64) string {
	if previous == 0 {
		if current == 0 {
			return digestLine(tr, "digest_change_none")
		}
		return digestLine(tr, "digest_change_new")
	}

	percent := int64(math.Round(float64(current-previous) / float64(previous) * 100))
	switch {
	case percent > 0:
		return digestLine(tr, "digest_change_up", i18n.TranslationParams{"percent": percent})
	case percent < 0:
		return digestLine(tr, "digest_change_down", i18n.TranslationParams{"percent": -percent})
	default:
		return digestLine(tr, "digest_change_same")
	}
}

// actionName localizes a recorded moderation action; an unknown action renders
// no line rather than a raw constant.
func actionName(tr *i18n.Translator, action string) string {
	var key string
	switch action {
	case models.ModerationActionBan:
		key = "digest_action_ban"
	case models.ModerationActionKick:
		key = "digest_action_kick"
	case models.ModerationActionMute:
		key = "digest_action_mute"
	case models.ModerationActionUnban:
		key = "digest_action_unban"
	default:
		return ""
	}
	name, err := tr.GetString(key)
	if err != nil {
		return ""
	}
	return name
}

// dashboardKeyboard adds the dashboard button only when a panel URL is known.
func dashboardKeyboard(tr *i18n.Translator, dashboardURL string) *gotgbot.InlineKeyboardMarkup {
	if dashboardURL == "" {
		return nil
	}
	label, err := tr.GetString("digest_dashboard_button")
	if err != nil {
		return nil
	}
	return &gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
			{Text: label, Url: dashboardURL},
		}},
	}
}

// digestLine reads a localized string. A missing key yields "", which callers
// treat as an absent line; the locale check enforces the key's presence.
func digestLine(tr *i18n.Translator, key string, params ...i18n.TranslationParams) string {
	text, _ := tr.GetString(key, params...)
	return text
}
