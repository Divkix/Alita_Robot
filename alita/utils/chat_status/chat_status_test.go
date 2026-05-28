package chat_status

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

func TestIsApproved(t *testing.T) {
	skipIfNoDb(t)

	chatID := int64(-999999999900000)

	t.Cleanup(func() {
		_ = db.RemoveAllApprovedUsers(chatID)
	})

	// Mock bot pointer - IsApproved doesn't actually use it, just needs non-nil
	// We pass nil intentionally since IsApproved delegates to db.IsUserApproved which ignores bot
	got := IsApproved(nil, chatID, 1001)
	if got != false {
		t.Fatalf("IsApproved(nil, chat, unapproved) = %v, want false", got)
	}

	// Approve user and verify
	if err := db.AddApprovedUser(chatID, 1001, 1, "test"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	got = IsApproved(nil, chatID, 1001)
	if got != true {
		t.Fatalf("IsApproved(nil, chat, approved) = %v, want true", got)
	}
}

func TestCanUserPromote(t *testing.T) {
	bot := newChatStatusBot(999)
	chat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Permission Chat"}
	ctx := makeCtxWithMessage("supergroup")

	if !CanUserPromote(bot, ctx, chat, 10) {
		t.Fatal("CanUserPromote(10) should be true for Full Admin")
	}
	if CanUserPromote(bot, ctx, chat, 11) {
		t.Fatal("CanUserPromote(11) should be false for Limited Admin")
	}
}

func TestCanInvite(t *testing.T) {
	bot := newChatStatusBot(999)
	chat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Permission Chat"}

	// Test 1: Nil chat and nil context
	if CanInvite(bot, nil, nil, nil) {
		t.Fatal("CanInvite(nil, nil, nil) should be false")
	}

	// Test 2: Public chat (Username != "")
	publicChat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Username: "public_group"}
	if !CanInvite(bot, nil, publicChat, nil) {
		t.Fatal("CanInvite should be true for public chats")
	}

	// Test 3: Bot lacks CanInviteUsers permission
	limitedBot := newChatStatusBot(998)
	ctx := makeCtxWithMessage("supergroup")
	msg := &gotgbot.Message{From: &gotgbot.User{Id: 10}}
	if CanInvite(limitedBot, ctx, chat, msg) {
		t.Fatal("CanInvite should be false if bot lacks invite permission")
	}

	// Test 4: Bot has invite permission, but msg.From is nil (channel post)
	nilFromMsg := &gotgbot.Message{From: nil}
	if CanInvite(bot, ctx, chat, nilFromMsg) {
		t.Fatal("CanInvite should be false if msg.From is nil")
	}

	// Test 5: Full Admin user (has invite permission)
	fullAdminMsg := &gotgbot.Message{From: &gotgbot.User{Id: 10}}
	if !CanInvite(bot, ctx, chat, fullAdminMsg) {
		t.Fatal("CanInvite should be true for Full Admin user")
	}

	// Test 6: Limited Admin user (lacks invite permission)
	limitedAdminMsg := &gotgbot.Message{From: &gotgbot.User{Id: 11}}
	if CanInvite(bot, ctx, chat, limitedAdminMsg) {
		t.Fatal("CanInvite should be false for Limited Admin user")
	}

	// Test 7: Owner/Creator (allowed without explicit invite flag)
	ownerMsg := &gotgbot.Message{From: &gotgbot.User{Id: 12}}
	if !CanInvite(bot, ctx, chat, ownerMsg) {
		t.Fatal("CanInvite should be true for Owner")
	}
}

func TestOtherExportedFunctions(t *testing.T) {
	bot := newChatStatusBot(999)
	chat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Permission Chat"}
	ctx := makeCtxWithMessage("supergroup")

	// 1. CanUserChangeInfo
	if !CanUserChangeInfo(bot, ctx, chat, 10) {
		t.Error("CanUserChangeInfo(10) should be true")
	}

	// 2. CanUserRestrict
	if !CanUserRestrict(bot, ctx, chat, 10) {
		t.Error("CanUserRestrict(10) should be true")
	}

	// 3. CanBotRestrict
	if !CanBotRestrict(bot, ctx, chat) {
		t.Error("CanBotRestrict() should be true")
	}

	// 4. CanBotPromote
	if !CanBotPromote(bot, ctx, chat) {
		t.Error("CanBotPromote() should be true")
	}

	// 5. CanUserPin
	if !CanUserPin(bot, ctx, chat, 10) {
		t.Error("CanUserPin(10) should be true")
	}

	// 6. CanBotPin
	if !CanBotPin(bot, ctx, chat) {
		t.Error("CanBotPin() should be true")
	}

	// 7. CanUserDelete
	if !CanUserDelete(bot, ctx, chat, 10) {
		t.Error("CanUserDelete(10) should be true")
	}

	// 8. CanBotDelete
	if !CanBotDelete(bot, ctx, chat) {
		t.Error("CanBotDelete() should be true")
	}

	// 9. RequireBotAdmin
	if !RequireBotAdmin(bot, ctx, chat) {
		t.Error("RequireBotAdmin() should be true")
	}

	// 10. RequireUserAdmin
	if !RequireUserAdmin(bot, ctx, chat, 10) {
		t.Error("RequireUserAdmin(10) should be true")
	}

	// 11. RequireUserOwner
	if !RequireUserOwner(bot, ctx, chat, 12) {
		t.Error("RequireUserOwner(12) should be true")
	}

	// 12. RequirePrivate
	privateCtx := makeCtxWithMessage("private")
	privateChat := &gotgbot.Chat{Type: "private"}
	if !RequirePrivate(bot, privateCtx, privateChat) {
		t.Error("RequirePrivate() should be true for private chat")
	}

	// 13. RequireGroup
	if !RequireGroup(bot, ctx, chat) {
		t.Error("RequireGroup() should be true for supergroup")
	}

	// 14. GetEffectiveUser & RequireUser
	if got := GetEffectiveUser(ctx); got == nil || got.Id != 42 {
		t.Errorf("GetEffectiveUser() = %v, want User 42", got)
	}
	if got := RequireUser(bot, ctx); got == nil || got.Id != 42 {
		t.Errorf("RequireUser() = %v, want User 42", got)
	}
	if got := GetEffectiveUser(nil); got != nil {
		t.Errorf("GetEffectiveUser(nil) = %v, want nil", got)
	}

	// 15. IsBotAdmin
	if !IsBotAdmin(bot, ctx, chat) {
		t.Error("IsBotAdmin() should be true for bot 999 in chat -1001")
	}
}


