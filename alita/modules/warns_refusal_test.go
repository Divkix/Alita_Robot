//go:build testtools

package modules

import (
	"errors"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// A refusal answers the command, so it ends the update: later handler groups
// (antiflood, locks, blacklists, filters) must not see the command message.
// When the answer cannot be sent, the handler returns the send error.
func TestWarnRefusalsEndTheUpdate(t *testing.T) {
	useEnglishLocale(t)
	for _, tc := range []struct {
		name string
		run  func(f adminChatFixture) error
	}{
		{"warn an admin", func(f adminChatFixture) error {
			return warnsModule.warnUser(f.bot, newWarnReplyContext(f.bot, f.chat, f.admin, f.admin, "/warn"))
		}},
		{"resetallwarns with nobody warned", func(f adminChatFixture) error {
			return warnsModule.resetAllWarns(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/resetallwarns"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			f.makeOwner()
			if err := tc.run(f); err != ext.EndGroups {
				t.Fatalf("error = %v, want EndGroups", err)
			}
			if calls := callsToChat(f.client, "sendMessage", f.chat.Id); len(calls) != 1 {
				t.Fatalf("replies = %d, want the refusal", len(calls))
			}
		})
		t.Run(tc.name+"/reply fails", func(t *testing.T) {
			f := newAdminChatFixture(t)
			f.makeOwner()
			sendErr := errors.New("send failed")
			f.client.errors["sendMessage"] = sendErr
			if err := tc.run(f); !errors.Is(err, sendErr) {
				t.Fatalf("error = %v, want the reply's error", err)
			}
		})
	}
}
