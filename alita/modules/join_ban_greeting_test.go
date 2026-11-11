package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
)

// A newcomer the join guards do not ban is welcomed on either path.
func TestJoinGuardsWelcomeNewcomerTheyDoNotBan(t *testing.T) {
	enableRaid := func(t *testing.T, chat gotgbot.Chat) {
		enabled, err := antiRaidModule.enableRaid(chat.Id, 3600)
		require.NoError(t, err)
		require.True(t, enabled)
		t.Cleanup(func() {
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
		// guardTries is whether the guard still calls banChatMember; Telegram
		// refuses a ban the bot has no right to make.
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
				if tc.guardTries {
					client.errors["banChatMember"] = &gotgbot.TelegramError{
						Method:      "banChatMember",
						Code:        400,
						Description: "Bad Request: not enough rights to restrict/unrestrict chat member",
					}
				}
				bot := newModuleTestBot(client)
				chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
				member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
				t.Cleanup(func() { cache.InvalidateAdminCache(chat.Id) })
				require.NoError(t, greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT))
				tc.setup(t, client, bot, chat, member)
				clearJoinClaim(chat.Id, member.Id)

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
				if tc.guardTries {
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

// dispatchJoinGuards runs update through the federation watcher and
// membership intake, whose stages are the join guards, as the bot registers
// them.
func dispatchJoinGuards(t *testing.T, bot *gotgbot.Bot, update *gotgbot.Update) {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadFederations(dispatcher)
	LoadAntiRaid(dispatcher)
	LoadGreetings(dispatcher)
	require.NoError(t, dispatcher.ProcessUpdate(bot, update, nil))
}

func captchaSentTo(t *testing.T, chatID, userID int64) bool {
	t.Helper()
	attempt, err := captcha.GetCaptchaAttempt(userID, chatID)
	require.NoError(t, err)
	return attempt != nil
}

func welcomeSent(client *moduleBotClient) bool {
	for _, call := range client.callsFor("sendMessage") {
		if strings.Contains(fmt.Sprint(call.Params["text"]), "Welcome") {
			return true
		}
	}
	return false
}

// A, B and C join together, and C's join crosses the auto-antiraid threshold.
// Antiraid bans only C, so A and B, whom nothing banned, must still get the
// captcha, once each, however Telegram reports the joins; the raid switching
// on is announced once.
func TestRaidSwitchedOnByJoinStillChallengesNewcomersItDidNotBan(t *testing.T) {
	for _, way := range joinWays {
		t.Run(strings.Join(way, " then "), func(t *testing.T) {
			useRealLocales(t)
			withMiniredis(t)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
			require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, chat.Title, true))
			require.NoError(t, antiraid.SetAutoAntiRaidThreshold(chat.Id, 3))
			t.Cleanup(func() {
				_, _ = antiRaidModule.disableRaid(chat.Id)
			})
			members := []gotgbot.User{
				{Id: uniquePositiveUserID(), FirstName: "A"},
				{Id: uniquePositiveUserID(), FirstName: "B"},
				{Id: uniquePositiveUserID(), FirstName: "C"},
			}
			for _, member := range members {
				clearJoinClaim(chat.Id, member.Id)
			}

			for _, via := range way {
				if via == viaMessage {
					dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
						MessageId: 11, Date: 1, Chat: chat, From: &members[0], NewChatMembers: members,
					}})
					continue
				}
				for i, member := range members {
					dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: int64(2 + i), ChatMember: &gotgbot.ChatMemberUpdated{
						Chat:          chat,
						From:          member,
						Date:          1,
						OldChatMember: gotgbot.ChatMemberLeft{User: member},
						NewChatMember: gotgbot.ChatMemberMember{User: member},
					}})
				}
			}

			require.True(t, antiRaidModule.isRaidActive(chat.Id), "the third join must switch the raid on")
			bans := client.callsFor("banChatMember")
			require.Len(t, bans, 1)
			require.Equal(t, fmt.Sprint(members[2].Id), fmt.Sprint(bans[0].Params["user_id"]))
			restricts := map[string]int{}
			for _, call := range client.callsFor("restrictChatMember") {
				restricts[fmt.Sprint(call.Params["user_id"])]++
			}
			for _, member := range members[:2] {
				require.True(t, captchaSentTo(t, chat.Id, member.Id), "%s joined before the raid and was not banned", member.FirstName)
				require.Equal(t, 1, restricts[fmt.Sprint(member.Id)], "%s is challenged once", member.FirstName)
			}
			require.False(t, captchaSentTo(t, chat.Id, members[2].Id), "C was banned on join")
			notice, err := i18n.MustNewTranslator("en").GetString("antiraid_auto_triggered", i18n.TranslationParams{"count": "3"})
			require.NoError(t, err)
			require.Equal(t, 1, intakeJoin{client: client}.sent(notice), "one notice when the raid switches on")
		})
	}
}

