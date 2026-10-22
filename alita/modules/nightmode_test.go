//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/nightmode"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// pinNightModeNow fixes the clock night mode reads at the given UTC time.
func pinNightModeNow(t *testing.T, hour, minute int) {
	t.Helper()
	previous := nightModeNow
	nightModeNow = func() time.Time { return time.Date(2026, 1, 15, hour, minute, 0, 0, time.UTC) }
	t.Cleanup(func() { nightModeNow = previous })
}

func cleanupNightMode(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.NightModeSettings{}).Error; err != nil {
			t.Errorf("cleanup night mode of %d: %v", chatID, err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatTimezone{}).Error; err != nil {
			t.Errorf("cleanup timezone of %d: %v", chatID, err)
		}
	})
}

func enableNightMode(t *testing.T, chatID int64, start, end string, types ...string) {
	t.Helper()
	parse := func(s string) int {
		v, err := time.Parse("15:04", s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return v.Hour()*60 + v.Minute()
	}
	if err := nightmode.Enable(chatID, parse(start), parse(end), types); err != nil {
		t.Fatalf("nightmode.Enable() error = %v", err)
	}
}

// nightLockChat is a supergroup whose bot can delete, with chatAdmin as its
// one human admin.
func nightLockChat(t *testing.T) (*moduleBotClient, *gotgbot.Bot, gotgbot.Chat) {
	t.Helper()
	client := newModuleBotClient()
	client.responses["getChatAdministrators"] = json.RawMessage(
		`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true,"can_restrict_members":true},` +
			`{"status":"administrator","user":{"id":4343,"is_bot":false,"first_name":"Chat Admin"},"can_delete_messages":true}]`,
	)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Night Chat"}
	cleanupNightMode(t, chat.Id)
	return client, newModuleTestBot(client), chat
}

// sendLockProbes runs a URL message through permHandler (group 5, the url
// lock) and a photo through restHandler (group 6, the media and all locks),
// and returns how many were deleted.
func sendLockProbes(t *testing.T, client *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat, from gotgbot.User) int {
	t.Helper()
	before := len(client.callsFor("deleteMessage"))

	urlCtx := newModuleMessageContext(bot, chat, from, "https://example.com")
	urlCtx.EffectiveMessage.Entities = []gotgbot.MessageEntity{{Type: "url", Offset: 0, Length: 19}}
	if err := locksModule.permHandler(bot, urlCtx); err != ext.ContinueGroups {
		t.Fatalf("permHandler error = %v, want ContinueGroups", err)
	}
	mediaCtx := newModuleMessageContext(bot, chat, from, "photo")
	mediaCtx.EffectiveMessage.Photo = []gotgbot.PhotoSize{{FileId: "photo-1"}}
	if err := locksModule.restHandler(bot, mediaCtx); err != ext.ContinueGroups {
		t.Fatalf("restHandler error = %v, want ContinueGroups", err)
	}
	return len(client.callsFor("deleteMessage")) - before
}

var nightMember = gotgbot.User{Id: 42, FirstName: "Member"}

func TestNightLocksDeleteOnlyInsideTheWindow(t *testing.T) {
	client, bot, chat := nightLockChat(t)
	enableNightMode(t, chat.Id, "23:00", "07:00", "media", "url")

	for _, tt := range []struct {
		hour, minute int
		want         int
	}{
		{22, 59, 0},
		{23, 0, 2},
		{3, 0, 2},
		{6, 59, 2},
		{7, 0, 0},
		{12, 0, 0},
	} {
		pinNightModeNow(t, tt.hour, tt.minute)
		if got := sendLockProbes(t, client, bot, chat, nightMember); got != tt.want {
			t.Errorf("at %02d:%02d UTC deleted %d messages, want %d", tt.hour, tt.minute, got, tt.want)
		}
	}
}

// The window is in the chat's timezone: 23:00-07:00 in Asia/Kolkata (UTC+5:30)
// is 17:30-01:30 UTC.
func TestNightLocksFollowTheChatTimezone(t *testing.T) {
	client, bot, chat := nightLockChat(t)
	if _, err := chattz.Set(chat.Id, "Asia/Kolkata"); err != nil {
		t.Fatalf("chattz.Set() error = %v", err)
	}
	enableNightMode(t, chat.Id, "23:00", "07:00", "media", "url")

	pinNightModeNow(t, 18, 0) // 23:30 in Kolkata
	if got := sendLockProbes(t, client, bot, chat, nightMember); got != 2 {
		t.Fatalf("at 23:30 Kolkata time deleted %d messages, want 2", got)
	}
	pinNightModeNow(t, 2, 0) // 07:30 in Kolkata, but inside the window in UTC
	if got := sendLockProbes(t, client, bot, chat, nightMember); got != 0 {
		t.Fatalf("at 07:30 Kolkata time deleted %d messages, want 0", got)
	}
}

func TestNightLocksExemptAdminsAndApprovedUsers(t *testing.T) {
	client, bot, chat := nightLockChat(t)
	enableNightMode(t, chat.Id, "23:00", "07:00", "media", "url")
	pinNightModeNow(t, 1, 0)

	approved := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Trusted"}
	if err := approvals.AddApprovedUser(chat.Id, approved.Id, 4343, "trusted"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	chatAdmin := gotgbot.User{Id: 4343, FirstName: "Chat Admin"}

	for name, user := range map[string]gotgbot.User{"admin": chatAdmin, "approved user": approved} {
		if got := sendLockProbes(t, client, bot, chat, user); got != 0 {
			t.Errorf("night locks deleted %d messages of the %s, want 0", got, name)
		}
	}
	if got := sendLockProbes(t, client, bot, chat, nightMember); got != 2 {
		t.Fatalf("night locks deleted %d messages of a member, want 2", got)
	}
}

// A chat whose night mode is off keeps exactly its day locks at night, and a
// night merge never leaks into the stored locks.
func TestNightLocksLeaveDayLocksAsTheyAre(t *testing.T) {
	client, bot, chat := nightLockChat(t)
	if err := locks.UpdateLock(chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock() error = %v", err)
	}
	pinNightModeNow(t, 1, 0)

	if got := sendLockProbes(t, client, bot, chat, nightMember); got != 1 {
		t.Fatalf("without night mode deleted %d messages, want 1 (the url day lock)", got)
	}

	enableNightMode(t, chat.Id, "23:00", "07:00", "media")
	if got := sendLockProbes(t, client, bot, chat, nightMember); got != 2 {
		t.Fatalf("with night mode deleted %d messages, want 2", got)
	}
	if stored := locks.GetChatLocks(chat.Id); stored["media"] {
		t.Fatalf("stored locks = %v: the night lock leaked into them", stored)
	}

	if err := nightmode.Disable(chat.Id); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if got := sendLockProbes(t, client, bot, chat, nightMember); got != 1 {
		t.Fatalf("after night mode off deleted %d messages, want 1 (the url day lock)", got)
	}
}

func nightModeDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(dispatcher)
	return dispatcher
}

func nightModeFixture(t *testing.T) adminChatFixture {
	t.Helper()
	useRealLocales(t)
	pinNightModeNow(t, 1, 0)
	f := newAdminChatFixture(t)
	cleanupNightMode(t, f.chat.Id)
	return f
}

func nightModeOf(t *testing.T, chatID int64) *models.NightModeSettings {
	t.Helper()
	settings, err := nightmode.Get(chatID)
	if err != nil {
		t.Fatalf("nightmode.Get(%d) error = %v", chatID, err)
	}
	return settings
}

// nightModeOnReply is the reply describing an enabled 23:00-07:00 night mode
// on media and url, in UTC at the pinned 01:00.
func nightModeOnReply(t *testing.T) string {
	t.Helper()
	return translatedWith(t, "nightmode_status_on", i18n.TranslationParams{
		"start": "23:00", "end": "07:00", "timezone": "UTC", "types": "media, url",
	}) + "\n" + translatedWith(t, "nightmode_now_active", i18n.TranslationParams{"time": "01:00"})
}

func TestNightModeBareShowsOffForAChatThatNeverSetIt(t *testing.T) {
	f := nightModeFixture(t)

	aispamProcessUpdate(t, nightModeDispatcher(), f.bot, f.chat, f.admin, 2001, "/nightmode")

	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != translated(t, "nightmode_status_off") {
		t.Fatalf("reply = %q, want the off status", reply)
	}
	if n := db.CountRows(&models.NightModeSettings{}, "chat_id = ?", f.chat.Id); n != 0 {
		t.Fatalf("showing night mode wrote %d rows, want 0", n)
	}
}

func TestNightModeSetShowAndTurnOff(t *testing.T) {
	f := nightModeFixture(t)
	dispatcher := nightModeDispatcher()

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 2011, "/nightmode 23:00 7:00 URL, media,url")
	settings := nightModeOf(t, f.chat.Id)
	if !settings.Enabled || settings.StartMinute != 23*60 || settings.EndMinute != 7*60 || settings.LockTypes != "media,url" {
		t.Fatalf("stored night mode = %+v, want on 23:00-07:00 with media,url", settings)
	}
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != nightModeOnReply(t) {
		t.Fatalf("reply to the set = %q, want %q", reply, nightModeOnReply(t))
	}
	if got := sendLockProbes(t, f.client, f.bot, f.chat, nightMember); got != 2 {
		t.Fatalf("at 01:00 the night locks deleted %d messages, want 2", got)
	}

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 2012, "/nightmode")
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != nightModeOnReply(t) {
		t.Fatalf("status reply = %q, want %q", reply, nightModeOnReply(t))
	}

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 2013, "/nightmode off")
	if nightModeOf(t, f.chat.Id).Enabled {
		t.Fatal("/nightmode off left night mode on")
	}
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != translated(t, "nightmode_disabled") {
		t.Fatalf("reply to off = %q, want the disabled reply", reply)
	}
	if got := sendLockProbes(t, f.client, f.bot, f.chat, nightMember); got != 0 {
		t.Fatalf("after /nightmode off deleted %d messages, want 0", got)
	}
}

