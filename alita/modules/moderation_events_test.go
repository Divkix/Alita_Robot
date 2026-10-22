package modules

import (
	"errors"
	"fmt"
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

// singleModerationEvent fails unless chatID has exactly one recorded event and
// returns it.
func singleModerationEvent(t *testing.T, chatID int64) models.ModerationEvent {
	t.Helper()
	rows := chatModerationEvents(t, chatID)
	if len(rows) != 1 {
		t.Fatalf("recorded %d events, want exactly 1", len(rows))
	}
	return rows[0]
}

// TestRestrictButtonsRecordOneEvent covers the /restrict menu buttons: each
// writes one row for the action it ran, with the pressing admin as the actor.
func TestRestrictButtonsRecordOneEvent(t *testing.T) {
	for _, tt := range []struct {
		action string
		event  string
		method string
	}{
		{action: "kick", event: models.ModerationActionKick, method: "unbanChatMember"},
		{action: "mute", event: models.ModerationActionMute, method: "restrictChatMember"},
		{action: "ban", event: models.ModerationActionBan, method: "banChatMember"},
	} {
		t.Run(tt.action, func(t *testing.T) {
			f := newAdminChatFixture(t)
			data := encodeCallbackData("restrict", map[string]string{"a": tt.action, "u": fmt.Sprint(f.member.Id)})
			if err := bansModule.restrictButtonHandler(
				f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data),
			); err != ext.EndGroups {
				t.Fatalf("restrictButtonHandler(%s) error = %v, want EndGroups", tt.action, err)
			}
			if calls := f.client.callsFor(tt.method); len(calls) != 1 {
				t.Fatalf("%s calls = %d, want the %s action", tt.method, len(calls), tt.action)
			}

			got := singleModerationEvent(t, f.chat.Id)
			if got.ChatId != f.chat.Id || got.ActorId != f.admin.Id || got.TargetUserId != f.member.Id {
				t.Fatalf("%s event = %+v, want chat %d, actor %d, target %d",
					tt.action, got, f.chat.Id, f.admin.Id, f.member.Id)
			}
			if got.Action != tt.event {
				t.Fatalf("%s event action = %q, want %q", tt.action, got.Action, tt.event)
			}
			if got.Reason != "" {
				t.Fatalf("%s event reason = %q, want none", tt.action, got.Reason)
			}
		})
	}
}

// TestFailedRestrictButtonWritesNoEvent proves the row follows the action, not
// the button press.
func TestFailedRestrictButtonWritesNoEvent(t *testing.T) {
	f := newAdminChatFixture(t)
	requestErr := errors.New("Forbidden: bot is not an administrator")
	f.client.errors["banChatMember"] = requestErr
	data := encodeCallbackData("restrict", map[string]string{"a": "ban", "u": fmt.Sprint(f.member.Id)})

	if err := bansModule.restrictButtonHandler(
		f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data),
	); !errors.Is(err, requestErr) {
		t.Fatalf("restrictButtonHandler(ban) error = %v, want the request error", err)
	}
	if rows := chatModerationEvents(t, f.chat.Id); len(rows) != 0 {
		t.Fatalf("failed restrict button recorded %d events, want none", len(rows))
	}
}

// TestUnrestrictButtonsRecordOnlyTheUnban mirrors the command path: /unban
// records, /unmute records nothing.
func TestUnrestrictButtonsRecordOnlyTheUnban(t *testing.T) {
	t.Run("unban", func(t *testing.T) {
		f := newAdminChatFixture(t)
		data := encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(f.member.Id)})
		if err := bansModule.unrestrictButtonHandler(
			f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data),
		); err != ext.EndGroups {
			t.Fatalf("unrestrictButtonHandler(unban) error = %v, want EndGroups", err)
		}
		if calls := f.client.callsFor("unbanChatMember"); len(calls) != 1 {
			t.Fatalf("unbanChatMember calls = %d, want the unban action", len(calls))
		}

		got := singleModerationEvent(t, f.chat.Id)
		if got.ChatId != f.chat.Id || got.ActorId != f.admin.Id || got.TargetUserId != f.member.Id {
			t.Fatalf("unban event = %+v, want chat %d, actor %d, target %d",
				got, f.chat.Id, f.admin.Id, f.member.Id)
		}
		if got.Action != models.ModerationActionUnban {
			t.Fatalf("unban event action = %q, want %q", got.Action, models.ModerationActionUnban)
		}
	})

	t.Run("unmute", func(t *testing.T) {
		f := newAdminChatFixture(t)
		data := encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": fmt.Sprint(f.member.Id)})
		if err := bansModule.unrestrictButtonHandler(
			f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data),
		); err != ext.EndGroups {
			t.Fatalf("unrestrictButtonHandler(unmute) error = %v, want EndGroups", err)
		}
		if calls := f.client.callsFor("restrictChatMember"); len(calls) != 1 {
			t.Fatalf("restrictChatMember calls = %d, want the unmute action", len(calls))
		}
		if rows := chatModerationEvents(t, f.chat.Id); len(rows) != 0 {
			t.Fatalf("unmute button recorded %d events, want none", len(rows))
		}
	})
}

