//go:build testtools

package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// ---------------------------------------------------------------------------
// Bot client mock for true-branch coverage
// ---------------------------------------------------------------------------

type cpBotClient struct{}

func (cpBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	switch method {
	case "getChatMember":
		switch fmt.Sprint(params["user_id"]) {
		case "999":
			return json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Bot"},"can_change_info":true,"can_restrict_members":true,"can_promote_members":true,"can_pin_messages":true,"can_delete_messages":true,"can_invite_users":true}`), nil
		case "998":
			return json.RawMessage(`{"status":"administrator","user":{"id":998,"is_bot":true,"first_name":"Limited Bot"},"can_change_info":false,"can_restrict_members":false,"can_promote_members":false,"can_pin_messages":false,"can_delete_messages":false,"can_invite_users":false}`), nil
		default:
			return json.RawMessage(`{"status":"member","user":{"id":42,"is_bot":false,"first_name":"Member"}}`), nil
		}
	case "sendMessage":
		return json.RawMessage(`{"message_id":1,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Test"}}`), nil
	default:
		return json.RawMessage(`true`), nil
	}
}

func (cpBotClient) GetAPIURL(*gotgbot.RequestOpts) string { return "https://api.telegram.org" }
func (cpBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func newCpBot(id int64) *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     fmt.Sprintf("%d:test", id),
		BotClient: cpBotClient{},
		User:      gotgbot.User{Id: id, IsBot: true, FirstName: "Bot"},
	}
}

func makeCpContext(chatType string) *ext.Context {
	msg := &gotgbot.Message{
		MessageId: 1,
		Date:      1,
		Chat:      gotgbot.Chat{Id: -1001, Type: chatType, Title: "Test Chat"},
		From:      &gotgbot.User{Id: 42, FirstName: "Member"},
	}
	return ext.NewContext(newCpBot(999), &gotgbot.Update{Message: msg}, nil)
}

// ---------------------------------------------------------------------------
// True-branch CheckFunc tests
// ---------------------------------------------------------------------------

func TestCheckFuncTrueBranches(t *testing.T) {
	tests := []struct {
		name  string
		check CheckFunc
		c     *CommandContext
		want  bool
	}{
		{
			name:  "RequireGroup in supergroup",
			check: RequireGroup(),
			c:     &CommandContext{Bot: newCpBot(999), Ctx: makeCpContext("supergroup")},
			want:  true,
		},
		{
			name:  "RequireBotAdmin when bot is admin",
			check: RequireBotAdmin(),
			c:     &CommandContext{Bot: newCpBot(999), Ctx: makeCpContext("supergroup")},
			want:  true,
		},
		{
			name:  "CanBotPromote when bot has permission",
			check: CanBotPromote(),
			c:     &CommandContext{Bot: newCpBot(999), Ctx: makeCpContext("supergroup")},
			want:  true,
		},
		{
			name:  "CanBotRestrict when bot has permission",
			check: CanBotRestrict(),
			c:     &CommandContext{Bot: newCpBot(999), Ctx: makeCpContext("supergroup")},
			want:  true,
		},
		{
			name:  "CanBotPin when bot has permission",
			check: CanBotPin(),
			c:     &CommandContext{Bot: newCpBot(999), Ctx: makeCpContext("supergroup")},
			want:  true,
		},
		{
			name:  "CanBotDelete when bot has permission",
			check: CanBotDelete(),
			c:     &CommandContext{Bot: newCpBot(999), Ctx: makeCpContext("supergroup")},
			want:  true,
		},
		{
			name: "CheckDisabled in private chat (always false)",
			check: CheckDisabled("kick"),
			c: &CommandContext{
				Bot: newCpBot(999),
				Msg: &gotgbot.Message{Chat: gotgbot.Chat{Id: 42, Type: "private"}},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.check(tc.c); got != tc.want {
				t.Fatalf("%s returned %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WrapCommandRaw success path
// ---------------------------------------------------------------------------

func TestWrapCommandRawSuccess(t *testing.T) {
	d := ext.NewDispatcher(&ext.DispatcherOpts{})

	called := false
	handler := func(_ *gotgbot.Bot, _ *ext.Context) error {
		called = true
		return nil
	}

	WrapCommandRaw(d, CommandDescriptor{
		Name:           "testraw",
		RequiredChecks: []CheckFunc{RequireGroup()},
	}, handler)

	bot := newCpBot(999)
	update := &gotgbot.Update{
		Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Test"},
			From:     &gotgbot.User{Id: 42, FirstName: "Member"},
			Text:     "/testraw",
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 8}},
		},
	}
	if err := d.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestWrapCommandRawShortCircuitsOnCheckFailure(t *testing.T) {
	d := ext.NewDispatcher(&ext.DispatcherOpts{})

	called := false
	handler := func(_ *gotgbot.Bot, _ *ext.Context) error {
		called = true
		return nil
	}

	 WrapCommandRaw(d, CommandDescriptor{
		Name:           "testrawfail",
		RequiredChecks: []CheckFunc{RequireGroup()},
	}, handler)

	bot := newCpBot(999)
	update := &gotgbot.Update{
		Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: 42, Type: "private", FirstName: "Tester"},
			From:     &gotgbot.User{Id: 42, FirstName: "Member"},
			Text:     "/testrawfail",
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 12}},
		},
	}
	if err := d.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate error: %v", err)
	}
	if called {
		t.Fatal("expected handler NOT to be called when check fails")
	}
}

