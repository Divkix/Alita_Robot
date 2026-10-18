package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// A join arrives as both a service message and a ChatMemberUpdated, in either
// order. Fed-ban and antiraid ban the newcomer from the service message, so
// neither path may welcome someone those guards ban on join.
func TestJoinGuardsKeepBannedNewcomerFromBeingWelcomed(t *testing.T) {
	for _, guard := range []struct {
		name  string
		setup func(t *testing.T, chat gotgbot.Chat, member gotgbot.User)
	}{
		// Control: without a guard the same join is welcomed.
		{name: "none", setup: func(*testing.T, gotgbot.Chat, gotgbot.User) {}},
		{name: "fed ban", setup: func(t *testing.T, chat gotgbot.Chat, member gotgbot.User) {
			ownerID := uniquePositiveUserID()
			fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
			require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
			_, _, err = federations.Fban(fed.FedID, member.Id, ownerID, "spam")
			require.NoError(t, err)
		}},
		{name: "antiraid", setup: func(t *testing.T, chat gotgbot.Chat, _ gotgbot.User) {
			enabled, err := antiRaidModule.enableRaid(chat.Id, 3600)
			require.NoError(t, err)
			require.True(t, enabled)
			t.Cleanup(func() {
				StopAntiRaidExpiryPoller()
				_, _ = antiRaidModule.disableRaid(chat.Id)
			})
		}},
	} {
		for _, path := range []string{"service message", "chat member update"} {
			t.Run(guard.name+"/"+path, func(t *testing.T) {
				client := newModuleBotClient()
				bot := newModuleTestBot(client)
				chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
				member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
				require.NoError(t, greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT))
				guard.setup(t, chat, member)
				clearRecentJoinProcessing(chat.Id, member.Id)

				dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				LoadFederations(dispatcher)
				LoadAntiRaid(dispatcher)
				LoadGreetings(dispatcher)

				update := &gotgbot.Update{UpdateId: 1}
				if path == "service message" {
					update.Message = &gotgbot.Message{
						MessageId:      11,
						Date:           1,
						Chat:           chat,
						From:           &member,
						NewChatMembers: []gotgbot.User{member},
					}
				} else {
					update.ChatMember = &gotgbot.ChatMemberUpdated{
						Chat:          chat,
						From:          member,
						Date:          1,
						OldChatMember: gotgbot.ChatMemberLeft{User: member},
						NewChatMember: gotgbot.ChatMemberMember{User: member},
					}
				}
				require.NoError(t, dispatcher.ProcessUpdate(bot, update, nil))

				guarded := guard.name != "none"
				wantBans := 0
				if guarded && path == "service message" {
					wantBans = 1
				}
				if calls := client.callsFor("banChatMember"); len(calls) != wantBans {
					t.Fatalf("banChatMember calls = %d, want %d", len(calls), wantBans)
				}
				welcomed := false
				for _, call := range client.callsFor("sendMessage") {
					if strings.Contains(fmt.Sprint(call.Params["text"]), "Welcome") {
						welcomed = true
					}
				}
				if welcomed == guarded {
					t.Fatalf("welcomed = %v, want %v with the %s guard", welcomed, !guarded, guard.name)
				}
			})
		}
	}
}

