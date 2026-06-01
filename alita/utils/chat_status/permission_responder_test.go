package chat_status

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestWithReplyOptions(t *testing.T) {
	tests := []struct {
		name                  string
		opt                   func(*respondCfg)
		wantUseReply          bool
		wantFallbackToSendMsg bool
	}{
		{
			name:                  "WithReply",
			opt:                   WithReply(),
			wantUseReply:          true,
			wantFallbackToSendMsg: false,
		},
		{
			name:                  "WithReplyFallback",
			opt:                   WithReplyFallback(),
			wantUseReply:          true,
			wantFallbackToSendMsg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg respondCfg
			tt.opt(&cfg)

			if cfg.useReply != tt.wantUseReply {
				t.Fatalf("useReply = %v, want %v", cfg.useReply, tt.wantUseReply)
			}
			if cfg.fallbackToSendMessage != tt.wantFallbackToSendMsg {
				t.Fatalf("fallbackToSendMessage = %v, want %v", cfg.fallbackToSendMessage, tt.wantFallbackToSendMsg)
			}
		})
	}
}

func TestNewPermissionResponder(t *testing.T) {
	bot := &gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}}
	r := NewPermissionResponder(bot)
	if r == nil {
		t.Fatal("NewPermissionResponder() should return non-nil")
	}
	if r.bot != bot {
		t.Fatal("NewPermissionResponder() bot field mismatch")
	}
}

// TestPermissionResponderRespondNegativeCases verifies that Respond returns false for nil context or nil EffectiveMessage.
func TestPermissionResponderRespondNegativeCases(t *testing.T) {
	tests := []struct {
		name string
		ctx  *ext.Context
	}{
		{name: "nil ctx", ctx: nil},
		{name: "nil EffectiveMessage", ctx: &ext.Context{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewPermissionResponder(&gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}})
			if r.Respond(tt.ctx, "key", "btn") != false {
				t.Fatalf("Respond(%s) should return false", tt.name)
			}
		})
	}
}

func TestPermissionResponderRespond(t *testing.T) {
	bot := newChatStatusBot(999)

	// Test 1: Callback Query path
	t.Run("CallbackQuery", func(t *testing.T) {
		ctx := makeCtxWithCallbackQuery()
		r := NewPermissionResponder(bot)
		// btnKey is non-empty, and update has callback query
		res := r.Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		if res != false {
			t.Fatal("Respond should return false")
		}
	})

	// Test 2: Reply path (WithReply)
	t.Run("WithReply", func(t *testing.T) {
		ctx := makeCtxWithMessage("supergroup")
		r := NewPermissionResponder(bot)
		res := r.Respond(ctx, "chat_status_restrict_cmd_error", "", WithReply())
		if res != false {
			t.Fatal("Respond should return false")
		}
	})

	// Test 3: Reply path with fallback (WithReplyFallback)
	t.Run("WithReplyFallback", func(t *testing.T) {
		ctx := makeCtxWithMessage("supergroup")
		r := NewPermissionResponder(bot)
		res := r.Respond(ctx, "chat_status_restrict_cmd_error", "", WithReplyFallback())
		if res != false {
			t.Fatal("Respond should return false")
		}
	})
}
