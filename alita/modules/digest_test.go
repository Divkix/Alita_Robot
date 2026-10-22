//go:build testtools

package modules

import (
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/db/digestsettings"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

func withDigestEnabled(t *testing.T, enabled bool) {
	t.Helper()
	previous := config.AppConfig.EnableDigest
	config.AppConfig.EnableDigest = enabled
	t.Cleanup(func() { config.AppConfig.EnableDigest = previous })
}

// cleanupDigestChat removes the per-chat rows the command writes.
func cleanupDigestChat(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.DigestSettings{}).Error; err != nil {
			t.Errorf("cleanup digest settings: %v", err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.LogChannel{}).Error; err != nil {
			t.Errorf("cleanup log channel: %v", err)
		}
	})
}

func digestSettingsFor(t *testing.T, chatID int64) *models.DigestSettings {
	t.Helper()
	settings, err := digestsettings.Get(chatID)
	if err != nil {
		t.Fatalf("digestsettings.Get(%d) error = %v", chatID, err)
	}
	return settings
}

// seedDailyStats writes counter rows and removes them after the test.
func seedDailyStats(t *testing.T, rows ...models.ChatDailyStat) {
	t.Helper()
	if err := chatstats.AddDaily(rows); err != nil {
		t.Fatalf("AddDaily() error = %v", err)
	}
	if len(rows) == 0 {
		return
	}
	chatID := rows[0].ChatID
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatDailyStat{}).Error; err != nil {
			t.Errorf("cleanup daily stats: %v", err)
		}
	})
}

func lastMessageTo(t *testing.T, client *moduleBotClient, chatID int64) string {
	t.Helper()
	texts := spamDBMessagesTo(client, chatID)
	if len(texts) == 0 {
		t.Fatalf("no message was sent to chat %d", chatID)
	}
	return texts[len(texts)-1]
}

// digestStatsFor flushes the in-process counters and returns the chat's
// totals for the last two UTC days, which covers a test running at midnight.
func digestStatsFor(t *testing.T, chatID int64) models.ChatDailyStat {
	t.Helper()
	if err := stats.Flush(); err != nil {
		t.Fatalf("stats.Flush() error = %v", err)
	}
	now := time.Now().UTC()
	rows, err := chatstats.GetRange(chatID, stats.Day(now.AddDate(0, 0, -1)), stats.Day(now))
	if err != nil {
		t.Fatalf("GetRange() error = %v", err)
	}
	var total models.ChatDailyStat
	for _, row := range rows {
		total.Messages += row.Messages
		total.Joins += row.Joins
		total.Leaves += row.Leaves
	}
	return total
}

func dispatchDigestUpdate(t *testing.T, dispatcher *ext.Dispatcher, bot *gotgbot.Bot, update *gotgbot.Update) {
	t.Helper()
	if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%d) error = %v", update.UpdateId, err)
	}
}

func TestDigestNowRendersTheWeekThroughTheDigestBuilder(t *testing.T) {
	useRealLocales(t)
	withDigestEnabled(t, true)

	previous := digestNow
	digestNow = func() time.Time {
		return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) // Wednesday
	}
	t.Cleanup(func() { digestNow = previous })

	f := newAdminChatFixture(t)
	cleanupDigestChat(t, f.chat.Id)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadDigest(dispatcher)

	seedDailyStats(t,
		models.ChatDailyStat{ChatID: f.chat.Id, Day: "2026-10-19", Messages: 3, Joins: 1, Leaves: 3},
		models.ChatDailyStat{ChatID: f.chat.Id, Day: "2026-10-21", Messages: 5, Joins: 2, Leaves: 1},
		// Sunday, before the week started: it must not reach the week total.
		models.ChatDailyStat{ChatID: f.chat.Id, Day: "2026-10-18", Messages: 100, Joins: 50, Leaves: 50},
	)

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 901, "/digest now")
	reply := lastMessageTo(t, f.client, f.chat.Id)

	tr := i18n.MustNewTranslator("en")
	newChange, err := tr.GetString("digest_change_new")
	if err != nil {
		t.Fatalf("GetString(digest_change_new) error = %v", err)
	}
	// The Sunday row is the previous week, so each line compares against it.
	down := func(percent int64) string {
		s, _ := tr.GetString("digest_change_down", i18n.TranslationParams{"percent": percent})
		return s
	}
	none, _ := tr.GetString("digest_change_none")
	for _, want := range []struct {
		key    string
		count  int64
		change string
	}{
		{"digest_message_messages", 8, down(92)},
		{"digest_message_joins", 3, down(94)},
		{"digest_message_leaves", 4, down(92)},
		{"digest_message_moderation", 0, none},
	} {
		line, err := tr.GetString(want.key, i18n.TranslationParams{"count": want.count, "change": want.change})
		if err != nil {
			t.Fatalf("GetString(%q) error = %v", want.key, err)
		}
		if !strings.Contains(reply, line) {
			t.Errorf("reply %q does not contain %q", reply, line)
		}
	}
	outside, err := tr.GetString("digest_message_messages", i18n.TranslationParams{"count": int64(108), "change": newChange})
	if err != nil {
		t.Fatalf("GetString(digest_message_messages) error = %v", err)
	}
	if strings.Contains(reply, outside) {
		t.Errorf("reply %q includes the previous Sunday's counters", reply)
	}
	if !strings.Contains(reply, "2026-10-19") {
		t.Errorf("reply %q does not name the week start", reply)
	}
}

