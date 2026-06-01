package modules

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/locks"
)

func TestLockTypesAndCurrentLocksCommands(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
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
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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

func TestLockCommandsRejectMissingAndInvalidTypes(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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

func TestBotLockHandlerBansNonAdminAddedBot(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	adder := gotgbot.User{Id: 42, FirstName: "Member"}
	addedBot := gotgbot.User{Id: 4242, IsBot: true, FirstName: "Added Bot"}
	if err := locks.UpdateLock(chat.Id, "bots", true); err != nil {
		t.Fatalf("UpdateLock bots setup error = %v", err)
	}
	update := &gotgbot.Update{
		UpdateId: 3,
		ChatMember: &gotgbot.ChatMemberUpdated{
			Chat:          chat,
			From:          adder,
			OldChatMember: gotgbot.ChatMemberLeft{User: addedBot},
			NewChatMember: gotgbot.ChatMemberMember{User: addedBot},
		},
	}
	ctx := ext.NewContext(bot, update, nil)

	if err := locksModule.botLockHandler(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("botLockHandler error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want bot lock notice", len(calls))
	}
}

func newBotLockContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	adder gotgbot.User,
	addedBot gotgbot.User,
) *ext.Context {
	update := &gotgbot.Update{
		UpdateId: 3,
		ChatMember: &gotgbot.ChatMemberUpdated{
			Chat:          chat,
			From:          adder,
			OldChatMember: gotgbot.ChatMemberLeft{User: addedBot},
			NewChatMember: gotgbot.ChatMemberMember{User: addedBot},
		},
	}
	ctx := ext.NewContext(bot, update, nil)
	ctx.EffectiveSender = &gotgbot.Sender{User: &adder, ChatId: chat.Id}
	return ctx
}

func TestBotLockHandlerSkipsUnlockedChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	unlockedChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Unlocked Lock Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	addedBot := gotgbot.User{Id: 4242, IsBot: true, FirstName: "Added Bot"}

	unlockedCtx := newBotLockContext(bot, unlockedChat, member, addedBot)
	if err := locksModule.botLockHandler(bot, unlockedCtx); err != ext.ContinueGroups {
		t.Fatalf("botLockHandler unlocked error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for unlocked bot lock", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want none for unlocked bot lock", len(calls))
	}
}

func TestBotLockHandlerReportsAndPropagatesGotgbotErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	addedBot := gotgbot.User{Id: 4242, IsBot: true, FirstName: "Added Bot"}

	for _, tt := range []struct {
		name   string
		method string
	}{
		{name: "bot admin lookup failure reports missing permission", method: "getChatMember"},
		{name: "ban failure", method: "banChatMember"},
		{name: "notice send failure", method: "sendMessage"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
			if err := locks.UpdateLock(chat.Id, "bots", true); err != nil {
				t.Fatalf("UpdateLock bots setup error = %v", err)
			}
			ctx := newBotLockContext(bot, chat, member, addedBot)

			err := locksModule.botLockHandler(bot, ctx)
			if tt.method == "getChatMember" {
				if err != ext.ContinueGroups {
					t.Fatalf("botLockHandler() error = %v, want ContinueGroups for permission notice", err)
				}
				if calls := client.callsFor("sendMessage"); len(calls) != 1 {
					t.Fatalf("sendMessage calls = %d, want no-permission notice", len(calls))
				}
				return
			}
			if !errors.Is(err, requestErr) {
				t.Fatalf("botLockHandler() error = %v, want request error", err)
			}
		})
	}
}