// With a raid on, antiraid bans a newcomer for the raid action time however
// Telegram reports the join, and neither challenge nor welcome follows. A
// member it removed is not looked up in the spam databases, and a ban that
// failed leaves the member to the next stage.
func TestAntiRaidStageGivesEveryJoinWayOneVerdict(t *testing.T) {
	type want struct{ bans, challenges, welcomes, logs, notices, lookups int }
	for _, tc := range []struct {
		name    string
		captcha bool
		// auto: the join crosses a threshold of one instead of meeting a raid.
		auto   bool
		banErr bool
		want   want
	}{
		{name: "raid on", want: want{bans: 1}},
		{name: "raid on, captcha on", captcha: true, want: want{bans: 1}},
		{name: "raid switched on by the join", auto: true, want: want{bans: 1, logs: 1, notices: 1}},
		// Antiraid's ban fails, so the spam database check bans (and fails) too.
		{name: "raid ban failed", captcha: true, banErr: true, want: want{bans: 2, challenges: 1, logs: 1, lookups: 1}},
	} {
		for _, way := range joinWays {
			t.Run(tc.name+"/"+strings.Join(way, " then "), func(t *testing.T) {
				useRealLocales(t)
				_, cas := withSpamDBCAS(t, true, http.StatusOK, spamDBCASListed)
				withMiniredis(t) // after the spam database's memory cache: raid counting needs Redis
				j := newIntakeJoin(t, false)
				require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, tc.captcha))
				require.NoError(t, logchannels.Set(j.chat.Id, j.chat.Title, spamDBLogChannel))
				require.NoError(t, spamdbsettings.SetSpamDBEnabled(j.chat.Id, true))
				require.NoError(t, spamdbsettings.SetSpamDBAction(j.chat.Id, spamdbsettings.ActionBan))
				t.Cleanup(func() { _, _ = antiRaidModule.disableRaid(j.chat.Id) })
				if tc.auto {
					require.NoError(t, antiraid.SetAutoAntiRaidThreshold(j.chat.Id, 1))
				} else {
					enabled, err := antiRaidModule.enableRaid(j.chat.Id, 3600)
					require.NoError(t, err)
					require.True(t, enabled)
				}
				if tc.banErr {
					j.client.errors["banChatMember"] = errors.New("Bad Request: not enough rights to restrict/unrestrict chat member")
				}
				notice, err := i18n.MustNewTranslator("en").GetString("antiraid_auto_triggered", i18n.TranslationParams{"count": "1"})
				require.NoError(t, err)

				j.deliver(t, way)

				logs := callsToChat(j.client, "sendMessage", spamDBLogChannel)
				got := want{j.bans(), j.challenges(t), j.welcomes(), len(logs), j.sent(notice), int(cas.hits.Load())}
				require.Equal(t, tc.want, got)
				if !tc.banErr {
					_, temporary := j.client.onlyCall(t, "banChatMember").Params["until_date"]
					require.True(t, temporary, "a raid ban lasts the raid action time")
					require.Equal(t, models.BanSourceAntiraid, singleBanRecord(t, j.chat.Id).Source)
				}
			})
		}
	}
}

