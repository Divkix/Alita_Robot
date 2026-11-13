//go:build testtools

package modules

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// A ban by ID may name a user the bot has never met, whose name lookup
// (getChat) fails. The action stands, so the admin still gets a reply that
// mentions the user by ID, and the log channel still gets its entry, even
// when the reply itself cannot be sent.
func TestModerationByIDWithoutNameConfirmsAndLogs(t *testing.T) {
	useEnglishLocale(t)
	const userID int64 = 5555
	for _, tc := range []struct {
		name   string
		text   string
		action string
		run    func(*gotgbot.Bot, *ext.Context) error
	}{
		{"ban", "/ban 5555", "banChatMember", bansModule.ban},
		{"tban", "/tban 5555 1h", "banChatMember", bansModule.tBan},
		{"kick", "/kick 5555", "unbanChatMember", bansModule.kick},
	} {
		for _, replyFails := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reply fails=%v", tc.name, replyFails), func(t *testing.T) {
				f := newAdminChatFixture(t)
				withBanLogChannel(t, f)
				lookupErr := errors.New("Bad Request: chat not found")
				f.client.errorsByChat = map[string]error{fmt.Sprintf("getChat|%d", userID): lookupErr}
				replyErr := errors.New("send failed")
				if replyFails {
					f.client.errorsByChat[fmt.Sprintf("sendMessage|%d", f.chat.Id)] = replyErr
				}

				err := tc.run(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, tc.text))
				want := error(ext.EndGroups)
				if replyFails {
					want = replyErr
				}
				if !errors.Is(err, want) {
					t.Fatalf("error = %v, want %v", err, want)
				}

				var acted bool
				for _, call := range f.client.callsFor(tc.action) {
					acted = acted || fmt.Sprint(call.Params["user_id"]) == fmt.Sprint(userID)
				}
				if !acted {
					t.Fatalf("%s for %d not sent", tc.action, userID)
				}
				replies := callsToChat(f.client, "sendMessage", f.chat.Id)
				if len(replies) != 1 {
					t.Fatalf("replies = %d, want the confirmation", len(replies))
				}
				if text, _ := replies[0].Params["text"].(string); !strings.Contains(text, fmt.Sprintf("tg://user?id=%d", userID)) {
					t.Fatalf("reply = %q, want a mention of user %d", text, userID)
				}
				if entries := callsToChat(f.client, "sendMessage", banLogChannelID); len(entries) != 1 {
					t.Fatalf("log channel entries = %d, want 1", len(entries))
				}
			})
		}
	}
}
