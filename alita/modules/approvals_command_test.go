package modules

import (
	"errors"
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

	statusMissingCtx := newModuleMessageContext(bot, chat, admin, "/approval 43")
	if err := approvalsModule.checkApprovalStatus(bot, statusMissingCtx); err != ext.EndGroups {
		t.Fatalf("checkApprovalStatus missing approval error = %v, want EndGroups", err)
	}
}

func TestApprovedListHandlesEmptyAndLargeLists(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Approval Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	emptyCtx := newModuleMessageContext(bot, chat, admin, "/approved")
	if err := approvalsModule.listApprovedUsers(bot, emptyCtx); err != ext.EndGroups {
		t.Fatalf("listApprovedUsers empty error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want empty-list reply", len(calls))
	}

	for i := 0; i < approvedUsersInlineLimit+1; i++ {
		userID := int64(10_000 + i)
		if err := db.AddApprovedUser(chat.Id, userID, admin.Id, "bulk reason"); err != nil {
			t.Fatalf("AddApprovedUser(%d) error = %v", userID, err)
		}
	}

	largeCtx := newModuleMessageContext(bot, chat, admin, "/approved")
	if err := approvalsModule.listApprovedUsers(bot, largeCtx); err != ext.EndGroups {
		t.Fatalf("listApprovedUsers large error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendDocument"); len(calls) != 1 {
		t.Fatalf("sendDocument calls = %d, want large-list document", len(calls))
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

func TestUnapproveAllCallbackCancelInvalidAndUnavailableMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Approval Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddApprovedUser(chat.Id, 42, owner.Id, "keep"); err != nil {
		t.Fatalf("AddApprovedUser setup error = %v", err)
	}

	cancelCtx := newModuleCallbackContext(bot, chat, owner, "rmAllApprovals.no")
	if err := approvalsModule.unapproveAllCallback(bot, cancelCtx); err != ext.EndGroups {
		t.Fatalf("unapproveAllCallback cancel error = %v, want EndGroups", err)
	}
	if !db.IsUserApproved(chat.Id, 42) {
		t.Fatal("cancel callback removed approved user")
	}

	invalidCtx := newModuleCallbackContext(bot, chat, owner, "rmAllApprovals")
	if err := approvalsModule.unapproveAllCallback(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("unapproveAllCallback invalid error = %v, want EndGroups", err)
	}

	defaultCtx := newModuleCallbackContext(bot, chat, owner, "rmAllApprovals.maybe")
	if err := approvalsModule.unapproveAllCallback(bot, defaultCtx); err != ext.EndGroups {
		t.Fatalf("unapproveAllCallback default error = %v, want EndGroups", err)
	}

	missingMessageCtx := newModuleCallbackContext(bot, chat, owner, "rmAllApprovals.yes")
	missingMessageCtx.CallbackQuery.Message = nil
	if err := approvalsModule.unapproveAllCallback(bot, missingMessageCtx); err != ext.EndGroups {
		t.Fatalf("unapproveAllCallback missing message error = %v, want EndGroups", err)
	}
	if !db.IsUserApproved(chat.Id, 42) {
		t.Fatal("missing-message callback removed approved user")
	}

	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want cancel, invalid, and default answers", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 2 {
		t.Fatalf("editMessageText calls = %d, want cancel and default edits", len(calls))
	}
}

func TestApprovalCommandsPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name   string
		text   string
		method string
		setup  func(t *testing.T, chat gotgbot.Chat)
		run    func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "approve missing target reply", text: "/approve", method: "sendMessage", run: approvalsModule.approveUser},
		{name: "approve success reply", text: "/approve 42 trusted", method: "sendMessage", run: approvalsModule.approveUser},
		{
			name:   "approve duplicate reply",
			text:   "/approve 42 again",
			method: "sendMessage",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := db.AddApprovedUser(chat.Id, 42, admin.Id, "already"); err != nil {
					t.Fatalf("AddApprovedUser setup error = %v", err)
				}
			},
			run: approvalsModule.approveUser,
		},
		{name: "unapprove missing target reply", text: "/unapprove", method: "sendMessage", run: approvalsModule.unapproveUser},
		{name: "unapprove not approved reply", text: "/unapprove 42", method: "sendMessage", run: approvalsModule.unapproveUser},
		{
			name:   "unapprove success reply",
			text:   "/unapprove 42",
			method: "sendMessage",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := db.AddApprovedUser(chat.Id, 42, admin.Id, "trusted"); err != nil {
					t.Fatalf("AddApprovedUser setup error = %v", err)
				}
			},
			run: approvalsModule.unapproveUser,
		},
		{name: "approval missing target reply", text: "/approval", method: "sendMessage", run: approvalsModule.checkApprovalStatus},
		{name: "approval not approved reply", text: "/approval 42", method: "sendMessage", run: approvalsModule.checkApprovalStatus},
		{
			name:   "approval status reply",
			text:   "/approval 42",
			method: "sendMessage",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := db.AddApprovedUser(chat.Id, 42, admin.Id, "trusted"); err != nil {
					t.Fatalf("AddApprovedUser setup error = %v", err)
				}
			},
			run: approvalsModule.checkApprovalStatus,
		},
		{name: "approved empty list reply", text: "/approved", method: "sendMessage", run: approvalsModule.listApprovedUsers},
		{
			name:   "approved inline list reply",
			text:   "/approved",
			method: "sendMessage",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := db.AddApprovedUser(chat.Id, 42, admin.Id, "trusted"); err != nil {
					t.Fatalf("AddApprovedUser setup error = %v", err)
				}
			},
			run: approvalsModule.listApprovedUsers,
		},
		{
			name:   "approved large list document",
			text:   "/approved",
			method: "sendDocument",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				for i := 0; i < approvedUsersInlineLimit+1; i++ {
					if err := db.AddApprovedUser(chat.Id, int64(10_000+i), admin.Id, "bulk"); err != nil {
						t.Fatalf("AddApprovedUser setup error = %v", err)
					}
				}
			},
			run: approvalsModule.listApprovedUsers,
		},
		{name: "unapprove all confirmation reply", text: "/unapproveall", method: "sendMessage", run: approvalsModule.unapproveAllHandler},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Approval Chat"}
			if tt.setup != nil {
				tt.setup(t, chat)
			}
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestApprovalCallbackHandlersPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	noCallbackClient := newModuleBotClient()
	noCallbackBot := newModuleTestBot(noCallbackClient)
	noCallbackChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Approval Chat"}
	noCallbackCtx := newModuleMessageContext(noCallbackBot, noCallbackChat, owner, "/unapproveall")
	if err := approvalsModule.unapproveAllCallback(noCallbackBot, noCallbackCtx); err != ext.EndGroups {
		t.Fatalf("unapproveAllCallback(no callback) error = %v, want EndGroups", err)
	}
	if calls := noCallbackClient.callsFor("answerCallbackQuery"); len(calls) != 0 {
		t.Fatalf("answerCallbackQuery calls = %d, want none without callback", len(calls))
	}

	for _, tt := range []struct {
		name   string
		method string
		data   string
	}{
		{
			name:   "edit failure",
			method: "editMessageText",
			data:   encodeCallbackData("rmAllApprovals", map[string]string{"a": "yes"}, "rmAllApprovals.yes"),
		},
		{
			name:   "answer failure",
			method: "answerCallbackQuery",
			data:   encodeCallbackData("rmAllApprovals", map[string]string{"a": "no"}, "rmAllApprovals.no"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Approval Chat"}
			if err := db.AddApprovedUser(chat.Id, 42, owner.Id, "trusted"); err != nil {
				t.Fatalf("AddApprovedUser setup error = %v", err)
			}
			ctx := newModuleCallbackContext(bot, chat, owner, tt.data)

			err := approvalsModule.unapproveAllCallback(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("unapproveAllCallback() error = %v, want request error", err)
			}
		})
	}
}
