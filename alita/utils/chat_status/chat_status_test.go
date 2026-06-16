package chat_status

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
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
		_ = approvals.RemoveAllApprovedUsers(chatID)
	})

	// Mock bot pointer - IsApproved doesn't actually use it, just needs non-nil
	// We pass nil intentionally since IsApproved delegates to approvals.IsUserApproved which ignores bot
	got := IsApproved(nil, chatID, 1001)
	if got != false {
		t.Fatalf("IsApproved(nil, chat, unapproved) = %v, want false", got)
	}

	// Approve user and verify
	if err := approvals.AddApprovedUser(chatID, 1001, 1, "test"); err != nil {
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

func TestGetMessageLinkFromMessageId(t *testing.T) {
	t.Parallel()

	t.Run("supergroup without username", func(t *testing.T) {
		t.Parallel()
		chat := &gotgbot.Chat{
			Id:       -1001234567890,
			Username: "",
		}
		link := GetMessageLinkFromMessageId(chat, 42)
		expected := "https://t.me/c/1234567890/42"
		if link != expected {
			t.Fatalf("expected %q, got %q", expected, link)
		}
	})

	t.Run("supergroup with username", func(t *testing.T) {
		t.Parallel()
		chat := &gotgbot.Chat{
			Id:       -1001234567890,
			Username: "mychannel",
		}
		link := GetMessageLinkFromMessageId(chat, 10)
		expected := "https://t.me/mychannel/10"
		if link != expected {
			t.Fatalf("expected %q, got %q", expected, link)
		}
	})

	t.Run("messageID 0 produces valid link", func(t *testing.T) {
		t.Parallel()
		chat := &gotgbot.Chat{
			Id:       -1001234567890,
			Username: "",
		}
		link := GetMessageLinkFromMessageId(chat, 0)
		if !strings.HasPrefix(link, "https://t.me/c/") {
			t.Fatalf("expected link starting with 'https://t.me/c/', got %q", link)
		}
	})
}

func TestExtractJoinLeftStatusChange(t *testing.T) {
	t.Parallel()

	t.Run("join event — left to member", func(t *testing.T) {
		t.Parallel()
		u := &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Type: "supergroup"},
			OldChatMember: gotgbot.ChatMemberLeft{},
			NewChatMember: gotgbot.ChatMemberMember{},
		}
		wasMember, isMember := ExtractJoinLeftStatusChange(u)
		if wasMember {
			t.Fatal("expected wasMember=false for left->member transition")
		}
		if !isMember {
			t.Fatal("expected isMember=true for left->member transition")
		}
	})

	t.Run("left event — member to left", func(t *testing.T) {
		t.Parallel()
		u := &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Type: "supergroup"},
			OldChatMember: gotgbot.ChatMemberMember{},
			NewChatMember: gotgbot.ChatMemberLeft{},
		}
		wasMember, isMember := ExtractJoinLeftStatusChange(u)
		if !wasMember {
			t.Fatal("expected wasMember=true for member->left transition")
		}
		if isMember {
			t.Fatal("expected isMember=false for member->left transition")
		}
	})

	t.Run("channel — returns false,false", func(t *testing.T) {
		t.Parallel()
		u := &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Type: "channel"},
			OldChatMember: gotgbot.ChatMemberLeft{},
			NewChatMember: gotgbot.ChatMemberMember{},
		}
		wasMember, isMember := ExtractJoinLeftStatusChange(u)
		if wasMember || isMember {
			t.Fatal("expected (false,false) for channel updates")
		}
	})

	t.Run("no status change — same status", func(t *testing.T) {
		t.Parallel()
		u := &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Type: "supergroup"},
			OldChatMember: gotgbot.ChatMemberMember{},
			NewChatMember: gotgbot.ChatMemberMember{},
		}
		wasMember, isMember := ExtractJoinLeftStatusChange(u)
		if wasMember || isMember {
			t.Fatal("expected (false,false) when status does not change")
		}
	})
}