func TestNightModeRefusesBadCallsWithItsHelp(t *testing.T) {
	f := nightModeFixture(t)
	dispatcher := nightModeDispatcher()

	for i, tt := range []struct {
		args, refusal string
	}{
		{"24:00 07:00 url", translatedWith(t, "nightmode_invalid_time", i18n.TranslationParams{"input": "24:00"})},
		{"23:00 7 url", translatedWith(t, "nightmode_invalid_time", i18n.TranslationParams{"input": "7"})},
		{"23:00 23:00 url", translated(t, "nightmode_same_times")},
		{"23:00 07:00 bots", translatedWith(t, "nightmode_invalid_type", i18n.TranslationParams{"type": "bots"})},
		{"23:00 07:00 url,<b>", translatedWith(t, "nightmode_invalid_type", i18n.TranslationParams{"type": "&lt;b&gt;"})},
		{"23:00 07:00", translated(t, "nightmode_no_types")},
		{"23:00", translated(t, "nightmode_no_types")},
		{"on", translatedWith(t, "nightmode_invalid_time", i18n.TranslationParams{"input": "on"})},
	} {
		aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, int64(2021+i), "/nightmode "+tt.args)
		reply := lastMessageTo(t, f.client, f.chat.Id)
		if !strings.Contains(reply, tt.refusal) {
			t.Errorf("/nightmode %s: reply %q does not carry %q", tt.args, reply, tt.refusal)
		}
		if example := commandExample("nightmode"); !strings.Contains(reply, example) {
			t.Errorf("/nightmode %s: reply %q does not show the example %q", tt.args, reply, example)
		}
	}
	if n := db.CountRows(&models.NightModeSettings{}, "chat_id = ?", f.chat.Id); n != 0 {
		t.Fatalf("refused calls wrote %d rows, want 0", n)
	}
}

