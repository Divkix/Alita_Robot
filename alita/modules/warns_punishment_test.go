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

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
)

func TestWarnLimitDispatcherDecidesPunishment(t *testing.T) {
	useEnglishLocale(t)
	for _, tc := range []struct {
		name     string
		settings models.WarnSettings
		mode     punishment.Mode
		duration time.Duration
		text     string
	}{
		{"kick", models.WarnSettings{WarnMode: "kick"}, punishment.Kick, 0, "kicked"},
		{"mute", models.WarnSettings{WarnMode: "mute"}, punishment.Mute, 0, "Muted"},
		{"ban", models.WarnSettings{WarnMode: "ban"}, punishment.Ban, 0, "banned"},
		{"tban", models.WarnSettings{WarnMode: "tban", WarnActionTime: 86400}, punishment.Ban, 24 * time.Hour, "banned for 1d"},
		{"tmute", models.WarnSettings{WarnMode: "tmute", WarnActionTime: 7200}, punishment.Mute, 2 * time.Hour, "muted for 2h"},
		{"longest", models.WarnSettings{WarnMode: "tban", WarnActionTime: 366 * 86400}, punishment.Ban, 366 * 24 * time.Hour, "banned for 366d"},
		// Rows without a usable duration keep the one-hour fallback.
		{"legacy zero", models.WarnSettings{WarnMode: "tmute"}, punishment.Mute, time.Hour, "muted for 1h"},
		{"too short", models.WarnSettings{WarnMode: "tban", WarnActionTime: 30}, punishment.Ban, time.Hour, "banned for 1h"},
		{"too long", models.WarnSettings{WarnMode: "tmute", WarnActionTime: 366*86400 + 1}, punishment.Mute, time.Hour, "muted for 1h"},
	} {
		// The bot's own warns (blacklists) punish as the bot.
		for _, issuer := range []string{"admin", "bot"} {
			t.Run(tc.name+"/"+issuer, func(t *testing.T) {
				f := newAdminChatFixture(t)
				settings := tc.settings
				settings.ChatId, settings.WarnLimit = f.chat.Id, 2
				if err := db.DB.Create(&settings).Error; err != nil {
					t.Fatal(err)
				}
				warnedBy, actorID := f.admin.Id, f.admin.Id
				if issuer == "bot" {
					warnedBy, actorID = f.bot.Id, 0
				}
				var requests []punishment.Request
				d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				d.AddHandlerToGroup(handlers.NewMessage(message.All, func(bot *gotgbot.Bot, ctx *ext.Context) error {
					return warnsModule.warnThisUserWithPunisher(bot, ctx, f.member.Id, "limit reached", "warn", models.WarnEntry{WarnedBy: warnedBy},
						func(ctx context.Context, bot *gotgbot.Bot, request punishment.Request) (punishment.Result, error) {
							requests = append(requests, request)
							return punishment.Apply(ctx, bot, request)
						})
				}), 0)
				for i := range 2 {
					if err := d.ProcessUpdate(f.bot, newWarnReplyContext(f.bot, f.chat, f.admin, f.member, "/warn limit reached").Update, nil); err != nil {
						t.Fatal(err)
					}
					if i == 0 && len(requests) != 0 {
						t.Fatalf("below the limit requests = %+v, want none", requests)
					}
				}
				want := punishment.Request{
					ChatID: f.chat.Id, Target: punishment.Target{ID: f.member.Id}, Mode: tc.mode, Duration: tc.duration,
					Source: models.BanSourceWarn, Reason: "limit reached", ActorID: actorID,
				}
				if len(requests) != 1 || requests[0] != want {
					t.Fatalf("punishment decisions = %+v, want [%+v]", requests, want)
				}
				if numWarns, _ := warns.GetWarns(f.member.Id, f.chat.Id); numWarns != 0 {
					t.Fatalf("numWarns after punishment = %d, want reset to 0", numWarns)
				}
				if reply := lastReply(t, f.client); !strings.Contains(reply, "2/2") || !strings.Contains(reply, tc.text) {
					t.Fatalf("limit reply = %q, want %q", reply, tc.text)
				}
			})
		}
	}
}

// Admins cannot be warned, so they never reach the limit's punishment.
func TestWarnLimitExemptsAdmins(t *testing.T) {
	f := newAdminChatFixture(t)
	if err := warns.SetWarnLimit(f.chat.Id, 1); err != nil {
		t.Fatal(err)
	}
	called := false
	err := warnsModule.warnThisUserWithPunisher(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/warn"), f.admin.Id, "", "warn", warnIssuer(newModuleMessageContext(f.bot, f.chat, f.admin, "/warn")),
		func(context.Context, *gotgbot.Bot, punishment.Request) (punishment.Result, error) {
			called = true
			return punishment.Result{}, nil
		})
	if err != ext.EndGroups || called {
		t.Fatalf("warn of an admin: err = %v, punished = %v, want EndGroups and no punishment", err, called)
	}
}

// A warn-limit ban keeps its record, its log entry's Unban button and reply.
func TestWarnLimitBanKeepsRecordAndLog(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	withBanLogChannel(t, f)
	// An admin's warn logs under the opt-in warns category.
	if err := logchannels.SetCategory(f.chat.Id, logchannels.CategoryWarns, true); err != nil {
		t.Fatal(err)
	}
	if err := warns.SetWarnLimit(f.chat.Id, 1); err != nil {
		t.Fatal(err)
	}
	if err := warns.SetWarnMode(f.chat.Id, "tban", 600); err != nil {
		t.Fatal(err)
	}
	if err := warnsModule.warnUser(f.bot, newWarnReplyContext(f.bot, f.chat, f.admin, f.member, "/warn flooding")); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}
	record := singleBanRecord(t, f.chat.Id)
	if record.Source != models.BanSourceWarn || record.BannedBy != f.admin.Id || record.Reason != "flooding" || record.Until == nil {
		t.Fatalf("warn-limit record = %+v", record)
	}
	if entries := logUnbanButtons(t, f); len(entries) != 1 || len(entries[0]) != 1 || entries[0][0] != unbanLogData(record.ID) {
		t.Fatalf("warn-limit log buttons = %+v", entries)
	}
	logs := callsToChat(f.client, "sendMessage", banLogChannelID)
	if text := fmt.Sprint(logs[0].Params["text"]); !strings.Contains(text, "#WARNLIMIT") || !strings.Contains(text, "tban 10m") {
		t.Fatalf("warn-limit log = %q", text)
	}
	if reply := lastReply(t, f.client); !strings.Contains(reply, "banned for 10m") {
		t.Fatalf("warn-limit reply = %q", reply)
	}
}
