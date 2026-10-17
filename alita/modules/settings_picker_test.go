//go:build testtools

package modules

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// pauseFixture turns the picker pause on for a fresh user who administers
// group A, so runs and tests never share a pause, and loads the real English
// locale so a missing or misspelt key shows as no text.
func pauseFixture(t *testing.T, cooldown time.Duration) (client *moduleBotClient, bot *gotgbot.Bot, groupA, groupB int64, user gotgbot.User) {
	t.Helper()
	client, bot, groupA, groupB = settingsFixture(t)
	user = gotgbot.User{Id: -uniqueModuleChatID(), FirstName: "Admin"}
	client.adminsByChat[fmt.Sprint(groupA)] = adminListJSON(user.Id)
	settingsPickerCooldown = cooldown
	t.Cleanup(func() {
		if err := connections.DisconnectId(user.Id); err != nil {
			t.Errorf("DisconnectId(%d) error = %v", user.Id, err)
		}
	})

	locale, err := os.ReadFile("../../locales/en.yml")
	if err != nil {
		t.Fatalf("read en.yml: %v", err)
	}
	restoreI18n, err := i18n.OverrideManagerForTest(string(locale))
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restoreI18n)
	return client, bot, groupA, groupB, user
}

var waitSeconds = regexp.MustCompile(`\b[1-9][0-9]* s\b`)

// assertWaitNotice checks the notice names the seconds left, not a placeholder.
func assertWaitNotice(t *testing.T, text string) {
	t.Helper()
	if text == "" || strings.Contains(text, "{seconds}") || !waitSeconds.MatchString(text) {
		t.Fatalf("wait notice = %q, want the seconds left filled in", text)
	}
}

func pressSettingsButton(t *testing.T, bot *gotgbot.Bot, user gotgbot.User, fields map[string]string) {
	t.Helper()
	ctx := newModuleCallbackContext(bot, privateChatOf(user), user, encodeCallbackData("settings", fields))
	if err := ConnectionsModule.settingsCallback(bot, ctx); err != ext.EndGroups {
		t.Fatalf("settingsCallback(%v) error = %v, want EndGroups", fields, err)
	}
}

// A second /settings inside the pause runs no group checks and gets a notice
// with the seconds left; after the pause the list is built again.
func TestSettingsPickerPausesRepeatedPresses(t *testing.T) {
	client, bot, groupA, groupB, user := pauseFixture(t, 300*time.Millisecond)

	if got := runSettingsInPM(t, bot, client, user); len(got) != 1 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("first picker chats = %v, want only %d", got, groupA)
	}
	cache.InvalidateAdminCache(groupA)
	cache.InvalidateAdminCache(groupB)
	resetCalls(client)

	ctx := newModuleMessageContext(bot, privateChatOf(user), user, "/settings")
	if err := ConnectionsModule.settings(bot, ctx); err != ext.EndGroups {
		t.Fatalf("settings error = %v, want EndGroups", err)
	}
	replies := client.callsFor("sendMessage")
	if len(replies) != 1 || len(markupOf(t, replies[0]).InlineKeyboard) != 0 {
		t.Fatalf("paused /settings replies = %+v, want one notice without a picker", replies)
	}
	assertWaitNotice(t, fmt.Sprint(replies[0].Params["text"]))
	if n := adminLookups(client); n != 0 {
		t.Fatalf("admin lookups during the pause = %d, want 0", n)
	}

	time.Sleep(350 * time.Millisecond)
	if got := runSettingsInPM(t, bot, client, user); len(got) != 1 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("picker chats after the pause = %v, want only %d", got, groupA)
	}
	if n := adminLookups(client); n == 0 {
		t.Fatal("the list after the pause was not built again")
	}
}

// The usual path /settings → pick a group → Another group, inside the pause,
// shows the list again without a notice and without checking groups again.
func TestSettingsPickerAnotherGroupAfterPickSkipsThePause(t *testing.T) {
	client, bot, groupA, groupB, user := pauseFixture(t, 5*time.Second)

	if got := runSettingsInPM(t, bot, client, user); len(got) != 1 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("first picker chats = %v, want only %d", got, groupA)
	}
	pressSettingsButton(t, bot, user, map[string]string{"a": "p", "c": fmt.Sprint(groupA)})
	if conn := connections.Connection(user.Id); !conn.Connected || conn.ChatId != groupA {
		t.Fatalf("connection = %+v, want connected to %d", conn, groupA)
	}
	cache.InvalidateAdminCache(groupA)
	cache.InvalidateAdminCache(groupB)
	resetCalls(client)

	pressSettingsButton(t, bot, user, map[string]string{"a": "l"})
	for _, answer := range client.callsFor("answerCallbackQuery") {
		if text := fmt.Sprint(answer.Params["text"]); answer.Params["text"] != nil && text != "" {
			t.Fatalf("Another group answered %q, want no notice", text)
		}
	}
	edits := client.callsFor("editMessageText")
	if len(edits) != 1 {
		t.Fatalf("editMessageText calls = %d, want the list once", len(edits))
	}
	markup := markupOf(t, edits[0])
	if got := pickedChats(t, markup); len(got) != 1 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("Another group chats = %v, want only %d", got, groupA)
	}
	if title := markup.InlineKeyboard[0][0].Text; !strings.HasPrefix(title, "✅") {
		t.Fatalf("connected group button = %q, want it marked connected", title)
	}
	if n := adminLookups(client); n != 0 {
		t.Fatalf("admin lookups for Another group inside the pause = %d, want 0", n)
	}
}

// Another group inside the pause with no list to show again gets the notice
// as a short callback answer and leaves the message as it is.
func TestSettingsPickerAnotherGroupWithoutAStoredListWaits(t *testing.T) {
	client, bot, groupA, _, user := pauseFixture(t, 5*time.Second)

	if got := runSettingsInPM(t, bot, client, user); len(got) != 1 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("first picker chats = %v, want only %d", got, groupA)
	}
	if err := cache.GetMarshal().Delete(cache.Context, settingsPickerListKey(user.Id)); err != nil {
		t.Fatalf("delete stored list: %v", err)
	}
	resetCalls(client)

	pressSettingsButton(t, bot, user, map[string]string{"a": "l"})
	answers := client.callsFor("answerCallbackQuery")
	if len(answers) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want one notice", len(answers))
	}
	assertWaitNotice(t, fmt.Sprint(answers[0].Params["text"]))
	if edits := client.callsFor("editMessageText"); len(edits) != 0 {
		t.Fatalf("paused Another group edited the message: %+v", edits)
	}
}
