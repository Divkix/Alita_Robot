package modules

import (
	"fmt"
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

func TestHandlePendingCaptchaMessageStoresAndDeletesUserMessages(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	tests := []struct {
		name        string
		build       func(*ext.Context)
		wantType    int
		wantContent string
		wantFileID  string
		wantCaption string
	}{
		{
			name:        "text",
			build:       func(ctx *ext.Context) {},
			wantType:    db.TEXT,
			wantContent: "pending text",
		},
		{
			name: "sticker",
			build: func(ctx *ext.Context) {
				ctx.EffectiveMessage.Text = ""
				ctx.EffectiveMessage.Sticker = &gotgbot.Sticker{FileId: "sticker-file"}
			},
			wantType:   db.STICKER,
			wantFileID: "sticker-file",
		},
		{
			name: "document",
			build: func(ctx *ext.Context) {
				ctx.EffectiveMessage.Text = ""
				ctx.EffectiveMessage.Document = &gotgbot.Document{FileId: "document-file"}
				ctx.EffectiveMessage.Caption = "document caption"
			},
			wantType:    db.DOCUMENT,
			wantFileID:  "document-file",
			wantCaption: "document caption",
		},
		{
			name: "photo",
			build: func(ctx *ext.Context) {
				ctx.EffectiveMessage.Text = ""
				ctx.EffectiveMessage.Photo = []gotgbot.PhotoSize{
					{FileId: "small-photo", FileUniqueId: "small", Width: 10, Height: 10},
					{FileId: "large-photo", FileUniqueId: "large", Width: 100, Height: 100},
				}
				ctx.EffectiveMessage.Caption = "photo caption"
			},
			wantType:    db.PHOTO,
			wantFileID:  "large-photo",
			wantCaption: "photo caption",
		},
		{
			name: "audio",
			build: func(ctx *ext.Context) {
				ctx.EffectiveMessage.Text = ""
				ctx.EffectiveMessage.Audio = &gotgbot.Audio{FileId: "audio-file"}
				ctx.EffectiveMessage.Caption = "audio caption"
			},
			wantType:    db.AUDIO,
			wantFileID:  "audio-file",
			wantCaption: "audio caption",
		},
		{
			name: "voice",
			build: func(ctx *ext.Context) {
				ctx.EffectiveMessage.Text = ""
				ctx.EffectiveMessage.Voice = &gotgbot.Voice{FileId: "voice-file"}
				ctx.EffectiveMessage.Caption = "voice caption"
			},
			wantType:    db.VOICE,
			wantFileID:  "voice-file",
			wantCaption: "voice caption",
		},
		{
			name: "video",
			build: func(ctx *ext.Context) {
				ctx.EffectiveMessage.Text = ""
				ctx.EffectiveMessage.Video = &gotgbot.Video{FileId: "video-file"}
				ctx.EffectiveMessage.Caption = "video caption"
			},
			wantType:    db.VIDEO,
			wantFileID:  "video-file",
			wantCaption: "video caption",
		},
		{
			name: "video note",
			build: func(ctx *ext.Context) {
				ctx.EffectiveMessage.Text = ""
				ctx.EffectiveMessage.VideoNote = &gotgbot.VideoNote{FileId: "video-note-file"}
			},
			wantType:   db.VideoNote,
			wantFileID: "video-note-file",
		},
		{
			name: "unsupported",
			build: func(ctx *ext.Context) {
				ctx.EffectiveMessage.Text = ""
			},
			wantType:    db.TEXT,
			wantContent: "[Unsupported message type]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
			attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
			if err != nil {
				t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
			}

			ctx := newModuleMessageContext(bot, chat, member, "pending text")
			tt.build(ctx)
			if err := captchaModule.handlePendingCaptchaMessage(bot, ctx); err != ext.EndGroups {
				t.Fatalf("handlePendingCaptchaMessage() error = %v, want EndGroups", err)
			}

			messages, err := db.GetStoredMessagesForAttempt(attempt.ID)
			if err != nil {
				t.Fatalf("GetStoredMessagesForAttempt() error = %v", err)
			}
			if len(messages) != 1 {
				t.Fatalf("stored messages = %d, want 1", len(messages))
			}
			if messages[0].MessageType != tt.wantType {
				t.Fatalf("MessageType = %d, want %d", messages[0].MessageType, tt.wantType)
			}
			if messages[0].Content != tt.wantContent {
				t.Fatalf("Content = %q, want %q", messages[0].Content, tt.wantContent)
			}
			if messages[0].FileID != tt.wantFileID {
				t.Fatalf("FileID = %q, want %q", messages[0].FileID, tt.wantFileID)
			}
			if messages[0].Caption != tt.wantCaption {
				t.Fatalf("Caption = %q, want %q", messages[0].Caption, tt.wantCaption)
			}
		})
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != len(tests) {
		t.Fatalf("deleteMessage calls = %d, want one per pending message", len(calls))
	}
}

func TestHandlePendingCaptchaMessageContinuesWithoutPendingAttempt(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, member, "normal text")

	if err := captchaModule.handlePendingCaptchaMessage(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("handlePendingCaptchaMessage() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want 0", len(calls))
	}
}

