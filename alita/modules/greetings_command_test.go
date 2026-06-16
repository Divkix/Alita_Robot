package modules

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
)

func newGreetingMessageContext(bot *gotgbot.Bot, chat gotgbot.Chat, from gotgbot.User, text string) *ext.Context {
	return newModuleMessageContext(bot, chat, from, text)
}

func TestWelcomeAndGoodbyeTogglesPersistForNewChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	welcomeOffCtx := newGreetingMessageContext(bot, chat, admin, "/welcome off")
	if err := greetingsModule.welcome(bot, welcomeOffCtx); err != ext.EndGroups {
		t.Fatalf("welcome off error = %v, want EndGroups", err)
	}
	if greetings.GetGreetingSettings(chat.Id).WelcomeSettings.ShouldWelcome {
		t.Fatal("welcome toggle stayed enabled for new chat")
	}

	goodbyeOnCtx := newGreetingMessageContext(bot, chat, admin, "/goodbye on")
	if err := greetingsModule.goodbye(bot, goodbyeOnCtx); err != ext.EndGroups {
		t.Fatalf("goodbye on error = %v, want EndGroups", err)
	}
	if !greetings.GetGreetingSettings(chat.Id).GoodbyeSettings.ShouldGoodbye {
		t.Fatal("goodbye toggle did not enable for new chat")
	}
}

func TestWelcomeAndGoodbyeToggleInvalidAndDisplayBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	welcomeOnCtx := newGreetingMessageContext(bot, chat, admin, "/welcome on")
	if err := greetingsModule.welcome(bot, welcomeOnCtx); err != ext.EndGroups {
		t.Fatalf("welcome on error = %v, want EndGroups", err)
	}
	if !greetings.GetGreetingSettings(chat.Id).WelcomeSettings.ShouldWelcome {
		t.Fatal("welcome toggle did not enable")
	}

	welcomeInvalidCtx := newGreetingMessageContext(bot, chat, admin, "/welcome maybe")
	if err := greetingsModule.welcome(bot, welcomeInvalidCtx); err != ext.EndGroups {
		t.Fatalf("welcome invalid error = %v, want EndGroups", err)
	}

	if err := greetings.SetGoodbyeText(chat.Id, "Bye raw {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetGoodbyeText setup error = %v", err)
	}
	goodbyeNoformatCtx := newGreetingMessageContext(bot, chat, admin, "/goodbye noformat")
	if err := greetingsModule.goodbye(bot, goodbyeNoformatCtx); err != ext.EndGroups {
		t.Fatalf("goodbye noformat error = %v, want EndGroups", err)
	}

	goodbyeOffCtx := newGreetingMessageContext(bot, chat, admin, "/goodbye off")
	if err := greetingsModule.goodbye(bot, goodbyeOffCtx); err != ext.EndGroups {
		t.Fatalf("goodbye off error = %v, want EndGroups", err)
	}
	if greetings.GetGreetingSettings(chat.Id).GoodbyeSettings.ShouldGoodbye {
		t.Fatal("goodbye toggle stayed enabled")
	}

	goodbyeInvalidCtx := newGreetingMessageContext(bot, chat, admin, "/goodbye maybe")
	if err := greetingsModule.goodbye(bot, goodbyeInvalidCtx); err != ext.EndGroups {
		t.Fatalf("goodbye invalid error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) < 6 {
		t.Fatalf("sendMessage calls = %d, want toggle/display replies", len(calls))
	}
}

func TestGreetingDisplayPropagatesMediaSendErrors(t *testing.T) {
	client := newModuleBotClient()
	client.errors["sendPhoto"] = errors.New("photo send failed")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := greetings.SetWelcomeText(chat.Id, "Welcome with photo", "photo-file", nil, db.PHOTO); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}

	ctx := newGreetingMessageContext(bot, chat, admin, "/welcome noformat")
	if err := greetingsModule.welcome(bot, ctx); err == nil {
		t.Fatal("welcome noformat media error = nil, want sendPhoto error")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want status reply before media send", len(calls))
	}
	if calls := client.callsFor("sendPhoto"); len(calls) != 1 {
		t.Fatalf("sendPhoto calls = %d, want greeting media send", len(calls))
	}
}

