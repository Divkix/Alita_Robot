package modules

import (
	"fmt"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
)

func resetAntifloodState(t *testing.T) {
	t.Helper()

	antifloodModule.syncHelperMap.Range(func(key, _ any) bool {
		antifloodModule.syncHelperMap.Delete(key)
		return true
	})
}

func TestAntifloodCommandsUpdateSettingsAndDisplay(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	setCtx := newModuleMessageContext(bot, chat, admin, "/setflood 4")
	if err := antifloodModule.setFlood(bot, setCtx); err != ext.EndGroups {
		t.Fatalf("setFlood() error = %v, want EndGroups", err)
	}
	if got := antiflood.GetFlood(chat.Id).Limit; got != 4 {
		t.Fatalf("flood limit = %d, want 4", got)
	}

	modeCtx := newModuleMessageContext(bot, chat, admin, "/setfloodmode ban")
	if err := antifloodModule.setFloodMode(bot, modeCtx); err != ext.EndGroups {
		t.Fatalf("setFloodMode() error = %v, want EndGroups", err)
	}
	if got := antiflood.GetFlood(chat.Id).Action; got != "ban" {
		t.Fatalf("flood action = %q, want ban", got)
	}

	deleteCtx := newModuleMessageContext(bot, chat, admin, "/delflood on")
	if err := antifloodModule.setFloodDeleter(bot, deleteCtx); err != ext.EndGroups {
		t.Fatalf("setFloodDeleter() error = %v, want EndGroups", err)
	}
	if got := antiflood.GetFlood(chat.Id).DeleteAntifloodMessage; !got {
		t.Fatal("DeleteAntifloodMessage = false, want true")
	}

	showCtx := newModuleMessageContext(bot, chat, admin, "/flood")
	if err := antifloodModule.flood(bot, showCtx); err != ext.EndGroups {
		t.Fatalf("flood() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 4 {
		t.Fatalf("sendMessage calls = %d, want one reply per command", len(calls))
	}
}

func TestAntifloodCommandsHandleDisabledAndValidationBranches(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "show disabled flood", text: "/flood", run: antifloodModule.flood},
		{name: "set missing limit", text: "/setflood", run: antifloodModule.setFlood},
		{name: "set non integer", text: "/setflood nope", run: antifloodModule.setFlood},
		{name: "set below range", text: "/setflood 2", run: antifloodModule.setFlood},
		{name: "disable flood", text: "/setflood off", run: antifloodModule.setFlood},
		{name: "mode missing", text: "/setfloodmode", run: antifloodModule.setFloodMode},
		{name: "mode invalid", text: "/setfloodmode warn", run: antifloodModule.setFloodMode},
		{name: "deleter current disabled", text: "/delflood", run: antifloodModule.setFloodDeleter},
		{name: "deleter invalid", text: "/delflood maybe", run: antifloodModule.setFloodDeleter},
		{name: "deleter off", text: "/delflood off", run: antifloodModule.setFloodDeleter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := tt.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.text, err)
			}
		})
	}
	settings := antiflood.GetFlood(chat.Id)
	if settings.Limit != 0 {
		t.Fatalf("flood limit = %d, want disabled", settings.Limit)
	}
	if settings.DeleteAntifloodMessage {
		t.Fatal("DeleteAntifloodMessage = true, want false after /delflood off")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != len(tests) {
		t.Fatalf("sendMessage calls = %d, want one reply per validation branch", len(calls))
	}
}

func TestAntifloodUpdateFloodTracksLimitAndResetsAfterPunishment(t *testing.T) {
	resetAntifloodState(t)
	chatID := uniqueModuleChatID()
	if err := antiflood.SetFlood(chatID, 2); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}

	flooded, state, settings := antifloodModule.updateFlood(chatID, 42, 100)
	if flooded {
		t.Fatal("first message flooded = true, want false")
	}
	if state.messageCount != 1 || len(state.messageIDs) != 1 {
		t.Fatalf("first state = %#v, want one tracked message", state)
	}
	if settings.Limit != 2 {
		t.Fatalf("settings limit = %d, want 2", settings.Limit)
	}

	flooded, state, _ = antifloodModule.updateFlood(chatID, 42, 101)
	if flooded {
		t.Fatal("second message flooded = true, want false at limit")
	}
	if state.messageCount != 2 {
		t.Fatalf("second message count = %d, want 2", state.messageCount)
	}

	flooded, _, _ = antifloodModule.updateFlood(chatID, 42, 102)
	if !flooded {
		t.Fatal("third message flooded = false, want true over limit")
	}
	stored, ok := antifloodModule.syncHelperMap.Load(floodKey{chatId: chatID, userId: 42})
	if !ok {
		t.Fatal("flood state was not stored after punishment")
	}
	if got := stored.(floodControl); got.messageCount != 0 || got.userId != 0 {
		t.Fatalf("stored state after punishment = %#v, want reset state", got)
	}
}

