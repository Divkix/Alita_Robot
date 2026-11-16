package chat_status

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"

	"github.com/divkix/Alita_Robot/alita/i18n"
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

	t.Run("CallbackQuery", func(t *testing.T) {
		ctx := makeCtxWithCallbackQuery()
		r := NewPermissionResponder(bot)
		res := r.Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		if res != false {
			t.Fatal("Respond should return false")
		}
	})

	t.Run("WithReply", func(t *testing.T) {
		ctx := makeCtxWithMessage("supergroup")
		r := NewPermissionResponder(bot)
		res := r.Respond(ctx, "chat_status_restrict_cmd_error", "", WithReply())
		if res != false {
			t.Fatal("Respond should return false")
		}
	})

	t.Run("WithReplyFallback", func(t *testing.T) {
		ctx := makeCtxWithMessage("supergroup")
		r := NewPermissionResponder(bot)
		res := r.Respond(ctx, "chat_status_restrict_cmd_error", "", WithReplyFallback())
		if res != false {
			t.Fatal("Respond should return false")
		}
	})
}

// permissionSendErrorClient fails sends while retaining the normal chat fixture responses.
type permissionSendErrorClient struct {
	chatStatusBotClient
	err error
}

func (c permissionSendErrorClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "sendMessage" {
		return nil, c.err
	}
	return c.chatStatusBotClient.RequestWithContext(ctx, token, method, params, opts)
}

func TestPermissionResponderRespondSendErrorLogLevel(t *testing.T) {
	tr, err := i18n.NewTestTranslator("chat_status_user_admin_cmd_error: Admin required")
	if err != nil {
		t.Fatal(err)
	}
	logger := log.StandardLogger()
	oldHooks := logger.ReplaceHooks(make(log.LevelHooks))
	oldLevel := logger.GetLevel()
	t.Cleanup(func() {
		logger.ReplaceHooks(oldHooks)
		logger.SetLevel(oldLevel)
	})
	logger.SetLevel(log.WarnLevel)
	hook := logtest.NewGlobal()
	for _, tc := range []struct {
		name      string
		errText   string
		wantLevel log.Level
	}{
		{"expected", "Bad Request: not enough rights to send text messages to the chat", log.WarnLevel},
		{"unexpected", "unexpected transport failure", log.ErrorLevel},
	} {
		for _, fallback := range []bool{true, false} {
			path := "direct"
			if fallback {
				path = "fallback"
			}
			t.Run(tc.name+"/"+path, func(t *testing.T) {
				hook.Reset()
				bot := newChatStatusBot(999)
				bot.BotClient = permissionSendErrorClient{err: errors.New(tc.errText)}
				opts := []respondOpt{WithTranslator(tr)}
				if fallback {
					opts = append(opts, WithReplyFallback())
				}
				NewPermissionResponder(bot).Respond(makeCtxWithMessage("supergroup"), "chat_status_user_admin_cmd_error", "", opts...)
				found := false
				for _, entry := range hook.AllEntries() {
					if tc.wantLevel == log.WarnLevel && entry.Level <= log.ErrorLevel {
						t.Errorf("unexpected error-level log: %s", entry.Message)
					}
					if strings.HasPrefix(entry.Message, "sendMessage ") {
						found = true
						if entry.Level != tc.wantLevel {
							t.Errorf("send failure level = %s, want %s", entry.Level, tc.wantLevel)
						}
					}
				}
				if !found {
					t.Fatal("missing send failure log")
				}
			})
		}
	}
}
