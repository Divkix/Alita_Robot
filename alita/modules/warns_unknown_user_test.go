//go:build testtools

package modules

import (
	"errors"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

const unknownUserName = "@nosuchuser"

// seedCallerWarns gives the command's caller warns in chatID, so a reply that
// falls back to the caller's own warns shows in the sent messages.
func seedCallerWarns(t *testing.T, callerID, chatID int64) {
	t.Helper()
	ensureWarnParents(t, chatID, callerID)
	for range 2 {
		if _, _, err := warns.WarnUser(callerID, chatID, models.WarnEntry{Reason: "caller seed"}); err != nil {
			t.Fatalf("WarnUser() error = %v", err)
		}
	}
}

// requireOnlyNotFoundReply checks that exactly one message went out, the
// "user not found" answer, and none of the caller's warns.
func requireOnlyNotFoundReply(t *testing.T, f adminChatFixture, chatID int64) {
	t.Helper()
	calls := f.client.callsFor("sendMessage")
	if len(calls) != 1 || !repliedIn(f, chatID, translated(t, "extraction_user_not_found")) {
		t.Fatalf("want exactly one %q reply in chat %d; sent:\n%s", translated(t, "extraction_user_not_found"), chatID, sentText(f.client))
	}
	if repliedIn(f, chatID, warnsCountText(t, 2)) {
		t.Fatalf("the caller's own warns were shown; sent:\n%s", sentText(f.client))
	}
}

// /warns on a name the bot cannot find says so once. It does not fall back to
// the caller's own warns.
func TestWarnsWithUnknownUsernameRepliesNotFoundOnly(t *testing.T) {
	useEnglishLocale(t)

	t.Run("group", func(t *testing.T) {
		f := newAdminChatFixture(t)
		f.client.errorsByChat = map[string]error{"getChat|" + unknownUserName: errors.New("Bad Request: chat not found")}
		seedCallerWarns(t, f.admin.Id, f.chat.Id)

		ctx := newModuleMessageContext(f.bot, f.chat, f.admin, "/warns "+unknownUserName)
		if err := warnsModule.warns(f.bot, ctx); err != ext.EndGroups {
			t.Fatalf("warns() error = %v, want EndGroups", err)
		}

		requireOnlyNotFoundReply(t, f, f.chat.Id)
	})

	for _, tc := range []struct {
		name, text string
	}{
		{"PM with the group named", "/warns @fixturegroup " + unknownUserName},
		{"PM on a connection", "/warns " + unknownUserName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, connected := pmWarnsFixture(t, 0)
			f.client.errorsByChat = map[string]error{"getChat|" + unknownUserName: errors.New("Bad Request: chat not found")}
			seedCallerWarns(t, f.admin.Id, f.chat.Id)
			seedCallerWarns(t, f.admin.Id, connected)

			processPM(t, warnsPMDispatcher(), f, f.admin.Id, tc.text)

			requireOnlyNotFoundReply(t, f, f.admin.Id)
			requireNothingSentTo(t, f, f.chat.Id)
			requireNothingSentTo(t, f, connected)
			if got := fmt.Sprint(len(f.client.callsFor("sendMessage"))); got != "1" {
				t.Fatalf("sendMessage calls = %s, want 1", got)
			}
		})
	}
}
