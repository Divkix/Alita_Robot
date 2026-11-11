//go:build testtools

package modules

import (
	"context"
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
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
)

func TestBlacklistDispatcherDecidesPunishment(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		action   string
		stored   int
		punishes bool
		mode     punishment.Mode
		duration time.Duration
	}{
		{"ban", 0, true, punishment.Ban, 0},
		{"kick", 0, true, punishment.Kick, 0},
		{"mute", 0, true, punishment.Mute, 0},
		{"tban", 600, true, punishment.Ban, 10 * time.Minute},
		{"tmute", 86400, true, punishment.Mute, 24 * time.Hour},
		// Rows without a usable duration keep the one-hour fallback.
		{"tban", 0, true, punishment.Ban, time.Hour},
		{"tban", 30, true, punishment.Ban, time.Hour},
		{"tmute", 366*86400 + 1, true, punishment.Mute, time.Hour},
		// A warn goes to the warns module; none only deletes the message.
		{"warn", 0, false, "", 0},
		{"none", 0, false, "", 0},
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
				var requests []punishment.Request
				d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				d.AddHandlerToGroup(handlers.NewMessage(message.All, func(bot *gotgbot.Bot, ctx *ext.Context) error {
					return blacklistsModule.blacklistWatcherWithPunisher(bot, ctx, func(ctx context.Context, bot *gotgbot.Bot, request punishment.Request) (punishment.Result, error) {
						requests = append(requests, request)
						return punishment.Apply(ctx, bot, request)
					})
				}), blacklistsModule.handlerGroup)
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
				if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
					t.Fatal(err)
				}
				if !tc.punishes || actor == "approved" || actor == "admin" {
					if len(requests) != 0 {
						t.Fatalf("requests = %+v, want none", requests)
					}
					return
				}
				if len(requests) != 1 {
					t.Fatalf("requests = %+v, want one", requests)
				}
				id := f.member.Id
				if actor == "channel" {
					id = channel.Id
				}
				want := punishment.Request{
					ChatID: f.chat.Id, Target: punishment.Target{ID: id, SenderChat: actor == "channel"},
					Mode: tc.mode, Duration: tc.duration, Source: models.BanSourceBlacklist,
					Reason: "Blacklisted word: 'spam'",
				}
				if requests[0] != want {
					t.Fatalf("punishment decision = %+v, want %+v", requests[0], want)
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
