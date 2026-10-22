//go:build testtools

package digest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/db/digestsettings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/panel"
)

var digestChatSeq atomic.Int64

// newDigestTestChat returns a chat ID no other test in this package uses and
// removes the rows it touched when the test ends.
func newDigestTestChat(t *testing.T) int64 {
	t.Helper()
	chatID := time.Now().UnixNano() + digestChatSeq.Add(1)
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.DigestSettings{}).Error; err != nil {
			t.Errorf("cleanup digest settings of chat %d: %v", chatID, err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatDailyStat{}).Error; err != nil {
			t.Errorf("cleanup daily stats of chat %d: %v", chatID, err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ModerationEvent{}).Error; err != nil {
			t.Errorf("cleanup moderation events of chat %d: %v", chatID, err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.LogChannel{}).Error; err != nil {
			t.Errorf("cleanup log channel of chat %d: %v", chatID, err)
		}
	})
	return chatID
}

// withDigestConfig pins the two operator switches for the test and restores
// whatever the environment had afterwards.
func withDigestConfig(t *testing.T, enableDigest, panelEnabled bool) {
	t.Helper()
	if config.AppConfig == nil {
		t.Fatal("config.AppConfig is nil")
	}
	previous := *config.AppConfig
	config.AppConfig.EnableDigest = enableDigest
	config.AppConfig.PanelEnabled = panelEnabled
	config.AppConfig.PanelMiniAppShortName = ""
	t.Cleanup(func() { *config.AppConfig = previous })
}

// seedWeek stores one week's counters on its first day and records the same
// week's moderation events.
func seedWeek(t *testing.T, chatID int64, weekStart time.Time, week Stats) {
	t.Helper()
	if err := chatstats.AddDaily([]models.ChatDailyStat{{
		ChatID:   chatID,
		Day:      weekStart.UTC().Format(dayLayout),
		Messages: week.Messages,
		Joins:    week.Joins,
		Leaves:   week.Leaves,
	}}); err != nil {
		t.Fatalf("AddDaily() error = %v", err)
	}
	for i := int64(0); i < week.Moderation; i++ {
		action := week.TopAction
		if action == "" {
			action = models.ModerationActionBan
		}
		if err := moderation.Record(models.ModerationEvent{
			ChatId:       chatID,
			ActorId:      1,
			TargetUserId: 2,
			Action:       action,
			CreatedAt:    weekStart.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	}
}

func enableDigest(t *testing.T, chatID int64, destination string) {
	t.Helper()
	if err := digestsettings.SetEnabled(chatID, true); err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	if destination != digestsettings.DestinationAuto {
		if err := digestsettings.SetDestination(chatID, destination); err != nil {
			t.Fatalf("SetDestination(%q) error = %v", destination, err)
		}
	}
}

func digestSettingsOf(t *testing.T, chatID int64) *models.DigestSettings {
	t.Helper()
	settings, err := digestsettings.Get(chatID)
	if err != nil {
		t.Fatalf("Get(%d) error = %v", chatID, err)
	}
	return settings
}

// dueRow returns the chat's row from the scheduler's query.
func dueRow(t *testing.T, chatID int64, boundary time.Time) *models.DigestSettings {
	t.Helper()
	due, err := digestsettings.ListDue(boundary)
	if err != nil {
		t.Fatalf("ListDue() error = %v", err)
	}
	for i := range due {
		if due[i].ChatID == chatID {
			return &due[i]
		}
	}
	t.Fatalf("chat %d is not in the due list", chatID)
	return nil
}

type digestBotCall struct {
	Method string
	Params map[string]any
}

// digestBotClient is a hand-written gotgbot.BotClient: it records every call
// and can fail a set number of sends, which stands in for an unreachable
// Telegram API.
type digestBotClient struct {
	mu        sync.Mutex
	calls     []digestBotCall
	failSends int
	// sendErr is what a failed send returns; nil means a transient network
	// failure.
	sendErr error
}

func (c *digestBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	copied := make(map[string]any, len(params))
	for key, value := range params {
		copied[key] = value
	}
	c.calls = append(c.calls, digestBotCall{Method: method, Params: copied})

	if method == "sendMessage" {
		if c.failSends > 0 {
			c.failSends--
			if c.sendErr != nil {
				return nil, c.sendErr
			}
			return nil, errors.New("telegram is unreachable")
		}
		return json.RawMessage(`{"message_id":1,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Test"}}`), nil
	}
	return json.RawMessage(`true`), nil
}

func (c *digestBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *digestBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func (c *digestBotClient) sends() []digestBotCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	var sends []digestBotCall
	for _, call := range c.calls {
		if call.Method == "sendMessage" {
			sends = append(sends, call)
		}
	}
	return sends
}

func newDigestTestBot(client *digestBotClient) *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User: gotgbot.User{
			Id:       999,
			IsBot:    true,
			Username: "DigestTestBot",
		},
	}
}

