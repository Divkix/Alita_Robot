package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
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
