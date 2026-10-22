package modules

import (
	"errors"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
)

// chatModerationEvents returns the moderation_events rows the fixture chat
// recorded, oldest first.
func chatModerationEvents(t *testing.T, chatID int64) []models.ModerationEvent {
	t.Helper()
	var rows []models.ModerationEvent
	if err := db.DB.Where("chat_id = ?", chatID).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("load moderation events for chat %d: %v", chatID, err)
	}
	return rows
}

// TestSuccessfulModerationCommandsRecordOneEvent covers the choke point: every
// successful ban, kick, mute and unban writes exactly one row, with the
// variant folded into its base action.
func TestSuccessfulModerationCommandsRecordOneEvent(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		action  string
		reason  string
		asReply bool
		run     func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "ban", text: "/ban spam", action: models.ModerationActionBan, reason: "spam", asReply: true, run: bansModule.ban},
		{name: "tban", text: "/tban 1h spam", action: models.ModerationActionBan, reason: "spam", asReply: true, run: bansModule.tBan},
		{name: "dban", text: "/dban spam", action: models.ModerationActionBan, reason: "spam", asReply: true, run: bansModule.dBan},
		{name: "sban", text: "/sban", action: models.ModerationActionBan, asReply: true, run: bansModule.sBan},
		{name: "kick", text: "/kick spam", action: models.ModerationActionKick, reason: "spam", asReply: true, run: bansModule.kick},
		{name: "dkick", text: "/dkick cleanup", action: models.ModerationActionKick, asReply: true, run: bansModule.dkick},
		{name: "skick", text: "/skick", action: models.ModerationActionKick, asReply: true, run: bansModule.sKick},
		{name: "mute", text: "/mute noisy", action: models.ModerationActionMute, reason: "noisy", asReply: true, run: mutesModule.mute},
		{name: "tmute", text: "/tmute 1h noisy", action: models.ModerationActionMute, reason: "noisy", asReply: true, run: mutesModule.tMute},
		{name: "dmute", text: "/dmute noisy", action: models.ModerationActionMute, reason: "noisy", asReply: true, run: mutesModule.dMute},
		{name: "smute", text: "/smute", action: models.ModerationActionMute, asReply: true, run: mutesModule.sMute},
		{name: "unban", text: "/unban 42", action: models.ModerationActionUnban, run: bansModule.unban},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			ctx := newModuleMessageContext(f.bot, f.chat, f.admin, tt.text)
			if tt.asReply {
				ctx = newMuteReplyContext(f.bot, f.chat, f.admin, f.member, tt.text)
			}

			if err := tt.run(f.bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s() error = %v, want EndGroups", tt.name, err)
			}

			rows := chatModerationEvents(t, f.chat.Id)
			if len(rows) != 1 {
				t.Fatalf("%s recorded %d events, want exactly 1", tt.name, len(rows))
			}
			got := rows[0]
			if got.ChatId != f.chat.Id || got.ActorId != f.admin.Id || got.TargetUserId != f.member.Id {
				t.Fatalf("%s event = %+v, want chat %d, actor %d, target %d",
					tt.name, got, f.chat.Id, f.admin.Id, f.member.Id)
			}
			if got.Action != tt.action {
				t.Fatalf("%s event action = %q, want %q", tt.name, got.Action, tt.action)
			}
			if got.Reason != tt.reason {
				t.Fatalf("%s event reason = %q, want %q", tt.name, got.Reason, tt.reason)
			}
			if got.CreatedAt.IsZero() {
				t.Fatalf("%s event has no timestamp", tt.name)
			}
			if _, offset := got.CreatedAt.Zone(); db.DB.Name() == "sqlite" && offset != 0 {
				t.Fatalf("%s event timestamp %v is not UTC", tt.name, got.CreatedAt)
			}
		})
	}
}

