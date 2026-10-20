package modules

import (
	"encoding/json"
	"fmt"
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
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// A newcomer the join guards do not ban is welcomed on either path.
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

// dispatchJoinGuards runs update through fed-ban, antiraid and greetings as the
// bot registers them, in handler-group order.
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

// One service message adds A, B and C, and C's join crosses the auto-antiraid
// threshold. Antiraid bans only C, so A and B, whom nothing banned, must still
// get the captcha.
func TestRaidSwitchedOnByJoinStillChallengesNewcomersItDidNotBan(t *testing.T) {
	withMiniredis(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Join Guard Chat"}
	require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, true))
	require.NoError(t, antiraid.SetAutoAntiRaidThreshold(chat.Id, 3))
	t.Cleanup(func() {
		StopAntiRaidExpiryPoller()
		_, _ = antiRaidModule.disableRaid(chat.Id)
	})
	members := []gotgbot.User{
		{Id: uniquePositiveUserID(), FirstName: "A"},
		{Id: uniquePositiveUserID(), FirstName: "B"},
		{Id: uniquePositiveUserID(), FirstName: "C"},
	}
	for _, member := range members {
		clearRecentJoinProcessing(chat.Id, member.Id)
	}

	dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
		MessageId: 11, Date: 1, Chat: chat, From: &members[0], NewChatMembers: members,
	}})

	require.True(t, antiRaidModule.isRaidActive(chat.Id), "the third join must switch the raid on")
	bans := client.callsFor("banChatMember")
	require.Len(t, bans, 1)
	require.Equal(t, fmt.Sprint(members[2].Id), fmt.Sprint(bans[0].Params["user_id"]))
	for _, member := range members[:2] {
		require.True(t, captchaSentTo(t, chat.Id, member.Id), "%s joined before the raid and was not banned", member.FirstName)
	}
	require.False(t, captchaSentTo(t, chat.Id, members[2].Id), "C was banned on join")
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
				StopAntiRaidExpiryPoller()
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
			require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, true))
			guard.setup(t, chat, member)
			clearRecentJoinProcessing(chat.Id, member.Id)

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
				StopAntiRaidExpiryPoller()
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
				require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, captchaOn))
				guard.setup(t, chat, member)
				clearRecentJoinProcessing(chat.Id, member.Id)

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
			require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, true))
			clearRecentJoinProcessing(chat.Id, member.Id)
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
					require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, true))
					ownerID := uniquePositiveUserID()
					fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
					require.NoError(t, err)
					t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
					require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
					if fbanned {
						_, _, err = federations.Fban(fed.FedID, member.Id, ownerID, "spam")
						require.NoError(t, err)
					}
					clearRecentJoinProcessing(chat.Id, member.Id)
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
			require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, true))
			ownerID := uniquePositiveUserID()
			fed, err := federations.CreateFederation(ownerID, "Join Guard Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
			require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fed.FedID))
			_, _, err = federations.Fban(fed.FedID, member.Id, ownerID, "spam")
			require.NoError(t, err)
			clearRecentJoinProcessing(chat.Id, member.Id)

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

// During a raid, fed-ban (group -6) bans a fbanned newcomer for good on
// whichever join update comes first. Antiraid (group -5) must not ban them again
// with an until_date, which could turn the permanent ban into a temporary one;
// everyone else still gets the raid's temp-ban.
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
				StopAntiRaidExpiryPoller()
				_, _ = antiRaidModule.disableRaid(chat.Id)
			})
			clearRecentJoinProcessing(chat.Id, member.Id)

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
		StopAntiRaidExpiryPoller()
		_, _ = antiRaidModule.disableRaid(chat.Id)
	})

	for i, member := range []gotgbot.User{fbanned, newcomer} {
		clearRecentJoinProcessing(chat.Id, member.Id)
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