// TestReadStatsSumsTheWindowOnly checks the numbers behind the digest: the
// counters inside [from, to) and the moderation events in the same window.
func TestReadStatsSumsTheWindowOnly(t *testing.T) {
	chatID := newDigestTestChat(t)
	from := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 7)
	seedWeek(t, chatID, from, Stats{Messages: 10, Joins: 2, Leaves: 1, Moderation: 2, TopAction: models.ModerationActionMute})

	if err := chatstats.AddDaily([]models.ChatDailyStat{
		{ChatID: chatID, Day: "2026-10-11", Messages: 100},
		{ChatID: chatID, Day: "2026-10-19", Messages: 200},
	}); err != nil {
		t.Fatalf("AddDaily(outside rows) error = %v", err)
	}
	if err := moderation.Record(models.ModerationEvent{
		ChatId: chatID, ActorId: 1, TargetUserId: 2,
		Action: models.ModerationActionBan, CreatedAt: from.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("Record(outside event) error = %v", err)
	}

	total, err := ReadStats(chatID, from, to)
	if err != nil {
		t.Fatalf("ReadStats() error = %v", err)
	}
	if total.Messages != 10 || total.Joins != 2 || total.Leaves != 1 {
		t.Fatalf("counters = %d/%d/%d, want 10/2/1 from the window only", total.Messages, total.Joins, total.Leaves)
	}
	if total.Moderation != 2 {
		t.Fatalf("moderation = %d, want 2 inside the window", total.Moderation)
	}
	if total.TopAction != models.ModerationActionMute {
		t.Fatalf("top action = %q, want mute", total.TopAction)
	}
}