func TestGreetingToggleRejectsNonAdminUsers(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newGreetingMessageContext(bot, chat, member, "/welcome off")
	if err := greetingsModule.welcome(bot, ctx); err != ext.EndGroups {
		t.Fatalf("welcome off by non-admin error = %v, want EndGroups", err)
	}
	if !greetings.GetGreetingSettings(chat.Id).WelcomeSettings.ShouldWelcome {
		t.Fatal("welcome toggle changed after non-admin command")
	}
}

func TestSetAndResetGreetingTextCommands(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	setWelcomeCtx := newGreetingMessageContext(bot, chat, admin, "/setwelcome Hello {first}")
	if err := greetingsModule.setWelcome(bot, setWelcomeCtx); err != ext.EndGroups {
		t.Fatalf("setWelcome error = %v, want EndGroups", err)
	}
	if got := greetings.GetGreetingSettings(chat.Id).WelcomeSettings.WelcomeText; got != "Hello {first}" {
		t.Fatalf("welcome text = %q, want command text", got)
	}

	setGoodbyeCtx := newGreetingMessageContext(bot, chat, admin, "/setgoodbye Bye {first}")
	if err := greetingsModule.setGoodbye(bot, setGoodbyeCtx); err != ext.EndGroups {
		t.Fatalf("setGoodbye error = %v, want EndGroups", err)
	}
	if got := greetings.GetGreetingSettings(chat.Id).GoodbyeSettings.GoodbyeText; got != "Bye {first}" {
		t.Fatalf("goodbye text = %q, want command text", got)
	}

	resetWelcomeCtx := newGreetingMessageContext(bot, chat, admin, "/resetwelcome")
	if err := greetingsModule.resetWelcome(bot, resetWelcomeCtx); err != ext.EndGroups {
		t.Fatalf("resetWelcome error = %v, want EndGroups", err)
	}
	if got := greetings.GetGreetingSettings(chat.Id).WelcomeSettings.WelcomeText; got != db.DefaultWelcome {
		t.Fatalf("welcome text after reset = %q, want default", got)
	}

	resetGoodbyeCtx := newGreetingMessageContext(bot, chat, admin, "/resetgoodbye")
	if err := greetingsModule.resetGoodbye(bot, resetGoodbyeCtx); err != ext.EndGroups {
		t.Fatalf("resetGoodbye error = %v, want EndGroups", err)
	}
	if got := greetings.GetGreetingSettings(chat.Id).GoodbyeSettings.GoodbyeText; got != db.DefaultGoodbye {
		t.Fatalf("goodbye text after reset = %q, want default", got)
	}
}

func TestSetGreetingCommandsRejectMissingContent(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	setWelcomeCtx := newGreetingMessageContext(bot, chat, admin, "/setwelcome")
	if err := greetingsModule.setWelcome(bot, setWelcomeCtx); err != ext.EndGroups {
		t.Fatalf("setWelcome missing error = %v, want EndGroups", err)
	}
	setGoodbyeCtx := newGreetingMessageContext(bot, chat, admin, "/setgoodbye")
	if err := greetingsModule.setGoodbye(bot, setGoodbyeCtx); err != ext.EndGroups {
		t.Fatalf("setGoodbye missing error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want validation replies", len(calls))
	}
}

func TestGreetingCleanupCommandsPersistForNewChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	cleanWelcomeCtx := newGreetingMessageContext(bot, chat, admin, "/cleanwelcome on")
	if err := greetingsModule.cleanWelcome(bot, cleanWelcomeCtx); err != ext.EndGroups {
		t.Fatalf("cleanWelcome error = %v, want EndGroups", err)
	}
	if !greetings.GetGreetingSettings(chat.Id).WelcomeSettings.CleanWelcome {
		t.Fatal("clean welcome did not enable for new chat")
	}

	cleanGoodbyeCtx := newGreetingMessageContext(bot, chat, admin, "/cleangoodbye on")
	if err := greetingsModule.cleanGoodbye(bot, cleanGoodbyeCtx); err != ext.EndGroups {
		t.Fatalf("cleanGoodbye error = %v, want EndGroups", err)
	}
	if !greetings.GetGreetingSettings(chat.Id).GoodbyeSettings.CleanGoodbye {
		t.Fatal("clean goodbye did not enable for new chat")
	}

	cleanServiceCtx := newGreetingMessageContext(bot, chat, admin, "/cleanservice on")
	if err := greetingsModule.delJoined(bot, cleanServiceCtx); err != ext.EndGroups {
		t.Fatalf("delJoined error = %v, want EndGroups", err)
	}
	if !greetings.GetGreetingSettings(chat.Id).ShouldCleanService {
		t.Fatal("clean service did not enable for new chat")
	}

	autoApproveCtx := newGreetingMessageContext(bot, chat, admin, "/autoapprove on")
	if err := greetingsModule.autoApprove(bot, autoApproveCtx); err != ext.EndGroups {
		t.Fatalf("autoApprove error = %v, want EndGroups", err)
	}
	if !greetings.GetGreetingSettings(chat.Id).ShouldAutoApprove {
		t.Fatal("auto approve did not enable for new chat")
	}
}

