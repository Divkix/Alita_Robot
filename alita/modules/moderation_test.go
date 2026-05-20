//go:build testtools

package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

func TestBuildModerationCtxRequiresEffectiveUser(t *testing.T) {
	t.Parallel()

	got, err := buildModerationCtx(&moduleStruct{}, nil, nil)
	if err != ext.EndGroups {
		t.Fatalf("buildModerationCtx() err = %v, want %v", err, ext.EndGroups)
	}
	if got != nil {
		t.Fatalf("buildModerationCtx() ctx = %#v, want nil", got)
	}
}

func TestExtractFromArgsWithNumericUserAndReason(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/ban 12345 repeated spam",
		Chat: gotgbot.Chat{Id: -100123, Type: "supergroup"},
		From: &gotgbot.User{Id: 7, FirstName: "Admin"},
		Entities: []gotgbot.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 4},
		},
	}
	ctx := ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 99, IsBot: true}},
		&gotgbot.Update{Message: msg},
		nil,
	)

	got, err := extractFromArgs(&moderationCtx{Ctx: ctx, Msg: msg})
	if err != nil {
		t.Fatalf("extractFromArgs() unexpected error: %v", err)
	}
	if got.userID != 12345 {
		t.Fatalf("extractFromArgs() userID = %d, want 12345", got.userID)
	}
	if got.reason != "repeated spam" {
		t.Fatalf("extractFromArgs() reason = %q, want repeated spam", got.reason)
	}
}

func TestExtractFromReplyUsesReplySender(t *testing.T) {
	t.Parallel()

	replyUser := &gotgbot.User{Id: 555, FirstName: "Target"}
	got, err := extractFromReply(&moderationCtx{
		Msg: &gotgbot.Message{
			ReplyToMessage: &gotgbot.Message{From: replyUser},
		},
	})
	if err != nil {
		t.Fatalf("extractFromReply() unexpected error: %v", err)
	}
	if got.userID != replyUser.Id {
		t.Fatalf("extractFromReply() userID = %d, want %d", got.userID, replyUser.Id)
	}
	if got.reason != "" {
		t.Fatalf("extractFromReply() reason = %q, want empty", got.reason)
	}
}

func TestModerationCommandRunShortCircuitsFailedGate(t *testing.T) {
	t.Parallel()

	ctx := moderationTestContext()
	calledExtract := false
	calledExecute := false

	cmd := &moderationCommand{
		module: &moduleStruct{moduleName: "Bans"},
		gates: []gateFn{
			func(*moderationCtx) bool { return false },
		},
		extract: func(*moderationCtx) (target, error) {
			calledExtract = true
			return target{userID: 1}, nil
		},
		execute: func(*moderationCtx, *target) error {
			calledExecute = true
			return nil
		},
	}

	if err := cmd.run(nil, ctx); err != ext.EndGroups {
		t.Fatalf("run() err = %v, want %v", err, ext.EndGroups)
	}
	if calledExtract {
		t.Fatal("run() should not extract when a gate fails")
	}
	if calledExecute {
		t.Fatal("run() should not execute when a gate fails")
	}
}

func TestModerationCommandRunExecutesPipelineInOrder(t *testing.T) {
	t.Parallel()

	ctx := moderationTestContext()
	var calls []string

	cmd := &moderationCommand{
		module: &moduleStruct{moduleName: "Bans"},
		gates: []gateFn{
			func(*moderationCtx) bool {
				calls = append(calls, "gate")
				return true
			},
		},
		extract: func(*moderationCtx) (target, error) {
			calls = append(calls, "extract")
			return target{userID: 123}, nil
		},
		validate: func(_ *moderationCtx, got *target) error {
			calls = append(calls, "validate")
			if got.userID != 123 {
				t.Fatalf("validate target userID = %d, want 123", got.userID)
			}
			return nil
		},
		execute: func(_ *moderationCtx, got *target) error {
			calls = append(calls, "execute")
			if got.userID != 123 {
				t.Fatalf("execute target userID = %d, want 123", got.userID)
			}
			return nil
		},
		reply: func(_ *moderationCtx, got *target) error {
			calls = append(calls, "reply")
			if got.userID != 123 {
				t.Fatalf("reply target userID = %d, want 123", got.userID)
			}
			return nil
		},
	}

	if err := cmd.run(nil, ctx); err != ext.EndGroups {
		t.Fatalf("run() err = %v, want %v", err, ext.EndGroups)
	}

	want := []string{"gate", "extract", "validate", "execute", "reply"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls = %v, want %v", calls, want)
		}
	}
}

func TestDeleteModGatesRequireRestrictAndDeletePermissions(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Moderation Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "/purge")
	mc := newModerationCtxForTest(bot, ctx, &moduleStruct{moduleName: "Bans"})

	if !deleteModGates(mc) {
		t.Fatal("deleteModGates() = false, want true for creator with bot admin permissions")
	}
}

func TestDefaultTargetValidationRejectsBotAndAllowsMember(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Moderation Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "/ban 42")
	mc := newModerationCtxForTest(bot, ctx, &moduleStruct{moduleName: "Bans"})

	if err := defaultTargetValidation(mc, &target{userID: 42}); err != nil {
		t.Fatalf("defaultTargetValidation(member) error = %v, want nil", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want no error reply for valid member", len(calls))
	}

	if err := defaultTargetValidation(mc, &target{userID: bot.Id}); err == nil {
		t.Fatal("defaultTargetValidation(bot) error = nil, want self-target error")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want one self-target error reply", len(calls))
	}
}

func TestMentionHtmlWrapperEscapesName(t *testing.T) {
	got := _mentionHtml(42, "A < B")
	if !strings.Contains(got, "tg://user?id=42") {
		t.Fatalf("_mentionHtml() = %q, want user mention link", got)
	}
	if strings.Contains(got, "A < B") {
		t.Fatalf("_mentionHtml() = %q, want escaped display name", got)
	}
}

func moderationTestContext() *ext.Context {
	msg := &gotgbot.Message{
		MessageId: 10,
		Text:      "/ban 123",
		Chat:      gotgbot.Chat{Id: 99, Type: "private"},
		From:      &gotgbot.User{Id: 7, FirstName: "Admin"},
	}
	return ext.NewContext(
		&gotgbot.Bot{User: gotgbot.User{Id: 99, IsBot: true}},
		&gotgbot.Update{Message: msg},
		nil,
	)
}

func newModerationCtxForTest(bot *gotgbot.Bot, ctx *ext.Context, module *moduleStruct) *moderationCtx {
	return &moderationCtx{
		Bot:    bot,
		Chat:   ctx.EffectiveChat,
		Msg:    ctx.EffectiveMessage,
		User:   ctx.EffectiveUser,
		Ctx:    ctx,
		Tr:     i18n.MustNewTranslator(db.GetLanguage(ctx)),
		Module: module,
	}
}
