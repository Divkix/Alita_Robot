package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestRegisterAnonymousAdminHandler(t *testing.T) {
	anonAdminRegistry = make(map[string]AnonymousAdminHandler)

	called := false
	RegisterAnonymousAdminHandler("test", func(b *gotgbot.Bot, ctx *ext.Context) error {
		called = true
		return nil
	})

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	
	err := HandleAnonymousAdmin(bot, ctx, "test")
	if err != nil {
		t.Fatalf("HandleAnonymousAdmin returned error: %v", err)
	}
	if !called {
		t.Fatal("Expected handler to be called")
	}
}

func TestHandleAnonymousAdmin_UnknownCommand(t *testing.T) {
	anonAdminRegistry = make(map[string]AnonymousAdminHandler)

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	
	err := HandleAnonymousAdmin(bot, ctx, "unknown")
	if err == nil {
		t.Fatal("Expected error for unknown command")
	}
}
