package chat_status

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type chatStatusBotClient struct{}

func (chatStatusBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	switch method {
	case "getChatMember":
		switch fmt.Sprint(params["user_id"]) {
		case "10":
			return json.RawMessage(`{"status":"administrator","user":{"id":10,"is_bot":false,"first_name":"Full Admin"},"can_change_info":true,"can_restrict_members":true,"can_promote_members":true,"can_pin_messages":true,"can_delete_messages":true,"can_invite_users":true}`), nil
		case "11":
			return json.RawMessage(`{"status":"administrator","user":{"id":11,"is_bot":false,"first_name":"Limited Admin"},"can_change_info":false,"can_restrict_members":false,"can_promote_members":false,"can_pin_messages":false,"can_delete_messages":false,"can_invite_users":false}`), nil
		case "12":
			return json.RawMessage(`{"status":"creator","user":{"id":12,"is_bot":false,"first_name":"Owner"}}`), nil
		case "999":
			return json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Bot"},"can_restrict_members":true,"can_promote_members":true,"can_pin_messages":true,"can_delete_messages":true,"can_invite_users":true}`), nil
		case "998":
			return json.RawMessage(`{"status":"administrator","user":{"id":998,"is_bot":true,"first_name":"Limited Bot"},"can_restrict_members":false,"can_promote_members":false,"can_pin_messages":false,"can_delete_messages":false,"can_invite_users":false}`), nil
		default:
			return json.RawMessage(`{"status":"member","user":{"id":42,"is_bot":false,"first_name":"Member"}}`), nil
		}
	case "getChat":
		return json.RawMessage(`{"id":-1001,"type":"supergroup","title":"Permission Chat"}`), nil
	case "getChatAdministrators":
		return json.RawMessage(`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Bot"}},{"status":"administrator","user":{"id":10,"is_bot":false,"first_name":"Full Admin"}},{"status":"creator","user":{"id":12,"is_bot":false,"first_name":"Owner"}}]`), nil
	case "sendMessage":
		return json.RawMessage(`{"message_id":1,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Permission Chat"}}`), nil
	case "answerCallbackQuery":
		return json.RawMessage(`true`), nil
	default:
		return json.RawMessage(`true`), nil
	}
}

func (chatStatusBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (chatStatusBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func newChatStatusBot(id int64) *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     "999:test",
		BotClient: chatStatusBotClient{},
		User:      gotgbot.User{Id: id, IsBot: true, FirstName: "Bot"},
	}
}

func makeCtxWithMessage(chatType string) *ext.Context {
	msg := &gotgbot.Message{Chat: gotgbot.Chat{Type: chatType}}
	bot := &gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}}
	return ext.NewContext(bot, &gotgbot.Update{Message: msg}, nil)
}

