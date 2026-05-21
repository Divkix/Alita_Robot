//go:build testtools

package helpers

import (
	"errors"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestBuildCommandContextNilUser(t *testing.T) {
	bot := &gotgbot.Bot{Token: "test"}
	ctx := ext.NewContext(bot, &gotgbot.Update{
		UpdateId: 1,
		Message: &gotgbot.Message{
			MessageId: 1,
			Chat:      gotgbot.Chat{Id: -1001, Type: "supergroup"},
		},
	}, nil)

	c, err := BuildCommandContext(bot, ctx)
	if c != nil {
		t.Fatalf("expected nil CommandContext, got %v", c)
	}
	if !errors.Is(err, ext.EndGroups) {
		t.Fatalf("expected ext.EndGroups, got %v", err)
	}
}

func TestBuildCommandContextSuccess(t *testing.T) {
	bot := &gotgbot.Bot{Token: "test"}
	user := &gotgbot.User{Id: 42, FirstName: "Test"}
	chat := gotgbot.Chat{Id: -1001, Type: "supergroup"}
	ctx := ext.NewContext(bot, &gotgbot.Update{
		UpdateId: 1,
		Message: &gotgbot.Message{
			MessageId: 1,
			From:      user,
			Chat:      chat,
		},
	}, nil)

	c, err := BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil CommandContext")
	}
	if c.Bot != bot {
		t.Fatal("Bot mismatch")
	}
	if c.Ctx != ctx {
		t.Fatal("Ctx mismatch")
	}
	if c.Chat == nil || c.Chat.Id != chat.Id {
		t.Fatal("Chat mismatch")
	}
	if c.Msg == nil || c.Msg.MessageId != 1 {
		t.Fatal("Msg mismatch")
	}
	if c.User != user {
		t.Fatal("User mismatch")
	}
	if c.Tr == nil {
		t.Fatal("expected non-nil Translator")
	}
}

func TestCheckFuncNilGuards(t *testing.T) {
	tests := []struct {
		name  string
		check CheckFunc
		c     *CommandContext
	}{
		{
			name:  "CheckDisabled with nil Msg",
			check: CheckDisabled("test"),
			c: &CommandContext{
				Bot: &gotgbot.Bot{Token: "test"},
				Msg: nil,
			},
		},
		{
			name:  "CheckDisabled with nil Bot",
			check: CheckDisabled("test"),
			c: &CommandContext{
				Bot: nil,
				Msg: &gotgbot.Message{},
			},
		},
		{
			name:  "RequireUserAdmin with nil User",
			check: RequireUserAdmin(),
			c: &CommandContext{
				Bot:  &gotgbot.Bot{Token: "test"},
				User: nil,
			},
		},
		{
			name:  "RequireUserOwner with nil User",
			check: RequireUserOwner(),
			c: &CommandContext{
				Bot:  &gotgbot.Bot{Token: "test"},
				User: nil,
			},
		},
		{
			name:  "CanUserPromote with nil User",
			check: CanUserPromote(),
			c: &CommandContext{
				Bot:  &gotgbot.Bot{Token: "test"},
				User: nil,
			},
		},
		{
			name:  "CanUserRestrict with nil User",
			check: CanUserRestrict(),
			c: &CommandContext{
				Bot:  &gotgbot.Bot{Token: "test"},
				User: nil,
			},
		},
		{
			name:  "CanUserPin with nil User",
			check: CanUserPin(),
			c: &CommandContext{
				Bot:  &gotgbot.Bot{Token: "test"},
				User: nil,
			},
		},
		{
			name:  "CanUserChangeInfo with nil User",
			check: CanUserChangeInfo(),
			c: &CommandContext{
				Bot:  &gotgbot.Bot{Token: "test"},
				User: nil,
			},
		},
		{
			name:  "CanUserDelete with nil User",
			check: CanUserDelete(),
			c: &CommandContext{
				Bot:  &gotgbot.Bot{Token: "test"},
				User: nil,
			},
		},
		{
			name:  "CanInvite with nil Msg",
			check: CanInvite(),
			c: &CommandContext{
				Bot: &gotgbot.Bot{Token: "test"},
				Msg: nil,
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.check(tc.c); got != false {
				t.Fatalf("%s returned %v, want false", tc.name, got)
			}
		})
	}
}