// Two replicas that read the same due row must send one message: the second
// claim updates zero rows. The message contains the totals, the changes and no
// dashboard button while the panel is off.
func TestSchedulerSendsOneDigestAcrossTwoReplicas(t *testing.T) {
	useDigestLocales(t)
	withDigestConfig(t, true, false)

	chatID := newDigestTestChat(t)
	weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	now := weekStart.Add(12 * time.Hour)
	seedWeek(t, chatID, weekStart.AddDate(0, 0, -7), Stats{Messages: 10, Joins: 2, Leaves: 1, Moderation: 2, TopAction: models.ModerationActionBan})
	seedWeek(t, chatID, weekStart.AddDate(0, 0, -14), Stats{Messages: 5, Joins: 1, Leaves: 1})
	enableDigest(t, chatID, digestsettings.DestinationGroup)

	row := dueRow(t, chatID, weekStart)

	client := &digestBotClient{}
	bot := newDigestTestBot(client)
	first := NewScheduler(bot)
	first.now = func() time.Time { return now }
	second := NewScheduler(bot)
	second.now = func() time.Time { return now }

	// Both replicas hold the row they listed before either claimed it.
	first.send(row, weekStart)
	second.send(row, weekStart)

	sends := client.sends()
	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want exactly 1 across two replicas", len(sends))
	}
	call := sends[0]
	if got := fmt.Sprint(call.Params["chat_id"]); got != fmt.Sprint(chatID) {
		t.Fatalf("digest sent to chat %s, want %d", got, chatID)
	}

	tr := i18n.MustNewTranslator("en")
	up100, _ := tr.GetString("digest_change_up", i18n.TranslationParams{"percent": int64(100)})
	same, _ := tr.GetString("digest_change_same")
	newChange, _ := tr.GetString("digest_change_new")
	for _, want := range []struct {
		key    string
		count  int64
		change string
	}{
		{"digest_message_messages", 10, up100},
		{"digest_message_joins", 2, up100},
		{"digest_message_leaves", 1, same},
		{"digest_message_moderation", 2, newChange},
	} {
		line, _ := tr.GetString(want.key, i18n.TranslationParams{"count": want.count, "change": want.change})
		if !strings.Contains(fmt.Sprint(call.Params["text"]), line) {
			t.Errorf("digest %q does not contain %q", call.Params["text"], line)
		}
	}
	if markup, present := call.Params["reply_markup"]; present && markup != nil {
		t.Fatal("digest has a keyboard although the panel is disabled")
	}

	settings := digestSettingsOf(t, chatID)
	if settings.LastSentAt == nil || !settings.LastSentAt.Equal(now) {
		t.Fatalf("last_sent_at = %v, want the winning claim %v", settings.LastSentAt, now)
	}
}

// A failed send gives the claim back, so the next tick retries the same week.
func TestSchedulerRetriesAFailedSend(t *testing.T) {
	useDigestLocales(t)
	withDigestConfig(t, true, false)

	chatID := newDigestTestChat(t)
	weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	now := weekStart.Add(12 * time.Hour)
	seedWeek(t, chatID, weekStart.AddDate(0, 0, -7), Stats{Messages: 4})
	enableDigest(t, chatID, digestsettings.DestinationGroup)

	client := &digestBotClient{failSends: 1}
	bot := newDigestTestBot(client)
	scheduler := NewScheduler(bot)
	scheduler.now = func() time.Time { return now }

	scheduler.runDue(now)
	if got := len(client.sends()); got != 1 {
		t.Fatalf("sendMessage attempts = %d, want 1", got)
	}
	if settings := digestSettingsOf(t, chatID); settings.LastSentAt != nil {
		t.Fatalf("last_sent_at = %v after a failed send, want nil", settings.LastSentAt)
	}

	next := now.Add(time.Hour)
	scheduler.now = func() time.Time { return next }
	scheduler.runDue(next)

	if got := len(client.sends()); got != 2 {
		t.Fatalf("sendMessage attempts = %d, want the retry as a second attempt", got)
	}
	settings := digestSettingsOf(t, chatID)
	if settings.LastSentAt == nil || !settings.LastSentAt.Equal(next) {
		t.Fatalf("last_sent_at = %v, want the retry claim %v", settings.LastSentAt, next)
	}
}

