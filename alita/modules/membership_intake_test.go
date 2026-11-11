//go:build testtools

package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/probation"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

const (
	viaMessage = "service message"
	viaUpdate  = "chat member update"
)

// joinWays are the ways Telegram reports one join: either update alone, or
// both in either order.
var joinWays = [][]string{{viaMessage}, {viaUpdate}, {viaMessage, viaUpdate}, {viaUpdate, viaMessage}}

// intakeJoin is a member joining a chat, added by adder (themselves when they
// joined on their own).
type intakeJoin struct {
	client *moduleBotClient
	bot    *gotgbot.Bot
	chat   gotgbot.Chat
	member gotgbot.User
	adder  gotgbot.User
}

func newIntakeJoin(t *testing.T, memberIsBot bool) intakeJoin {
	t.Helper()
	client := newModuleBotClient()
	j := intakeJoin{
		client: client,
		bot:    newModuleTestBot(client),
		chat:   gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Intake Chat"},
		member: gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer", IsBot: memberIsBot},
	}
	j.adder = j.member
	if memberIsBot {
		j.adder = gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Adder"}
	}
	t.Cleanup(func() { cache.InvalidateAdminCache(j.chat.Id) })
	require.NoError(t, greetings.SetWelcomeText(j.chat.Id, "Welcome {first}", "", nil, db.TEXT))
	clearJoinClaim(j.chat.Id, j.member.Id)
	return j
}

func (j intakeJoin) updates() map[string]*gotgbot.Update {
	return map[string]*gotgbot.Update{
		viaMessage: {UpdateId: 1, Message: &gotgbot.Message{
			MessageId: 11, Date: 1, Chat: j.chat, From: &j.adder, NewChatMembers: []gotgbot.User{j.member},
		}},
		viaUpdate: {UpdateId: 2, ChatMember: &gotgbot.ChatMemberUpdated{
			Chat:          j.chat,
			From:          j.adder,
			Date:          1,
			OldChatMember: gotgbot.ChatMemberLeft{User: j.member},
			NewChatMember: gotgbot.ChatMemberMember{User: j.member},
		}},
	}
}

func (j intakeJoin) deliver(t *testing.T, way []string) {
	t.Helper()
	updates := j.updates()
	for _, via := range way {
		dispatchJoinGuards(t, j.bot, updates[via])
	}
}

func (j intakeJoin) leave(t *testing.T, newStatus gotgbot.ChatMember, from gotgbot.User) {
	t.Helper()
	dispatchJoinGuards(t, j.bot, &gotgbot.Update{UpdateId: 3, ChatMember: &gotgbot.ChatMemberUpdated{
		Chat:          j.chat,
		From:          from,
		Date:          2,
		OldChatMember: gotgbot.ChatMemberMember{User: j.member},
		NewChatMember: newStatus,
	}})
}

func (j intakeJoin) welcomes() int {
	n := 0
	for _, call := range j.client.callsFor("sendMessage") {
		if strings.Contains(fmt.Sprint(call.Params["text"]), "Welcome") {
			n++
		}
	}
	return n
}

func (j intakeJoin) sent(text string) int {
	n := 0
	for _, call := range j.client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["text"]) == text {
			n++
		}
	}
	return n
}

// challenges counts captcha mutes: a challenge restricts the newcomer once.
func (j intakeJoin) challenges(t *testing.T) int {
	t.Helper()
	if !captchaSentTo(t, j.chat.Id, j.member.Id) {
		return 0
	}
	return len(j.client.callsFor("restrictChatMember"))
}

func (j intakeJoin) bans() int {
	n := 0
	for _, call := range j.client.callsFor("banChatMember") {
		if fmt.Sprint(call.Params["user_id"]) == fmt.Sprint(j.member.Id) {
			n++
		}
	}
	return n
}

func localeText(t *testing.T, key string) string {
	t.Helper()
	text, err := i18n.MustNewTranslator("en").GetString(key)
	require.NoError(t, err)
	return text
}