func TestDigestNowRefusesNonAdmin(t *testing.T) {
	useRealLocales(t)
	withDigestEnabled(t, true)

	f := newAdminChatFixture(t)
	cleanupDigestChat(t, f.chat.Id)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadDigest(dispatcher)

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.member, 902, "/digest now")

	tr := i18n.MustNewTranslator("en")
	title, err := tr.GetString("digest_message_title")
	if err != nil {
		t.Fatalf("GetString(digest_message_title) error = %v", err)
	}
	for _, text := range spamDBMessagesTo(f.client, f.chat.Id) {
		if strings.Contains(text, title) {
			t.Fatalf("a non-admin received the preview: %q", text)
		}
	}
}

func TestDigestNowRefusesWhenGloballyDisabled(t *testing.T) {
	useRealLocales(t)
	withDigestEnabled(t, false)

	f := newAdminChatFixture(t)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadDigest(dispatcher)

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 903, "/digest now")
	reply := lastMessageTo(t, f.client, f.chat.Id)

	tr := i18n.MustNewTranslator("en")
	want, err := tr.GetString("digest_globally_disabled")
	if err != nil {
		t.Fatalf("GetString(digest_globally_disabled) error = %v", err)
	}
	if reply != want {
		t.Fatalf("reply = %q, want %q", reply, want)
	}
}

func TestDigestBareShowsTheChatsState(t *testing.T) {
	useRealLocales(t)
	withDigestEnabled(t, true)

	f := newAdminChatFixture(t)
	cleanupDigestChat(t, f.chat.Id)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadDigest(dispatcher)

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 904, "/digest")
	reply := lastMessageTo(t, f.client, f.chat.Id)

	tr := i18n.MustNewTranslator("en")
	disabled, _ := tr.GetString("digest_status_disabled")
	groupDestination, _ := tr.GetString("digest_destination_group")
	defaultLine, _ := tr.GetString("digest_status_destination_default", i18n.TranslationParams{"destination": groupDestination})
	if !strings.Contains(reply, disabled) {
		t.Errorf("reply %q does not show the disabled state", reply)
	}
	if !strings.Contains(reply, defaultLine) {
		t.Errorf("reply %q does not show the default destination %q", reply, defaultLine)
	}
	// The chat has no counters, so a preview would be the empty-week message.
	empty, _ := tr.GetString("digest_message_empty")
	if strings.Contains(reply, empty) {
		t.Errorf("reply %q renders the preview for a bare call", reply)
	}
}