// A permanent send failure (bot removed, chat gone, no send rights) cannot be
// fixed by retrying: the claim stays, the failure is logged once at warn, and
// the next tick sends nothing for that week.
func TestSchedulerDoesNotRetryAPermanentSendFailure(t *testing.T) {
	permanent := map[string]error{
		"bot removed":    errors.New("Forbidden: bot was kicked from the group chat"),
		"bot not member": errors.New("Forbidden: bot is not a member of the supergroup chat"),
		"chat not found": errors.New("Bad Request: chat not found"),
		"deactivated":    errors.New("Forbidden: group chat was deactivated"),
		"upgraded":       errors.New("Bad Request: group chat was upgraded to a supergroup chat"),
		"no send rights": errors.New("Bad Request: not enough rights to send text messages to the chat"),
	}
	for name, sendErr := range permanent {
		t.Run(name, func(t *testing.T) {
			useDigestLocales(t)
			withDigestConfig(t, true, false)
			hook := logrustest.NewGlobal()
			t.Cleanup(hook.Reset)

			chatID := newDigestTestChat(t)
			weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
			now := weekStart.Add(12 * time.Hour)
			seedWeek(t, chatID, weekStart.AddDate(0, 0, -7), Stats{Messages: 4})
			enableDigest(t, chatID, digestsettings.DestinationGroup)

			client := &digestBotClient{failSends: 10, sendErr: sendErr}
			scheduler := NewScheduler(newDigestTestBot(client))
			scheduler.now = func() time.Time { return now }
			scheduler.runDue(now)

			if got := len(client.sends()); got != 1 {
				t.Fatalf("sendMessage attempts = %d, want 1", got)
			}
			settings := digestSettingsOf(t, chatID)
			if settings.LastSentAt == nil || !settings.LastSentAt.Equal(now) {
				t.Fatalf("last_sent_at = %v after a permanent failure, want the claim %v kept", settings.LastSentAt, now)
			}

			warns := 0
			for _, entry := range hook.AllEntries() {
				if !strings.Contains(entry.Message, "[Digest]") {
					continue
				}
				if entry.Level == log.ErrorLevel {
					t.Errorf("permanent failure logged at error: %q", entry.Message)
				}
				if entry.Level == log.WarnLevel {
					warns++
				}
			}
			if warns != 1 {
				t.Fatalf("digest warnings = %d, want the one permanent-failure warning", warns)
			}

			next := now.Add(time.Hour)
			scheduler.now = func() time.Time { return next }
			scheduler.runDue(next)
			if got := len(client.sends()); got != 1 {
				t.Fatalf("sendMessage attempts after the next tick = %d, want no retry", got)
			}
		})
	}
}

// The operator switch stops every send and leaves the chat's own settings
// exactly as they were.
func TestSchedulerSendsNothingWhenTheSwitchIsOff(t *testing.T) {
	useDigestLocales(t)
	withDigestConfig(t, false, false)

	chatID := newDigestTestChat(t)
	weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	now := weekStart.Add(12 * time.Hour)
	seedWeek(t, chatID, weekStart.AddDate(0, 0, -7), Stats{Messages: 4})
	enableDigest(t, chatID, digestsettings.DestinationLog)

	client := &digestBotClient{}
	scheduler := NewScheduler(newDigestTestBot(client))
	scheduler.now = func() time.Time { return now }
	scheduler.runDue(now)

	if sends := client.sends(); len(sends) != 0 {
		t.Fatalf("sendMessage calls = %d with the switch off, want none", len(sends))
	}
	settings := digestSettingsOf(t, chatID)
	if !settings.Enabled {
		t.Fatal("the switch off disabled the chat's digest")
	}
	if settings.Destination != digestsettings.DestinationLog {
		t.Fatalf("destination = %q, want the chat's choice kept", settings.Destination)
	}
	if settings.LastSentAt != nil {
		t.Fatalf("last_sent_at = %v with the switch off, want untouched", settings.LastSentAt)
	}
}

