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

	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// punishmentCalls returns the Bot API calls that apply or lift a punishment,
// grouped by method: banChatMember, banChatSenderChat, unbanChatMember,
// unbanChatSenderChat and restrictChatMember. Lookups such as getChatMember
// are not punishments.
func punishmentCalls(client *moduleBotClient) []moduleBotCall {
	var out []moduleBotCall
	for _, method := range []string{
		"banChatMember", "banChatSenderChat", "unbanChatMember", "unbanChatSenderChat", "restrictChatMember",
	} {
		out = append(out, client.callsFor(method)...)
	}
	return out
}

func TestAntifloodDispatcherDecidesPunishment(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		action   string
		stored   int
		method   string
		duration time.Duration
		timed    bool
	}{
		{"ban", 0, "banChatMember", 0, false},
		{"kick", 0, "unbanChatMember", 0, false},
		{"mute", 0, "restrictChatMember", 0, false},
		{"tban", 600, "banChatMember", 10 * time.Minute, true},
		{"tmute", 86400, "restrictChatMember", 24 * time.Hour, true},
		{"tban", 0, "banChatMember", time.Hour, true},
		{"tmute", 30, "restrictChatMember", time.Hour, true},
		{"tmute", 367 * 86400, "restrictChatMember", time.Hour, true},
	} {
		for _, actor := range []string{"member", "channel", "approved", "admin", "linked channel"} {
			t.Run(fmt.Sprintf("%s/%d/%s", tc.action, tc.stored, actor), func(t *testing.T) {
				resetAntifloodState(t)
				f := newAdminChatFixture(t)
				if err := antiflood.SetFlood(f.chat.Id, 3); err != nil {
					t.Fatal(err)
				}
				if err := antiflood.SetFloodMode(f.chat.Id, tc.action, tc.stored); err != nil {
					t.Fatal(err)
				}
				if actor == "approved" {
					if err := approvals.AddApprovedUser(f.chat.Id, f.member.Id, f.admin.Id, "trusted"); err != nil {
						t.Fatal(err)
					}
				}
				d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				d.AddHandlerToGroup(handlers.NewMessage(message.All, antifloodModule.checkFlood), antifloodModule.handlerGroup)
				channel := gotgbot.Chat{Id: -1001234567892, Type: "channel", Title: "Flood channel"}
				before := time.Now().Unix()
				for i := range 4 {
					user := f.member
					if actor == "admin" {
						user = f.admin
					}
					ctx := newModuleMessageContext(f.bot, f.chat, user, "flood")
					ctx.EffectiveMessage.MessageId = int64(600 + i)
					if actor == "channel" || actor == "linked channel" {
						ctx.EffectiveMessage.From = nil
						ctx.EffectiveMessage.SenderChat = &channel
						ctx.EffectiveMessage.IsAutomaticForward = actor == "linked channel"
					}
					if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
						t.Fatal(err)
					}
				}
				after := time.Now().Unix()

				calls := punishmentCalls(f.client)
				if actor != "member" && actor != "channel" {
					if len(calls) != 0 {
						t.Fatalf("exempt sender punished: %+v", calls)
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

func TestAntifloodBanKeepsNoticeAndLog(t *testing.T) {
	useEnglishLocale(t)
	resetAntifloodState(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	if err := antiflood.SetFlood(f.chat.Id, 3); err != nil {
		t.Fatal(err)
	}
	if err := antiflood.SetFloodMode(f.chat.Id, "tban", 600); err != nil {
		t.Fatal(err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAntiflood(d)
	for i := range 4 {
		ctx := newModuleMessageContext(f.bot, f.chat, f.member, "flood")
		ctx.EffectiveMessage.MessageId = int64(700 + i)
		if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
			t.Fatal(err)
		}
	}
	record := singleBanRecord(t, f.chat.Id)
	if record.Source != models.BanSourceAntiflood || record.BannedBy != 0 || record.Until == nil {
		t.Fatalf("antiflood record = %+v", record)
	}
	entries := logUnbanButtons(t, f)
	if len(entries) != 1 || len(entries[0]) != 1 || entries[0][0] != unbanLogData(record.ID) {
		t.Fatalf("antiflood log buttons = %+v", entries)
	}
	logs := callsToChat(f.client, "sendMessage", banLogChannelID)
	if text := fmt.Sprint(logs[0].Params["text"]); !strings.Contains(text, "#ANTIFLOOD") || !strings.Contains(text, "tban 10m") {
		t.Fatalf("antiflood log = %q", text)
	}
	notices := callsToChat(f.client, "sendMessage", f.chat.Id)
	if len(notices) != 1 || !strings.Contains(fmt.Sprint(notices[0].Params["text"]), "banned for 10m") {
		t.Fatalf("antiflood notice = %+v", notices)
	}
}
