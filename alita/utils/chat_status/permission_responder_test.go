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

func TestPermissionRespondReturnsFalseOnNilCtx(t *testing.T) {
	r := NewPermissionResponder(&gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}})
	if r.Respond(nil, "key", "btn") != false {
		t.Fatal("Respond(nil) should return false")
	}
}

func TestPermissionRespondReturnsFalseOnNilMsg(t *testing.T) {
	r := NewPermissionResponder(&gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}})
	ctx := &ext.Context{} // empty context with nil EffectiveMessage
	if r.Respond(ctx, "key", "btn") != false {
		t.Fatal("Respond(ctx with nil EffectiveMessage) should return false")
	}
}
