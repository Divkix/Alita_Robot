package modules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestCaptchaCommandRejectsUnknownOption(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "/captcha maybe")

	if err := captchaModule.captchaCommand(bot, ctx); err != nil {
		t.Fatalf("captchaCommand(unknown) error = %v", err)
	}
	settings, err := db.GetCaptchaSettings(chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaSettings() error = %v", err)
	}
	if settings.Enabled {
		t.Fatal("captcha enabled = true after invalid option, want false")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want usage reply", len(calls))
	}
}

func TestRecoverOrphanedCaptchasCleansPendingAttempts(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	now := time.Now()
	kickChat := uniqueModuleChatID()
	banChat := uniqueModuleChatID()
	validChat := uniqueModuleChatID()

	if err := db.DB.Where("1 = 1").Delete(&db.StoredMessages{}).Error; err != nil {
		t.Fatalf("stored message cleanup setup error = %v", err)
	}
	if err := db.DB.Where("1 = 1").Delete(&db.CaptchaAttempts{}).Error; err != nil {
		t.Fatalf("captcha attempt cleanup setup error = %v", err)
	}
	if err := db.SetCaptchaFailureAction(kickChat, "kick"); err != nil {
		t.Fatalf("SetCaptchaFailureAction(kick) error = %v", err)
	}
	if err := db.SetCaptchaFailureAction(banChat, "ban"); err != nil {
		t.Fatalf("SetCaptchaFailureAction(ban) error = %v", err)
	}

	attempts := []db.CaptchaAttempts{
		{
			UserID:    7001,
			ChatID:    kickChat,
			Answer:    "1",
			MessageID: 8101,
			CreatedAt: now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Hour),
		},
		{
			UserID:    7002,
			ChatID:    banChat,
			Answer:    "2",
			MessageID: 8102,
			CreatedAt: now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Hour),
		},
		{
			UserID:    7003,
			ChatID:    validChat,
			Answer:    "3",
			MessageID: 8103,
			CreatedAt: now,
			ExpiresAt: now.Add(time.Hour),
		},
	}
	for i := range attempts {
		if err := db.DB.Create(&attempts[i]).Error; err != nil {
			t.Fatalf("captcha attempt %d setup error = %v", i, err)
		}
		if err := db.StoreMessageForCaptcha(attempts[i].UserID, attempts[i].ChatID, attempts[i].ID, db.TEXT, "pending", "", ""); err != nil {
			t.Fatalf("stored message %d setup error = %v", i, err)
		}
	}

	captchaRecoveryOnce = sync.Once{}
	recoverOrphanedCaptchas(bot)

	var remainingAttempts int64
	if err := db.DB.Model(&db.CaptchaAttempts{}).Count(&remainingAttempts).Error; err != nil {
		t.Fatalf("count captcha attempts error = %v", err)
	}
	if remainingAttempts != 0 {
		t.Fatalf("remaining captcha attempts = %d, want 0", remainingAttempts)
	}
	var remainingMessages int64
	if err := db.DB.Model(&db.StoredMessages{}).Count(&remainingMessages).Error; err != nil {
		t.Fatalf("count stored messages error = %v", err)
	}
	if remainingMessages != 0 {
		t.Fatalf("remaining stored messages = %d, want 0", remainingMessages)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != len(attempts) {
		t.Fatalf("deleteMessage calls = %d, want one per attempt", len(calls))
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 2 {
		t.Fatalf("banChatMember calls = %d, want kick and ban actions", len(calls))
	}
}

func TestCleanupExpiredCaptchaAttemptsDeletesMessagesAndRecords(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	previousBotRef := captchaBotRef
	captchaBotRef = bot
	t.Cleanup(func() {
		captchaBotRef = previousBotRef
	})

	if err := db.DB.Where("1 = 1").Delete(&db.StoredMessages{}).Error; err != nil {
		t.Fatalf("stored message cleanup setup error = %v", err)
	}
	if err := db.DB.Where("1 = 1").Delete(&db.CaptchaAttempts{}).Error; err != nil {
		t.Fatalf("captcha attempt cleanup setup error = %v", err)
	}

	now := time.Now()
	expired := db.CaptchaAttempts{
		UserID:    7101,
		ChatID:    uniqueModuleChatID(),
		Answer:    "4",
		MessageID: 9101,
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}
	active := db.CaptchaAttempts{
		UserID:    7102,
		ChatID:    uniqueModuleChatID(),
		Answer:    "5",
		MessageID: 9102,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := db.DB.Create(&expired).Error; err != nil {
		t.Fatalf("expired captcha attempt setup error = %v", err)
	}
	if err := db.DB.Create(&active).Error; err != nil {
		t.Fatalf("active captcha attempt setup error = %v", err)
	}
	if err := db.StoreMessageForCaptcha(expired.UserID, expired.ChatID, expired.ID, db.TEXT, "expired", "", ""); err != nil {
		t.Fatalf("expired stored message setup error = %v", err)
	}
	if err := db.StoreMessageForCaptcha(active.UserID, active.ChatID, active.ID, db.TEXT, "active", "", ""); err != nil {
		t.Fatalf("active stored message setup error = %v", err)
	}

	if err := cleanupExpiredCaptchaAttempts(context.Background()); err != nil {
		t.Fatalf("cleanupExpiredCaptchaAttempts() error = %v", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want expired captcha message deleted once", len(calls))
	}
	gotExpired, err := db.GetCaptchaAttemptByID(expired.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID(expired) error = %v", err)
	}
	if gotExpired != nil {
		t.Fatalf("expired captcha attempt still exists: %+v", gotExpired)
	}
	gotActive, err := db.GetCaptchaAttemptByID(active.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID(active) error = %v", err)
	}
	if gotActive == nil {
		t.Fatal("active captcha attempt was deleted")
	}
	expiredMessages, err := db.CountStoredMessagesForAttempt(expired.ID)
	if err != nil {
		t.Fatalf("CountStoredMessagesForAttempt(expired) error = %v", err)
	}
	if expiredMessages != 0 {
		t.Fatalf("expired stored messages = %d, want 0", expiredMessages)
	}
	activeMessages, err := db.CountStoredMessagesForAttempt(active.ID)
	if err != nil {
		t.Fatalf("CountStoredMessagesForAttempt(active) error = %v", err)
	}
	if activeMessages != 1 {
		t.Fatalf("active stored messages = %d, want 1", activeMessages)
	}
}

func TestCleanupExpiredCaptchaAttemptsHonorsCancelledContext(t *testing.T) {
	if err := db.DB.Where("1 = 1").Delete(&db.CaptchaAttempts{}).Error; err != nil {
		t.Fatalf("captcha attempt cleanup setup error = %v", err)
	}
	now := time.Now()
	attempt := db.CaptchaAttempts{
		UserID:    7201,
		ChatID:    uniqueModuleChatID(),
		Answer:    "6",
		MessageID: 9201,
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}
	if err := db.DB.Create(&attempt).Error; err != nil {
		t.Fatalf("captcha attempt setup error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := cleanupExpiredCaptchaAttempts(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cleanupExpiredCaptchaAttempts(cancelled) error = %v, want context.Canceled", err)
	}
	got, err := db.GetCaptchaAttemptByID(attempt.ID)
	if err != nil {
		t.Fatalf("GetCaptchaAttemptByID() error = %v", err)
	}
	if got == nil {
		t.Fatal("cancelled cleanup deleted the captcha attempt")
	}
}

func TestRunCaptchaCleanupTickHandlesEmptyAndCancelledContexts(t *testing.T) {
	if err := db.DB.Where("1 = 1").Delete(&db.CaptchaAttempts{}).Error; err != nil {
		t.Fatalf("captcha attempt cleanup setup error = %v", err)
	}

	runCaptchaCleanupTick(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runCaptchaCleanupTick(ctx)
}

func TestUnmuteExpiredCaptchaUsersGrantsPermissionsAndCleansRecords(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	previousBotRef := captchaBotRef
	captchaBotRef = bot
	t.Cleanup(func() {
		captchaBotRef = previousBotRef
	})

	if err := db.DB.Where("1 = 1").Delete(&db.CaptchaMutedUsers{}).Error; err != nil {
		t.Fatalf("muted user cleanup setup error = %v", err)
	}
	if err := db.CreateMutedUser(7301, uniqueModuleChatID(), time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("CreateMutedUser() error = %v", err)
	}

	unmuteExpiredCaptchaUsers()

	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want one unmute request", len(calls))
	}
	users, err := db.GetUsersToUnmute()
	if err != nil {
		t.Fatalf("GetUsersToUnmute() error = %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("users to unmute = %d, want 0", len(users))
	}
}

func TestUnmuteExpiredCaptchaUsersKeepsTransientFailures(t *testing.T) {
	client := newModuleBotClient()
	client.errors["restrictChatMember"] = errors.New("temporary network failure")
	bot := newModuleTestBot(client)
	previousBotRef := captchaBotRef
	captchaBotRef = bot
	t.Cleanup(func() {
		captchaBotRef = previousBotRef
	})

	if err := db.DB.Where("1 = 1").Delete(&db.CaptchaMutedUsers{}).Error; err != nil {
		t.Fatalf("muted user cleanup setup error = %v", err)
	}
	if err := db.CreateMutedUser(7401, uniqueModuleChatID(), time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("CreateMutedUser() error = %v", err)
	}

	unmuteExpiredCaptchaUsers()

	users, err := db.GetUsersToUnmute()
	if err != nil {
		t.Fatalf("GetUsersToUnmute() error = %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("users to unmute = %d, want transient failure retained", len(users))
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

func TestCaptchaSubcommandsRequireArgumentsOrShowCurrentSettings(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.SetCaptchaMaxAttempts(chat.Id, 6); err != nil {
		t.Fatalf("SetCaptchaMaxAttempts() setup error = %v", err)
	}

	tests := []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "mode missing", text: "/captchamode", run: captchaModule.captchaModeCommand},
		{name: "timeout missing", text: "/captchatime", run: captchaModule.captchaTimeCommand},
		{name: "action missing", text: "/captchaaction", run: captchaModule.captchaActionCommand},
		{name: "max attempts current", text: "/captchamaxattempts", run: captchaModule.captchaMaxAttemptsCommand},
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
		t.Fatalf("sendMessage calls = %d, want one reply per command", len(calls))
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

func TestSendCaptchaSkipsDisabledSettings(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	ctx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 777000, FirstName: "Telegram"}, "join")

	if err := SendCaptcha(bot, ctx, 42, "Member"); err != nil {
		t.Fatalf("SendCaptcha(disabled) error = %v", err)
	}

	if calls := client.callsFor("getChatMember"); len(calls) != 0 {
		t.Fatalf("getChatMember calls = %d, want no Telegram validation when disabled", len(calls))
	}
	if calls := client.callsFor("sendPhoto"); len(calls) != 0 {
		t.Fatalf("sendPhoto calls = %d, want no challenge when disabled", len(calls))
	}
}

func TestSendCaptchaRejectsInvalidChatDataAfterValidation(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup"}
	ctx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 777000, FirstName: "Telegram"}, "join")
	if err := db.SetCaptchaEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}

	if err := SendCaptcha(bot, ctx, 42, "Member"); err == nil {
		t.Fatal("SendCaptcha(invalid chat) error = nil, want invalid chat data error")
	}
	if calls := client.callsFor("getChatMember"); len(calls) != 1 {
		t.Fatalf("getChatMember calls = %d, want Telegram user validation before chat rejection", len(calls))
	}
	if calls := client.callsFor("sendPhoto"); len(calls) != 0 {
		t.Fatalf("sendPhoto calls = %d, want no challenge for invalid chat", len(calls))
	}
}

func TestSendCaptchaTextModeUsesImageChallengeWithRefresh(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	ctx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 777000, FirstName: "Telegram"}, "join")
	if err := db.SetCaptchaEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}
	if err := db.SetCaptchaMode(chat.Id, "text"); err != nil {
		t.Fatalf("SetCaptchaMode() error = %v", err)
	}

	if err := SendCaptcha(bot, ctx, 42, "Member"); err != nil {
		t.Fatalf("SendCaptcha(text mode) error = %v", err)
	}

	calls := client.callsFor("sendPhoto")
	if len(calls) != 1 {
		t.Fatalf("sendPhoto calls = %d, want text captcha image challenge", len(calls))
	}
	markup, ok := calls[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("reply_markup type = %T, want InlineKeyboardMarkup", calls[0].Params["reply_markup"])
	}
	var refreshFound bool
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			if strings.HasPrefix(button.CallbackData, "captcha_refresh") {
				refreshFound = true
			}
		}
	}
	if !refreshFound {
		t.Fatal("text captcha challenge did not include refresh callback")
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

func TestCaptchaCommandsPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name   string
		text   string
		method string
		run    func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "captcha status", text: "/captcha", method: "sendMessage", run: captchaModule.captchaCommand},
		{name: "captcha enable", text: "/captcha on", method: "sendMessage", run: captchaModule.captchaCommand},
		{name: "captcha mode", text: "/captchamode text", method: "sendMessage", run: captchaModule.captchaModeCommand},
		{name: "captcha timeout", text: "/captchatime 5", method: "sendMessage", run: captchaModule.captchaTimeCommand},
		{name: "captcha action", text: "/captchaaction ban", method: "sendMessage", run: captchaModule.captchaActionCommand},
		{name: "captcha max attempts", text: "/captchamaxattempts 4", method: "sendMessage", run: captchaModule.captchaMaxAttemptsCommand},
		{name: "captcha pending empty", text: "/captchapending 424242", method: "sendMessage", run: captchaModule.viewPendingMessages},
		{name: "captcha clear empty", text: "/captchaclear 424242", method: "sendMessage", run: captchaModule.clearPendingMessages},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestSendCaptchaPropagatesGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name   string
		method string
	}{
		{name: "user validation", method: "getChatMember"},
		{name: "challenge send", method: "sendPhoto"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
			if err := db.SetCaptchaEnabled(chat.Id, true); err != nil {
				t.Fatalf("SetCaptchaEnabled() error = %v", err)
			}
			ctx := newModuleMessageContext(bot, chat, admin, "join")

			err := SendCaptcha(bot, ctx, 42, "Member")
			if !errors.Is(err, requestErr) {
				t.Fatalf("SendCaptcha returned error %v, want request error", err)
			}
		})
	}
}

func TestCaptchaCallbacksPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, tt := range []struct {
		name    string
		method  string
		build   func(t *testing.T, bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context
		wantErr error
		run     func(*gotgbot.Bot, *ext.Context) error
	}{
		{
			name:   "verify wrong answer response",
			method: "answerCallbackQuery",
			build: func(t *testing.T, bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				t.Helper()
				attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
				if err != nil {
					t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
				}
				return newModuleCallbackContext(bot, chat, member, fmt.Sprintf("captcha_verify.%d.42.8", attempt.ID))
			},
			run: captchaModule.captchaVerifyCallback,
		},
		{
			name:   "verify unmute request",
			method: "restrictChatMember",
			build: func(t *testing.T, bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				t.Helper()
				attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
				if err != nil {
					t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
				}
				return newModuleCallbackContext(bot, chat, member, fmt.Sprintf("captcha_verify.%d.42.7", attempt.ID))
			},
			run: captchaModule.captchaVerifyCallback,
		},
		{
			name:   "refresh challenge send",
			method: "sendPhoto",
			build: func(t *testing.T, bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				t.Helper()
				attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
				if err != nil {
					t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
				}
				return newModuleCallbackContext(bot, chat, member, fmt.Sprintf("captcha_refresh.%d.42", attempt.ID))
			},
			run: captchaModule.captchaRefreshCallback,
		},
		{
			name:   "refresh success response",
			method: "answerCallbackQuery",
			build: func(t *testing.T, bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				t.Helper()
				attempt, err := db.CreateCaptchaAttemptPreMessage(member.Id, chat.Id, "7", 2)
				if err != nil {
					t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
				}
				return newModuleCallbackContext(bot, chat, member, fmt.Sprintf("captcha_refresh.%d.42", attempt.ID))
			},
			run: captchaModule.captchaRefreshCallback,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}

			err := tt.run(bot, tt.build(t, bot, chat))
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.name, err)
			}
		})
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
