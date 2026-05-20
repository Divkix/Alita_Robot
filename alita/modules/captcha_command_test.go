package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func TestCaptchaCommandTogglesAndDisplaysSettings(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	onCtx := newModuleMessageContext(bot, chat, admin, "/captcha on")
	if err := captchaModule.captchaCommand(bot, onCtx); err != nil {
		t.Fatalf("captchaCommand(on) error = %v", err)
	}
	settings, err := db.GetCaptchaSettings(chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() after on error = %v", err)
	}
	if !settings.Enabled {
		t.Fatal("captcha enabled = false, want true")
	}

	showCtx := newModuleMessageContext(bot, chat, admin, "/captcha")
	if err := captchaModule.captchaCommand(bot, showCtx); err != nil {
		t.Fatalf("captchaCommand(show) error = %v", err)
	}

	offCtx := newModuleMessageContext(bot, chat, admin, "/captcha off")
	if err := captchaModule.captchaCommand(bot, offCtx); err != nil {
		t.Fatalf("captchaCommand(off) error = %v", err)
	}
	settings, err = db.GetCaptchaSettings(chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() after off error = %v", err)
	}
	if settings.Enabled {
		t.Fatal("captcha enabled = true, want false")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want one reply per captcha command", len(calls))
	}
}

func TestCaptchaSubcommandsCreateSettingsWithDefaults(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
		want db.CaptchaSettings
	}{
		{
			name: "mode",
			text: "/captchamode text",
			run:  captchaModule.captchaModeCommand,
			want: db.CaptchaSettings{CaptchaMode: "text", Timeout: 2, FailureAction: "kick", MaxAttempts: 3},
		},
		{
			name: "timeout",
			text: "/captchatime 5",
			run:  captchaModule.captchaTimeCommand,
			want: db.CaptchaSettings{CaptchaMode: "math", Timeout: 5, FailureAction: "kick", MaxAttempts: 3},
		},
		{
			name: "failure action",
			text: "/captchaaction ban",
			run:  captchaModule.captchaActionCommand,
			want: db.CaptchaSettings{CaptchaMode: "math", Timeout: 2, FailureAction: "ban", MaxAttempts: 3},
		},
		{
			name: "max attempts",
			text: "/captchamaxattempts 4",
			run:  captchaModule.captchaMaxAttemptsCommand,
			want: db.CaptchaSettings{CaptchaMode: "math", Timeout: 2, FailureAction: "kick", MaxAttempts: 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := tt.run(bot, ctx); err != nil {
				t.Fatalf("%s error = %v", tt.text, err)
			}
			assertCaptchaSettings(t, chat.Id, tt.want)
		})
	}
}

func TestCaptchaSubcommandsRejectInvalidValues(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "mode", text: "/captchamode image", run: captchaModule.captchaModeCommand},
		{name: "timeout low", text: "/captchatime 0", run: captchaModule.captchaTimeCommand},
		{name: "timeout high", text: "/captchatime 11", run: captchaModule.captchaTimeCommand},
		{name: "failure action", text: "/captchaaction warn", run: captchaModule.captchaActionCommand},
		{name: "max attempts", text: "/captchamaxattempts 0", run: captchaModule.captchaMaxAttemptsCommand},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := tt.run(bot, ctx); err != nil {
				t.Fatalf("%s error = %v", tt.text, err)
			}
		})
	}

	assertCaptchaSettings(t, chat.Id, db.CaptchaSettings{
		CaptchaMode:   "math",
		Timeout:       2,
		FailureAction: "kick",
		MaxAttempts:   3,
	})
	if calls := client.callsFor("sendMessage"); len(calls) != len(tests) {
		t.Fatalf("sendMessage calls = %d, want one validation reply per invalid command", len(calls))
	}
}

func TestCaptchaPendingMessageCommandsShowAndClearStoredMessages(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	targetID := int64(424242)

	attempt, err := db.CreateCaptchaAttemptPreMessage(targetID, chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if err := db.StoreMessageForCaptcha(targetID, chat.Id, attempt.ID, db.TEXT, "pending text", "", ""); err != nil {
		t.Fatalf("StoreMessageForCaptcha(text) error = %v", err)
	}
	if err := db.StoreMessageForCaptcha(targetID, chat.Id, attempt.ID, db.PHOTO, "", "photo-file", "caption"); err != nil {
		t.Fatalf("StoreMessageForCaptcha(photo) error = %v", err)
	}

	viewCtx := newModuleMessageContext(bot, chat, admin, "/captchapending 424242")
	if err := captchaModule.viewPendingMessages(bot, viewCtx); err != nil {
		t.Fatalf("viewPendingMessages() error = %v", err)
	}
	if messages, err := db.GetStoredMessagesForUser(targetID, chat.Id); err != nil || len(messages) != 2 {
		t.Fatalf("stored messages before clear = %d, %v; want 2, nil", len(messages), err)
	}

	clearCtx := newModuleMessageContext(bot, chat, admin, "/captchaclear 424242")
	if err := captchaModule.clearPendingMessages(bot, clearCtx); err != nil {
		t.Fatalf("clearPendingMessages() error = %v", err)
	}
	messages, err := db.GetStoredMessagesForUser(targetID, chat.Id)
	if err != nil {
		t.Fatalf("GetStoredMessagesForUser() after clear error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("stored messages after clear = %d, want 0", len(messages))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want view and clear replies", len(calls))
	}
}

func TestCaptchaPendingMessageCommandsValidateArguments(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	tests := []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "view missing user", text: "/captchapending", run: captchaModule.viewPendingMessages},
		{name: "view no pending messages", text: "/captchapending 12345", run: captchaModule.viewPendingMessages},
		{name: "clear missing user", text: "/captchaclear", run: captchaModule.clearPendingMessages},
		{name: "clear no pending messages", text: "/captchaclear 12345", run: captchaModule.clearPendingMessages},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := tt.run(bot, ctx); err != nil {
				t.Fatalf("%s error = %v", tt.text, err)
			}
		})
	}
	if calls := client.callsFor("sendMessage"); len(calls) != len(tests) {
		t.Fatalf("sendMessage calls = %d, want one reply per validation case", len(calls))
	}
}

func assertCaptchaSettings(t *testing.T, chatID int64, want db.CaptchaSettings) {
	t.Helper()

	settings, err := db.GetCaptchaSettings(chatID)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() error = %v", err)
	}
	if settings.CaptchaMode != want.CaptchaMode {
		t.Fatalf("CaptchaMode = %q, want %q", settings.CaptchaMode, want.CaptchaMode)
	}
	if settings.Timeout != want.Timeout {
		t.Fatalf("Timeout = %d, want %d", settings.Timeout, want.Timeout)
	}
	if settings.FailureAction != want.FailureAction {
		t.Fatalf("FailureAction = %q, want %q", settings.FailureAction, want.FailureAction)
	}
	if settings.MaxAttempts != want.MaxAttempts {
		t.Fatalf("MaxAttempts = %d, want %d", settings.MaxAttempts, want.MaxAttempts)
	}
}