func TestExtractChatFromContext(t *testing.T) {
	t.Parallel()

	explicit := &gotgbot.Chat{Id: 10, Type: "supergroup"}
	if got := extractChatFromContext(nil, explicit); got != explicit {
		t.Fatal("extractChatFromContext() should prefer explicit chat")
	}

	messageCtx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}},
		&gotgbot.Update{Message: &gotgbot.Message{Chat: gotgbot.Chat{Id: 20, Type: "group"}}},
		nil,
	)
	if got := extractChatFromContext(messageCtx, nil); got == nil || got.Id != 20 {
		t.Fatalf("extractChatFromContext(message) = %#v, want chat id 20", got)
	}

	callbackCtx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}},
		&gotgbot.Update{
			CallbackQuery: &gotgbot.CallbackQuery{
				Message: gotgbot.Message{Chat: gotgbot.Chat{Id: 30, Type: "group"}},
			},
		},
		nil,
	)
	if got := extractChatFromContext(callbackCtx, nil); got == nil || got.Id != 30 {
		t.Fatalf("extractChatFromContext(callback) = %#v, want chat id 30", got)
	}

	myChatMemberCtx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}},
		&gotgbot.Update{
			MyChatMember: &gotgbot.ChatMemberUpdated{
				Chat: gotgbot.Chat{Id: 40, Type: "channel"},
			},
		},
		nil,
	)
	if got := extractChatFromContext(myChatMemberCtx, nil); got == nil || got.Id != 40 {
		t.Fatalf("extractChatFromContext(my_chat_member) = %#v, want chat id 40", got)
	}

	chatMemberCtx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}},
		&gotgbot.Update{
			ChatMember: &gotgbot.ChatMemberUpdated{
				Chat: gotgbot.Chat{Id: 50, Type: "supergroup"},
			},
		},
		nil,
	)
	if got := extractChatFromContext(chatMemberCtx, nil); got == nil || got.Id != 50 {
		t.Fatalf("extractChatFromContext(chat_member) = %#v, want chat id 50", got)
	}

	joinRequestCtx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}},
		&gotgbot.Update{
			ChatJoinRequest: &gotgbot.ChatJoinRequest{
				Chat: gotgbot.Chat{Id: 60, Type: "supergroup"},
			},
		},
		nil,
	)
	if got := extractChatFromContext(joinRequestCtx, nil); got == nil || got.Id != 60 {
		t.Fatalf("extractChatFromContext(chat_join_request) = %#v, want chat id 60", got)
	}

	if got := extractChatFromContext(nil, nil); got != nil {
		t.Fatalf("extractChatFromContext(nil, nil) = %#v, want nil", got)
	}

	if got := extractChatFromContext(&ext.Context{}, nil); got != nil {
		t.Fatalf("extractChatFromContext(ctx with nil update, nil) = %#v, want nil", got)
	}
}

func TestCallbackQueryFromContext(t *testing.T) {
	t.Parallel()

	query := &gotgbot.CallbackQuery{Id: "callback-id"}

	tests := []struct {
		name string
		ctx  *ext.Context
		want *gotgbot.CallbackQuery
		ok   bool
	}{
		{name: "nil context", ctx: nil, ok: false},
		{name: "nil update", ctx: &ext.Context{}, ok: false},
		{name: "nil callback query", ctx: &ext.Context{Update: &gotgbot.Update{}}, ok: false},
		{
			name: "callback query present",
			ctx:  &ext.Context{Update: &gotgbot.Update{CallbackQuery: query}},
			want: query,
			ok:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := callbackQueryFromContext(tc.ctx)
			if ok != tc.ok {
				t.Fatalf("callbackQueryFromContext() ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("callbackQueryFromContext() query = %p, want %p", got, tc.want)
			}
		})
	}
}

func TestHasUserPermissionRejectsMissingContextOrChat(t *testing.T) {
	t.Parallel()

	allow := func(*gotgbot.MergedChatMember) bool { return true }
	if hasUserPermission(nil, nil, &gotgbot.Chat{Id: 1, Type: "group"}, 1, allow) {
		t.Fatal("hasUserPermission() with nil context should be false")
	}

	emptyCtx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 1, IsBot: true}},
		&gotgbot.Update{},
		nil,
	)
	if hasUserPermission(nil, emptyCtx, nil, 1, allow) {
		t.Fatal("hasUserPermission() with no chat in context should be false")
	}
}

func TestPermissionHelpersUseGotgbotMemberPermissions(t *testing.T) {
	bot := newChatStatusBot(999)
	chat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Permission Chat"}
	ctx := makeCtxWithMessage("supergroup")

	tests := []struct {
		name string
		fn   func() bool
	}{
		{name: "change info", fn: func() bool { return canUserChangeInfo(bot, ctx, chat, 10) }},
		{name: "restrict", fn: func() bool { return canUserRestrict(bot, ctx, chat, 10) }},
		{name: "promote", fn: func() bool { return canUserPromote(bot, ctx, chat, 10) }},
		{name: "pin", fn: func() bool { return canUserPin(bot, ctx, chat, 10) }},
		{name: "delete", fn: func() bool { return canUserDelete(bot, ctx, chat, 10) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.fn() {
				t.Fatalf("%s permission = false, want true for full admin", tt.name)
			}
		})
	}
}