func TestNightModeRefusesMembers(t *testing.T) {
	f := nightModeFixture(t)
	dispatcher := nightModeDispatcher()

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.member, 2031, "/nightmode 23:00 07:00 url")
	if n := db.CountRows(&models.NightModeSettings{}, "chat_id = ?", f.chat.Id); n != 0 {
		t.Fatalf("a member's /nightmode wrote %d rows, want 0", n)
	}

	enableNightMode(t, f.chat.Id, "23:00", "07:00", "media", "url")
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.member, 2032, "/nightmode off")
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.member, 2033, "/nightmode")
	if !nightModeOf(t, f.chat.Id).Enabled {
		t.Fatal("a member turned night mode off")
	}
	if repliedIn(f, f.chat.Id, translatedWith(t, "nightmode_now_active", i18n.TranslationParams{"time": "01:00"})) {
		t.Fatal("a member was shown the night mode setting")
	}
}

// From PM while connected, /nightmode acts on the connected group and answers
// in PM, naming the group.
func TestNightModeInPMActsOnTheConnectedGroup(t *testing.T) {
	f := nightModeFixture(t)
	f.connectAdmin(t)

	processPM(t, nightModeDispatcher(), f, f.admin.Id, "/nightmode 23:00 07:00 media url")

	if settings := nightModeOf(t, f.chat.Id); !settings.Enabled || settings.LockTypes != "media,url" {
		t.Fatalf("connected group's night mode = %+v, want on with media,url", settings)
	}
	if n := db.CountRows(&models.NightModeSettings{}, "chat_id = ?", f.admin.Id); n != 0 {
		t.Fatal("night mode was stored for the PM")
	}
	if !repliedIn(f, f.admin.Id, nightModeOnReply(t)) || calledFor(f, "sendMessage", f.chat.Id) {
		t.Fatalf("/nightmode from PM did not answer in PM only; sent:\n%s", sentText(f.client))
	}
	if !repliedIn(f, f.admin.Id, f.chat.Title) {
		t.Fatalf("the PM reply does not name the group; sent:\n%s", sentText(f.client))
	}
}

func TestSettingsSummaryShowsNightMode(t *testing.T) {
	chatID := newSettingsTestChat(t)
	cleanupNightMode(t, chatID)
	row := func() *settingsRow {
		t.Helper()
		for _, section := range collectChatSettings(context.Background(), chatID) {
			for i := range section.Rows {
				if section.Rows[i].Command == "/nightmode" {
					return &section.Rows[i]
				}
			}
		}
		t.Fatal("the summary has no /nightmode row")
		return nil
	}

	enableNightMode(t, chatID, "23:00", "07:00", "url", "media")
	if got := row(); got.Value != "23:00-07:00 media, url" || !got.Changed {
		t.Fatalf("/nightmode row = %+v, want 23:00-07:00 media, url marked changed", got)
	}
	if err := nightmode.Disable(chatID); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if got := row(); got.Value != "off" || got.Changed {
		t.Fatalf("/nightmode row after off = %+v, want off and unchanged", got)
	}
}
