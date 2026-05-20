package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
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
	if got := db.GetFlood(chat.Id).Limit; got != 4 {
		t.Fatalf("flood limit = %d, want 4", got)
	}

	modeCtx := newModuleMessageContext(bot, chat, admin, "/setfloodmode ban")
	if err := antifloodModule.setFloodMode(bot, modeCtx); err != ext.EndGroups {
		t.Fatalf("setFloodMode() error = %v, want EndGroups", err)
	}
	if got := db.GetFlood(chat.Id).Action; got != "ban" {
		t.Fatalf("flood action = %q, want ban", got)
	}

	deleteCtx := newModuleMessageContext(bot, chat, admin, "/delflood on")
	if err := antifloodModule.setFloodDeleter(bot, deleteCtx); err != ext.EndGroups {
		t.Fatalf("setFloodDeleter() error = %v, want EndGroups", err)
	}
	if got := db.GetFlood(chat.Id).DeleteAntifloodMessage; !got {
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
	settings := db.GetFlood(chat.Id)
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
	if err := db.SetFlood(chatID, 2); err != nil {
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
	if err := db.SetFlood(chat.Id, 1); err != nil {
		t.Fatalf("SetFlood() error = %v", err)
	}
	if err := db.SetFloodMode(chat.Id, "mute"); err != nil {
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