func TestGreetingCleanupCommandsHandleStatusOffAndInvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		command string
		run     func(*gotgbot.Bot, *ext.Context) error
		verify  func(int64) bool
	}{
		{
			name:    "clean welcome off",
			command: "/cleanwelcome off",
			run:     greetingsModule.cleanWelcome,
			verify: func(chatID int64) bool {
				return !greetings.GetGreetingSettings(chatID).WelcomeSettings.CleanWelcome
			},
		},
		{
			name:    "clean goodbye off",
			command: "/cleangoodbye no",
			run:     greetingsModule.cleanGoodbye,
			verify: func(chatID int64) bool {
				return !greetings.GetGreetingSettings(chatID).GoodbyeSettings.CleanGoodbye
			},
		},
		{
			name:    "clean service off",
			command: "/cleanservice off",
			run:     greetingsModule.delJoined,
			verify: func(chatID int64) bool {
				return !greetings.GetGreetingSettings(chatID).ShouldCleanService
			},
		},
		{
			name:    "auto approve off",
			command: "/autoapprove no",
			run:     greetingsModule.autoApprove,
			verify: func(chatID int64) bool {
				return !greetings.GetGreetingSettings(chatID).ShouldAutoApprove
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
			admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
			ctx := newGreetingMessageContext(bot, chat, admin, tt.command)
			if err := tt.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.name, err)
			}
			if !tt.verify(chat.Id) {
				t.Fatalf("%s did not persist expected disabled state", tt.name)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != 1 {
				t.Fatalf("sendMessage calls = %d, want command response", len(calls))
			}
		})
	}

	t.Run("status and invalid options reply without mutating settings", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

		for _, step := range []struct {
			command string
			run     func(*gotgbot.Bot, *ext.Context) error
		}{
			{command: "/cleanwelcome", run: greetingsModule.cleanWelcome},
			{command: "/cleanwelcome maybe", run: greetingsModule.cleanWelcome},
			{command: "/cleangoodbye", run: greetingsModule.cleanGoodbye},
			{command: "/cleangoodbye maybe", run: greetingsModule.cleanGoodbye},
			{command: "/cleanservice", run: greetingsModule.delJoined},
			{command: "/cleanservice maybe", run: greetingsModule.delJoined},
			{command: "/autoapprove", run: greetingsModule.autoApprove},
			{command: "/autoapprove maybe", run: greetingsModule.autoApprove},
		} {
			ctx := newGreetingMessageContext(bot, chat, admin, step.command)
			if err := step.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", step.command, err)
			}
		}

		if calls := client.callsFor("sendMessage"); len(calls) != 8 {
			t.Fatalf("sendMessage calls = %d, want one reply per status/invalid command", len(calls))
		}
	})
}

func TestWelcomeDisplaySendsStatusAndGreeting(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}

	ctx := newGreetingMessageContext(bot, chat, admin, "/welcome")
	if err := greetingsModule.welcome(bot, ctx); err != ext.EndGroups {
		t.Fatalf("welcome display error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) < 2 {
		t.Fatalf("sendMessage calls = %d, want status and greeting", len(calls))
	}
	lastText, _ := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(lastText, "Welcome") {
		t.Fatalf("welcome greeting text = %q, want configured greeting", lastText)
	}
}

func newChatMemberContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	actor gotgbot.User,
	oldMember gotgbot.ChatMember,
	newMember gotgbot.ChatMember,
) *ext.Context {
	update := &gotgbot.Update{
		UpdateId: 3,
		ChatMember: &gotgbot.ChatMemberUpdated{
			Chat:          chat,
			From:          actor,
			Date:          1,
			OldChatMember: oldMember,
			NewChatMember: newMember,
		},
	}
	return ext.NewContext(bot, update, nil)
}

func newServiceJoinContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	from gotgbot.User,
	newMembers []gotgbot.User,
) *ext.Context {
	msg := &gotgbot.Message{
		MessageId:       301,
		Date:            1,
		Chat:            chat,
		From:            &from,
		NewChatMembers:  newMembers,
		MessageThreadId: 7,
	}
	return ext.NewContext(bot, &gotgbot.Update{UpdateId: 4, Message: msg}, nil)
}

func newJoinRequestContext(bot *gotgbot.Bot, chat gotgbot.Chat, user gotgbot.User) *ext.Context {
	update := &gotgbot.Update{
		UpdateId: 5,
		ChatJoinRequest: &gotgbot.ChatJoinRequest{
			Chat:       chat,
			From:       user,
			UserChatId: user.Id,
			Date:       1,
		},
	}
	return ext.NewContext(bot, update, nil)
}

func TestMemberJoinAndLeaveSendConfiguredGreetings(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 4242, FirstName: "Newbie"}
	if err := greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}
	if err := greetings.SetGoodbyeText(chat.Id, "Bye {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetGoodbyeText setup error = %v", err)
	}
	if err := greetings.SetGoodbyeToggle(chat.Id, true); err != nil {
		t.Fatalf("SetGoodbyeToggle setup error = %v", err)
	}

	joinCtx := newChatMemberContext(
		bot,
		chat,
		admin,
		gotgbot.ChatMemberLeft{User: member},
		gotgbot.ChatMemberMember{User: member},
	)
	clearRecentJoinProcessing(chat.Id, member.Id)
	if err := greetingsModule.newMember(bot, joinCtx); err != ext.EndGroups {
		t.Fatalf("newMember error = %v, want EndGroups", err)
	}

	leaveCtx := newChatMemberContext(
		bot,
		chat,
		admin,
		gotgbot.ChatMemberMember{User: member},
		gotgbot.ChatMemberLeft{User: member},
	)
	if err := greetingsModule.leftMember(bot, leaveCtx); err != ext.EndGroups {
		t.Fatalf("leftMember error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) < 2 {
		t.Fatalf("sendMessage calls = %d, want welcome and goodbye", len(calls))
	}
	if first := calls[len(calls)-2].Params["text"].(string); !strings.Contains(first, "Welcome") {
		t.Fatalf("welcome text = %q, want configured welcome", first)
	}
	if last := calls[len(calls)-1].Params["text"].(string); !strings.Contains(last, "Bye") {
		t.Fatalf("goodbye text = %q, want configured goodbye", last)
	}
}

func TestCleanServiceProcessesJoinAndDeletesServiceMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 4343, FirstName: "ServiceUser"}
	if err := greetings.SetWelcomeText(chat.Id, "Service welcome {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}
	if err := greetings.SetShouldCleanService(chat.Id, true); err != nil {
		t.Fatalf("SetShouldCleanService setup error = %v", err)
	}

	clearRecentJoinProcessing(chat.Id, member.Id)
	ctx := newServiceJoinContext(bot, chat, admin, []gotgbot.User{member})
	if err := greetingsModule.cleanService(bot, ctx); err != ext.EndGroups {
		t.Fatalf("cleanService error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want welcome", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want service cleanup", len(calls))
	}
}

func TestCleanServiceProcessesMultipleNewMembersWithoutCaptcha(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	members := []gotgbot.User{
		{Id: 4441, FirstName: "One"},
		{Id: 4442, FirstName: "Two"},
	}
	if err := greetings.SetWelcomeText(chat.Id, "Hello {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}
	if err := greetings.SetShouldCleanService(chat.Id, true); err != nil {
		t.Fatalf("SetShouldCleanService setup error = %v", err)
	}
	for _, member := range members {
		clearRecentJoinProcessing(chat.Id, member.Id)
	}

	ctx := newServiceJoinContext(bot, chat, admin, members)
	if err := greetingsModule.cleanService(bot, ctx); err != ext.EndGroups {
		t.Fatalf("cleanService multiple members error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != len(members) {
		t.Fatalf("sendMessage calls = %d, want one welcome per new member", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want service cleanup", len(calls))
	}
}

func TestProcessSingleNewMemberSkipsDuplicatesAndFallsBackWhenCaptchaMuteFails(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 4545, FirstName: "CaptchaUser"}
	if err := greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}

	ctx := newServiceJoinContext(bot, chat, admin, []gotgbot.User{member})
	processSingleNewMember(bot, ctx, gotgbot.User{Id: bot.Id, FirstName: "Alita", IsBot: true}, false)
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want none for bot join", len(calls))
	}

	clearRecentJoinProcessing(chat.Id, member.Id)
	if !claimRecentJoinProcessing(chat.Id, member.Id) {
		t.Fatal("claimRecentJoinProcessing setup returned false")
	}
	processSingleNewMember(bot, ctx, member, false)
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want none for duplicate join", len(calls))
	}

	clearRecentJoinProcessing(chat.Id, member.Id)
	client.errors["restrictChatMember"] = errors.New("telegram: bot lacks restrict permission")
	processSingleNewMember(bot, ctx, member, true)
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want mute attempt", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want welcome fallback", len(calls))
	}
}