func TestWrapCommandSuccess(t *testing.T) {
	d := ext.NewDispatcher(&ext.DispatcherOpts{})

	called := false
	var gotC *CommandContext
	handler := func(c *CommandContext) error {
		called = true
		gotC = c
		return nil
	}

	WrapCommand(d, CommandDescriptor{
		Name:           "testwrap",
		RequiredChecks: []CheckFunc{RequireGroup()},
	}, handler)

	bot := newCpBot(999)
	update := &gotgbot.Update{
		Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Test"},
			From:     &gotgbot.User{Id: 42, FirstName: "Member"},
			Text:     "/testwrap",
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 9}},
		},
	}
	if err := d.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
	if gotC == nil {
		t.Fatal("expected non-nil CommandContext in handler")
	}
	if gotC.Bot != bot {
		t.Fatal("CommandContext.Bot mismatch")
	}
}

func TestWrapCommandWithDisableable(t *testing.T) {
	d := ext.NewDispatcher(&ext.DispatcherOpts{})

	handler := func(_ *CommandContext) error {
		return nil
	}

	cmdsMu.Lock()
	orig := make([]string, len(DisableCmds))
	copy(orig, DisableCmds)
	cmdsMu.Unlock()
	defer func() {
		cmdsMu.Lock()
		DisableCmds = orig
		cmdsMu.Unlock()
	}()

	WrapCommand(d, CommandDescriptor{
		Name:           "testdisableable",
		Disableable:    true,
		RequiredChecks: []CheckFunc{},
	}, handler)

	found := false
	cmdsMu.Lock()
	for _, c := range DisableCmds {
		if c == "testdisableable" {
			found = true
			break
		}
	}
	cmdsMu.Unlock()
	if !found {
		t.Fatal("expected testdisableable to be registered as disableable")
	}
}

func TestRegisterWithGroup(t *testing.T) {
	d := ext.NewDispatcher(&ext.DispatcherOpts{})

	called := false
	handler := func(_ *gotgbot.Bot, _ *ext.Context) error {
		called = true
		return nil
	}

	cmdDesc := CommandDescriptor{
		Name:    "groupcmd",
		Group:   5,
		Aliases: []string{"groupcmda"},
	}
	register(d, cmdDesc, handler)

	bot := newCpBot(999)
	for _, text := range []string{"/groupcmd", "/groupcmda"} {
		called = false
		update := &gotgbot.Update{
			Message: &gotgbot.Message{
				Chat:     gotgbot.Chat{Id: 1, Type: "private", FirstName: "T"},
				From:     &gotgbot.User{Id: 1, FirstName: "U"},
				Text:     text,
				Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len(text))}},
			},
		}
		if err := d.ProcessUpdate(bot, update, nil); err != nil {
			t.Fatalf("ProcessUpdate(%s) error: %v", text, err)
		}
		if !called {
			t.Fatalf("expected handler to be called for %s", text)
		}
	}
}