func approve(t *testing.T, chatID, userID int64) {
	t.Helper()
	require.NoError(t, approvals.AddApprovedUser(chatID, userID, uniquePositiveUserID(), ""))
	t.Cleanup(func() { _ = approvals.RemoveApprovedUser(chatID, userID) })
}

func lockBots(t *testing.T, chatID int64) {
	t.Helper()
	require.NoError(t, locks.UpdateLock(chatID, "bots", true))
}

func TestMembershipIntakeGivesEveryJoinOneVerdict(t *testing.T) {
	useRealLocales(t)
	type want struct{ challenges, welcomes, bans int }
	for _, tc := range []struct {
		name  string
		bot   bool
		setup func(t *testing.T, j intakeJoin)
		want  want
		check func(t *testing.T, j intakeJoin)
	}{
		{name: "newcomer, captcha on", want: want{challenges: 1}, setup: func(t *testing.T, j intakeJoin) {
			require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, true))
		}},
		{name: "newcomer, captcha off", want: want{welcomes: 1}},
		{name: "approved newcomer, captcha on", want: want{welcomes: 1}, setup: func(t *testing.T, j intakeJoin) {
			require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, true))
			approve(t, j.chat.Id, j.member.Id)
		}},
		{name: "approved newcomer during a raid, captcha on", want: want{welcomes: 1}, setup: func(t *testing.T, j intakeJoin) {
			require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, true))
			approve(t, j.chat.Id, j.member.Id)
			enabled, err := antiRaidModule.enableRaid(j.chat.Id, 3600)
			require.NoError(t, err)
			require.True(t, enabled)
			t.Cleanup(func() { _, _ = antiRaidModule.disableRaid(j.chat.Id) })
		}},
		{name: "admin newcomer, fbanned during a raid", want: want{welcomes: 1}, setup: func(t *testing.T, j intakeJoin) {
			j.client.adminsByChat = map[string]json.RawMessage{fmt.Sprint(j.chat.Id): adminListJSON(j.member.Id)}
			ownerID := uniquePositiveUserID()
			fed, err := federations.CreateFederation(ownerID, "Intake Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
			require.NoError(t, federations.JoinFed(j.chat.Id, j.chat.Title, fed.FedID))
			_, _, err = federations.Fban(fed.FedID, j.member.Id, ownerID, "spam")
			require.NoError(t, err)
			enabled, err := antiRaidModule.enableRaid(j.chat.Id, 3600)
			require.NoError(t, err)
			require.True(t, enabled)
			t.Cleanup(func() { _, _ = antiRaidModule.disableRaid(j.chat.Id) })
		}},
		{name: "bot added by a member, bots locked", bot: true, want: want{bans: 1}, setup: func(t *testing.T, j intakeJoin) {
			require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, true))
			lockBots(t, j.chat.Id)
		}, check: func(t *testing.T, j intakeJoin) {
			require.Equal(t, 1, j.sent(localeText(t, "locks_bot_only_admins")), "one bots-lock notice per join")
		}},
		{name: "bot added by an admin, bots locked", bot: true, want: want{welcomes: 1}, setup: func(t *testing.T, j intakeJoin) {
			lockBots(t, j.chat.Id)
			j.client.adminsByChat = map[string]json.RawMessage{fmt.Sprint(j.chat.Id): adminListJSON(j.adder.Id)}
		}},
		{name: "bot added by an approved user, bots locked", bot: true, want: want{welcomes: 1}, setup: func(t *testing.T, j intakeJoin) {
			lockBots(t, j.chat.Id)
			approve(t, j.chat.Id, j.adder.Id)
		}},
		{name: "bots locked, bot not an admin", bot: true, want: want{welcomes: 1}, setup: func(t *testing.T, j intakeJoin) {
			require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, true))
			lockBots(t, j.chat.Id)
			j.client.adminsByChat = map[string]json.RawMessage{fmt.Sprint(j.chat.Id): json.RawMessage(`[]`)}
			j.client.membersByChat = map[string]json.RawMessage{fmt.Sprint(j.chat.Id) + "|999": json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)}
		}, check: func(t *testing.T, j intakeJoin) {
			require.Equal(t, 1, j.sent(localeText(t, "locks_bot_lock_no_permission")), "one missing-admin notice per join")
		}},
		{name: "bots locked, bot cannot restrict", bot: true, want: want{welcomes: 1}, setup: func(t *testing.T, j intakeJoin) {
			lockBots(t, j.chat.Id)
			j.client.responses["getChatAdministrators"] = json.RawMessage(
				`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true,"can_restrict_members":false}]`,
			)
		}, check: func(t *testing.T, j intakeJoin) {
			require.Equal(t, 1, j.sent(localeText(t, "locks_bot_lock_no_ban_permission")), "one missing-right notice per join")
		}},
		{name: "bots locked, ban fails", bot: true, want: want{welcomes: 1, bans: 1}, setup: func(t *testing.T, j intakeJoin) {
			lockBots(t, j.chat.Id)
			j.client.errors["banChatMember"] = errors.New("Bad Request: not enough rights to restrict/unrestrict chat member")
		}, check: func(t *testing.T, j intakeJoin) {
			require.Zero(t, j.sent(localeText(t, "locks_bot_only_admins")), "a failed ban posts no bots-lock notice")
		}},
		{name: "Redis failing, newcomer", want: want{}, setup: func(t *testing.T, j intakeJoin) {
			require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, true))
			withUnreachableRedis(t)
		}},
		{name: "Redis failing, bot added by a member, bots locked", bot: true, want: want{bans: 1}, setup: func(t *testing.T, j intakeJoin) {
			lockBots(t, j.chat.Id)
			withUnreachableRedis(t)
		}, check: func(t *testing.T, j intakeJoin) {
			require.Equal(t, 1, j.sent(localeText(t, "locks_bot_only_admins")), "one bots-lock notice per join")
		}},
		{name: "newcomer, probation on", want: want{welcomes: 1}, setup: func(t *testing.T, j intakeJoin) {
			require.NoError(t, probation.SetEnabled(j.chat.Id, true))
		}, check: func(t *testing.T, j intakeJoin) {
			require.True(t, onProbation(t, j.chat.Id, j.member.Id), "a newcomer starts probation on join")
		}},
	} {
		for _, way := range joinWays {
			t.Run(tc.name+"/"+strings.Join(way, " then "), func(t *testing.T) {
				j := newIntakeJoin(t, tc.bot)
				if tc.setup != nil {
					tc.setup(t, j)
				}

				j.deliver(t, way)

				got := want{challenges: j.challenges(t), welcomes: j.welcomes(), bans: j.bans()}
				require.Equal(t, tc.want, got)
				if tc.check != nil {
					tc.check(t, j)
				}
			})
		}
	}
}

