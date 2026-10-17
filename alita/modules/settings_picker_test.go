//go:build testtools

package modules

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// Repeated picker presses from one user inside the pause run no group checks
// and get a short notice instead; after the pause the list works again.
func TestSettingsPickerPausesRepeatedPresses(t *testing.T) {
	client, bot, groupA, groupB := settingsFixture(t)
	user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}
	settingsPickerCooldown = 300 * time.Millisecond
	// The real English locale, so a missing or misspelt key shows as no text.
	locale, err := os.ReadFile("../../locales/en.yml")
	if err != nil {
		t.Fatalf("read en.yml: %v", err)
	}
	restoreI18n, err := i18n.OverrideManagerForTest(string(locale))
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restoreI18n)
	listButton := encodeCallbackData("settings", map[string]string{"a": "l"})

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
	if len(replies) != 1 || fmt.Sprint(replies[0].Params["text"]) == "" || len(markupOf(t, replies[0]).InlineKeyboard) != 0 {
		t.Fatalf("paused /settings replies = %+v, want one notice without a picker", replies)
	}
	if err := ConnectionsModule.settingsCallback(bot, newModuleCallbackContext(bot, privateChatOf(user), user, listButton)); err != ext.EndGroups {
		t.Fatalf("settingsCallback error = %v, want EndGroups", err)
	}
	answers := client.callsFor("answerCallbackQuery")
	if len(answers) != 1 || fmt.Sprint(answers[0].Params["text"]) == "" {
		t.Fatalf("paused list answers = %+v, want one notice", answers)
	}
	if edits := client.callsFor("editMessageText"); len(edits) != 0 {
		t.Fatalf("paused list edited the message: %+v", edits)
	}
	if n := adminLookups(client); n != 0 {
		t.Fatalf("admin lookups during the pause = %d, want 0", n)
	}

	time.Sleep(350 * time.Millisecond)
	if err := ConnectionsModule.settingsCallback(bot, newModuleCallbackContext(bot, privateChatOf(user), user, listButton)); err != ext.EndGroups {
		t.Fatalf("settingsCallback error = %v, want EndGroups", err)
	}
	edits := client.callsFor("editMessageText")
	if len(edits) != 1 {
		t.Fatalf("editMessageText calls after the pause = %d, want 1", len(edits))
	}
	if got := pickedChats(t, markupOf(t, edits[0])); len(got) != 1 || got[0] != fmt.Sprint(groupA) {
		t.Fatalf("picker chats after the pause = %v, want only %d", got, groupA)
	}
}