// A join Telegram reports only as a member update counts towards the
// auto-antiraid threshold like one with a service message.
func TestAntiRaidCountsJoinsSeenOnlyAsMemberUpdates(t *testing.T) {
	withMiniredis(t)
	first := newIntakeJoin(t, false)
	second := first
	second.member = gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Second"}
	second.adder = second.member
	clearJoinClaim(second.chat.Id, second.member.Id)
	require.NoError(t, captcha.SetCaptchaEnabled(first.chat.Id, first.chat.Title, true))
	require.NoError(t, antiraid.SetAutoAntiRaidThreshold(first.chat.Id, 2))
	t.Cleanup(func() { _, _ = antiRaidModule.disableRaid(first.chat.Id) })

	first.deliver(t, []string{viaUpdate})
	second.deliver(t, []string{viaUpdate})

	require.True(t, antiRaidModule.isRaidActive(first.chat.Id), "the second join must switch the raid on")
	require.Equal(t, 0, first.bans())
	require.True(t, captchaSentTo(t, first.chat.Id, first.member.Id), "the join before the raid is challenged")
	require.Equal(t, 1, second.bans())
	require.False(t, captchaSentTo(t, second.chat.Id, second.member.Id), "the join that switched the raid on is banned")
}

// When banChatMember fails (flood limit, rights changed inside the admin-cache
// TTL), the join guard banned no one, so the newcomer gets the captcha as if
// no guard had matched.
func TestJoinGuardBanThatFailsStillChallengesNewcomer(t *testing.T) {
	for _, guard := range []struct {
		name  string
		setup func(t *testing.T, chat gotgbot.Chat, member gotgbot.User)
	}{
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
				_, _ = antiRaidModule.disableRaid(chat.Id)
			})
		}},
	} {
		t.Run(guard.name, func(t *testing.T) {
			client := newModuleBotClient()
			client.errors["banChatMember"] = &gotgbot.TelegramError{
				Method:      "banChatMember",
				Code:        429,
				Description: "Too Many Requests: retry after 5",
			}
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
			member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
			require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, chat.Title, true))
			guard.setup(t, chat, member)
			clearJoinClaim(chat.Id, member.Id)

			dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
				MessageId: 11, Date: 1, Chat: chat, From: &member, NewChatMembers: []gotgbot.User{member},
			}})

			require.Len(t, client.callsFor("banChatMember"), 1, "the guard tries to ban")
			require.True(t, captchaSentTo(t, chat.Id, member.Id), "a ban that failed must not skip the captcha")
		})
	}
}

// A ban that went through skips both the captcha and the welcome, also on the
// ChatMemberUpdated of the same join that arrives after the service message.
func TestJoinGuardBanThatSucceedsSkipsCaptchaAndWelcome(t *testing.T) {
	for _, guard := range []struct {
		name  string
		setup func(t *testing.T, chat gotgbot.Chat, member gotgbot.User)
	}{
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
				_, _ = antiRaidModule.disableRaid(chat.Id)
			})
		}},
	} {
		for _, captchaOn := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/captcha=%v", guard.name, captchaOn), func(t *testing.T) {
				client := newModuleBotClient()
				bot := newModuleTestBot(client)
				chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
				member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
				require.NoError(t, greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT))
				require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, chat.Title, captchaOn))
				guard.setup(t, chat, member)
				clearJoinClaim(chat.Id, member.Id)

				dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
					MessageId: 11, Date: 1, Chat: chat, From: &member, NewChatMembers: []gotgbot.User{member},
				}})
				dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: 2, ChatMember: &gotgbot.ChatMemberUpdated{
					Chat:          chat,
					From:          member,
					Date:          1,
					OldChatMember: gotgbot.ChatMemberLeft{User: member},
					NewChatMember: gotgbot.ChatMemberMember{User: member},
				}})

				require.Len(t, client.callsFor("banChatMember"), 1)
				require.False(t, captchaSentTo(t, chat.Id, member.Id), "banned newcomer got a captcha")
				require.False(t, welcomeSent(client), "banned newcomer was welcomed")
			})
		}
	}
}

