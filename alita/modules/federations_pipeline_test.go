//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/federations"
)

func dispatchFederationCommand(t *testing.T, bot *gotgbot.Bot, ctx *ext.Context) error {
	t.Helper()
	withDisableCommands(t)
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadFederations(d)
	return d.ProcessUpdate(bot, ctx.Update, nil)
}

func TestFederationCommandsKeepCallerRolesAndRefusalOrder(t *testing.T) {
	useRealLocales(t)
	for _, role := range []string{"member", "chat-admin", "chat-owner", "fed-admin", "fed-owner"} {
		for _, botAdmin := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/bot-admin=%t", role, botAdmin), func(t *testing.T) {
				f := newAdminChatFixture(t)
				if role == "chat-owner" {
					f.makeOwner()
				}
				actor := f.member
				if strings.HasPrefix(role, "chat-") {
					actor = f.admin
				}
				ownerID := uniquePositiveUserID()
				if role == "fed-owner" {
					ownerID = actor.Id
				}
				fed, err := federations.CreateFederation(ownerID, "Role Fed")
				require.NoError(t, err)
				t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
				require.NoError(t, federations.JoinFed(f.chat.Id, f.chat.Title, fed.FedID))
				if role == "fed-admin" {
					require.NoError(t, federations.PromoteFedAdmin(fed.FedID, actor.Id))
				}
				if !botAdmin {
					f.client.membersByUser["999"] = json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
					f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + string(f.client.membersByUser[fmt.Sprint(f.admin.Id)]) + "]")
				}
				run := func(text string) string {
					t.Helper()
					before := len(f.client.callsFor("sendMessage"))
					require.NoError(t, dispatchFederationCommand(t, f.bot, newModuleMessageContext(f.bot, f.chat, actor, text)))
					calls := f.client.callsFor("sendMessage")[before:]
					require.NotEmpty(t, calls)
					return fmt.Sprint(calls[0].Params["text"])
				}
				wantJoin := "Only the group creator can join or leave a federation."
				if role == "chat-owner" {
					wantJoin = "That doesn't look like a federation ID."
				}
				require.Equal(t, wantJoin, run("/joinfed invalid"))
				wantQuiet := "Only admins can execute this command!"
				if strings.HasPrefix(role, "chat-") {
					wantQuiet = "Use on or off."
				}
				require.Equal(t, wantQuiet, run("/quietfed invalid"))
				wantBan := "You are not a federation admin here."
				if strings.HasPrefix(role, "fed-") {
					wantBan = "I don't know who you're talking about, you're going to need to specify a user...!"
				}
				require.Equal(t, wantBan, run("/fban"))
				wantOwner := "You don't own a federation. Create one with /newfed."
				if role == "fed-owner" {
					wantOwner = "Use on or off."
				}
				require.Equal(t, wantOwner, run("/fedreason invalid"))
				require.Contains(t, run("/fedinfo"), "Role Fed")
				require.Contains(t, run("/fedadmins"), "Role Fed")
				require.Contains(t, run("/chatfed"), "Role Fed")
			})
		}
	}
}

func TestFederationCommandsKeepPrivateTargetsAndValidationOrder(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	fed, err := federations.CreateFederation(f.member.Id, "Private Owned Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	other, err := federations.CreateFederation(uniquePositiveUserID(), "Connected Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(other.FedID) })
	require.NoError(t, federations.JoinFed(f.chat.Id, f.chat.Title, other.FedID))
	require.NoError(t, connections.ConnectId(f.member.Id, f.chat.Id))
	t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })
	for _, tc := range []struct {
		command string
		want    string
	}{
		{"/joinfed invalid", "This command can only be used in groups."},
		{"/leavefed", "This command can only be used in groups."},
		{"/quietfed invalid", "This command can only be used in groups."},
		{"/chatfed", "This command can only be used in groups."},
		{"/setfedlog", "This command can only be used in groups."},
		{"/fban 45678 spam", "Private Owned Fed"},
		{"/funban 45678", "Private Owned Fed"},
		{"/fedinfo", "Private Owned Fed"},
		{"/fedadmins", "Private Owned Fed"},
		{"/fedinfo " + fmt.Sprint(f.chat.Id), "That doesn't look like a federation ID."},
	} {
		t.Run(tc.command, func(t *testing.T) {
			before := len(f.client.callsFor("sendMessage"))
			require.NoError(t, dispatchFederationCommand(t, f.bot, newModuleMessageContext(f.bot, privateChatOf(f.member), f.member, tc.command)))
			calls := f.client.callsFor("sendMessage")[before:]
			require.NotEmpty(t, calls)
			require.Contains(t, fmt.Sprint(calls[0].Params["text"]), tc.want)
			assertParams(t, calls[0], map[string]any{"chat_id": f.member.Id})
		})
	}
	for _, tc := range []struct{ command, want string }{
		{"/renamefed", "Give your federation a name."},
		{"/feddemoteme invalid", "That doesn't look like a federation ID."},
		{"/newfed", "This command can only be used in private chat with me."},
		{"/delfed", "This command can only be used in private chat with me."},
	} {
		before := len(f.client.callsFor("sendMessage"))
		require.NoError(t, dispatchFederationCommand(t, f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, tc.command)))
		calls := f.client.callsFor("sendMessage")[before:]
		require.Len(t, calls, 1)
		require.Equal(t, tc.want, calls[0].Params["text"])
	}
}

func TestFederationStatisticsAliasesShareDisableSetting(t *testing.T) {
	useRealLocales(t)
	for _, stored := range []string{"fedstat", "fbanstat"} {
		t.Run(stored, func(t *testing.T) {
			f := newAdminChatFixture(t)
			withDisableCommands(t)
			d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			LoadFederations(d)
			LoadDisabling(d)
			run := func(actor gotgbot.User, text string) []moduleBotCall {
				t.Helper()
				before := len(f.client.callsFor("sendMessage"))
				require.NoError(t, d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, actor, text).Update, nil))
				return f.client.callsFor("sendMessage")[before:]
			}
			list := run(f.member, "/disableable")
			require.Len(t, list, 1)
			require.Contains(t, fmt.Sprint(list[0].Params["text"]), "fbanstat")
			for _, typed := range []string{"fedstat", "fbanstat"} {
				require.Len(t, run(f.member, "/"+typed), 1)
			}
			run(f.admin, "/disable "+stored)
			require.True(t, disabling.IsCommandDisabled(f.chat.Id, stored), "/disable must accept every alias")
			list = run(f.admin, "/disabled")
			require.Len(t, list, 1)
			require.Contains(t, fmt.Sprint(list[0].Params["text"]), stored)
			require.NoError(t, disabling.ToggleDel(f.chat.Id, true))
			for _, typed := range []string{"fedstat", "fbanstat"} {
				before := len(f.client.callsFor("deleteMessage"))
				require.Empty(t, run(f.member, "/"+typed), "one stored spelling must silence every alias")
				require.Len(t, f.client.callsFor("deleteMessage"), before+1)
				require.Len(t, run(f.admin, "/"+typed), 1, "admins must bypass every alias row")
				require.Len(t, f.client.callsFor("deleteMessage"), before+1)
			}
			run(f.admin, "/enable "+stored)
			require.False(t, disabling.IsCommandDisabled(f.chat.Id, stored))
			for _, typed := range []string{"fedstat", "fbanstat"} {
				require.Len(t, run(f.member, "/"+typed), 1)
			}
		})
	}
}
