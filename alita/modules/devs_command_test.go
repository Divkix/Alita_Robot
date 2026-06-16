package modules

import (
	"errors"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/devs"
)

func withOwnerID(t *testing.T, ownerID int64) {
	t.Helper()

	previousOwnerID := config.AppConfig.OwnerId
	config.AppConfig.OwnerId = ownerID
	t.Cleanup(func() {
		config.AppConfig.OwnerId = previousOwnerID
	})
}

func TestDevLeaveChatHandlesMissingArgsAndRequestErrors(t *testing.T) {
	withOwnerID(t, 777000)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Owner"}
	owner := gotgbot.User{Id: 777000, FirstName: "Owner"}

	missingClient := newModuleBotClient()
	missingBot := newModuleTestBot(missingClient)
	missingCtx := newModuleMessageContext(missingBot, chat, owner, "/leavechat")
	if err := devsModule.leaveChat(missingBot, missingCtx); err != ext.ContinueGroups {
		t.Fatalf("leaveChat(missing args) error = %v, want ContinueGroups", err)
	}
	if calls := missingClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want missing-arg reply", len(calls))
	}

	requestClient := newModuleBotClient()
	requestClient.errors["leaveChat"] = errors.New("leave failed")
	requestBot := newModuleTestBot(requestClient)
	requestCtx := newModuleMessageContext(requestBot, chat, owner, "/leavechat -100123")
	if err := devsModule.leaveChat(requestBot, requestCtx); err == nil {
		t.Fatal("leaveChat(request error) = nil, want error")
	}
}

func TestDevTeamRoleCommandsHandleNoopAndAuthorizationBranches(t *testing.T) {
	withOwnerID(t, 777000)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Owner"}
	owner := gotgbot.User{Id: 777000, FirstName: "Owner"}
	guest := gotgbot.User{Id: 55, FirstName: "Guest"}

	guestCtx := newModuleMessageContext(bot, chat, guest, "/addsudo 42")
	if err := devsModule.addSudo(bot, guestCtx); err != ext.ContinueGroups {
		t.Fatalf("addSudo(guest) error = %v, want ContinueGroups", err)
	}

	channelCtx := newModuleMessageContext(bot, chat, owner, "/addsudo -1001234567890")
	if err := devsModule.addSudo(bot, channelCtx); err != ext.ContinueGroups {
		t.Fatalf("addSudo(channel id) error = %v, want ContinueGroups", err)
	}

	if err := devs.AddSudo(42); err != nil {
		t.Fatalf("AddSudo setup error = %v", err)
	}
	alreadySudoCtx := newModuleMessageContext(bot, chat, owner, "/addsudo 42")
	if err := devsModule.addSudo(bot, alreadySudoCtx); err != ext.ContinueGroups {
		t.Fatalf("addSudo(already sudo) error = %v, want ContinueGroups", err)
	}

	notDevCtx := newModuleMessageContext(bot, chat, owner, "/remdev 43")
	if err := devsModule.remDev(bot, notDevCtx); err != ext.ContinueGroups {
		t.Fatalf("remDev(not dev) error = %v, want ContinueGroups", err)
	}
}

func TestDevStatsPropagatesSendAndEditErrors(t *testing.T) {
	withOwnerID(t, 777000)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Owner"}
	owner := gotgbot.User{Id: 777000, FirstName: "Owner"}

	sendClient := newModuleBotClient()
	sendClient.errors["sendMessage"] = errors.New("send failed")
	sendBot := newModuleTestBot(sendClient)
	sendCtx := newModuleMessageContext(sendBot, chat, owner, "/stats")
	if err := devsModule.getStats(sendBot, sendCtx); err == nil {
		t.Fatal("getStats(send error) = nil, want error")
	}

	editClient := newModuleBotClient()
	editClient.errors["editMessageText"] = errors.New("edit failed")
	editBot := newModuleTestBot(editClient)
	editCtx := newModuleMessageContext(editBot, chat, owner, "/stats")
	if err := devsModule.getStats(editBot, editCtx); err == nil {
		t.Fatal("getStats(edit error) = nil, want error")
	}
}