// With Redis down the shared join claim fails closed, but fed-ban must still
// ban a fbanned newcomer on join, once per join on this replica.
func TestFedBanOnJoinStillBansWhenRedisIsDown(t *testing.T) {
	restore, err := i18n.OverrideManagerForTest(`feds_passive_ban: "{user} is fbanned in {name}."`)
	require.NoError(t, err)
	t.Cleanup(restore)

	for _, order := range [][]string{
		{"service message"},
		{"chat member update"},
		{"service message", "chat member update"},
		{"chat member update", "service message"},
	} {
		t.Run(strings.Join(order, " then "), func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
			member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
			ownerID := uniquePositiveUserID()
			fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
			require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
			_, _, err = federations.Fban(fed.FedID, member.Id, ownerID, "spam")
			require.NoError(t, err)
			require.NoError(t, greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT))
			require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, chat.Title, true))
			clearJoinClaim(chat.Id, member.Id)
			withUnreachableRedis(t)

			updates := map[string]*gotgbot.Update{
				"service message": {UpdateId: 1, Message: &gotgbot.Message{
					MessageId: 11, Date: 1, Chat: chat, From: &member, NewChatMembers: []gotgbot.User{member},
				}},
				"chat member update": {UpdateId: 2, ChatMember: &gotgbot.ChatMemberUpdated{
					Chat:          chat,
					From:          member,
					Date:          1,
					OldChatMember: gotgbot.ChatMemberLeft{User: member},
					NewChatMember: gotgbot.ChatMemberMember{User: member},
				}},
			}
			for _, path := range order {
				dispatchJoinGuards(t, bot, updates[path])
			}

			bans := client.callsFor("banChatMember")
			require.Len(t, bans, 1, "a fbanned newcomer must be banned exactly once while Redis is down")
			require.Equal(t, fmt.Sprint(member.Id), fmt.Sprint(bans[0].Params["user_id"]))
			notices := 0
			for _, call := range client.callsFor("sendMessage") {
				if strings.Contains(fmt.Sprint(call.Params["text"]), "is fbanned") {
					notices++
				}
			}
			require.Equal(t, 1, notices, "one fed-ban notice per join")
			require.False(t, captchaSentTo(t, chat.Id, member.Id), "banned newcomer got a captcha")
			require.False(t, welcomeSent(client), "banned newcomer was welcomed")
		})
	}
}

// A join reaches the bot as a service message, a ChatMemberUpdated, or both in
// either order, and large groups may send no service message at all. Fed-ban
// must ban a fbanned newcomer on whichever update comes, exactly once, with a
// single notice, and leave everyone else alone.
func TestFedBanEnforcedOnEveryJoinUpdateExactlyOnce(t *testing.T) {
	restore, err := i18n.OverrideManagerForTest(`feds_passive_ban: "{user} is fbanned in {name}."`)
	require.NoError(t, err)
	t.Cleanup(restore)

	joinUpdates := func(chat gotgbot.Chat, member gotgbot.User) map[string]*gotgbot.Update {
		return map[string]*gotgbot.Update{
			"service message": {UpdateId: 1, Message: &gotgbot.Message{
				MessageId: 11, Date: 1, Chat: chat, From: &member, NewChatMembers: []gotgbot.User{member},
			}},
			"chat member update": {UpdateId: 2, ChatMember: &gotgbot.ChatMemberUpdated{
				Chat:          chat,
				From:          member,
				Date:          1,
				OldChatMember: gotgbot.ChatMemberLeft{User: member},
				NewChatMember: gotgbot.ChatMemberMember{User: member},
			}},
		}
	}
	notices := func(client *moduleBotClient) int {
		n := 0
		for _, call := range client.callsFor("sendMessage") {
			if strings.Contains(fmt.Sprint(call.Params["text"]), "is fbanned") {
				n++
			}
		}
		return n
	}

	for _, redis := range []bool{false, true} {
		for _, fbanned := range []bool{true, false} {
			for _, order := range [][]string{
				{"chat member update"},
				{"chat member update", "service message"},
				{"service message", "chat member update"},
				{"concurrent"},
			} {
				name := fmt.Sprintf("redis=%v/fbanned=%v/%s", redis, fbanned, strings.Join(order, " then "))
				t.Run(name, func(t *testing.T) {
					if redis {
						withMiniredis(t)
					}
					client := newModuleBotClient()
					bot := newModuleTestBot(client)
					chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
					member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
					require.NoError(t, greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT))
					require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, chat.Title, true))
					ownerID := uniquePositiveUserID()
					fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
					require.NoError(t, err)
					t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
					require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
					if fbanned {
						_, _, err = federations.Fban(fed.FedID, member.Id, ownerID, "spam")
						require.NoError(t, err)
					}
					clearJoinClaim(chat.Id, member.Id)
					updates := joinUpdates(chat, member)

					if order[0] == "concurrent" {
						var wg sync.WaitGroup
						for _, update := range updates {
							wg.Add(1)
							go func(update *gotgbot.Update) {
								defer wg.Done()
								dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
								LoadFederations(dispatcher)
								LoadAntiRaid(dispatcher)
								LoadGreetings(dispatcher)
								_ = dispatcher.ProcessUpdate(bot, update, nil)
							}(update)
						}
						wg.Wait()
					} else {
						for _, path := range order {
							dispatchJoinGuards(t, bot, updates[path])
						}
					}

					if !fbanned {
						require.Empty(t, client.callsFor("banChatMember"), "a newcomer nobody fbanned was banned")
						require.Zero(t, notices(client))
						require.True(t, captchaSentTo(t, chat.Id, member.Id), "a newcomer nobody fbanned must get the captcha")
						return
					}
					bans := client.callsFor("banChatMember")
					require.Len(t, bans, 1, "a fbanned newcomer must be banned exactly once")
					require.Equal(t, fmt.Sprint(member.Id), fmt.Sprint(bans[0].Params["user_id"]))
					require.Equal(t, 1, notices(client), "one fed-ban notice per join")
					require.False(t, captchaSentTo(t, chat.Id, member.Id), "banned newcomer got a captcha")
					require.False(t, welcomeSent(client), "banned newcomer was welcomed")
				})
			}
		}
	}
}

