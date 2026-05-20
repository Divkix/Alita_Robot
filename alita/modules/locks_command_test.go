package modules

import (
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func TestLockTypesAndCurrentLocksCommands(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.UpdateLock(chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock setup error = %v", err)
	}
	if err := db.UpdateLock(chat.Id, "media", false); err != nil {
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
	if !db.IsPermLocked(chat.Id, "url") || !db.IsPermLocked(chat.Id, "media") {
		t.Fatalf("locks were not persisted: %+v", db.GetChatLocks(chat.Id))
	}

	unlockCtx := newModuleMessageContext(bot, chat, admin, "/unlock url")
	if err := locksModule.unlockPerm(bot, unlockCtx); err != ext.EndGroups {
		t.Fatalf("unlockPerm error = %v, want EndGroups", err)
	}
	if db.IsPermLocked(chat.Id, "url") {
		t.Fatal("url lock is still enabled after unlock")
	}
	if !db.IsPermLocked(chat.Id, "media") {
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

func TestLockWatchersDeleteLockedContent(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Lock Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.UpdateLock(chat.Id, "url", true); err != nil {
		t.Fatalf("UpdateLock url setup error = %v", err)
	}
	if err := db.UpdateLock(chat.Id, "media", true); err != nil {
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
	if err := db.UpdateLock(chat.Id, "bots", true); err != nil {
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