// TestReportCardButtonsRecordOneEvent covers the log-channel card: its ban and
// mute decisions are the group's actions, recorded once each.
func TestReportCardButtonsRecordOneEvent(t *testing.T) {
	for _, tt := range []struct {
		action string
		event  string
	}{
		{action: "ban", event: models.ModerationActionBan},
		{action: "mute", event: models.ModerationActionMute},
	} {
		t.Run(tt.action, func(t *testing.T) {
			f := newReportCardFixture(t, true)
			admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
			seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
				Status: "administrator", User: admin, CanRestrictMembers: true, CanDeleteMessages: true,
			})
			actions := reportedCard(t, f)
			if err := pressCard(t, f, reportCardChannel, 9001, admin, actions[tt.action]); err != ext.EndGroups {
				t.Fatalf("%s press error = %v, want EndGroups", tt.action, err)
			}

			got := singleModerationEvent(t, f.chat.Id)
			if got.ChatId != f.chat.Id || got.ActorId != reportCardAdminID || got.TargetUserId != f.target.Id {
				t.Fatalf("%s event = %+v, want chat %d, actor %d, target %d",
					tt.action, got, f.chat.Id, reportCardAdminID, f.target.Id)
			}
			if got.Action != tt.event {
				t.Fatalf("%s event action = %q, want %q", tt.action, got.Action, tt.event)
			}
		})
	}
}

// TestReportCardDeleteAndDismissRecordNoEvent keeps non-moderation decisions
// out of the evidence table.
func TestReportCardDeleteAndDismissRecordNoEvent(t *testing.T) {
	for _, action := range []string{"delete", "dismiss"} {
		t.Run(action, func(t *testing.T) {
			f := newReportCardFixture(t, true)
			admin := gotgbot.User{Id: reportCardAdminID, FirstName: "Group Admin"}
			seedCallbackAdmins(t, f.chat.Id, reportCardBotMember(), gotgbot.MergedChatMember{
				Status: "administrator", User: admin, CanRestrictMembers: true, CanDeleteMessages: true,
			})
			actions := reportedCard(t, f)
			if err := pressCard(t, f, reportCardChannel, 9001, admin, actions[action]); err != ext.EndGroups {
				t.Fatalf("%s press error = %v, want EndGroups", action, err)
			}
			if rows := chatModerationEvents(t, f.chat.Id); len(rows) != 0 {
				t.Fatalf("%s recorded %d events, want none", action, len(rows))
			}
		})
	}
}

// TestLegacyReportButtonsRecordOneEvent covers the in-group report message's
// kick and ban buttons.
func TestLegacyReportButtonsRecordOneEvent(t *testing.T) {
	for _, tt := range []struct {
		action string
		event  string
	}{
		{action: "kick", event: models.ModerationActionKick},
		{action: "ban", event: models.ModerationActionBan},
	} {
		t.Run(tt.action, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Report Chat"}
			admin := gotgbot.User{Id: 42, FirstName: "Restrict Admin"}
			seedCallbackAdmins(t, chat.Id,
				gotgbot.MergedChatMember{Status: "administrator", User: gotgbot.User{Id: 999, IsBot: true}, CanRestrictMembers: true},
				gotgbot.MergedChatMember{Status: "administrator", User: admin, CanRestrictMembers: true},
			)
			data := encodeCallbackData("report", map[string]string{"a": tt.action, "u": "43", "m": "505"})
			if err := reportsModule.markResolvedButtonHandler(
				bot, newModuleCallbackContext(bot, chat, admin, data),
			); err != ext.EndGroups {
				t.Fatalf("%s press error = %v, want EndGroups", tt.action, err)
			}

			got := singleModerationEvent(t, chat.Id)
			if got.ChatId != chat.Id || got.ActorId != admin.Id || got.TargetUserId != 43 {
				t.Fatalf("%s event = %+v, want chat %d, actor %d, target 43",
					tt.action, got, chat.Id, admin.Id)
			}
			if got.Action != tt.event {
				t.Fatalf("%s event action = %q, want %q", tt.action, got.Action, tt.event)
			}
		})
	}
}

