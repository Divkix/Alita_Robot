//go:build testtools

package modules

import (
	"encoding/json"
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