// A newcomer the join guards would not actually ban is welcomed on either
// path: the greeting predicate must agree with what fed-ban and antiraid do.
func TestJoinGuardsWelcomeNewcomerTheyDoNotBan(t *testing.T) {
	enableRaid := func(t *testing.T, chat gotgbot.Chat) {
		enabled, err := antiRaidModule.enableRaid(chat.Id, 3600)
		require.NoError(t, err)
		require.True(t, enabled)
		t.Cleanup(func() {
			StopAntiRaidExpiryPoller()
			_, _ = antiRaidModule.disableRaid(chat.Id)
		})
	}
	fedBan := func(t *testing.T, chat gotgbot.Chat, member gotgbot.User) {
		ownerID := uniquePositiveUserID()
		fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
		require.NoError(t, err)
		t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
		require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
		_, _, err = federations.Fban(fed.FedID, member.Id, ownerID, "spam")
		require.NoError(t, err)
	}
	makeAdmin := func(client *moduleBotClient, chat gotgbot.Chat, member gotgbot.User) {
		client.adminsByChat = map[string]json.RawMessage{fmt.Sprint(chat.Id): adminListJSON(member.Id)}
	}
	botCannotRestrict := func(t *testing.T, client *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat) {
		client.adminsByChat = map[string]json.RawMessage{fmt.Sprint(chat.Id): json.RawMessage(
			`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_restrict_members":false,"can_delete_messages":true}]`,
		)}
		cache.LoadAdminCache(bot, chat.Id)
		t.Cleanup(func() { cache.InvalidateAdminCache(chat.Id) })
	}

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, client *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat, member gotgbot.User)
		// guardTries is whether the guard still calls banChatMember from the
		// service message (Telegram refuses a ban the bot has no right to make).
		guardTries bool
	}{
		{name: "raid, approved newcomer", setup: func(t *testing.T, _ *moduleBotClient, _ *gotgbot.Bot, chat gotgbot.Chat, member gotgbot.User) {
			enableRaid(t, chat)
			require.NoError(t, approvals.AddApprovedUser(chat.Id, member.Id, uniquePositiveUserID(), ""))
		}},
		{name: "raid, admin newcomer", setup: func(t *testing.T, client *moduleBotClient, _ *gotgbot.Bot, chat gotgbot.Chat, member gotgbot.User) {
			enableRaid(t, chat)
			makeAdmin(client, chat, member)
		}},
		{name: "raid, bot cannot restrict", setup: func(t *testing.T, client *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat, _ gotgbot.User) {
			enableRaid(t, chat)
			botCannotRestrict(t, client, bot, chat)
		}},
		{name: "raid, invalid action time", setup: func(t *testing.T, _ *moduleBotClient, _ *gotgbot.Bot, chat gotgbot.Chat, _ gotgbot.User) {
			require.NoError(t, antiraid.SetRaidActionTime(chat.Id, 10))
			enableRaid(t, chat)
		}},
		{name: "fed ban, admin newcomer", setup: func(t *testing.T, client *moduleBotClient, _ *gotgbot.Bot, chat gotgbot.Chat, member gotgbot.User) {
			fedBan(t, chat, member)
			makeAdmin(client, chat, member)
		}},
		{name: "fed ban, bot cannot restrict", guardTries: true, setup: func(t *testing.T, client *moduleBotClient, bot *gotgbot.Bot, chat gotgbot.Chat, member gotgbot.User) {
			fedBan(t, chat, member)
			botCannotRestrict(t, client, bot, chat)
		}},
	} {
		for _, path := range []string{"service message", "chat member update"} {
			t.Run(tc.name+"/"+path, func(t *testing.T) {
				client := newModuleBotClient()
				bot := newModuleTestBot(client)
				chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
				member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
				t.Cleanup(func() { cache.InvalidateAdminCache(chat.Id) })
				require.NoError(t, greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT))
				tc.setup(t, client, bot, chat, member)
				clearRecentJoinProcessing(chat.Id, member.Id)

				dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				LoadFederations(dispatcher)
				LoadAntiRaid(dispatcher)
				LoadGreetings(dispatcher)

				update := &gotgbot.Update{UpdateId: 1}
				if path == "service message" {
					update.Message = &gotgbot.Message{MessageId: 11, Date: 1, Chat: chat, From: &member, NewChatMembers: []gotgbot.User{member}}
				} else {
					update.ChatMember = &gotgbot.ChatMemberUpdated{
						Chat:          chat,
						From:          member,
						Date:          1,
						OldChatMember: gotgbot.ChatMemberLeft{User: member},
						NewChatMember: gotgbot.ChatMemberMember{User: member},
					}
				}
				require.NoError(t, dispatcher.ProcessUpdate(bot, update, nil))

				wantBans := 0
				if tc.guardTries && path == "service message" {
					wantBans = 1
				}
				if calls := client.callsFor("banChatMember"); len(calls) != wantBans {
					t.Fatalf("banChatMember calls = %d, want %d", len(calls), wantBans)
				}
				welcomed := false
				for _, call := range client.callsFor("sendMessage") {
					if strings.Contains(fmt.Sprint(call.Params["text"]), "Welcome") {
						welcomed = true
					}
				}
				if !welcomed {
					t.Fatalf("%s: newcomer was not welcomed", tc.name)
				}
			})
		}
	}
}
