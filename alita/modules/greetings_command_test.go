package modules

import (
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
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
	if db.GetGreetingSettings(chat.Id).WelcomeSettings.ShouldWelcome {
		t.Fatal("welcome toggle stayed enabled for new chat")
	}

	goodbyeOnCtx := newGreetingMessageContext(bot, chat, admin, "/goodbye on")
	if err := greetingsModule.goodbye(bot, goodbyeOnCtx); err != ext.EndGroups {
		t.Fatalf("goodbye on error = %v, want EndGroups", err)
	}
	if !db.GetGreetingSettings(chat.Id).GoodbyeSettings.ShouldGoodbye {
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
	if !db.GetGreetingSettings(chat.Id).WelcomeSettings.ShouldWelcome {
		t.Fatal("welcome toggle did not enable")
	}

	welcomeInvalidCtx := newGreetingMessageContext(bot, chat, admin, "/welcome maybe")
	if err := greetingsModule.welcome(bot, welcomeInvalidCtx); err != ext.EndGroups {
		t.Fatalf("welcome invalid error = %v, want EndGroups", err)
	}

	if err := db.SetGoodbyeText(chat.Id, "Bye raw {first}", "", nil, db.TEXT); err != nil {
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
	if db.GetGreetingSettings(chat.Id).GoodbyeSettings.ShouldGoodbye {
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

func TestSetAndResetGreetingTextCommands(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Greeting Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	setWelcomeCtx := newGreetingMessageContext(bot, chat, admin, "/setwelcome Hello {first}")
	if err := greetingsModule.setWelcome(bot, setWelcomeCtx); err != ext.EndGroups {
		t.Fatalf("setWelcome error = %v, want EndGroups", err)
	}
	if got := db.GetGreetingSettings(chat.Id).WelcomeSettings.WelcomeText; got != "Hello {first}" {
		t.Fatalf("welcome text = %q, want command text", got)
	}

	setGoodbyeCtx := newGreetingMessageContext(bot, chat, admin, "/setgoodbye Bye {first}")
	if err := greetingsModule.setGoodbye(bot, setGoodbyeCtx); err != ext.EndGroups {
		t.Fatalf("setGoodbye error = %v, want EndGroups", err)
	}
	if got := db.GetGreetingSettings(chat.Id).GoodbyeSettings.GoodbyeText; got != "Bye {first}" {
		t.Fatalf("goodbye text = %q, want command text", got)
	}

	resetWelcomeCtx := newGreetingMessageContext(bot, chat, admin, "/resetwelcome")
	if err := greetingsModule.resetWelcome(bot, resetWelcomeCtx); err != ext.EndGroups {
		t.Fatalf("resetWelcome error = %v, want EndGroups", err)
	}
	if got := db.GetGreetingSettings(chat.Id).WelcomeSettings.WelcomeText; got != db.DefaultWelcome {
		t.Fatalf("welcome text after reset = %q, want default", got)
	}

	resetGoodbyeCtx := newGreetingMessageContext(bot, chat, admin, "/resetgoodbye")
	if err := greetingsModule.resetGoodbye(bot, resetGoodbyeCtx); err != ext.EndGroups {
		t.Fatalf("resetGoodbye error = %v, want EndGroups", err)
	}
	if got := db.GetGreetingSettings(chat.Id).GoodbyeSettings.GoodbyeText; got != db.DefaultGoodbye {
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
	if !db.GetGreetingSettings(chat.Id).WelcomeSettings.CleanWelcome {
		t.Fatal("clean welcome did not enable for new chat")
	}

	cleanGoodbyeCtx := newGreetingMessageContext(bot, chat, admin, "/cleangoodbye on")
	if err := greetingsModule.cleanGoodbye(bot, cleanGoodbyeCtx); err != ext.EndGroups {
		t.Fatalf("cleanGoodbye error = %v, want EndGroups", err)
	}
	if !db.GetGreetingSettings(chat.Id).GoodbyeSettings.CleanGoodbye {
		t.Fatal("clean goodbye did not enable for new chat")
	}

	cleanServiceCtx := newGreetingMessageContext(bot, chat, admin, "/cleanservice on")
	if err := greetingsModule.delJoined(bot, cleanServiceCtx); err != ext.EndGroups {
		t.Fatalf("delJoined error = %v, want EndGroups", err)
	}
	if !db.GetGreetingSettings(chat.Id).ShouldCleanService {
		t.Fatal("clean service did not enable for new chat")
	}

	autoApproveCtx := newGreetingMessageContext(bot, chat, admin, "/autoapprove on")
	if err := greetingsModule.autoApprove(bot, autoApproveCtx); err != ext.EndGroups {
		t.Fatalf("autoApprove error = %v, want EndGroups", err)
	}
	if !db.GetGreetingSettings(chat.Id).ShouldAutoApprove {
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
				return !db.GetGreetingSettings(chatID).WelcomeSettings.CleanWelcome
			},
		},
		{
			name:    "clean goodbye off",
			command: "/cleangoodbye no",
			run:     greetingsModule.cleanGoodbye,
			verify: func(chatID int64) bool {
				return !db.GetGreetingSettings(chatID).GoodbyeSettings.CleanGoodbye
			},
		},
		{
			name:    "clean service off",
			command: "/cleanservice off",
			run:     greetingsModule.delJoined,
			verify: func(chatID int64) bool {
				return !db.GetGreetingSettings(chatID).ShouldCleanService
			},
		},
		{
			name:    "auto approve off",
			command: "/autoapprove no",
			run:     greetingsModule.autoApprove,
			verify: func(chatID int64) bool {
				return !db.GetGreetingSettings(chatID).ShouldAutoApprove
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
	if err := db.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
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
	if err := db.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}
	if err := db.SetGoodbyeText(chat.Id, "Bye {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetGoodbyeText setup error = %v", err)
	}
	if err := db.SetGoodbyeToggle(chat.Id, true); err != nil {
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
	if err := db.SetWelcomeText(chat.Id, "Service welcome {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}
	if err := db.SetShouldCleanService(chat.Id, true); err != nil {
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
	if err := db.SetWelcomeText(chat.Id, "Hello {first}", "", nil, db.TEXT); err != nil {
		t.Fatalf("SetWelcomeText setup error = %v", err)
	}
	if err := db.SetShouldCleanService(chat.Id, true); err != nil {
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
	if err := db.SetWelcomeText(chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
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
		"join_request.accept.5151",
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
			name:        "legacy decline",
			data:        "join_request.decline.7171",
			wantDecline: 1,
		},
		{
			name:        "encoded ban",
			data:        encodeCallbackData("join_request", map[string]string{"a": "ban", "u": "7171"}, "join_request.ban.7171"),
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

		data := encodeCallbackData("join_request", map[string]string{"a": "accept", "u": "nan"}, "join_request.accept.nan")
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
	if err := db.SetShouldAutoApprove(chat.Id, true); err != nil {
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
