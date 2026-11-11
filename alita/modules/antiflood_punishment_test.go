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

	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
)

func TestAntifloodDispatcherDecidesPunishment(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		action   string
		stored   int
		mode     punishment.Mode
		duration time.Duration
	}{
		{"ban", 0, punishment.Ban, 0},
		{"kick", 0, punishment.Kick, 0},
		{"mute", 0, punishment.Mute, 0},
		{"tban", 600, punishment.Ban, 10 * time.Minute},
		{"tmute", 86400, punishment.Mute, 24 * time.Hour},
		{"tban", 0, punishment.Ban, time.Hour},
		{"tmute", 30, punishment.Mute, time.Hour},
		{"tmute", 367 * 86400, punishment.Mute, time.Hour},
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
				var requests []punishment.Request
				d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				d.AddHandlerToGroup(handlers.NewMessage(message.All, func(bot *gotgbot.Bot, ctx *ext.Context) error {
					return antifloodModule.checkFloodWithPunisher(bot, ctx, func(ctx context.Context, bot *gotgbot.Bot, request punishment.Request) (punishment.Result, error) {
						requests = append(requests, request)
						return punishment.Apply(ctx, bot, request)
					})
				}), antifloodModule.handlerGroup)
				channel := gotgbot.Chat{Id: -1001234567892, Type: "channel", Title: "Flood channel"}
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
				if actor != "member" && actor != "channel" {
					if len(requests) != 0 {
						t.Fatalf("exempt sender punished: %+v", requests)
					}
					return
				}
				if len(requests) != 1 {
					t.Fatalf("requests = %+v, want one after limit", requests)
				}
				request := requests[0]
				id := f.member.Id
				if actor == "channel" {
					id = channel.Id
				}
				if request.ChatID != f.chat.Id || request.Target.ID != id || request.Target.SenderChat != (actor == "channel") || request.Mode != tc.mode || request.Duration != tc.duration || request.Source != models.BanSourceAntiflood || request.ActorID != 0 || request.Reason != "" || request.Detail != "" {
					t.Fatalf("punishment decision = %+v", request)
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