func TestProcessSingleNewMemberCaptchaSuccessSkipsWelcome(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 4646, FirstName: "CaptchaPass"}
	if err := captcha.SetCaptchaEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetCaptchaEnabled setup error = %v", err)
	}
	if err := greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}

	ctx := newServiceJoinContext(bot, chat, admin, []gotgbot.User{member})
	clearRecentJoinProcessing(chat.Id, member.Id)
	processSingleNewMember(bot, ctx, member, true)

	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want initial mute", len(calls))
	}
	if calls := client.callsFor("sendPhoto"); len(calls) != 1 {
		t.Fatalf("sendPhoto calls = %d, want captcha image", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want no welcome before verification", len(calls))
	}
	attempt, err := captcha.GetCaptchaAttempt(member.Id, chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt error = %v", err)
	}
	if attempt == nil {
		t.Fatal("captcha attempt was not created")
	}
	if attempt.MessageID == 0 {
		t.Fatal("captcha attempt message ID was not stored")
	}
}

func TestProcessSingleNewMemberCaptchaSendFailureUnmutesAndWelcomes(t *testing.T) {
	client := newModuleBotClient()
	client.errors["sendPhoto"] = errors.New("telegram send photo failed")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 4747, FirstName: "CaptchaFallback"}
	if err := captcha.SetCaptchaEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetCaptchaEnabled setup error = %v", err)
	}
	if err := greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}

	ctx := newServiceJoinContext(bot, chat, admin, []gotgbot.User{member})
	clearRecentJoinProcessing(chat.Id, member.Id)
	processSingleNewMember(bot, ctx, member, true)

	if calls := client.callsFor("restrictChatMember"); len(calls) != 2 {
		t.Fatalf("restrictChatMember calls = %d, want mute then unmute", len(calls))
	}
	if calls := client.callsFor("sendPhoto"); len(calls) != 1 {
		t.Fatalf("sendPhoto calls = %d, want captcha send attempt", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want welcome fallback", len(calls))
	}
	attempt, err := captcha.GetCaptchaAttempt(member.Id, chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt error = %v", err)
	}
	if attempt != nil {
		t.Fatal("captcha attempt remained after send failure")
	}
}

func TestNewMemberCaptchaSuccessAndSendFailurePaths(t *testing.T) {
	for _, tt := range []struct {
		name         string
		sendPhotoErr bool
		wantRestrict int
		wantPhoto    int
		wantWelcome  int
	}{
		{
			name:         "captcha sent",
			wantRestrict: 1,
			wantPhoto:    1,
		},
		{
			name:         "captcha send fails",
			sendPhotoErr: true,
			wantRestrict: 2,
			wantPhoto:    1,
			wantWelcome:  1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			if tt.sendPhotoErr {
				client.errors["sendPhoto"] = errors.New("telegram send photo failed")
			}
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
			admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
			member := gotgbot.User{Id: uniqueModuleChatID(), FirstName: "NewCaptcha"}
			if err := captcha.SetCaptchaEnabled(chat.Id, true); err != nil {
				t.Fatalf("SetCaptchaEnabled setup error = %v", err)
			}
			if err := greetings.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
				t.Fatalf("SetWelcomeText setup error = %v", err)
			}

			ctx := newChatMemberContext(
				bot,
				chat,
				admin,
				gotgbot.ChatMemberLeft{User: member},
				gotgbot.ChatMemberMember{User: member},
			)
			clearRecentJoinProcessing(chat.Id, member.Id)
			if err := greetingsModule.newMember(bot, ctx); err != ext.EndGroups {
				t.Fatalf("newMember error = %v, want EndGroups", err)
			}

			if calls := client.callsFor("restrictChatMember"); len(calls) != tt.wantRestrict {
				t.Fatalf("restrictChatMember calls = %d, want %d", len(calls), tt.wantRestrict)
			}
			if calls := client.callsFor("sendPhoto"); len(calls) != tt.wantPhoto {
				t.Fatalf("sendPhoto calls = %d, want %d", len(calls), tt.wantPhoto)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != tt.wantWelcome {
				t.Fatalf("sendMessage calls = %d, want %d", len(calls), tt.wantWelcome)
			}
		})
	}
}