// The fed-ban on a join's ChatMemberUpdated makes Telegram send a second one,
// member→kicked, which greetings handles as a leave. The join's service message
// may still come after it; it must not ban again or post a second notice.
func TestFedBanOnJoinPostsOneNoticeWhenServiceMessageFollowsTheKick(t *testing.T) {
	restore, err := i18n.OverrideManagerForTest(`feds_passive_ban: "{user} is fbanned in {name}."`)
	require.NoError(t, err)
	t.Cleanup(restore)

	for _, redis := range []bool{false, true} {
		t.Run(fmt.Sprintf("redis=%v", redis), func(t *testing.T) {
			if redis {
				withMiniredis(t)
			}
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
			member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
			require.NoError(t, greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT))
			require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, chat.Title, true))
			ownerID := uniquePositiveUserID()
			fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
			require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
			_, _, err = federations.Fban(fed.FedID, member.Id, ownerID, "spam")
			require.NoError(t, err)
			clearJoinClaim(chat.Id, member.Id)

			dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: 1, ChatMember: &gotgbot.ChatMemberUpdated{
				Chat:          chat,
				From:          member,
				Date:          1,
				OldChatMember: gotgbot.ChatMemberLeft{User: member},
				NewChatMember: gotgbot.ChatMemberMember{User: member},
			}})
			dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: 2, ChatMember: &gotgbot.ChatMemberUpdated{
				Chat:          chat,
				From:          bot.User,
				Date:          2,
				OldChatMember: gotgbot.ChatMemberMember{User: member},
				NewChatMember: gotgbot.ChatMemberBanned{User: member},
			}})
			dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: 3, Message: &gotgbot.Message{
				MessageId: 11, Date: 1, Chat: chat, From: &member, NewChatMembers: []gotgbot.User{member},
			}})

			require.Len(t, client.callsFor("banChatMember"), 1, "a fbanned newcomer must be banned once per join")
			notices := 0
			for _, call := range client.callsFor("sendMessage") {
				if strings.Contains(fmt.Sprint(call.Params["text"]), "is fbanned") {
					notices++
				}
			}
			require.Equal(t, 1, notices, "one fed-ban notice per join")
			require.False(t, captchaSentTo(t, chat.Id, member.Id), "banned newcomer got a captcha")
			require.False(t, welcomeSent(client), "banned newcomer was welcomed")
		})
	}
}

