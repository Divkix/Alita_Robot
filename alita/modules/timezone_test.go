//go:build testtools

package modules

import (
	"html"
	"strings"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// pinTimezoneNow fixes the clock the /timezone replies read, in January so
// zones with DST are on standard time.
func pinTimezoneNow(t *testing.T) {
	t.Helper()
	previous := timezoneNow
	timezoneNow = func() time.Time { return time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { timezoneNow = previous })
}

// timezoneOf is the timezone chatID is on, read as features read it.
func timezoneOf(t *testing.T, chatID int64) string {
	t.Helper()
	name, err := chattz.Name(chatID)
	if err != nil {
		t.Fatalf("chattz.Name(%d) error = %v", chatID, err)
	}
	return name
}

func cleanupChatTimezone(t *testing.T, chatIDs ...int64) {
	t.Helper()
	t.Cleanup(func() {
		for _, chatID := range chatIDs {
			if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatTimezone{}).Error; err != nil {
				t.Errorf("cleanup timezone of %d: %v", chatID, err)
			}
		}
	})
}

func timezoneFixture(t *testing.T) adminChatFixture {
	t.Helper()
	useRealLocales(t)
	pinTimezoneNow(t)
	f := newAdminChatFixture(t)
	cleanupChatTimezone(t, f.chat.Id)
	return f
}

func TestTimezoneSetStoresTheCanonicalNameAndShowsTheLocalTime(t *testing.T) {
	f := timezoneFixture(t)
	dispatcher := adminPMDispatcher()

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1011, "/timezone asia/kolkata")
	if got := timezoneOf(t, f.chat.Id); got != "Asia/Kolkata" {
		t.Fatalf("stored timezone = %q, want Asia/Kolkata", got)
	}
	want := translatedWith(t, "timezone_set", i18n.TranslationParams{"timezone": "Asia/Kolkata", "offset": "+05:30", "time": "17:30"})
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != want {
		t.Fatalf("reply = %q, want %q", reply, want)
	}

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1012, "/timezone")
	want = translatedWith(t, "timezone_status", i18n.TranslationParams{"timezone": "Asia/Kolkata", "offset": "+05:30", "time": "17:30"})
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != want {
		t.Fatalf("status after set = %q, want %q", reply, want)
	}

	// A zone west of UTC renders a negative offset; a second set overwrites.
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1013, "/timezone America/New_York")
	if got := timezoneOf(t, f.chat.Id); got != "America/New_York" {
		t.Fatalf("stored timezone = %q, want America/New_York", got)
	}
	want = translatedWith(t, "timezone_set", i18n.TranslationParams{"timezone": "America/New_York", "offset": "-05:00", "time": "07:00"})
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != want {
		t.Fatalf("reply = %q, want %q", reply, want)
	}
	if loc := chattz.Location(f.chat.Id); loc.String() != "America/New_York" {
		t.Fatalf("chattz.Location() = %v, want America/New_York", loc)
	}
}

func TestTimezoneRefusesAnUnknownNameWithItsHelp(t *testing.T) {
	f := timezoneFixture(t)
	dispatcher := adminPMDispatcher()

	for i, input := range []string{"Mars/Olympus", "<b>x</b>", "+05:30"} {
		aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, int64(1021+i), "/timezone "+input)
		reply := lastMessageTo(t, f.client, f.chat.Id)
		refusal := translatedWith(t, "timezone_invalid", i18n.TranslationParams{"input": html.EscapeString(input)})
		if !strings.Contains(reply, refusal) {
			t.Fatalf("/timezone %s: reply %q does not carry %q", input, reply, refusal)
		}
		if example := commandExample("timezone"); !strings.Contains(reply, example) {
			t.Fatalf("/timezone %s: reply %q does not show the example %q", input, reply, example)
		}
	}
	if got := timezoneOf(t, f.chat.Id); got != chattz.DefaultName {
		t.Fatalf("stored timezone = %q after refused names, want %q", got, chattz.DefaultName)
	}
	if n := db.CountRows(&models.ChatTimezone{}, "chat_id = ?", f.chat.Id); n != 0 {
		t.Fatalf("refused names wrote %d rows, want 0", n)
	}
}

// From PM while connected, a bare /timezone and a set act on the connected
// group and answer in PM, naming the group.
func TestTimezoneInPMActsOnTheConnectedGroup(t *testing.T) {
	f := timezoneFixture(t)
	f.connectAdmin(t)
	dispatcher := adminPMDispatcher()

	processPM(t, dispatcher, f, f.admin.Id, "/timezone Europe/Berlin")

	if got := timezoneOf(t, f.chat.Id); got != "Europe/Berlin" {
		t.Fatalf("connected group's timezone = %q, want Europe/Berlin", got)
	}
	if got := timezoneOf(t, f.admin.Id); got != chattz.DefaultName {
		t.Fatalf("the PM got timezone %q, want it untouched", got)
	}
	set := translatedWith(t, "timezone_set", i18n.TranslationParams{"timezone": "Europe/Berlin", "offset": "+01:00", "time": "13:00"})
	if !repliedIn(f, f.admin.Id, set) || calledFor(f, "sendMessage", f.chat.Id) {
		t.Fatalf("/timezone from PM did not answer in PM only; sent:\n%s", sentText(f.client))
	}
	if !repliedIn(f, f.admin.Id, f.chat.Title) {
		t.Fatalf("the PM reply does not name the group; sent:\n%s", sentText(f.client))
	}
}