func TestLeftMemberDeletesPendingCaptchaAttemptAndCleanGoodbye(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 4848, FirstName: "Leaving"}
	if err := greetings.SetGoodbyeText(chat.Id, "Bye {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetGoodbyeText setup error = %v", err)
	}
	if err := greetings.SetGoodbyeToggle(chat.Id, true); err != nil {
		t.Fatalf("SetGoodbyeToggle setup error = %v", err)
	}
	if err := greetings.SetCleanGoodbyeSetting(chat.Id, true); err != nil {
		t.Fatalf("SetCleanGoodbyeSetting setup error = %v", err)
	}
	if err := greetings.SetCleanGoodbyeMsgId(chat.Id, 1234); err != nil {
		t.Fatalf("SetCleanGoodbyeMsgId setup error = %v", err)
	}
	attempt := &db.CaptchaAttempts{
		UserID:    member.Id,
		ChatID:    chat.Id,
		Answer:    "42",
		MessageID: 4321,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	if err := db.DB.Create(attempt).Error; err != nil {
		t.Fatalf("captcha attempt setup error = %v", err)
	}

	ctx := newChatMemberContext(
		bot,
		chat,
		admin,
		gotgbot.ChatMemberMember{User: member},
		gotgbot.ChatMemberLeft{User: member},
	)
	if err := greetingsModule.leftMember(bot, ctx); err != ext.EndGroups {
		t.Fatalf("leftMember error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 2 {
		t.Fatalf("deleteMessage calls = %d, want captcha cleanup and old goodbye cleanup", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want goodbye message", len(calls))
	}
	got, err := captcha.GetCaptchaAttempt(member.Id, chat.Id)
	if err != nil {
		t.Fatalf("GetCaptchaAttempt error = %v", err)
	}
	if got != nil {
		t.Fatal("captcha attempt was not deleted for leaving member")
	}
	if lastID := greetings.GetGreetingSettings(chat.Id).GoodbyeSettings.LastMsgId; lastID == 1234 {
		t.Fatal("clean goodbye message ID was not updated")
	}
}

func TestPendingJoinRequestAndCallbacks(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChat"] = []byte(`{"id":5151,"type":"private","first_name":"Applicant"}`)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	applicant := gotgbot.User{Id: 5151, FirstName: "Applicant"}

	ctx := newJoinRequestContext(bot, chat, applicant)
	if err := greetingsModule.pendingJoins(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("pendingJoins error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want join request notice", len(calls))
	}

	data := encodeCallbackData(
		"join_request",
		map[string]string{"a": "accept", "u": "5151"},
	)
	callbackCtx := newModuleCallbackContext(bot, chat, admin, data)
	if err := greetingsModule.joinRequestHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("joinRequestHandler accept error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("approveChatJoinRequest"); len(calls) != 1 {
		t.Fatalf("approveChatJoinRequest calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}

func TestJoinRequestCallbacksHandleDeclineBanAndInvalidData(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantApprove int
		wantDecline int
		wantBan     int
	}{
		{
			name:        "encoded decline",
			data:        encodeCallbackData("join_request", map[string]string{"a": "decline", "u": "7171"}),
			wantDecline: 1,
		},
		{
			name:        "encoded ban",
			data:        encodeCallbackData("join_request", map[string]string{"a": "ban", "u": "7171"}),
			wantDecline: 1,
			wantBan:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			client.responses["getChat"] = []byte(`{"id":7171,"type":"private","first_name":"Applicant"}`)
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
			admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

			ctx := newModuleCallbackContext(bot, chat, admin, tt.data)
			if err := greetingsModule.joinRequestHandler(bot, ctx); err != ext.EndGroups {
				t.Fatalf("joinRequestHandler error = %v, want EndGroups", err)
			}
			if calls := client.callsFor("approveChatJoinRequest"); len(calls) != tt.wantApprove {
				t.Fatalf("approveChatJoinRequest calls = %d, want %d", len(calls), tt.wantApprove)
			}
			if calls := client.callsFor("declineChatJoinRequest"); len(calls) != tt.wantDecline {
				t.Fatalf("declineChatJoinRequest calls = %d, want %d", len(calls), tt.wantDecline)
			}
			if calls := client.callsFor("banChatMember"); len(calls) != tt.wantBan {
				t.Fatalf("banChatMember calls = %d, want %d", len(calls), tt.wantBan)
			}
			if calls := client.callsFor("editMessageText"); len(calls) != 1 {
				t.Fatalf("editMessageText calls = %d, want callback message update", len(calls))
			}
			if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
				t.Fatalf("answerCallbackQuery calls = %d, want callback acknowledgement", len(calls))
			}
		})
	}

	t.Run("invalid callback data is answered without lookup", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

		ctx := newModuleCallbackContext(bot, chat, admin, "join_request.invalid")
		if err := greetingsModule.joinRequestHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("joinRequestHandler invalid data error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("getChat"); len(calls) != 0 {
			t.Fatalf("getChat calls = %d, want no user lookup for invalid data", len(calls))
		}
		if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
			t.Fatalf("answerCallbackQuery calls = %d, want invalid request acknowledgement", len(calls))
		}
	})

	t.Run("invalid join user id is answered without lookup", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

		data := encodeCallbackData("join_request", map[string]string{"a": "accept", "u": "nan"})
		ctx := newModuleCallbackContext(bot, chat, admin, data)
		if err := greetingsModule.joinRequestHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("joinRequestHandler invalid user error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("getChat"); len(calls) != 0 {
			t.Fatalf("getChat calls = %d, want no user lookup for invalid user id", len(calls))
		}
		if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
			t.Fatalf("answerCallbackQuery calls = %d, want invalid request acknowledgement", len(calls))
		}
	})
}

