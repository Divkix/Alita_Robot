package chat_status

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestWithReplySetsUseReply(t *testing.T) {
	var cfg respondCfg
	opt := WithReply()
	opt(&cfg)

	if !cfg.useReply {
		t.Fatal("WithReply() should set useReply to true")
	}
	if cfg.fallbackToSendMessage {
		t.Fatal("WithReply() should leave fallbackToSendMessage as false")
	}
}

func TestWithReplyFallbackSetsBoth(t *testing.T) {
	var cfg respondCfg
	opt := WithReplyFallback()
	opt(&cfg)

	if !cfg.useReply {
		t.Fatal("WithReplyFallback() should set useReply to true")
	}
	if !cfg.fallbackToSendMessage {
		t.Fatal("WithReplyFallback() should set fallbackToSendMessage to true")
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
