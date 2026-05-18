package chat_status

import (
	"math"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestIsValidUserId(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   int64
		want bool
	}{
		{name: "positive user ID", id: 123456789, want: true},
		{name: "minimum valid (1)", id: 1, want: true},
		{name: "zero", id: 0, want: false},
		{name: "negative one", id: -1, want: false},
		{name: "channel ID -1001234567890", id: -1001234567890, want: false},
		{name: "MaxInt64", id: math.MaxInt64, want: true},
		{name: "MinInt64", id: math.MinInt64, want: false},
		{name: "Group Anonymous Bot 1087968824", id: 1087968824, want: true},
		{name: "Telegram 777000", id: 777000, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsValidUserId(tc.id)
			if got != tc.want {
				t.Fatalf("IsValidUserId(%d) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

func TestIsChannelId(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   int64
		want bool
	}{
		{name: "typical channel ID -1001234567890", id: -1001234567890, want: true},
		{name: "boundary first channel ID -1000000000001", id: -1000000000001, want: true},
		{name: "boundary not channel -1000000000000", id: -1000000000000, want: false},
		{name: "positive user ID 123456789", id: 123456789, want: false},
		{name: "zero", id: 0, want: false},
		{name: "MaxInt64", id: math.MaxInt64, want: false},
		{name: "MinInt64", id: math.MinInt64, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsChannelId(tc.id)
			if got != tc.want {
				t.Fatalf("IsChannelId(%d) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

func TestExtractChatFromContext(t *testing.T) {
	t.Parallel()

	chatA := &gotgbot.Chat{Id: 123, Type: "supergroup"}

	tests := []struct {
		name     string
		chat     *gotgbot.Chat
		ctx      *ext.Context
		wantNil  bool
		wantChat *gotgbot.Chat
	}{
		{
			name:     "non-nil chat returns chat directly",
			chat:     chatA,
			ctx:      nil,
			wantNil:  false,
			wantChat: chatA,
		},
		{
			name:    "nil context returns nil",
			chat:    nil,
			ctx:     nil,
			wantNil: true,
		},
		{
			name:    "empty context returns nil",
			chat:    nil,
			ctx:     &ext.Context{Update: &gotgbot.Update{}},
			wantNil: true,
		},
		{
			name: "callback query message returns chat",
			chat: nil,
			ctx: &ext.Context{
				Update: &gotgbot.Update{
					CallbackQuery: &gotgbot.CallbackQuery{
						Id:   "cq1",
						From: gotgbot.User{Id: 1, FirstName: "A"},
						Message: gotgbot.Message{
							MessageId: 1,
							Chat:      gotgbot.Chat{Id: 456, Type: "supergroup"},
						},
					},
				},
			},
			wantNil:  false,
			wantChat: &gotgbot.Chat{Id: 456, Type: "supergroup"},
		},
		{
			name: "regular message returns chat",
			chat: nil,
			ctx: &ext.Context{
				Update: &gotgbot.Update{
					Message: &gotgbot.Message{
						MessageId: 2,
						Chat:      gotgbot.Chat{Id: 789, Type: "group"},
					},
				},
			},
			wantNil:  false,
			wantChat: &gotgbot.Chat{Id: 789, Type: "group"},
		},
		{
			name: "myChatMember returns chat",
			chat: nil,
			ctx: &ext.Context{
				Update: &gotgbot.Update{
					MyChatMember: &gotgbot.ChatMemberUpdated{
						Chat: gotgbot.Chat{Id: 101112, Type: "private"},
						From: gotgbot.User{Id: 1, FirstName: "A"},
					},
				},
			},
			wantNil:  false,
			wantChat: &gotgbot.Chat{Id: 101112, Type: "private"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractChatFromContext(tc.ctx, tc.chat)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("extractChatFromContext() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("extractChatFromContext() = nil, want non-nil")
			}
			if tc.wantChat != nil {
				if got.Id != tc.wantChat.Id || got.Type != tc.wantChat.Type {
					t.Fatalf("extractChatFromContext() chat = {Id:%d Type:%s}, want {Id:%d Type:%s}",
						got.Id, got.Type, tc.wantChat.Id, tc.wantChat.Type)
				}
			}
			// When chat param is non-nil, returned pointer must be the same.
			if tc.chat != nil && got != tc.chat {
				t.Fatalf("extractChatFromContext() returned different pointer, want same as input")
			}
		})
	}
}

func TestGetEffectiveUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ctx     *ext.Context
		wantNil bool
		wantID  int64
	}{
		{
			name:    "nil context returns nil",
			ctx:     nil,
			wantNil: true,
		},
		{
			name: "nil sender returns nil",
			ctx: &ext.Context{
				Update:          &gotgbot.Update{},
				EffectiveSender: nil,
			},
			wantNil: true,
		},
		{
			name: "valid sender returns user",
			ctx: &ext.Context{
				Update: &gotgbot.Update{},
				EffectiveSender: &gotgbot.Sender{
					User: &gotgbot.User{Id: 42, FirstName: "Test"},
				},
			},
			wantNil: false,
			wantID:  42,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := GetEffectiveUser(tc.ctx)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("GetEffectiveUser() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("GetEffectiveUser() = nil, want non-nil")
			}
			if got.Id != tc.wantID {
				t.Fatalf("GetEffectiveUser() user.Id = %d, want %d", got.Id, tc.wantID)
			}
		})
	}
}