func TestAutoApproveJoinRequestSkipsAdminNotice(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	applicant := gotgbot.User{Id: 6161, FirstName: "Applicant"}
	if err := greetings.SetShouldAutoApprove(chat.Id, true); err != nil {
		t.Fatalf("SetShouldAutoApprove setup error = %v", err)
	}

	ctx := newJoinRequestContext(bot, chat, applicant)
	if err := greetingsModule.pendingJoins(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("pendingJoins auto approve error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("approveChatJoinRequest"); len(calls) != 1 {
		t.Fatalf("approveChatJoinRequest calls = %d, want auto approve", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want no admin notice", len(calls))
	}
}

func TestGreetingCommandsPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "welcome status", text: "/welcome", run: greetingsModule.welcome},
		{name: "welcome toggle", text: "/welcome on", run: greetingsModule.welcome},
		{name: "set welcome", text: "/setwelcome Hello", run: greetingsModule.setWelcome},
		{name: "reset welcome", text: "/resetwelcome", run: greetingsModule.resetWelcome},
		{name: "goodbye status", text: "/goodbye", run: greetingsModule.goodbye},
		{name: "goodbye toggle", text: "/goodbye on", run: greetingsModule.goodbye},
		{name: "set goodbye", text: "/setgoodbye Bye", run: greetingsModule.setGoodbye},
		{name: "reset goodbye", text: "/resetgoodbye", run: greetingsModule.resetGoodbye},
		{name: "clean welcome", text: "/cleanwelcome", run: greetingsModule.cleanWelcome},
		{name: "clean goodbye", text: "/cleangoodbye", run: greetingsModule.cleanGoodbye},
		{name: "clean service", text: "/cleanservice", run: greetingsModule.delJoined},
		{name: "auto approve", text: "/autoapprove", run: greetingsModule.autoApprove},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors["sendMessage"] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
			ctx := newGreetingMessageContext(bot, chat, admin, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestJoinRequestFlowPropagatesGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	applicant := gotgbot.User{Id: 5151, FirstName: "Applicant"}

	for _, tt := range []struct {
		name    string
		method  string
		build   func(*gotgbot.Bot, gotgbot.Chat) *ext.Context
		run     func(*gotgbot.Bot, *ext.Context) error
		wantErr bool
	}{
		{
			name:   "pending join notice",
			method: "sendMessage",
			build: func(bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				return newJoinRequestContext(bot, chat, applicant)
			},
			run: greetingsModule.pendingJoins,
		},
		{
			name:   "auto approve join request",
			method: "approveChatJoinRequest",
			build: func(bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				if err := greetings.SetShouldAutoApprove(chat.Id, true); err != nil {
					t.Fatalf("SetShouldAutoApprove() error = %v", err)
				}
				return newJoinRequestContext(bot, chat, applicant)
			},
			run: greetingsModule.pendingJoins,
		},
		{
			name:   "join callback lookup",
			method: "getChat",
			build: func(bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				data := encodeCallbackData("join_request", map[string]string{"a": "accept", "u": "5151"})
				return newModuleCallbackContext(bot, chat, admin, data)
			},
			run: greetingsModule.joinRequestHandler,
		},
		{
			name:   "join callback approve",
			method: "approveChatJoinRequest",
			build: func(bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				data := encodeCallbackData("join_request", map[string]string{"a": "accept", "u": "5151"})
				return newModuleCallbackContext(bot, chat, admin, data)
			},
			run: greetingsModule.joinRequestHandler,
		},
		{
			name:   "join callback decline",
			method: "declineChatJoinRequest",
			build: func(bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				data := encodeCallbackData("join_request", map[string]string{"a": "decline", "u": "5151"})
				return newModuleCallbackContext(bot, chat, admin, data)
			},
			run: greetingsModule.joinRequestHandler,
		},
		{
			name:   "join callback ban",
			method: "banChatMember",
			build: func(bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				data := encodeCallbackData("join_request", map[string]string{"a": "ban", "u": "5151"})
				return newModuleCallbackContext(bot, chat, admin, data)
			},
			run: greetingsModule.joinRequestHandler,
		},
		{
			name:   "join callback edit",
			method: "editMessageText",
			build: func(bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				data := encodeCallbackData("join_request", map[string]string{"a": "accept", "u": "5151"})
				return newModuleCallbackContext(bot, chat, admin, data)
			},
			run: greetingsModule.joinRequestHandler,
		},
		{
			name:   "join callback answer",
			method: "answerCallbackQuery",
			build: func(bot *gotgbot.Bot, chat gotgbot.Chat) *ext.Context {
				data := encodeCallbackData("join_request", map[string]string{"a": "accept", "u": "5151"})
				return newModuleCallbackContext(bot, chat, admin, data)
			},
			run: greetingsModule.joinRequestHandler,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			client.responses["getChat"] = []byte(`{"id":5151,"type":"private","first_name":"Applicant"}`)
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}

			err := tt.run(bot, tt.build(bot, chat))
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.name, err)
			}
		})
	}
}