func TestSendCaptchaCreatesAttemptAndSendsChallenge(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	ctx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 777000, FirstName: "Telegram"}, "join")
	if err := db.SetCaptchaEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}

	if err := SendCaptcha(bot, ctx, 42, "Member"); err != nil {
		t.Fatalf("SendCaptcha() error = %v", err)
	}

	attempt, err := db.GetCaptchaAttempt(42, chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt() error = %v", err)
	}
	if attempt == nil {
		t.Fatal("captcha attempt was not created")
	}
	if attempt.MessageID == 0 {
		t.Fatal("captcha attempt MessageID = 0, want sent message id")
	}
	if len(client.callsFor("sendPhoto"))+len(client.callsFor("sendMessage")) == 0 {
		t.Fatal("SendCaptcha did not send a challenge message")
	}
}

func TestCaptchaVerifyCallbackWrongAnswerIncrementsAttempts(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if err := db.UpdateCaptchaAttemptMessageID(attempt.ID, 123); err != nil {
		t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v", err)
	}

	data := encodeCallbackData(
		"captcha_verify",
		map[string]string{"a": fmt.Sprint(attempt.ID), "u": "42", "s": "8"},
		fmt.Sprintf("captcha_verify.%d.42.8", attempt.ID),
	)
	ctx := newModuleCallbackContext(bot, chat, member, data)
	if err := captchaModule.captchaVerifyCallback(bot, ctx); err != nil {
		t.Fatalf("captchaVerifyCallback() error = %v", err)
	}

	updated, err := db.GetCaptchaAttempt(member.Id, chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt() error = %v", err)
	}
	if updated == nil || updated.Attempts != 1 {
		t.Fatalf("Attempts = %#v, want 1", updated)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}

func TestCaptchaVerifyCallbackValidationBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	tests := []struct {
		name string
		user gotgbot.User
		data string
	}{
		{name: "malformed", user: member, data: "captcha_verify"},
		{name: "bad attempt id", user: member, data: "captcha_verify.nope.42.7"},
		{name: "bad user id", user: member, data: fmt.Sprintf("captcha_verify.%d.nope.7", attempt.ID)},
		{name: "wrong user", user: gotgbot.User{Id: 43, FirstName: "Other"}, data: fmt.Sprintf("captcha_verify.%d.42.7", attempt.ID)},
		{name: "attempt mismatch", user: member, data: "captcha_verify.999999.42.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleCallbackContext(bot, chat, tt.user, tt.data)
			if err := captchaModule.captchaVerifyCallback(bot, ctx); err != nil {
				t.Fatalf("captchaVerifyCallback(%s) error = %v", tt.name, err)
			}
		})
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != len(tests) {
		t.Fatalf("answerCallbackQuery calls = %d, want one per validation branch", len(calls))
	}
}