func TestDigestTogglesPersistAndShowTheDestination(t *testing.T) {
	useRealLocales(t)
	withDigestEnabled(t, true)

	f := newAdminChatFixture(t)
	cleanupDigestChat(t, f.chat.Id)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadDigest(dispatcher)
	tr := i18n.MustNewTranslator("en")

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 910, "/digest on")
	enabled, _ := tr.GetString("digest_status_enabled")
	if reply := lastMessageTo(t, f.client, f.chat.Id); !strings.Contains(reply, enabled) {
		t.Fatalf("reply %q does not confirm the digest is on", reply)
	}
	if settings := digestSettingsFor(t, f.chat.Id); !settings.Enabled {
		t.Fatal("the switch was not persisted")
	}

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 911, "/digest off")
	disabled, _ := tr.GetString("digest_status_disabled")
	if reply := lastMessageTo(t, f.client, f.chat.Id); !strings.Contains(reply, disabled) {
		t.Fatalf("reply %q does not confirm the digest is off", reply)
	}
	if settings := digestSettingsFor(t, f.chat.Id); settings.Enabled {
		t.Fatal("the switch off was not persisted")
	}

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 912, "/digest here")
	groupDestination, _ := tr.GetString("digest_destination_group")
	explicitLine, _ := tr.GetString("digest_status_destination_set", i18n.TranslationParams{"destination": groupDestination})
	if reply := lastMessageTo(t, f.client, f.chat.Id); !strings.Contains(reply, explicitLine) {
		t.Fatalf("reply %q does not show the group as the chosen destination", reply)
	}
	if settings := digestSettingsFor(t, f.chat.Id); settings.Destination != digestsettings.DestinationGroup {
		t.Fatalf("destination = %q, want group", settings.Destination)
	}

	// Without a log channel the command refuses instead of saving a
	// destination the scheduler would have to fall back from.
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 913, "/digest log")
	unavailable, _ := tr.GetString("digest_log_unavailable")
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != unavailable {
		t.Fatalf("reply = %q, want %q", reply, unavailable)
	}
	if settings := digestSettingsFor(t, f.chat.Id); settings.Destination != digestsettings.DestinationGroup {
		t.Fatalf("destination = %q after a refused log choice, want group", settings.Destination)
	}

	// With a log channel the choice saves and the status names it.
	if err := logchannels.Set(f.chat.Id, f.chat.Title, f.chat.Id+1); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 914, "/digest log")
	logDestination, _ := tr.GetString("digest_destination_log")
	explicitLogLine, _ := tr.GetString("digest_status_destination_set", i18n.TranslationParams{"destination": logDestination})
	if reply := lastMessageTo(t, f.client, f.chat.Id); !strings.Contains(reply, explicitLogLine) {
		t.Fatalf("reply %q does not show the log channel as the chosen destination", reply)
	}
	if settings := digestSettingsFor(t, f.chat.Id); settings.Destination != digestsettings.DestinationLog {
		t.Fatalf("destination = %q, want log", settings.Destination)
	}
}

func TestDigestRefusesNonAdminChanges(t *testing.T) {
	useRealLocales(t)
	withDigestEnabled(t, true)

	f := newAdminChatFixture(t)
	cleanupDigestChat(t, f.chat.Id)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadDigest(dispatcher)

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.member, 920, "/digest on")

	tr := i18n.MustNewTranslator("en")
	title, _ := tr.GetString("digest_status_title")
	for _, text := range spamDBMessagesTo(f.client, f.chat.Id) {
		if strings.Contains(text, title) {
			t.Fatalf("a non-admin received the digest state: %q", text)
		}
	}
	if settings := digestSettingsFor(t, f.chat.Id); settings.Enabled {
		t.Fatal("a non-admin turned the digest on")
	}
}

func TestUsersWatcherCountsNewMessagesOnlyOnce(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Counting Chat"}
	member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Member"}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadUsers(dispatcher)
	t.Cleanup(DrainUsersAsyncWrites)

	aispamProcessUpdate(t, dispatcher, bot, chat, member, 905, "hello")
	dispatchDigestUpdate(t, dispatcher, bot, &gotgbot.Update{
		UpdateId: 906,
		EditedMessage: &gotgbot.Message{
			MessageId: 905, Date: 1, Chat: chat, From: &member, Text: "hello, edited",
		},
	})
	// Telegram sends an anonymous admin's message with its fake bot user in
	// from and the group in sender_chat; a channel sender has only sender_chat.
	dispatchDigestUpdate(t, dispatcher, bot, &gotgbot.Update{
		UpdateId: 907,
		Message: &gotgbot.Message{
			MessageId: 907, Date: 1, Chat: chat, Text: "from an anonymous admin",
			From:       &gotgbot.User{Id: 1087968824, FirstName: "Group", IsBot: true},
			SenderChat: &chat,
		},
	})
	dispatchDigestUpdate(t, dispatcher, bot, &gotgbot.Update{
		UpdateId: 908,
		Message: &gotgbot.Message{
			MessageId: 908, Date: 1, Chat: chat, Text: "from a channel",
			SenderChat: &gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel", Title: "Linked"},
		},
	})

	got := digestStatsFor(t, chat.Id)
	if got.Messages != 3 {
		t.Fatalf("messages = %d, want 3: an edit must not count again, every sender counts once", got.Messages)
	}
}