// TestModerationCommandsWithoutARecordableActionWriteNoEvent covers the
// commands the choke point must skip: a self-target action, a call with no
// target, and unmute.
func TestModerationCommandsWithoutARecordableActionWriteNoEvent(t *testing.T) {
	tests := []struct {
		name string
		run  func(f adminChatFixture) error
	}{
		{
			name: "kickme",
			run: func(f adminChatFixture) error {
				return bansModule.kickme(f.bot, newModuleMessageContext(f.bot, f.chat, f.member, "/kickme"))
			},
		},
		{
			name: "no target",
			run: func(f adminChatFixture) error {
				return bansModule.ban(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/ban"))
			},
		},
		{
			name: "unmute",
			run: func(f adminChatFixture) error {
				return mutesModule.unmute(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/unmute 42"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			if err := tt.run(f); err != ext.EndGroups {
				t.Fatalf("%s() error = %v, want EndGroups", tt.name, err)
			}
			if rows := chatModerationEvents(t, f.chat.Id); len(rows) != 0 {
				t.Fatalf("%s recorded %d events, want none", tt.name, len(rows))
			}
		})
	}
}

func TestFailedModerationActionWritesNoEvent(t *testing.T) {
	f := newAdminChatFixture(t)
	requestErr := errors.New("Forbidden: bot is not an administrator")
	f.client.errors["banChatMember"] = requestErr
	ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, "/ban spam")

	if err := bansModule.ban(f.bot, ctx); !errors.Is(err, requestErr) {
		t.Fatalf("ban() error = %v, want the request error", err)
	}
	if rows := chatModerationEvents(t, f.chat.Id); len(rows) != 0 {
		t.Fatalf("failed ban recorded %d events, want none", len(rows))
	}
	if calls := f.client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("failed ban sent %d messages, want none", len(calls))
	}
}

// TestEventWriteFailureKeepsTheModerationReply drops the events table so the
// write fails for real, and checks the ban still answers as usual.
func TestEventWriteFailureKeepsTheModerationReply(t *testing.T) {
	f := newAdminChatFixture(t)
	if err := db.DB.Migrator().DropTable(&models.ModerationEvent{}); err != nil {
		t.Fatalf("drop moderation_events: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.ModerationEvent{}); err != nil {
			t.Errorf("restore moderation_events: %v", err)
		}
	})
	ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, "/ban spam")

	if err := bansModule.ban(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("ban() error = %v, want EndGroups despite the event write failure", err)
	}
	calls := f.client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want the normal ban reply", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("ban reply lost its unban button after the event write failed")
	}
}

// TestModerationEventRetentionKeepsTheLast180Days drives the hourly sweep with
// its own clock: the event past 180 days goes, the one inside stays.
func TestModerationEventRetentionKeepsTheLast180Days(t *testing.T) {
	chatID := uniqueModuleChatID()
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ModerationEvent{}).Error; err != nil {
			t.Errorf("delete events of chat %d: %v", chatID, err)
		}
	})
	// Whole seconds survive the round trip through every supported database.
	now := time.Now().UTC().Truncate(time.Second)
	freshAt := now.Add(-179 * 24 * time.Hour)
	for _, ev := range []models.ModerationEvent{
		{ChatId: chatID, ActorId: 1, TargetUserId: 1, Action: models.ModerationActionBan,
			CreatedAt: now.Add(-181 * 24 * time.Hour)},
		{ChatId: chatID, ActorId: 1, TargetUserId: 2, Action: models.ModerationActionBan,
			CreatedAt: freshAt},
	} {
		if err := moderation.Record(ev); err != nil {
			t.Fatalf("Record(%+v) error = %v", ev, err)
		}
	}

	sweepExpiredModerationEvents(now)

	rows := chatModerationEvents(t, chatID)
	if len(rows) != 1 {
		t.Fatalf("events after the sweep = %d, want only the one inside 180 days", len(rows))
	}
	if !rows[0].CreatedAt.Equal(freshAt) || rows[0].TargetUserId != 2 {
		t.Fatalf("event after the sweep = %+v, want the 179-day-old event", rows[0])
	}
}