// Destination resolution from the scheduler's side: the log channel by
// default, an explicit group, and a fallback to the group when an explicit
// log channel has been removed.
func TestSchedulerDestinations(t *testing.T) {
	cases := []struct {
		name        string
		destination string
		hasLog      bool
		sendToLog   bool
	}{
		{"auto uses the log channel", digestsettings.DestinationAuto, true, true},
		{"auto without a log channel uses the group", digestsettings.DestinationAuto, false, false},
		{"explicit group ignores the log channel", digestsettings.DestinationGroup, true, false},
		{"explicit log without a channel falls back", digestsettings.DestinationLog, false, false},
		{"explicit log with a channel", digestsettings.DestinationLog, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useDigestLocales(t)
			withDigestConfig(t, true, false)

			chatID := newDigestTestChat(t)
			weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
			now := weekStart.Add(time.Hour)
			seedWeek(t, chatID, weekStart.AddDate(0, 0, -7), Stats{Messages: 1})
			enableDigest(t, chatID, tc.destination)
			logChannelID := chatID + 1
			if tc.hasLog {
				if err := logchannels.Set(chatID, "Digest Chat", logChannelID); err != nil {
					t.Fatalf("logchannels.Set() error = %v", err)
				}
			}

			client := &digestBotClient{}
			scheduler := NewScheduler(newDigestTestBot(client))
			scheduler.now = func() time.Time { return now }
			scheduler.runDue(now)

			sends := client.sends()
			if len(sends) != 1 {
				t.Fatalf("sendMessage calls = %d, want 1", len(sends))
			}
			want := chatID
			if tc.sendToLog {
				want = logChannelID
			}
			if got := fmt.Sprint(sends[0].Params["chat_id"]); got != fmt.Sprint(want) {
				t.Fatalf("digest sent to %s, want %d", got, want)
			}
		})
	}
}

// The group's language decides the digest's language.
func TestSchedulerUsesTheChatLanguage(t *testing.T) {
	useDigestLocales(t)
	withDigestConfig(t, true, false)

	chatID := newDigestTestChat(t)
	weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	now := weekStart.Add(time.Hour)
	seedWeek(t, chatID, weekStart.AddDate(0, 0, -7), Stats{Messages: 3})
	enableDigest(t, chatID, digestsettings.DestinationGroup)
	if err := lang.ChangeGroupLanguage(chatID, "es"); err != nil {
		t.Fatalf("ChangeGroupLanguage() error = %v", err)
	}

	client := &digestBotClient{}
	scheduler := NewScheduler(newDigestTestBot(client))
	scheduler.now = func() time.Time { return now }
	scheduler.runDue(now)

	sends := client.sends()
	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(sends))
	}
	es := i18n.MustNewTranslator("es")
	title, _ := es.GetString("digest_message_title")
	if !strings.Contains(fmt.Sprint(sends[0].Params["text"]), title) {
		t.Fatalf("digest %q is not in the chat's language", sends[0].Params["text"])
	}
}

// The dashboard button is part of the digest only while the panel is on, and
// it points at the panel's entry URL for the chat.
func TestSchedulerAddsTheDashboardButtonOnlyWithThePanel(t *testing.T) {
	for _, panelEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("panel=%v", panelEnabled), func(t *testing.T) {
			useDigestLocales(t)
			withDigestConfig(t, true, panelEnabled)

			chatID := newDigestTestChat(t)
			weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
			now := weekStart.Add(time.Hour)
			seedWeek(t, chatID, weekStart.AddDate(0, 0, -7), Stats{Messages: 2})
			enableDigest(t, chatID, digestsettings.DestinationGroup)

			client := &digestBotClient{}
			bot := newDigestTestBot(client)
			scheduler := NewScheduler(bot)
			scheduler.now = func() time.Time { return now }
			scheduler.runDue(now)

			sends := client.sends()
			if len(sends) != 1 {
				t.Fatalf("sendMessage calls = %d, want 1", len(sends))
			}
			keyboard, hasKeyboard := sends[0].Params["reply_markup"].(*gotgbot.InlineKeyboardMarkup)
			if keyboard == nil {
				hasKeyboard = false
			}
			if !panelEnabled {
				if hasKeyboard {
					t.Fatalf("keyboard = %+v with the panel off, want none", keyboard)
				}
				return
			}
			if !hasKeyboard || keyboard == nil {
				t.Fatal("no keyboard with the panel on, want the dashboard button")
			}
			button := keyboard.InlineKeyboard[0][0]
			if want := panel.EntryURL(bot.Username, "", chatID); button.Url != want {
				t.Fatalf("button URL = %q, want %q", button.Url, want)
			}
		})
	}
}