func TestCaptchaVerifyCallbackCorrectAnswerUnmutesAndCleansAttempt(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if err := db.UpdateCaptchaAttemptMessageID(attempt.ID, 123); err != nil {
		t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v", err)
	}
	if err := db.StoreMessageForCaptcha(member.Id, chat.Id, attempt.ID, db.TEXT, "blocked text", "", ""); err != nil {
		t.Fatalf("StoreMessageForCaptcha() error = %v", err)
	}

	data := encodeCallbackData(
		"captcha_verify",
		map[string]string{"a": fmt.Sprint(attempt.ID), "u": "42", "s": "7"},
		fmt.Sprintf("captcha_verify.%d.42.7", attempt.ID),
	)
	ctx := newModuleCallbackContext(bot, chat, member, data)
	if err := captchaModule.captchaVerifyCallback(bot, ctx); err != nil {
		t.Fatalf("captchaVerifyCallback(correct) error = %v", err)
	}

	if current, err := db.GetCaptchaAttempt(member.Id, chat.Id); err != nil || current != nil {
		t.Fatalf("GetCaptchaAttempt() after success = %#v, %v; want nil, nil", current, err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want unmute action", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) == 0 {
		t.Fatal("deleteMessage calls = 0, want captcha cleanup")
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want success answer", len(calls))
	}
}

func TestCaptchaVerifyCallbackFinalWrongAnswerAppliesFailureAction(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.SetCaptchaFailureAction(chat.Id, "ban"); err != nil {
		t.Fatalf("SetCaptchaFailureAction() error = %v", err)
	}
	if err := db.SetCaptchaMaxAttempts(chat.Id, 1); err != nil {
		t.Fatalf("SetCaptchaMaxAttempts() error = %v", err)
	}
	attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if err := db.UpdateCaptchaAttemptMessageID(attempt.ID, 123); err != nil {
		t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v", err)
	}

	ctx := newModuleCallbackContext(bot, chat, member, fmt.Sprintf("captcha_verify.%d.42.8", attempt.ID))
	if err := captchaModule.captchaVerifyCallback(bot, ctx); err != nil {
		t.Fatalf("captchaVerifyCallback(final wrong) error = %v", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want failure ban", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want final wrong answer", len(calls))
	}
}

func TestHandleCaptchaTimeoutAppliesKickMuteAndSkipsStaleAttempts(t *testing.T) {
	tests := []struct {
		name           string
		action         string
		wantBan        int
		wantUnban      int
		wantRestrict   int
		withStoredText bool
	}{
		{name: "kick", action: "kick", wantBan: 1, wantUnban: 1},
		{name: "mute with stored messages", action: "mute", wantRestrict: 1, withStoredText: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
			member := gotgbot.User{Id: 42, FirstName: "Member"}
			attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
			if err != nil {
				t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
			}
			if tt.withStoredText {
				if err := db.StoreMessageForCaptcha(member.Id, chat.Id, attempt.ID, db.TEXT, "blocked", "", ""); err != nil {
					t.Fatalf("StoreMessageForCaptcha() error = %v", err)
				}
			}

			handleCaptchaTimeout(bot, chat.Id, member.Id, attempt.ID, 456, tt.action)

			if current, err := db.GetCaptchaAttempt(member.Id, chat.Id); err != nil || current != nil {
				t.Fatalf("GetCaptchaAttempt() after timeout = %#v, %v; want nil, nil", current, err)
			}
			if calls := client.callsFor("banChatMember"); len(calls) != tt.wantBan {
				t.Fatalf("banChatMember calls = %d, want %d", len(calls), tt.wantBan)
			}
			if calls := client.callsFor("unbanChatMember"); len(calls) != tt.wantUnban {
				t.Fatalf("unbanChatMember calls = %d, want %d", len(calls), tt.wantUnban)
			}
			if calls := client.callsFor("restrictChatMember"); len(calls) != tt.wantRestrict {
				t.Fatalf("restrictChatMember calls = %d, want %d", len(calls), tt.wantRestrict)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != 1 {
				t.Fatalf("sendMessage calls = %d, want timeout notice", len(calls))
			}
			if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
				t.Fatalf("deleteMessage calls = %d, want captcha message cleanup", len(calls))
			}
		})
	}

	t.Run("missing attempt", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		handleCaptchaTimeout(bot, uniqueModuleChatID(), 42, 999999, 456, "kick")
		if calls := client.callsFor("sendMessage"); len(calls) != 0 {
			t.Fatalf("sendMessage calls = %d, want none for missing attempt", len(calls))
		}
	})

	t.Run("identity mismatch", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
		attempt, err := db.CreateCaptchaAttemptPreMessage(42, chat.Id, "7", 2)
		if err != nil {
			t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
		}
		handleCaptchaTimeout(bot, chat.Id, 43, attempt.ID, 456, "kick")
		if current, err := db.GetCaptchaAttemptByID(attempt.ID); err != nil || current == nil {
			t.Fatalf("GetCaptchaAttemptByID() after mismatch = %#v, %v; want retained attempt", current, err)
		}
		if calls := client.callsFor("sendMessage"); len(calls) != 0 {
			t.Fatalf("sendMessage calls = %d, want none for identity mismatch", len(calls))
		}
	})
}

func TestCaptchaRefreshCallbackValidationBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}

	tests := []struct {
		name string
		user gotgbot.User
		data string
	}{
		{name: "malformed", user: member, data: "captcha_refresh"},
		{name: "bad attempt id", user: member, data: "captcha_refresh.nope.42"},
		{name: "bad user id", user: member, data: fmt.Sprintf("captcha_refresh.%d.nope", attempt.ID)},
		{name: "wrong user", user: gotgbot.User{Id: 43, FirstName: "Other"}, data: fmt.Sprintf("captcha_refresh.%d.42", attempt.ID)},
		{name: "attempt mismatch", user: member, data: "captcha_refresh.999999.42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleCallbackContext(bot, chat, tt.user, tt.data)
			if err := captchaModule.captchaRefreshCallback(bot, ctx); err != nil {
				t.Fatalf("captchaRefreshCallback(%s) error = %v", tt.name, err)
			}
		})
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != len(tests) {
		t.Fatalf("answerCallbackQuery calls = %d, want one per validation branch", len(calls))
	}
}

func TestCaptchaRefreshCallbackSendsNewChallengeAndUpdatesAttempt(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.SetCaptchaMode(chat.Id, "text"); err != nil {
		t.Fatalf("SetCaptchaMode() error = %v", err)
	}
	attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "old-answer", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if err := db.UpdateCaptchaAttemptMessageID(attempt.ID, 123); err != nil {
		t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v", err)
	}

	data := encodeCallbackData(
		"captcha_refresh",
		map[string]string{"a": fmt.Sprint(attempt.ID), "u": "42"},
		fmt.Sprintf("captcha_refresh.%d.42", attempt.ID),
	)
	ctx := newModuleCallbackContext(bot, chat, member, data)
	if err := captchaModule.captchaRefreshCallback(bot, ctx); err != nil {
		t.Fatalf("captchaRefreshCallback(success) error = %v", err)
	}

	updated, err := db.GetCaptchaAttemptByID(attempt.ID)
	if err != nil || updated == nil {
		t.Fatalf("GetCaptchaAttemptByID() after refresh = %#v, %v; want attempt, nil", updated, err)
	}
	if updated.Answer == "old-answer" {
		t.Fatal("captcha answer was not refreshed")
	}
	if updated.MessageID == 123 {
		t.Fatal("captcha message id was not refreshed")
	}
	if updated.RefreshCount != 1 {
		t.Fatalf("RefreshCount = %d, want 1", updated.RefreshCount)
	}
	if calls := client.callsFor("sendPhoto"); len(calls) != 1 {
		t.Fatalf("sendPhoto calls = %d, want refreshed challenge photo", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want refresh success answer", len(calls))
	}
}

func TestLoadCaptchaRegistersHelpAndHandlers(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadCaptcha(dispatcher)

	if moduleName, enabled := DefaultHelpRegistry().AbleMap.Load(captchaModule.moduleName); moduleName != captchaModule.moduleName || !enabled {
		t.Fatalf("captcha help registration = (%q, %v), want enabled", moduleName, enabled)
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
