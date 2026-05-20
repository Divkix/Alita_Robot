package chat_status

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func makeCtxWithMessage(chatType string) *ext.Context {
	msg := &gotgbot.Message{Chat: gotgbot.Chat{Type: chatType}}
	bot := &gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}}
	return ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)
}

func TestRequireGroupPure(t *testing.T) {
	tests := []struct {
		name     string
		chatType string
		want     bool
	}{
		{"private chat", "private", false},
		{"group chat", "group", true},
		{"supergroup chat", "supergroup", true},
		{"channel chat", "channel", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chat := &gotgbot.Chat{Type: tt.chatType}
			got := requireGroupPure(nil, nil, chat)
			if got != tt.want {
				t.Fatalf("requireGroupPure(%q) = %v, want %v", tt.chatType, got, tt.want)
			}
		})
	}
}

func TestRequirePrivatePure(t *testing.T) {
	tests := []struct {
		name     string
		chatType string
		want     bool
	}{
		{"private chat", "private", true},
		{"group chat", "group", false},
		{"supergroup chat", "supergroup", false},
		{"channel chat", "channel", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chat := &gotgbot.Chat{Type: tt.chatType}
			got := requirePrivatePure(nil, nil, chat)
			if got != tt.want {
				t.Fatalf("requirePrivatePure(%q) = %v, want %v", tt.chatType, got, tt.want)
			}
		})
	}
}

func TestRequireGroupPure_NilChat(t *testing.T) {
	ctx := makeCtxWithMessage("group")
	// When chat is nil, extractChatFromContext pulls from ctx's embedded Update.Message.Chat
	if !requireGroupPure(nil, ctx, nil) {
		t.Fatal("requireGroupPure(nil, ctxWithGroup, nil) should be true")
	}
}

func TestRequirePrivatePure_NilChat(t *testing.T) {
	ctx := makeCtxWithMessage("private")
	if !requirePrivatePure(nil, ctx, nil) {
		t.Fatal("requirePrivatePure(nil, ctxWithPrivate, nil) should be true")
	}
}

func TestRequireGroupPure_NilContextAndChat(t *testing.T) {
	if requireGroupPure(nil, nil, nil) {
		t.Fatal("requireGroupPure(nil, nil, nil) should be false")
	}
}

func TestRequirePrivatePure_NilContextAndChat(t *testing.T) {
	if requirePrivatePure(nil, nil, nil) {
		t.Fatal("requirePrivatePure(nil, nil, nil) should be false")
	}
}

func TestIsBotAdminPure_NilBot(t *testing.T) {
	ctx := makeCtxWithMessage("private")
	// Private chats always return true from IsBotAdmin.
	if !isBotAdminPure(nil, ctx, nil) {
		t.Fatal("isBotAdminPure(nil, privateCtx, nil) should be true for private chats")
	}
}

func TestRequireBotAdminPure_NilBot(t *testing.T) {
	ctx := makeCtxWithMessage("private")
	if !requireBotAdminPure(nil, ctx, nil) {
		t.Fatal("requireBotAdminPure(nil, privateCtx, nil) should be true for private chats")
	}
}

func TestRequireUserOwnerPure_NilChat(t *testing.T) {
	if requireUserOwnerPure(nil, nil, nil, 12345) {
		t.Fatal("requireUserOwnerPure(nil, nil, nil, user) should be false")
	}
}

func TestCanBotRestrict_NilBotAndChat(t *testing.T) {
	if canBotRestrict(nil, nil, nil) {
		t.Fatal("canBotRestrict(nil, nil, nil) should be false")
	}
}

func TestCanBotPromote_NilBotAndChat(t *testing.T) {
	if canBotPromote(nil, nil, nil) {
		t.Fatal("canBotPromote(nil, nil, nil) should be false")
	}
}

func TestCanBotPin_NilBotAndChat(t *testing.T) {
	if canBotPin(nil, nil, nil) {
		t.Fatal("canBotPin(nil, nil, nil) should be false")
	}
}

func TestCanBotDelete_NilBotAndChat(t *testing.T) {
	if canBotDelete(nil, nil, nil) {
		t.Fatal("canBotDelete(nil, nil, nil) should be false")
	}
}

func TestCanUserChangeInfo_NilBotAndChat(t *testing.T) {
	if canUserChangeInfo(nil, nil, nil, 1) {
		t.Fatal("canUserChangeInfo(nil, nil, nil, 1) should be false")
	}
}

func TestCanUserRestrict_NilBotAndChat(t *testing.T) {
	if canUserRestrict(nil, nil, nil, 1) {
		t.Fatal("canUserRestrict(nil, nil, nil, 1) should be false")
	}
}

func TestCanUserPromote_NilBotAndChat(t *testing.T) {
	if canUserPromote(nil, nil, nil, 1) {
		t.Fatal("canUserPromote(nil, nil, nil, 1) should be false")
	}
}

func TestCanUserPin_NilBotAndChat(t *testing.T) {
	if canUserPin(nil, nil, nil, 1) {
		t.Fatal("canUserPin(nil, nil, nil, 1) should be false")
	}
}

func TestCanUserDelete_NilBotAndChat(t *testing.T) {
	if canUserDelete(nil, nil, nil, 1) {
		t.Fatal("canUserDelete(nil, nil, nil, 1) should be false")
	}
}

func TestIsValidUserId(t *testing.T) {
	if !IsValidUserId(1) {
		t.Fatal("IsValidUserId(1) should be true")
	}
	if IsValidUserId(0) {
		t.Fatal("IsValidUserId(0) should be false")
	}
	if IsValidUserId(-1) {
		t.Fatal("IsValidUserId(-1) should be false")
	}
}

func TestIsChannelId(t *testing.T) {
	if !IsChannelId(-1001234567890) {
		t.Fatal("IsChannelId(-1001234567890) should be true")
	}
	if IsChannelId(-1) {
		t.Fatal("IsChannelId(-1) should be false")
	}
	if IsChannelId(1) {
		t.Fatal("IsChannelId(1) should be false")
	}
}