func TestGreetingsCountOneJoinAcrossBothJoinUpdates(t *testing.T) {
	for _, order := range []string{"chat member first", "service message first"} {
		t.Run(order, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Chat"}
			member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Joiner"}
			adder := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Adder"}
			t.Cleanup(func() { clearRecentJoinProcessing(chat.Id, member.Id) })

			dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			LoadGreetings(dispatcher)

			memberUpdate := &gotgbot.Update{UpdateId: 1, ChatMember: &gotgbot.ChatMemberUpdated{
				Chat:          chat,
				From:          member,
				Date:          1,
				OldChatMember: gotgbot.ChatMemberLeft{User: member},
				NewChatMember: gotgbot.ChatMemberMember{User: member},
			}}
			serviceUpdate := &gotgbot.Update{UpdateId: 2, Message: &gotgbot.Message{
				MessageId: 2, Date: 1, Chat: chat, From: &adder,
				NewChatMembers: []gotgbot.User{member},
			}}

			if order == "chat member first" {
				dispatchDigestUpdate(t, dispatcher, bot, memberUpdate)
				dispatchDigestUpdate(t, dispatcher, bot, serviceUpdate)
			} else {
				dispatchDigestUpdate(t, dispatcher, bot, serviceUpdate)
				dispatchDigestUpdate(t, dispatcher, bot, memberUpdate)
			}

			got := digestStatsFor(t, chat.Id)
			if got.Joins != 1 {
				t.Fatalf("joins = %d, want 1 for one membership change seen as two updates", got.Joins)
			}
		})
	}
}

func TestGreetingsCountOneLeaveAcrossBothLeaveUpdates(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Leave Chat"}
	member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Leaver"}
	actor := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Actor"}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadGreetings(dispatcher)

	dispatchDigestUpdate(t, dispatcher, bot, &gotgbot.Update{UpdateId: 1, ChatMember: &gotgbot.ChatMemberUpdated{
		Chat:          chat,
		From:          actor,
		Date:          1,
		OldChatMember: gotgbot.ChatMemberMember{User: member},
		NewChatMember: gotgbot.ChatMemberLeft{User: member},
	}})
	dispatchDigestUpdate(t, dispatcher, bot, &gotgbot.Update{UpdateId: 2, Message: &gotgbot.Message{
		MessageId: 2, Date: 1, Chat: chat, From: &actor,
		LeftChatMember: &member,
	}})

	got := digestStatsFor(t, chat.Id)
	if got.Leaves != 1 {
		t.Fatalf("leaves = %d, want 1 for one membership change seen as two updates", got.Leaves)
	}
}

func TestDigestRetentionSweepsRowsOlderThan400Days(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	chatID := uniqueModuleChatID()
	old := stats.Day(now.AddDate(0, 0, -(digestStatsRetentionDays + 1)))
	kept := stats.Day(now.AddDate(0, 0, -(digestStatsRetentionDays - 1)))
	today := stats.Day(now)
	seedDailyStats(t,
		models.ChatDailyStat{ChatID: chatID, Day: old, Messages: 1},
		models.ChatDailyStat{ChatID: chatID, Day: kept, Messages: 1},
		models.ChatDailyStat{ChatID: chatID, Day: today, Messages: 1},
	)

	sweepExpiredDailyStats(now)

	rows, err := chatstats.GetRange(chatID, old, today)
	if err != nil {
		t.Fatalf("GetRange() error = %v", err)
	}
	if len(rows) != 2 || rows[0].Day != kept || rows[1].Day != today {
		t.Fatalf("rows = %v, want the %s and %s rows only", rows, kept, today)
	}
}