func TestAntifloodWatcherMutesAndDeletesAfterLimit(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := antiflood.SetFlood(chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodMode(chat.Id, "mute"); err != nil {
		t.Fatalf("SetFloodMode() error = %v", err)
	}

	firstCtx := newModuleMessageContext(bot, chat, member, "one")
	if err := antifloodModule.checkFlood(bot, firstCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood first error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none before limit", len(calls))
	}

	secondCtx := newModuleMessageContext(bot, chat, member, "two")
	secondCtx.EffectiveMessage.MessageId = 102
	if err := antifloodModule.checkFlood(bot, secondCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood second error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want triggering message deleted", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want mute action", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want antiflood notice", len(calls))
	}
}

func TestAntifloodWatcherAppliesKickAndBanActions(t *testing.T) {
	for _, tt := range []struct {
		name    string
		action  string
		method  string
		userID  int64
		message string
	}{
		{name: "kick", action: "kick", method: "banChatMember", userID: 43, message: "kick me"},
		{name: "ban", action: "ban", method: "banChatMember", userID: 44, message: "ban me"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetAntifloodState(t)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
			member := gotgbot.User{Id: tt.userID, FirstName: "Flooder"}
			if err := antiflood.SetFlood(chat.Id, 1); err != nil {
				t.Fatalf("SetFlood() error = %v", err)
			}
			if err := antiflood.SetFloodMode(chat.Id, tt.action); err != nil {
				t.Fatalf("SetFloodMode() error = %v", err)
			}

			firstCtx := newModuleMessageContext(bot, chat, member, "one")
			if err := antifloodModule.checkFlood(bot, firstCtx); err != ext.ContinueGroups {
				t.Fatalf("checkFlood first error = %v, want ContinueGroups", err)
			}
			secondCtx := newModuleMessageContext(bot, chat, member, tt.message)
			secondCtx.EffectiveMessage.MessageId = 202
			if err := antifloodModule.checkFlood(bot, secondCtx); err != ext.ContinueGroups {
				t.Fatalf("checkFlood second error = %v, want ContinueGroups", err)
			}

			if calls := client.callsFor(tt.method); len(calls) != 1 {
				t.Fatalf("%s calls = %d, want action request", tt.method, len(calls))
			}
			if calls := client.callsFor("sendMessage"); len(calls) != 1 {
				t.Fatalf("sendMessage calls = %d, want antiflood notice", len(calls))
			}
		})
	}
}

func TestAntifloodWatcherBulkDeletesTrackedMessages(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 45, FirstName: "BulkFlooder"}
	if err := antiflood.SetFlood(chat.Id, 4); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodMode(chat.Id, "ban"); err != nil {
		t.Fatalf("SetFloodMode() error = %v", err)
	}
	if err := antiflood.SetFloodMsgDel(chat.Id, true); err != nil {
		t.Fatalf("SetFloodMsgDel() error = %v", err)
	}

	for i := 0; i < 5; i++ {
		ctx := newModuleMessageContext(bot, chat, member, "bulk")
		ctx.EffectiveMessage.MessageId = int64(300 + i)
		if err := antifloodModule.checkFlood(bot, ctx); err != ext.ContinueGroups {
			t.Fatalf("checkFlood message %d error = %v, want ContinueGroups", i, err)
		}
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 5 {
		t.Fatalf("deleteMessage calls = %d, want all tracked flood messages deleted", len(calls))
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want ban action", len(calls))
	}
}

func TestAntifloodWatcherFailsOpenWhenAdminSemaphoreFull(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 46, FirstName: "Flooder"}
	if err := antiflood.SetFlood(chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}

	for i := 0; i < maxConcurrentAdminChecks; i++ {
		antifloodModule.adminCheckSemaphore <- struct{}{}
	}
	t.Cleanup(func() {
		for i := 0; i < maxConcurrentAdminChecks; i++ {
			<-antifloodModule.adminCheckSemaphore
		}
	})

	ctx := newModuleMessageContext(bot, chat, member, "flood")
	if err := antifloodModule.checkFlood(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood(semaphore full) error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want fail-open without action", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want fail-open without action", len(calls))
	}
}

func TestAntifloodWatcherSkipsUntargetableMessages(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}

	nilSenderCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "system")
	nilSenderCtx.EffectiveMessage.From = nil
	if err := antifloodModule.checkFlood(bot, nilSenderCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood(nil sender) error = %v, want ContinueGroups", err)
	}

	mediaGroupCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "album")
	mediaGroupCtx.EffectiveMessage.MediaGroupId = "album-1"
	if err := antifloodModule.checkFlood(bot, mediaGroupCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood(media group) error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want no action for skipped messages", len(calls))
	}
}

