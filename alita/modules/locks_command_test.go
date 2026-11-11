package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestLockCommandRightsThroughDispatcher(t *testing.T) {
	useRealLocales(t)
	withDisableCommands(t)
	for _, command := range []struct {
		text   string
		member bool
	}{
		{"/lock", false}, {"/unlock", false}, {"/locks", false}, {"/locktypes", true},
	} {
		for _, actorAdmin := range []bool{false, true} {
			for _, botAdmin := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/admin=%t/bot=%t", command.text, actorAdmin, botAdmin), func(t *testing.T) {
					f := newAdminChatFixture(t)
					adminJSON := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"first_name":"Admin"}}`, f.admin.Id)
					f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + adminJSON + "]")
					f.client.membersByUser[fmt.Sprint(f.admin.Id)] = json.RawMessage(adminJSON)
					if botAdmin {
						f.addAdmins(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
					} else {
						f.client.membersByUser["999"] = json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
					}
					actor := f.member
					if actorAdmin {
						actor = f.admin
					}
					d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
					LoadLocks(d)
					if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, actor, command.text).Update, nil); err != nil {
						t.Fatal(err)
					}
					text := fmt.Sprint(f.client.onlyCall(t, "sendMessage").Params["text"])
					switch {
					case !botAdmin:
						if want := translated(t, "connections_is_user_connected_bot_not_admin"); text != want {
							t.Fatalf("reply = %q, want %q", text, want)
						}
					case !actorAdmin && !command.member:
						if want := translated(t, "connections_is_user_connected_user_not_admin"); text != want {
							t.Fatalf("reply = %q, want %q", text, want)
						}
					case command.text == "/locktypes":
						if !strings.HasPrefix(text, translated(t, "locks_locktypes_header")) {
							t.Fatalf("reply = %q, want lock types", text)
						}
					case command.text == "/locks":
						if !strings.HasPrefix(text, translated(t, "locks_current_locks_header")) {
							t.Fatalf("reply = %q, want current locks", text)
						}
					default:
						key := "locks_what_to_lock"
						if command.text == "/unlock" {
							key = "locks_what_to_unlock"
						}
						if want := translated(t, key); text != want {
							t.Fatalf("reply = %q, want %q", text, want)
						}
					}
				})
			}
		}
	}
}

func TestDisabledLockTypesInPMHonoursTargetGroup(t *testing.T) {
	useRealLocales(t)
	withDisableCommands(t)
	f := newAdminChatFixture(t)
	if err := connections.ConnectId(f.member.Id, f.chat.Id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })
	if err := disabling.DisableCMD(f.chat.Id, "locktypes"); err != nil {
		t.Fatal(err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)
	runCommandInPM(t, f.bot, f.member, "/locktypes", d)
	if calls := f.client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("disabled /locktypes replied in PM: %+v", calls)
	}
	if calls := f.client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("disabled /locktypes deleted a PM message: %+v", calls)
	}
}

func TestLockTypesAndCurrentLocksCommands(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin
	if err := locks.UpdateLock(chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock setup error = %v", err)
	}
	if err := locks.UpdateLock(chat.Id, "media", false); err != nil {
		t.Fatalf("UpdateLock setup error = %v", err)
	}

	typesCtx := newModuleMessageContext(bot, chat, admin, "/locktypes")
	if err := locksModule.locktypes(bot, typesCtx); err != ext.EndGroups {
		t.Fatalf("locktypes error = %v, want EndGroups", err)
	}
	lockTypes := locksModule.getLockMapAsArray()
	if !slices.IsSorted(lockTypes) {
		t.Fatalf("lock types are not sorted: %v", lockTypes)
	}
	if !slices.Contains(lockTypes, "url") || !slices.Contains(lockTypes, "messages") {
		t.Fatalf("lock types = %v, want lock and restriction entries", lockTypes)
	}

	locksCtx := newModuleMessageContext(bot, chat, admin, "/locks")
	if err := locksModule.locks(bot, locksCtx); err != ext.EndGroups {
		t.Fatalf("locks error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	lastText := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(lastText, "url = true") || !strings.Contains(lastText, "media = false") {
		t.Fatalf("locks text = %q, want persisted lock states", lastText)
	}
}

func TestLockAndUnlockCommandsPersistSettings(t *testing.T) {
	f := newAdminChatFixture(t)
	bot, chat, admin := f.bot, f.chat, f.admin

	lockCtx := newModuleMessageContext(bot, chat, admin, "/lock url media")
	if err := locksModule.lockPerm(bot, lockCtx); err != ext.EndGroups {
		t.Fatalf("lockPerm error = %v, want EndGroups", err)
	}
	if !locks.IsPermLocked(chat.Id, "url") || !locks.IsPermLocked(chat.Id, "media") {
		t.Fatalf("locks were not persisted: %+v", locks.GetChatLocks(chat.Id))
	}

	unlockCtx := newModuleMessageContext(bot, chat, admin, "/unlock url")
	if err := locksModule.unlockPerm(bot, unlockCtx); err != ext.EndGroups {
		t.Fatalf("unlockPerm error = %v, want EndGroups", err)
	}
	if locks.IsPermLocked(chat.Id, "url") {
		t.Fatal("url lock is still enabled after unlock")
	}
	if !locks.IsPermLocked(chat.Id, "media") {
		t.Fatal("media lock should remain enabled after unlocking url")
	}
}

// From PM the commands act on the connected group, so the admin checks must
// run there too, not on the PM.
func TestLockCommandsFromPMActOnConnectedGroup(t *testing.T) {
	f := newAdminChatFixture(t)
	bot, chat, admin := f.bot, f.chat, f.admin
	pm := f.connectAdmin(t)
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)

	if err := d.ProcessUpdate(bot, newModuleMessageContext(bot, pm, admin, "/lock url").Update, nil); err != nil {
		t.Fatal(err)
	}
	if !locks.IsPermLocked(chat.Id, "url") {
		t.Fatal("a connected admin could not lock url from PM")
	}

	if err := d.ProcessUpdate(bot, newModuleMessageContext(bot, pm, admin, "/unlock url").Update, nil); err != nil {
		t.Fatal(err)
	}
	if locks.IsPermLocked(chat.Id, "url") {
		t.Fatal("a connected admin could not unlock url from PM")
	}
}

func TestLockCommandsRefuseMembers(t *testing.T) {
	f := newAdminChatFixture(t)
	bot, chat, member := f.bot, f.chat, f.member
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLocks(d)
	if err := locks.UpdateLock(chat.Id, "media", true); err != nil {
		t.Fatalf("UpdateLock setup error = %v", err)
	}

	if err := d.ProcessUpdate(bot, newModuleMessageContext(bot, chat, member, "/lock url").Update, nil); err != nil {
		t.Fatal(err)
	}
	if locks.IsPermLocked(chat.Id, "url") {
		t.Fatal("a member locked url")
	}
	if err := d.ProcessUpdate(bot, newModuleMessageContext(bot, chat, member, "/unlock media").Update, nil); err != nil {
		t.Fatal(err)
	}
	if !locks.IsPermLocked(chat.Id, "media") {
		t.Fatal("a member unlocked media")
	}
}

func TestLockCommandsRejectMissingAndInvalidTypes(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

	missingCtx := newModuleMessageContext(bot, chat, admin, "/lock")
	if err := locksModule.lockPerm(bot, missingCtx); err != ext.EndGroups {
		t.Fatalf("lockPerm missing args error = %v, want EndGroups", err)
	}
	invalidCtx := newModuleMessageContext(bot, chat, admin, "/unlock nonsense")
	if err := locksModule.unlockPerm(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("unlockPerm invalid args error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want two validation replies", len(calls))
	}
}

func TestLockCommandsSkipMissingSenderAndPropagateReplyErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := locks.UpdateLock(chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock setup error = %v", err)
	}

	t.Run("lock missing sender", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		ctx := newModuleMessageContext(bot, chat, admin, "/lock url")
		ctx.EffectiveSender = nil
		if err := locksModule.lockPerm(bot, ctx); err != ext.EndGroups {
			t.Fatalf("lockPerm missing sender error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("sendMessage"); len(calls) != 0 {
			t.Fatalf("sendMessage calls = %d, want none without sender", len(calls))
		}
	})

	t.Run("unlock missing sender", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		ctx := newModuleMessageContext(bot, chat, admin, "/unlock url")
		ctx.EffectiveSender = nil
		if err := locksModule.unlockPerm(bot, ctx); err != ext.EndGroups {
			t.Fatalf("unlockPerm missing sender error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("sendMessage"); len(calls) != 0 {
			t.Fatalf("sendMessage calls = %d, want none without sender", len(calls))
		}
	})

	for _, tt := range []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "locktypes reply failure", text: "/locktypes", run: locksModule.locktypes},
		{name: "locks reply failure", text: "/locks", run: locksModule.locks},
		{name: "lock missing args reply failure", text: "/lock", run: locksModule.lockPerm},
		{name: "lock invalid type reply failure", text: "/lock nope", run: locksModule.lockPerm},
		{name: "lock success reply failure", text: "/lock url", run: locksModule.lockPerm},
		{name: "unlock missing args reply failure", text: "/unlock", run: locksModule.unlockPerm},
		{name: "unlock invalid type reply failure", text: "/unlock nope", run: locksModule.unlockPerm},
		{name: "unlock success reply failure", text: "/unlock url", run: locksModule.unlockPerm},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors["sendMessage"] = requestErr
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestLockWatchersDeleteLockedContent(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := locks.UpdateLock(chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock url setup error = %v", err)
	}
	if err := locks.UpdateLock(chat.Id, "media", true); err != nil {
		t.Fatalf("UpdateLock media setup error = %v", err)
	}

	urlCtx := newModuleMessageContext(bot, chat, member, "https://example.com")
	urlCtx.EffectiveMessage.Entities = []gotgbot.MessageEntity{{Type: "url", Offset: 0, Length: 19}}
	if err := locksModule.permHandler(bot, urlCtx); err != ext.ContinueGroups {
		t.Fatalf("permHandler error = %v, want ContinueGroups", err)
	}

	mediaCtx := newModuleMessageContext(bot, chat, member, "photo")
	mediaCtx.EffectiveMessage.Photo = []gotgbot.PhotoSize{{FileId: "photo-1"}}
	if err := locksModule.restHandler(bot, mediaCtx); err != ext.ContinueGroups {
		t.Fatalf("restHandler error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 2 {
		t.Fatalf("deleteMessage calls = %d, want 2", len(calls))
	}
}

func TestLockWatchersSkipPermissionLookupsWhenMessageIsNotLocked(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := locks.UpdateLock(chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock url setup error = %v", err)
	}
	if err := locks.UpdateLock(chat.Id, "media", true); err != nil {
		t.Fatalf("UpdateLock media setup error = %v", err)
	}

	permCtx := newModuleMessageContext(bot, chat, member, "hello")
	if err := locksModule.permHandler(bot, permCtx); err != ext.ContinueGroups {
		t.Fatalf("permHandler error = %v, want ContinueGroups", err)
	}
	restCtx := newModuleMessageContext(bot, chat, member, "hello")
	if err := locksModule.restHandler(bot, restCtx); err != ext.ContinueGroups {
		t.Fatalf("restHandler error = %v, want ContinueGroups", err)
	}

	for _, method := range []string{"getChat", "getChatAdministrators", "getChatMember", "deleteMessage"} {
		if calls := client.callsFor(method); len(calls) != 0 {
			t.Fatalf("%s calls = %d, want none for unlocked content", method, len(calls))
		}
	}
}

func TestLockWatchersDoNotDeleteForAdmins(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	chatAdmin := gotgbot.User{Id: 4343, FirstName: "Chat Admin"}
	client.responses["getChatAdministrators"] = json.RawMessage(
		`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true,"can_restrict_members":true},` +
			`{"status":"administrator","user":{"id":4343,"is_bot":false,"first_name":"Chat Admin"},"can_delete_messages":true}]`,
	)
	if err := locks.UpdateLock(chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock url setup error = %v", err)
	}
	if err := locks.UpdateLock(chat.Id, "media", true); err != nil {
		t.Fatalf("UpdateLock media setup error = %v", err)
	}

	urlCtx := newModuleMessageContext(bot, chat, chatAdmin, "https://example.com")
	urlCtx.EffectiveMessage.Entities = []gotgbot.MessageEntity{{Type: "url", Offset: 0, Length: 19}}
	if err := locksModule.permHandler(bot, urlCtx); err != ext.ContinueGroups {
		t.Fatalf("permHandler error = %v, want ContinueGroups", err)
	}

	mediaCtx := newModuleMessageContext(bot, chat, chatAdmin, "photo")
	mediaCtx.EffectiveMessage.Photo = []gotgbot.PhotoSize{{FileId: "photo-1"}}
	if err := locksModule.restHandler(bot, mediaCtx); err != ext.ContinueGroups {
		t.Fatalf("restHandler error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want none for admin", len(calls))
	}
}

// dispatchBotJoin delivers adder adding addedBot to chat as a member update.
func dispatchBotJoin(t *testing.T, bot *gotgbot.Bot, chat gotgbot.Chat, adder, addedBot gotgbot.User) {
	t.Helper()
	clearJoinClaim(chat.Id, addedBot.Id)
	dispatchJoinGuards(t, bot, &gotgbot.Update{UpdateId: 3, ChatMember: &gotgbot.ChatMemberUpdated{
		Chat:          chat,
		From:          adder,
		Date:          1,
		OldChatMember: gotgbot.ChatMemberLeft{User: addedBot},
		NewChatMember: gotgbot.ChatMemberMember{User: addedBot},
	}})
}

func sentLocaleText(client *moduleBotClient, key string) int {
	want, _ := i18n.MustNewTranslator("en").GetString(key)
	n := 0
	for _, call := range client.callsFor("sendMessage") {
		if got, _ := call.Params["text"].(string); got == want {
			n++
		}
	}
	return n
}

func TestBotsLockSkipsUnlockedChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	unlockedChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Unlocked Lock Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	addedBot := gotgbot.User{Id: 4242, IsBot: true, FirstName: "Added Bot"}

	dispatchBotJoin(t, bot, unlockedChat, member, addedBot)

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for unlocked bot lock", len(calls))
	}
	if n := sentLocaleText(client, "locks_bot_only_admins"); n != 0 {
		t.Fatalf("bots-lock notices = %d, want none for unlocked bot lock", n)
	}
}

// A Telegram error in the bots lock leaves the bot to the rest of the intake;
// a lookup failure still reports the missing right.
func TestBotsLockReportsGotgbotErrors(t *testing.T) {
	useRealLocales(t)
	requestErr := errors.New("telegram request failed")
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	addedBot := gotgbot.User{Id: 4242, IsBot: true, FirstName: "Added Bot"}

	for _, tt := range []struct {
		name                   string
		method                 string
		wantBans, wantNoRights int
	}{
		{name: "bot admin lookup failure reports missing permission", method: "getChatMember", wantNoRights: 1},
		{name: "ban failure", method: "banChatMember", wantBans: 1},
		{name: "notice send failure", method: "sendMessage", wantBans: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
			if err := locks.UpdateLock(chat.Id, "bots", true); err != nil {
				t.Fatalf("UpdateLock bots setup error = %v", err)
			}

			dispatchBotJoin(t, bot, chat, member, addedBot)

			if calls := client.callsFor("banChatMember"); len(calls) != tt.wantBans {
				t.Fatalf("banChatMember calls = %d, want %d", len(calls), tt.wantBans)
			}
			if n := sentLocaleText(client, "locks_bot_lock_no_permission"); n != tt.wantNoRights {
				t.Fatalf("no-permission notices = %d, want %d", n, tt.wantNoRights)
			}
			if tt.method == "banChatMember" && sentLocaleText(client, "locks_bot_only_admins") != 0 {
				t.Fatal("a failed ban posted the bots-lock notice")
			}
		})
	}
}

// An invalid lock type is echoed back HTML-escaped, so Telegram accepts the
// reply.
func TestLockInvalidTypeReplyEscapesInput(t *testing.T) {
	f := newAdminChatFixture(t)
	if err := locksModule.lockPerm(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/lock <b")); err != ext.EndGroups {
		t.Fatalf("lockPerm error = %v, want EndGroups", err)
	}
	calls := f.client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text, _ := calls[0].Params["text"].(string)
	if !strings.Contains(text, "&lt;b") || strings.Contains(text, "<b") {
		t.Fatalf("invalid lock reply = %q, want the input escaped", text)
	}
}

// Anonymous admins post as the group itself; they are admins and exempt
// from locks.
func TestLockWatchersDoNotDeleteForAnonymousAdmins(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	anonBot := gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Group", Username: "GroupAnonymousBot"}
	client.responses["getChatAdministrators"] = json.RawMessage(
		`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true,"can_restrict_members":true}]`,
	)
	if err := locks.UpdateLock(chat.Id, "sticker", true); err != nil {
		t.Fatalf("UpdateLock sticker setup error = %v", err)
	}
	if err := locks.UpdateLock(chat.Id, "media", true); err != nil {
		t.Fatalf("UpdateLock media setup error = %v", err)
	}
	asGroup := func(ctx *ext.Context) *ext.Context {
		groupChat := chat
		ctx.EffectiveMessage.SenderChat = &groupChat
		ctx.EffectiveSender = &gotgbot.Sender{Chat: &groupChat, User: &anonBot, ChatId: chat.Id}
		return ctx
	}

	stickerCtx := asGroup(newModuleMessageContext(bot, chat, anonBot, ""))
	stickerCtx.EffectiveMessage.Sticker = &gotgbot.Sticker{FileId: "sticker-1"}
	if err := locksModule.permHandler(bot, stickerCtx); err != ext.ContinueGroups {
		t.Fatalf("permHandler error = %v, want ContinueGroups", err)
	}

	mediaCtx := asGroup(newModuleMessageContext(bot, chat, anonBot, "photo"))
	mediaCtx.EffectiveMessage.Photo = []gotgbot.PhotoSize{{FileId: "photo-1"}}
	if err := locksModule.restHandler(bot, mediaCtx); err != ext.ContinueGroups {
		t.Fatalf("restHandler error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want none for an anonymous admin", len(calls))
	}
}

// When the bot is an admin without ban rights, the bots lock tells the chat
// so instead of failing silently.
func TestBotsLockReportsMissingBanPermission(t *testing.T) {
	useRealLocales(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	t.Cleanup(func() { cache.InvalidateAdminCache(chat.Id) })
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	addedBot := gotgbot.User{Id: 4242, IsBot: true, FirstName: "Added Bot"}
	client.responses["getChatAdministrators"] = json.RawMessage(
		`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true,"can_restrict_members":false}]`,
	)
	if err := locks.UpdateLock(chat.Id, "bots", true); err != nil {
		t.Fatalf("UpdateLock bots setup error = %v", err)
	}

	dispatchBotJoin(t, bot, chat, member, addedBot)

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none without ban rights", len(calls))
	}
	want, _ := i18n.MustNewTranslator("en").GetString("locks_bot_lock_no_ban_permission")
	for _, call := range client.callsFor("sendMessage") {
		if got, _ := call.Params["text"].(string); got == want {
			if chatID := fmt.Sprint(call.Params["chat_id"]); chatID != fmt.Sprint(chat.Id) {
				t.Errorf("notice chat_id = %s, want %d", chatID, chat.Id)
			}
			return
		}
	}
	t.Fatalf("sendMessage calls = %v, want the missing-permission notice", client.callsFor("sendMessage"))
}