func TestExtractAdminUpdateStatusChange(t *testing.T) {
	t.Parallel()

	t.Run("promotion — member to administrator", func(t *testing.T) {
		t.Parallel()
		u := &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Type: "supergroup"},
			OldChatMember: gotgbot.ChatMemberMember{},
			NewChatMember: gotgbot.ChatMemberAdministrator{},
		}
		if !ExtractAdminUpdateStatusChange(u) {
			t.Fatal("expected true for member->administrator promotion")
		}
	})

	t.Run("demotion — administrator to member", func(t *testing.T) {
		t.Parallel()
		u := &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Type: "supergroup"},
			OldChatMember: gotgbot.ChatMemberAdministrator{},
			NewChatMember: gotgbot.ChatMemberMember{},
		}
		if !ExtractAdminUpdateStatusChange(u) {
			t.Fatal("expected true for administrator->member demotion")
		}
	})

	t.Run("channel — returns false", func(t *testing.T) {
		t.Parallel()
		u := &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Type: "channel"},
			OldChatMember: gotgbot.ChatMemberMember{},
			NewChatMember: gotgbot.ChatMemberAdministrator{},
		}
		if ExtractAdminUpdateStatusChange(u) {
			t.Fatal("expected false for channel admin updates")
		}
	})

	t.Run("no admin change — member to left", func(t *testing.T) {
		t.Parallel()
		u := &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Type: "supergroup"},
			OldChatMember: gotgbot.ChatMemberMember{},
			NewChatMember: gotgbot.ChatMemberLeft{},
		}
		if ExtractAdminUpdateStatusChange(u) {
			t.Fatal("expected false for member->left — no admin change")
		}
	})
}

// TestIsBotAdminUsesCacheAndStatus verifies that IsBotAdmin correctly reports
// true when the bot is an administrator and false when it is a regular member.
// After Step 2 the code path goes through getUserMemberWithCache which falls
// back to a live getChatMember on a cache miss — the fake bot client simulates
// both outcomes.
func TestIsBotAdminUsesCacheAndStatus(t *testing.T) {
	chat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Permission Chat"}

	// Bot ID 999 → chatStatusBotClient returns status "administrator" → want true.
	adminBot := newChatStatusBot(999)
	adminCtx := makeCtxForBot(adminBot, "supergroup")
	if !IsBotAdmin(adminBot, adminCtx, chat) {
		t.Error("IsBotAdmin(adminBot) = false, want true for administrator status")
	}

	// Bot ID 100 → chatStatusBotClient default case returns status "member" → want false.
	memberBot := &gotgbot.Bot{
		Token:     "100:test",
		BotClient: chatStatusBotClient{},
		User:      gotgbot.User{Id: 100, IsBot: true, FirstName: "MemberBot"},
	}
	memberCtx := makeCtxForBot(memberBot, "supergroup")
	if IsBotAdmin(memberBot, memberCtx, chat) {
		t.Error("IsBotAdmin(memberBot) = true, want false for member status")
	}

	// Private chat → always true regardless of bot status.
	privateChat := &gotgbot.Chat{Id: 42, Type: "private"}
	privateCtx := makeCtxForBot(memberBot, "private")
	if !IsBotAdmin(memberBot, privateCtx, privateChat) {
		t.Error("IsBotAdmin(private chat) = false, want true (private always true)")
	}
}

// makeCtxForBot creates a minimal ext.Context whose message chat type matches chatType.
func makeCtxForBot(b *gotgbot.Bot, chatType string) *ext.Context {
	msg := &gotgbot.Message{
		MessageId: 200,
		Date:      1,
		Chat:      gotgbot.Chat{Id: -1001, Type: chatType, Title: "Permission Chat"},
		From:      &gotgbot.User{Id: 42, FirstName: "Member"},
	}
	return ext.NewContext(b, &gotgbot.Update{Message: msg}, nil)
}