func TestAntifloodWatcherSkipsApprovedUsersAndBotRestrictFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		userID int64
		setup  func(t *testing.T, chatID int64, client *moduleBotClient)
	}{
		{
			name:   "approved user",
			userID: 47,
			setup: func(t *testing.T, chatID int64, _ *moduleBotClient) {
				t.Helper()
				if err := approvals.AddApprovedUser(chatID, 47, 777000, "trusted"); err != nil {
					t.Fatalf("AddApprovedUser() error = %v", err)
				}
			},
		},
		{
			name:   "bot cannot restrict",
			userID: 48,
			setup: func(_ *testing.T, _ int64, client *moduleBotClient) {
				client.errors["getChatMember"] = fmt.Errorf("telegram admin lookup failed")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetAntifloodState(t)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
			member := gotgbot.User{Id: tt.userID, FirstName: "Flooder"}
			if err := antiflood.SetFlood(chat.Id, 1); err != nil {
				t.Fatalf("SetFlood() error = %v", err)
			}
			if err := antiflood.SetFloodMode(chat.Id, "mute"); err != nil {
				t.Fatalf("SetFloodMode() error = %v", err)
			}
			tt.setup(t, chat.Id, client)

			for i := 0; i < 2; i++ {
				ctx := newModuleMessageContext(bot, chat, member, "flood")
				ctx.EffectiveMessage.MessageId = int64(500 + i)
				if err := antifloodModule.checkFlood(bot, ctx); err != ext.ContinueGroups {
					t.Fatalf("checkFlood message %d error = %v, want ContinueGroups", i, err)
				}
			}

			if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
				t.Fatalf("restrictChatMember calls = %d, want skipped mute action", len(calls))
			}
		})
	}
}

// countFloodMuEntries returns the number of entries currently held in floodMu.
func countFloodMuEntries() int {
	n := 0
	floodMu.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

// TestAntifloodCleanupRemovesMutexEntries verifies that cleanupOnce removes both
// the syncHelperMap entry and the corresponding floodMu entry when the entry has
// been idle for more than 600 seconds, and that a non-idle entry is left intact.
func TestAntifloodCleanupRemovesMutexEntries(t *testing.T) {
	resetAntifloodState(t)

	// Also clear any floodMu entries left over from other tests.
	floodMu.Range(func(key, _ any) bool {
		floodMu.Delete(key)
		return true
	})

	staleKey := floodKey{chatId: -9001, userId: 1001}
	freshKey := floodKey{chatId: -9001, userId: 1002}

	now := int64(1_000_000)
	staleActivity := now - 601 // idle > 600 s → must be removed
	freshActivity := now - 10  // idle 10 s → must survive

	// Populate syncHelperMap.
	antifloodModule.syncHelperMap.Store(staleKey, floodControl{userId: 1001, lastActivity: staleActivity})
	antifloodModule.syncHelperMap.Store(freshKey, floodControl{userId: 1002, lastActivity: freshActivity})

	// Populate floodMu (mirrors what updateFlood does via LoadOrStore).
	floodMu.Store(staleKey, &sync.Mutex{})
	floodMu.Store(freshKey, &sync.Mutex{})

	if got := countFloodMuEntries(); got < 2 {
		t.Fatalf("pre-cleanup floodMu entries = %d, want >= 2", got)
	}

	// Run exactly one cleanup pass at the synthetic "now".
	antifloodModule.cleanupOnce(now)

	// Stale entry must be gone from both maps.
	if _, ok := antifloodModule.syncHelperMap.Load(staleKey); ok {
		t.Error("stale key still present in syncHelperMap after cleanup")
	}
	if _, ok := floodMu.Load(staleKey); ok {
		t.Error("stale key still present in floodMu after cleanup")
	}

	// Fresh entry must still be present in both maps.
	if _, ok := antifloodModule.syncHelperMap.Load(freshKey); !ok {
		t.Error("fresh key was incorrectly removed from syncHelperMap")
	}
	if _, ok := floodMu.Load(freshKey); !ok {
		t.Error("fresh key was incorrectly removed from floodMu")
	}

	// Cleanup after test.
	antifloodModule.syncHelperMap.Delete(freshKey)
	floodMu.Delete(freshKey)
}

func TestAntifloodWatcherPropagatesFloodMessageDeleteErrors(t *testing.T) {
	resetAntifloodState(t)
	client := newModuleBotClient()
	client.errors["deleteMessage"] = fmt.Errorf("delete failed")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Flood Chat"}
	member := gotgbot.User{Id: 49, FirstName: "Flooder"}
	if err := antiflood.SetFlood(chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := antiflood.SetFloodMode(chat.Id, "ban"); err != nil {
		t.Fatalf("SetFloodMode() error = %v", err)
	}
	if err := antiflood.SetFloodMsgDel(chat.Id, true); err != nil {
		t.Fatalf("SetFloodMsgDel() error = %v", err)
	}

	firstCtx := newModuleMessageContext(bot, chat, member, "one")
	if err := antifloodModule.checkFlood(bot, firstCtx); err != ext.ContinueGroups {
		t.Fatalf("checkFlood first error = %v, want ContinueGroups", err)
	}
	secondCtx := newModuleMessageContext(bot, chat, member, "two")
	secondCtx.EffectiveMessage.MessageId = 602
	if err := antifloodModule.checkFlood(bot, secondCtx); err == nil {
		t.Fatal("checkFlood delete failure error = nil, want delete error")
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want no action after delete failure", len(calls))
	}
}