func TestPermissionHelpersAllowCreatorWithoutSpecificFlags(t *testing.T) {
	bot := newChatStatusBot(999)
	chat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Permission Chat"}
	ctx := makeCtxWithMessage("supergroup")

	if !canUserRestrict(bot, ctx, chat, 12) {
		t.Fatal("canUserRestrict() = false, want true for creator")
	}
	if !canUserDelete(bot, ctx, chat, 12) {
		t.Fatal("canUserDelete() = false, want true for creator")
	}
	if !requireUserOwnerPure(bot, ctx, chat, 12) {
		t.Fatal("requireUserOwnerPure() = false, want true for creator")
	}
}

func TestPermissionHelpersRejectMissingMemberPermissions(t *testing.T) {
	bot := newChatStatusBot(999)
	chat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Permission Chat"}
	ctx := makeCtxWithMessage("supergroup")

	tests := []struct {
		name string
		fn   func() bool
	}{
		{name: "change info", fn: func() bool { return canUserChangeInfo(bot, ctx, chat, 11) }},
		{name: "restrict", fn: func() bool { return canUserRestrict(bot, ctx, chat, 11) }},
		{name: "promote", fn: func() bool { return canUserPromote(bot, ctx, chat, 11) }},
		{name: "pin", fn: func() bool { return canUserPin(bot, ctx, chat, 11) }},
		{name: "delete", fn: func() bool { return canUserDelete(bot, ctx, chat, 11) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fn() {
				t.Fatalf("%s permission = true, want false for limited admin", tt.name)
			}
		})
	}
}

func TestBotPermissionHelpersUseGotgbotMemberPermissions(t *testing.T) {
	chat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Permission Chat"}
	ctx := makeCtxWithMessage("supergroup")

	fullBot := newChatStatusBot(999)
	fullTests := []struct {
		name string
		fn   func() bool
	}{
		{name: "restrict", fn: func() bool { return canBotRestrict(fullBot, ctx, chat) }},
		{name: "promote", fn: func() bool { return canBotPromote(fullBot, ctx, chat) }},
		{name: "pin", fn: func() bool { return canBotPin(fullBot, ctx, chat) }},
		{name: "delete", fn: func() bool { return canBotDelete(fullBot, ctx, chat) }},
	}
	for _, tt := range fullTests {
		t.Run("full/"+tt.name, func(t *testing.T) {
			if !tt.fn() {
				t.Fatalf("%s bot permission = false, want true", tt.name)
			}
		})
	}

	limitedBot := newChatStatusBot(998)
	limitedTests := []struct {
		name string
		fn   func() bool
	}{
		{name: "restrict", fn: func() bool { return canBotRestrict(limitedBot, ctx, chat) }},
		{name: "promote", fn: func() bool { return canBotPromote(limitedBot, ctx, chat) }},
		{name: "pin", fn: func() bool { return canBotPin(limitedBot, ctx, chat) }},
		{name: "delete", fn: func() bool { return canBotDelete(limitedBot, ctx, chat) }},
	}
	for _, tt := range limitedTests {
		t.Run("limited/"+tt.name, func(t *testing.T) {
			if tt.fn() {
				t.Fatalf("%s bot permission = true, want false", tt.name)
			}
		})
	}
}

func TestRequireUserAdminPureUsesGotgbotAdminList(t *testing.T) {
	bot := newChatStatusBot(999)
	chat := &gotgbot.Chat{Id: -1001, Type: "supergroup", Title: "Permission Chat"}
	ctx := makeCtxWithMessage("supergroup")

	if !requireUserAdminPure(bot, ctx, chat, 10) {
		t.Fatal("requireUserAdminPure(full admin) = false, want true")
	}
	if !requireUserAdminPure(bot, ctx, chat, 777000) {
		t.Fatal("requireUserAdminPure(Telegram service user) = false, want true")
	}
	if requireUserAdminPure(bot, ctx, chat, 42) {
		t.Fatal("requireUserAdminPure(member) = true, want false")
	}
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
