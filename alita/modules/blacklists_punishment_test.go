//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"

	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestBlacklistDispatcherDecidesPunishment(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		action   string
		stored   int
		punishes bool
		method   string
		duration time.Duration
		timed    bool
	}{
		{"ban", 0, true, "banChatMember", 0, false},
		{"kick", 0, true, "unbanChatMember", 0, false},
		{"mute", 0, true, "restrictChatMember", 0, false},
		{"tban", 600, true, "banChatMember", 10 * time.Minute, true},
		{"tmute", 86400, true, "restrictChatMember", 24 * time.Hour, true},
		// Rows without a usable duration keep the one-hour fallback.
		{"tban", 0, true, "banChatMember", time.Hour, true},
		{"tban", 30, true, "banChatMember", time.Hour, true},
		{"tmute", 366*86400 + 1, true, "restrictChatMember", time.Hour, true},
		// A warn goes to the warns module; none only deletes the message.
		{"warn", 0, false, "", 0, false},
		{"none", 0, false, "", 0, false},
	} {
		for _, actor := range []string{"member", "channel", "approved", "admin"} {
			t.Run(fmt.Sprintf("%s/%d/%s", tc.action, tc.stored, actor), func(t *testing.T) {
				f := newAdminChatFixture(t)
				if err := blacklists.AddBlacklist(f.chat.Id, "spam"); err != nil {
					t.Fatal(err)
				}
				if err := blacklists.SetBlacklistAction(f.chat.Id, tc.action, tc.stored); err != nil {
					t.Fatal(err)
				}
				if actor == "approved" {
					if err := approvals.AddApprovedUser(f.chat.Id, f.member.Id, f.admin.Id, "trusted"); err != nil {
						t.Fatal(err)
					}
				}
				d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				d.AddHandlerToGroup(handlers.NewMessage(message.All, blacklistsModule.blacklistWatcher), blacklistsModule.handlerGroup)
				user := f.member
				if actor == "admin" {
					user = f.admin
				}
				ctx := newModuleMessageContext(f.bot, f.chat, user, "this has spam inside")
				channel := gotgbot.Chat{Id: -1001234567893, Type: "channel", Title: "Spam channel"}
				if actor == "channel" {
					ctx.EffectiveMessage.From = nil
					ctx.EffectiveMessage.SenderChat = &channel
				}
				before := time.Now().Unix()
				if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
					t.Fatal(err)
				}
				after := time.Now().Unix()

				calls := punishmentCalls(f.client)
				if !tc.punishes || actor == "approved" || actor == "admin" {
					if len(calls) != 0 {
						t.Fatalf("punishment calls = %+v, want none", calls)
					}
					return
				}
				if actor == "channel" {
					// A sender chat takes only a permanent ban; every other
					// mode is refused inside punishment.Apply before any call.
					if tc.action != "ban" {
						if len(calls) != 0 {
							t.Fatalf("sender chat punished with %s: %+v", tc.action, calls)
						}
						return
					}
					if len(calls) != 1 || calls[0].Method != "banChatSenderChat" {
						t.Fatalf("sender chat punishment calls = %+v, want one banChatSenderChat", calls)
					}
					assertParams(t, calls[0], map[string]any{"chat_id": f.chat.Id, "sender_chat_id": channel.Id})
					return
				}
				if len(calls) != 1 || calls[0].Method != tc.method {
					t.Fatalf("punishment calls = %+v, want one %s", calls, tc.method)
				}
				assertParams(t, calls[0], targetParams(f.chat.Id, f.member.Id))
				if !tc.timed {
					if calls[0].Params["until_date"] != nil {
						t.Fatalf("%s params = %#v, want no until_date", calls[0].Method, calls[0].Params)
					}
					return
				}
				secs := int64(tc.duration / time.Second)
				if until := untilDateParam(t, calls[0]); until < before+secs || until > after+secs {
					t.Fatalf("until_date = %d, want now+%d (%d..%d)", until, secs, before+secs, after+secs)
				}
			})
		}
	}
}

// A blacklist ban keeps its record, its log entry's Unban button and its
// notice.
func TestBlacklistBanKeepsRecordNoticeAndLog(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	if err := blacklists.AddBlacklist(f.chat.Id, "spam"); err != nil {
		t.Fatal(err)
	}
	if err := blacklists.SetBlacklistAction(f.chat.Id, "tban", 600); err != nil {
		t.Fatal(err)
	}
	if err := blacklistsModule.blacklistWatcher(f.bot, newModuleMessageContext(f.bot, f.chat, f.member, "this has spam inside")); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher() error = %v, want ContinueGroups", err)
	}
	record := singleBanRecord(t, f.chat.Id)
	if record.Source != models.BanSourceBlacklist || record.BannedBy != 0 || record.Reason != "Blacklisted word: 'spam'" || record.Until == nil {
		t.Fatalf("blacklist record = %+v", record)
	}
	if entries := logUnbanButtons(t, f); len(entries) != 1 || len(entries[0]) != 1 || entries[0][0] != unbanLogData(record.ID) {
		t.Fatalf("blacklist log buttons = %+v", entries)
	}
	logs := callsToChat(f.client, "sendMessage", banLogChannelID)
	if text := fmt.Sprint(logs[0].Params["text"]); !strings.Contains(text, "#BLACKLIST") || !strings.Contains(text, "tban 10m") || !strings.Contains(text, "spam") {
		t.Fatalf("blacklist log = %q", text)
	}
	notices := callsToChat(f.client, "sendMessage", f.chat.Id)
	if len(notices) != 1 || !strings.Contains(fmt.Sprint(notices[0].Params["text"]), "Banned") || !strings.Contains(fmt.Sprint(notices[0].Params["text"]), "for 10m due to") {
		t.Fatalf("blacklist notice = %+v", notices)
	}
}
