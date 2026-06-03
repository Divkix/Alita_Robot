package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestRegisterDeepLinkHandler(t *testing.T) {
	originalRegistry := deepLinkRegistry
	defer func() { deepLinkRegistry = originalRegistry }()
	deepLinkRegistry = make(map[string]DeepLinkHandler)

	called := false
	RegisterDeepLinkHandler("test_", func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
		called = true
		return nil
	})

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	user := &gotgbot.User{}
	
	_ = HandleDeepLink(bot, ctx, user, "test_foo")
	if !called {
		t.Fatal("Expected handler to be called")
	}
}

func TestHandleDeepLink_LongestPrefixMatch(t *testing.T) {
	originalRegistry := deepLinkRegistry
	defer func() { deepLinkRegistry = originalRegistry }()
	deepLinkRegistry = make(map[string]DeepLinkHandler)

	shortCalled := false
	longCalled := false
	
	RegisterDeepLinkHandler("note", func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
		shortCalled = true
		return nil
	})
	
	RegisterDeepLinkHandler("notes_", func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
		longCalled = true
		return nil
	})

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	user := &gotgbot.User{}
	
	_ = HandleDeepLink(bot, ctx, user, "notes_list")
	if shortCalled {
		t.Fatal("Short handler should not be called for 'notes_list'")
	}
	if !longCalled {
		t.Fatal("Long handler should be called for 'notes_list'")
	}
}