// During a raid, the federation ban stage bans a fbanned newcomer for good on
// whichever join update comes first. The antiraid stage must not ban them
// again with an until_date, which could turn the permanent ban into a
// temporary one; everyone else still gets the raid's temp-ban.
func TestAntiRaidLeavesFedBannedNewcomerToFedBan(t *testing.T) {
	for _, tc := range []struct {
		fbanned bool
		order   []string
	}{
		{fbanned: true, order: []string{"service message"}},
		{fbanned: true, order: []string{"chat member update", "service message"}},
		{fbanned: false, order: []string{"service message"}},
	} {
		t.Run(fmt.Sprintf("fbanned=%v/%s", tc.fbanned, strings.Join(tc.order, " then ")), func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
			member := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
			ownerID := uniquePositiveUserID()
			fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
			require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
			if tc.fbanned {
				_, _, err = federations.Fban(fed.FedID, member.Id, ownerID, "spam")
				require.NoError(t, err)
			}
			enabled, err := antiRaidModule.enableRaid(chat.Id, 3600)
			require.NoError(t, err)
			require.True(t, enabled)
			t.Cleanup(func() {
				_, _ = antiRaidModule.disableRaid(chat.Id)
			})
			clearJoinClaim(chat.Id, member.Id)

			updates := map[string]*gotgbot.Update{
				"service message": {UpdateId: 1, Message: &gotgbot.Message{
					MessageId: 11, Date: 1, Chat: chat, From: &member, NewChatMembers: []gotgbot.User{member},
				}},
				"chat member update": {UpdateId: 2, ChatMember: &gotgbot.ChatMemberUpdated{
					Chat:          chat,
					From:          member,
					Date:          1,
					OldChatMember: gotgbot.ChatMemberLeft{User: member},
					NewChatMember: gotgbot.ChatMemberMember{User: member},
				}},
			}
			for _, path := range tc.order {
				dispatchJoinGuards(t, bot, updates[path])
			}

			bans := client.callsFor("banChatMember")
			require.Len(t, bans, 1)
			require.Equal(t, fmt.Sprint(member.Id), fmt.Sprint(bans[0].Params["user_id"]))
			_, temporary := bans[0].Params["until_date"]
			require.Equal(t, !tc.fbanned, temporary, "fed-ban bans for good, antiraid for the raid action time")
		})
	}
}

// A fbanned newcomer's join still counts towards the auto-antiraid threshold:
// a join wave is a raid whoever is in it.
func TestFedBannedJoinCountsTowardsAutoAntiRaid(t *testing.T) {
	withMiniredis(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
	fbanned := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Fbanned"}
	newcomer := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Newcomer"}
	ownerID := uniquePositiveUserID()
	fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
	_, _, err = federations.Fban(fed.FedID, fbanned.Id, ownerID, "spam")
	require.NoError(t, err)
	require.NoError(t, antiraid.SetAutoAntiRaidThreshold(chat.Id, 2))
	t.Cleanup(func() {
		_, _ = antiRaidModule.disableRaid(chat.Id)
	})

	for i, member := range []gotgbot.User{fbanned, newcomer} {
		clearJoinClaim(chat.Id, member.Id)
		dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: int64(i + 1), Message: &gotgbot.Message{
			MessageId: int64(11 + i), Date: 1, Chat: chat, From: &member, NewChatMembers: []gotgbot.User{member},
		}})
	}

	require.True(t, antiRaidModule.isRaidActive(chat.Id), "the second join must switch the raid on")
	bans := client.callsFor("banChatMember")
	require.Len(t, bans, 2)
	require.Equal(t, fmt.Sprint(newcomer.Id), fmt.Sprint(bans[1].Params["user_id"]))
	_, temporary := bans[1].Params["until_date"]
	require.True(t, temporary, "the newcomer gets the raid's temp-ban")
}

// fedBan puts chat in a new federation that bans userID, quiet or not.
func fedBan(t *testing.T, chat gotgbot.Chat, userID int64, quiet bool) {
	t.Helper()
	ownerID := uniquePositiveUserID()
	fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
	require.NoError(t, federations.SetQuietFed(chat.Id, quiet))
	_, _, err = federations.Fban(fed.FedID, userID, ownerID, "spam")
	require.NoError(t, err)
}

func fedBanNotices(client *moduleBotClient) int {
	n := 0
	for _, call := range client.callsFor("sendMessage") {
		if strings.Contains(fmt.Sprint(call.Params["text"]), "is fbanned in") {
			n++
		}
	}
	return n
}