func TestDevTeamRoleCommandsMutateRoles(t *testing.T) {
	withOwnerID(t, 777000)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Owner"}
	owner := gotgbot.User{Id: 777000, FirstName: "Owner"}

	addSudoCtx := newModuleMessageContext(bot, chat, owner, "/addsudo 42")
	if err := devsModule.addSudo(bot, addSudoCtx); err != ext.ContinueGroups {
		t.Fatalf("addSudo() error = %v, want ContinueGroups", err)
	}
	if got := devs.GetTeamMemInfo(42); !got.Sudo {
		t.Fatalf("Sudo = false after addsudo, full settings: %#v", got)
	}

	addDevCtx := newModuleMessageContext(bot, chat, owner, "/adddev 42")
	if err := devsModule.addDev(bot, addDevCtx); err != ext.ContinueGroups {
		t.Fatalf("addDev() error = %v, want ContinueGroups", err)
	}
	if got := devs.GetTeamMemInfo(42); !got.IsDev {
		t.Fatalf("IsDev = false after adddev, full settings: %#v", got)
	}

	remSudoCtx := newModuleMessageContext(bot, chat, owner, "/remsudo 42")
	if err := devsModule.remSudo(bot, remSudoCtx); err != ext.ContinueGroups {
		t.Fatalf("remSudo() error = %v, want ContinueGroups", err)
	}
	if got := devs.GetTeamMemInfo(42); got.Sudo {
		t.Fatalf("Sudo = true after remsudo, full settings: %#v", got)
	}

	remDevCtx := newModuleMessageContext(bot, chat, owner, "/remdev 42")
	if err := devsModule.remDev(bot, remDevCtx); err != ext.ContinueGroups {
		t.Fatalf("remDev() error = %v, want ContinueGroups", err)
	}
	if got := devs.GetTeamMemInfo(42); got.IsDev {
		t.Fatalf("IsDev = true after remdev, full settings: %#v", got)
	}
}

func TestDevCommandsRejectNonTeamUsersWithoutTelegramCalls(t *testing.T) {
	withOwnerID(t, 777000)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Guest"}
	guest := gotgbot.User{Id: 42, FirstName: "Guest"}

	ctx := newModuleMessageContext(bot, chat, guest, "/chatinfo -100123")
	if err := devsModule.chatInfo(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("chatInfo() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("getChat"); len(calls) != 0 {
		t.Fatalf("getChat calls = %d, want none for unauthorized user", len(calls))
	}
}

func TestDevChatInfoLeaveChatStatsAndTeamList(t *testing.T) {
	withOwnerID(t, 777000)
	if err := devs.AddDev(42); err != nil {
		t.Fatalf("AddDev() error = %v", err)
	}
	if err := devs.AddSudo(43); err != nil {
		t.Fatalf("AddSudo() error = %v", err)
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Owner"}
	owner := gotgbot.User{Id: 777000, FirstName: "Owner"}

	chatInfoCtx := newModuleMessageContext(bot, chat, owner, "/chatinfo -100123")
	if err := devsModule.chatInfo(bot, chatInfoCtx); err != ext.ContinueGroups {
		t.Fatalf("chatInfo() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("getChatMemberCount"); len(calls) != 1 {
		t.Fatalf("getChatMemberCount calls = %d, want 1", len(calls))
	}

	leaveCtx := newModuleMessageContext(bot, chat, owner, "/leavechat -100123")
	if err := devsModule.leaveChat(bot, leaveCtx); err != ext.ContinueGroups {
		t.Fatalf("leaveChat() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("leaveChat"); len(calls) != 1 {
		t.Fatalf("leaveChat calls = %d, want 1", len(calls))
	}

	teamCtx := newModuleMessageContext(bot, chat, owner, "/teamusers")
	if err := devsModule.listTeam(bot, teamCtx); err != ext.EndGroups {
		t.Fatalf("listTeam() error = %v, want EndGroups", err)
	}

	statsCtx := newModuleMessageContext(bot, chat, owner, "/stats")
	if err := devsModule.getStats(bot, statsCtx); err != ext.ContinueGroups {
		t.Fatalf("getStats() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want 1 stats edit", len(calls))
	}
}

func TestDevChatListSendsDocument(t *testing.T) {
	withOwnerID(t, 777000)
	if err := db.DB.Create(&db.Chat{ChatId: uniqueModuleChatID(), ChatName: "Active", IsInactive: false}).Error; err != nil {
		t.Fatalf("Create active chat failed: %v", err)
	}
	if err := db.DB.Create(&db.Chat{ChatId: uniqueModuleChatID(), ChatName: "Inactive", IsInactive: true}).Error; err != nil {
		t.Fatalf("Create inactive chat failed: %v", err)
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Owner"}
	owner := gotgbot.User{Id: 777000, FirstName: "Owner"}

	ctx := newModuleMessageContext(bot, chat, owner, "/chatlist")
	if err := devsModule.chatList(bot, ctx); err != ext.EndGroups {
		t.Fatalf("chatList() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendDocument"); len(calls) != 1 {
		t.Fatalf("sendDocument calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want status message deletion", len(calls))
	}
}