// A member who leaves and joins again is a new join.
func TestMembershipIntakeProcessesARejoin(t *testing.T) {
	for _, way := range joinWays {
		t.Run(strings.Join(way, " then "), func(t *testing.T) {
			j := newIntakeJoin(t, false)

			j.deliver(t, way)
			j.leave(t, gotgbot.ChatMemberLeft{User: j.member}, j.member)
			j.deliver(t, way)

			require.Equal(t, 2, j.welcomes())
		})
	}
}

// Banning a joining bot makes Telegram report a leave (member→kicked). It
// keeps the join claim, so the join's other update does nothing.
func TestMembershipIntakeKeepsTheClaimOnABanLeave(t *testing.T) {
	useRealLocales(t)
	for _, way := range joinWays {
		t.Run(strings.Join(way, " then "), func(t *testing.T) {
			j := newIntakeJoin(t, true)
			lockBots(t, j.chat.Id)
			updates := j.updates()
			rest := way[1:]
			if len(way) == 1 {
				other := viaUpdate
				if way[0] == viaUpdate {
					other = viaMessage
				}
				rest = []string{other}
			}

			dispatchJoinGuards(t, j.bot, updates[way[0]])
			j.leave(t, gotgbot.ChatMemberBanned{User: j.member}, j.bot.User)
			j.deliver(t, rest)

			require.Equal(t, 1, j.bans())
			require.Equal(t, 1, j.sent(localeText(t, "locks_bot_only_admins")))
			require.Zero(t, j.welcomes())
		})
	}
}
