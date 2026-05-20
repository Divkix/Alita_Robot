package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func TestApproveApprovalListAndUnapproveCommands(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Approval Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	approveCtx := newModuleMessageContext(bot, chat, admin, "/approve 42 trusted")
	if err := approvalsModule.approveUser(bot, approveCtx); err != ext.EndGroups {
		t.Fatalf("approveUser error = %v, want EndGroups", err)
	}
	if !db.IsUserApproved(chat.Id, 42) {
		t.Fatal("user was not approved")
	}
	approved := db.GetApprovedUsers(chat.Id)
	if len(approved) != 1 || approved[0].Reason != "trusted" {
		t.Fatalf("approved users = %+v, want reason trusted", approved)
	}

	statusCtx := newModuleMessageContext(bot, chat, admin, "/approval 42")
	if err := approvalsModule.checkApprovalStatus(bot, statusCtx); err != ext.EndGroups {
		t.Fatalf("checkApprovalStatus error = %v, want EndGroups", err)
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/approved")
	if err := approvalsModule.listApprovedUsers(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("listApprovedUsers error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) < 3 {
		t.Fatalf("sendMessage calls = %d, want approve, status, and list", len(calls))
	}
	lastText := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(lastText, "trusted") {
		t.Fatalf("approved list text = %q, want reason", lastText)
	}

	unapproveCtx := newModuleMessageContext(bot, chat, admin, "/unapprove 42")
	if err := approvalsModule.unapproveUser(bot, unapproveCtx); err != ext.EndGroups {
		t.Fatalf("unapproveUser error = %v, want EndGroups", err)
	}
	if db.IsUserApproved(chat.Id, 42) {
		t.Fatal("user stayed approved after /unapprove")
	}
}

func TestApprovalCommandsHandleMissingAndDuplicateUsers(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Approval Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddApprovedUser(chat.Id, 42, admin.Id, "already"); err != nil {
		t.Fatalf("AddApprovedUser setup error = %v", err)
	}

	missingApproveCtx := newModuleMessageContext(bot, chat, admin, "/approve")
	if err := approvalsModule.approveUser(bot, missingApproveCtx); err != ext.EndGroups {
		t.Fatalf("approveUser missing error = %v, want EndGroups", err)
	}

	duplicateCtx := newModuleMessageContext(bot, chat, admin, "/approve 42 again")
	if err := approvalsModule.approveUser(bot, duplicateCtx); err != ext.EndGroups {
		t.Fatalf("approveUser duplicate error = %v, want EndGroups", err)
	}
	if got := len(db.GetApprovedUsers(chat.Id)); got != 1 {
		t.Fatalf("approved users after duplicate = %d, want 1", got)
	}

	notApprovedCtx := newModuleMessageContext(bot, chat, admin, "/unapprove 43")
	if err := approvalsModule.unapproveUser(bot, notApprovedCtx); err != ext.EndGroups {
		t.Fatalf("unapproveUser missing approval error = %v, want EndGroups", err)
	}
}

func TestUnapproveAllConfirmationAndCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Approval Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddApprovedUser(chat.Id, 42, owner.Id, "one"); err != nil {
		t.Fatalf("AddApprovedUser setup error = %v", err)
	}
	if err := db.AddApprovedUser(chat.Id, 43, owner.Id, "two"); err != nil {
		t.Fatalf("AddApprovedUser setup error = %v", err)
	}

	confirmCtx := newModuleMessageContext(bot, chat, owner, "/unapproveall")
	if err := approvalsModule.unapproveAllHandler(bot, confirmCtx); err != ext.EndGroups {
		t.Fatalf("unapproveAllHandler error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want confirmation", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("unapprove all confirmation did not include reply_markup")
	}

	data := encodeCallbackData("rmAllApprovals", map[string]string{"a": "yes"}, "rmAllApprovals.yes")
	callbackCtx := newModuleCallbackContext(bot, chat, owner, data)
	if err := approvalsModule.unapproveAllCallback(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("unapproveAllCallback error = %v, want EndGroups", err)
	}
	if got := len(db.GetApprovedUsers(chat.Id)); got != 0 {
		t.Fatalf("approved users after callback = %d, want none", got)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}