// TestJoinRequestBanRecordsOneEvent covers the join-request card's ban button,
// and proves a failed ban records nothing.
func TestJoinRequestBanRecordsOneEvent(t *testing.T) {
	newJoinBanFixture := func(t *testing.T) (*moduleBotClient, *gotgbot.Bot, gotgbot.Chat, gotgbot.User) {
		t.Helper()
		client := newModuleBotClient()
		client.responses["getChat"] = []byte(`{"id":5151,"type":"private","first_name":"Applicant"}`)
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
		admin := gotgbot.User{Id: 42, FirstName: "Invite Admin"}
		seedCallbackAdmins(t, chat.Id,
			gotgbot.MergedChatMember{
				Status: "administrator", User: gotgbot.User{Id: 999, IsBot: true},
				CanRestrictMembers: true, CanInviteUsers: true,
			},
			gotgbot.MergedChatMember{
				Status: "administrator", User: admin,
				CanRestrictMembers: true, CanInviteUsers: true,
			},
		)
		return client, bot, chat, admin
	}

	t.Run("ban succeeds", func(t *testing.T) {
		client, bot, chat, admin := newJoinBanFixture(t)
		data := encodeCallbackData("join_request", map[string]string{"a": "ban", "u": "5151"})
		if err := greetingsModule.joinRequestHandler(
			bot, newModuleCallbackContext(bot, chat, admin, data),
		); err != ext.EndGroups {
			t.Fatalf("joinRequestHandler(ban) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("banChatMember"); len(calls) != 1 {
			t.Fatalf("banChatMember calls = %d, want the ban action", len(calls))
		}

		got := singleModerationEvent(t, chat.Id)
		if got.ChatId != chat.Id || got.ActorId != admin.Id || got.TargetUserId != 5151 {
			t.Fatalf("join-request ban event = %+v, want chat %d, actor %d, target 5151",
				got, chat.Id, admin.Id)
		}
		if got.Action != models.ModerationActionBan {
			t.Fatalf("join-request ban event action = %q, want %q", got.Action, models.ModerationActionBan)
		}
	})

	t.Run("ban fails", func(t *testing.T) {
		client, bot, chat, admin := newJoinBanFixture(t)
		requestErr := errors.New("Forbidden: bot is not an administrator")
		client.errors["banChatMember"] = requestErr
		data := encodeCallbackData("join_request", map[string]string{"a": "ban", "u": "5151"})
		if err := greetingsModule.joinRequestHandler(
			bot, newModuleCallbackContext(bot, chat, admin, data),
		); !errors.Is(err, requestErr) {
			t.Fatalf("joinRequestHandler(ban) error = %v, want the request error", err)
		}
		if rows := chatModerationEvents(t, chat.Id); len(rows) != 0 {
			t.Fatalf("failed join-request ban recorded %d events, want none", len(rows))
		}
	})
}

// TestSelfDestructModerationRecordsTheEventWhenDeleteFails covers L3: the ban
// or kick stands when deleting the command message fails, and the evidence row
// must survive that failure.
func TestSelfDestructModerationRecordsTheEventWhenDeleteFails(t *testing.T) {
	for _, tt := range []struct {
		name   string
		text   string
		method string
		event  string
		run    func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "sban", text: "/sban", method: "banChatMember", event: models.ModerationActionBan, run: bansModule.sBan},
		{name: "skick", text: "/skick", method: "unbanChatMember", event: models.ModerationActionKick, run: bansModule.sKick},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			deleteErr := errors.New("Bad Request: message to delete not found")
			f.client.errors["deleteMessage"] = deleteErr
			ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, tt.text)

			if err := tt.run(f.bot, ctx); !errors.Is(err, deleteErr) {
				t.Fatalf("%s error = %v, want the delete error", tt.name, err)
			}
			if calls := f.client.callsFor(tt.method); len(calls) != 1 {
				t.Fatalf("%s calls = %d, want the action to have run", tt.method, len(calls))
			}

			got := singleModerationEvent(t, f.chat.Id)
			if got.ChatId != f.chat.Id || got.ActorId != f.admin.Id || got.TargetUserId != f.member.Id {
				t.Fatalf("%s event = %+v, want chat %d, actor %d, target %d",
					tt.name, got, f.chat.Id, f.admin.Id, f.member.Id)
			}
			if got.Action != tt.event {
				t.Fatalf("%s event action = %q, want %q", tt.name, got.Action, tt.event)
			}
		})
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