// The federation ban is the intake's first stage: it bans a fbanned newcomer
// for good however Telegram reports the join, with the federation's notice or
// silently when it is quiet, and neither challenge nor welcome follows, nor a
// spam database lookup. It waits out a short flood wait; after a long one the
// member goes on to the next stage, and from there to the captcha.
func TestFedBanStageGivesEveryJoinWayOneVerdict(t *testing.T) {
	type want struct{ banCalls, notices, challenges, welcomes, lookups, logs int }
	for _, tc := range []struct {
		name       string
		quiet      bool
		retryAfter int64 // a 429 on the first ban call
		listed     bool  // the spam databases list the newcomer too
		want       want
		source     string // the one ban record, if any
	}{
		{name: "fbanned", want: want{banCalls: 1, notices: 1}, source: models.BanSourceFed},
		{name: "fbanned in a quiet federation", quiet: true, want: want{banCalls: 1}, source: models.BanSourceFed},
		{name: "short flood wait", retryAfter: 1, want: want{banCalls: 2, notices: 1}, source: models.BanSourceFed},
		{name: "long flood wait", retryAfter: 4, want: want{banCalls: 1, challenges: 1, lookups: 1}},
		{name: "fbanned and listed", listed: true, want: want{banCalls: 1, notices: 1}, source: models.BanSourceFed},
		{name: "fbanned and listed, long flood wait", listed: true, retryAfter: 4, want: want{banCalls: 2, lookups: 1, logs: 1}, source: models.BanSourceSpamDB},
	} {
		for _, way := range joinWays {
			t.Run(tc.name+"/"+strings.Join(way, " then "), func(t *testing.T) {
				useRealLocales(t)
				casBody := spamDBCASNotFound
				if tc.listed {
					casBody = spamDBCASListed
				}
				_, cas := withSpamDBCAS(t, true, http.StatusOK, casBody)
				j := newIntakeJoin(t, false)
				if tc.retryAfter > 0 {
					j.bot.BotClient = &floodOnceBotClient{moduleBotClient: j.client, method: "banChatMember", retryAfter: tc.retryAfter}
				}
				fedBan(t, j.chat, j.member.Id, tc.quiet)
				require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, true))
				require.NoError(t, logchannels.Set(j.chat.Id, j.chat.Title, spamDBLogChannel))
				require.NoError(t, spamdbsettings.SetSpamDBEnabled(j.chat.Id, true))
				require.NoError(t, spamdbsettings.SetSpamDBAction(j.chat.Id, spamdbsettings.ActionBan))

				j.deliver(t, way)

				logs := callsToChat(j.client, "sendMessage", spamDBLogChannel)
				got := want{j.bans(), fedBanNotices(j.client), j.challenges(t), j.welcomes(), int(cas.hits.Load()), len(logs)}
				require.Equal(t, tc.want, got)
				if tc.source == "" {
					require.Empty(t, banRecordsOf(t, j.chat.Id))
					return
				}
				require.Equal(t, tc.source, singleBanRecord(t, j.chat.Id).Source)
				_, temporary := j.client.callsFor("banChatMember")[tc.want.banCalls-1].Params["until_date"]
				require.False(t, temporary, "a guard ban of a fbanned newcomer is for good")
				if tc.want.logs > 0 {
					require.Contains(t, fmt.Sprint(logs[0].Params["reply_markup"]), punishment.LogUnbanNamespace+"|", "the log entry keeps its Unban button")
				}
			})
		}
	}
}

// A member the federation watcher banned on an ordinary message may have
// joined unseen: the join's update arriving later is handled already, so it
// neither bans again nor challenges or welcomes them.
func TestFedBanOnAMessageMarksTheJoinHandled(t *testing.T) {
	for _, way := range joinWays {
		t.Run(strings.Join(way, " then "), func(t *testing.T) {
			useRealLocales(t)
			j := newIntakeJoin(t, false)
			fedBan(t, j.chat, j.member.Id, false)
			require.NoError(t, captcha.SetCaptchaEnabled(j.chat.Id, j.chat.Title, true))

			dispatchJoinGuards(t, j.bot, &gotgbot.Update{UpdateId: 4, Message: &gotgbot.Message{
				MessageId: 12, Date: 1, Chat: j.chat, From: &j.member, Text: "hello",
			}})
			j.deliver(t, way)

			require.Equal(t, 1, j.bans())
			require.Equal(t, 1, fedBanNotices(j.client))
			require.Zero(t, j.challenges(t))
			require.Zero(t, j.welcomes())
		})
	}
}