func TestJoinRequestHandlerAcceptsExpectedTelegramErrors(t *testing.T) {
	expectedErr := fmt.Errorf("Forbidden: bot is not a member of the supergroup chat")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	applicant := gotgbot.User{Id: 5151, FirstName: "Applicant"}

	for _, tt := range []struct {
		name        string
		method      string
		data        string
		autoApprove bool
		want        error
	}{
		{
			name:   "join callback approve",
			method: "approveChatJoinRequest",
			data: encodeCallbackData(
				"join_request",
				map[string]string{"a": "accept", "u": "5151"},
			),
			want: ext.EndGroups,
		},
		{
			name:   "join callback decline",
			method: "declineChatJoinRequest",
			data: encodeCallbackData(
				"join_request",
				map[string]string{"a": "decline", "u": "5151"},
			),
			want: ext.EndGroups,
		},
		{
			name:        "auto approve join request",
			method:      "approveChatJoinRequest",
			autoApprove: true,
			want:        ext.ContinueGroups,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			client.responses["getChat"] = []byte(`{"id":5151,"type":"private","first_name":"Applicant"}`)
			bot := newModuleTestBot(client)
			client.errors[tt.method] = expectedErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}

			var (
				ctx *ext.Context
				err error
			)
			if tt.autoApprove {
				if err := greetings.SetShouldAutoApprove(chat.Id, true); err != nil {
					t.Fatalf("SetShouldAutoApprove() error = %v", err)
				}
				ctx = newJoinRequestContext(bot, chat, applicant)
				err = greetingsModule.pendingJoins(bot, ctx)
			} else {
				ctx = newModuleCallbackContext(bot, chat, admin, tt.data)
				err = greetingsModule.joinRequestHandler(bot, ctx)
			}

			if err != tt.want {
				t.Fatalf("%s error = %v, want %v", tt.name, err, tt.want)
			}
		})
	}
}
