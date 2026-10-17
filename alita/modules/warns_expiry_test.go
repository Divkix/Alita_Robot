//go:build testtools

package modules

import (
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

// seedDatedWarns stores one warn per age for userID, oldest first.
func seedDatedWarns(t *testing.T, chatID, userID int64, reasons []string, ages []time.Duration) {
	t.Helper()
	entries := make([]models.WarnEntry, len(reasons))
	for i, reason := range reasons {
		at := time.Now().UTC().Add(-ages[i])
		entries[i] = models.WarnEntry{UserId: userID, ChatId: chatID, Reason: reason, WarnedBy: 777000, WarnedAt: &at}
	}
	if err := db.DB.Create(&models.Warns{
		UserId: userID, ChatId: chatID, NumWarns: len(reasons), Reasons: models.StringArray(reasons),
	}).Error; err != nil {
		t.Fatalf("create warns row: %v", err)
	}
	if err := db.DB.Create(&entries).Error; err != nil {
		t.Fatalf("create warn entries: %v", err)
	}
	dbcache.ResetLocalForTest()
}

func lastReply(t *testing.T, client *moduleBotClient) string {
	t.Helper()
	calls := client.callsFor("sendMessage")
	if len(calls) == 0 {
		t.Fatal("sendMessage calls = 0, want a reply")
	}
	return calls[len(calls)-1].Params["text"].(string)
}

func TestSetWarnTimeStoresExpiry(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	run := func(text string) {
		t.Helper()
		if err := warnsModule.setWarnTime(bot, newModuleMessageContext(bot, chat, admin, text)); err != ext.EndGroups {
			t.Fatalf("%s error = %v, want EndGroups", text, err)
		}
	}
	assertExpiry := func(text string, want int) {
		t.Helper()
		if got := warns.GetWarnSetting(chat.Id).WarnExpiry; got != want {
			t.Fatalf("after %q: expiry = %d, want %d", text, got, want)
		}
	}

	run("/setwarntime 30d")
	assertExpiry("/setwarntime 30d", 30*86400)
	if text := lastReply(t, client); !strings.Contains(text, "30d") {
		t.Fatalf("reply = %q, want the new expiry", text)
	}
	run("/setwarntime 2w")
	assertExpiry("/setwarntime 2w", 14*86400)

	for _, text := range []string{"/setwarntime 5x", "/setwarntime 0h", "/setwarntime 400d", "/setwarntime 30d extra"} {
		before := len(client.callsFor("sendMessage"))
		run(text)
		assertExpiry(text, 14*86400)
		if got := len(client.callsFor("sendMessage")); got != before+1 {
			t.Fatalf("%q sent %d replies, want one error reply", text, got-before)
		}
	}

	run("/setwarntime")
	assertExpiry("/setwarntime", 14*86400)
	if text := lastReply(t, client); !strings.Contains(text, "2w") {
		t.Fatalf("reply = %q, want the current expiry", text)
	}

	run("/setwarntime off")
	assertExpiry("/setwarntime off", 0)
	if text := lastReply(t, client); text != translated(t, "warns_expiry_off") {
		t.Fatalf("reply = %q, want the expiry-off message", text)
	}
}

func TestSetWarnTimeRequiresAdmin(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	if err := warnsModule.setWarnTime(bot, newModuleMessageContext(bot, chat, member, "/setwarntime 1d")); err != ext.EndGroups {
		t.Fatalf("setWarnTime() error = %v, want EndGroups", err)
	}
	if got := warns.GetWarnSetting(chat.Id).WarnExpiry; got != 0 {
		t.Fatalf("expiry set by a member = %d, want 0", got)
	}
}

func TestWarnLimitIgnoresExpiredWarns(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.DB.Create(&models.WarnSettings{ChatId: chat.Id, WarnLimit: 3, WarnMode: "ban", WarnExpiry: 30 * 86400}).Error; err != nil {
		t.Fatalf("create warn settings: %v", err)
	}
	seedDatedWarns(t, chat.Id, target.Id, []string{"old one", "old two"}, []time.Duration{50 * 24 * time.Hour, 40 * 24 * time.Hour})

	if err := warnsModule.warnUser(bot, newWarnReplyContext(bot, chat, admin, target, "/warn fresh")); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none: the two old warns expired", len(calls))
	}
	if text := lastReply(t, client); !strings.Contains(text, "1/3") {
		t.Fatalf("warn reply = %q, want 1/3", text)
	}
}

func TestWarnsListSkipsExpiredWarns(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	if err := warns.SetWarnExpiry(chat.Id, 30*86400); err != nil {
		t.Fatalf("SetWarnExpiry() error = %v", err)
	}
	seedDatedWarns(t, chat.Id, 42, []string{"old spam", "recent flood"}, []time.Duration{40 * 24 * time.Hour, time.Hour})

	text := warnsListText(t, client, bot, chat, 42)
	if strings.Contains(text, "old spam") || !strings.Contains(text, "recent flood") || !strings.Contains(text, "1/3") {
		t.Fatalf("/warns text = %q, want only the recent warn, counted 1/3", text)
	}
}

func TestWarningsShowsExpiry(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	show := func() string {
		t.Helper()
		if err := warnsModule.warnings(bot, newModuleMessageContext(bot, chat, admin, "/warnings")); err != ext.EndGroups {
			t.Fatalf("warnings() error = %v, want EndGroups", err)
		}
		return lastReply(t, client)
	}

	if text, off := show(), translated(t, "warns_expiry_never"); !strings.Contains(text, off) {
		t.Fatalf("/warnings text = %q, want expiry %q", text, off)
	}
	if err := warns.SetWarnExpiry(chat.Id, 30*86400); err != nil {
		t.Fatalf("SetWarnExpiry() error = %v", err)
	}
	if text := show(); !strings.Contains(text, "30d") {
		t.Fatalf("/warnings text = %q, want expiry 30d", text)
	}
}
