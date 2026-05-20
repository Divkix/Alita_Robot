package modules

import (
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func newWarnReplyContext(
	bot *gotgbot.Bot,
	chat gotgbot.Chat,
	admin gotgbot.User,
	target gotgbot.User,
	text string,
) *ext.Context {
	ctx := newModuleMessageContext(bot, chat, admin, text)
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 202,
		Date:      1,
		Chat:      chat,
		From:      &target,
		Text:      "message being warned",
	}
	return ctx
}

func TestWarnSettingsCommandsUpdateAndDisplay(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	limitCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit 5")
	if err := warnsModule.setWarnLimit(bot, limitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit() error = %v, want EndGroups", err)
	}
	if got := db.GetWarnSetting(chat.Id).WarnLimit; got != 5 {
		t.Fatalf("warn limit = %d, want 5", got)
	}

	invalidCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit 0")
	if err := warnsModule.setWarnLimit(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit invalid error = %v, want EndGroups", err)
	}
	if got := db.GetWarnSetting(chat.Id).WarnLimit; got != 5 {
		t.Fatalf("invalid warn limit changed setting to %d", got)
	}

	modeCtx := newModuleMessageContext(bot, chat, admin, "/setwarnmode ban")
	if err := warnsModule.setWarnMode(bot, modeCtx); err != ext.EndGroups {
		t.Fatalf("setWarnMode() error = %v, want EndGroups", err)
	}
	if got := db.GetWarnSetting(chat.Id).WarnMode; got != "ban" {
		t.Fatalf("warn mode = %q, want ban", got)
	}

	for _, mode := range []string{"kick", "mute"} {
		modeCtx := newModuleMessageContext(bot, chat, admin, "/setwarnmode "+mode)
		if err := warnsModule.setWarnMode(bot, modeCtx); err != ext.EndGroups {
			t.Fatalf("setWarnMode(%s) error = %v, want EndGroups", mode, err)
		}
		if got := db.GetWarnSetting(chat.Id).WarnMode; got != mode {
			t.Fatalf("warn mode = %q, want %q", got, mode)
		}
	}

	missingLimitCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit")
	if err := warnsModule.setWarnLimit(bot, missingLimitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit missing error = %v, want EndGroups", err)
	}
	badLimitCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit nope")
	if err := warnsModule.setWarnLimit(bot, badLimitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit bad number error = %v, want EndGroups", err)
	}
	bigLimitCtx := newModuleMessageContext(bot, chat, admin, "/setwarnlimit 101")
	if err := warnsModule.setWarnLimit(bot, bigLimitCtx); err != ext.EndGroups {
		t.Fatalf("setWarnLimit range error = %v, want EndGroups", err)
	}

	missingModeCtx := newModuleMessageContext(bot, chat, admin, "/setwarnmode")
	if err := warnsModule.setWarnMode(bot, missingModeCtx); err != ext.EndGroups {
		t.Fatalf("setWarnMode missing error = %v, want EndGroups", err)
	}
	unknownModeCtx := newModuleMessageContext(bot, chat, admin, "/setwarnmode freeze")
	if err := warnsModule.setWarnMode(bot, unknownModeCtx); err != ext.EndGroups {
		t.Fatalf("setWarnMode unknown error = %v, want EndGroups", err)
	}

	displayCtx := newModuleMessageContext(bot, chat, admin, "/warnings")
	if err := warnsModule.warnings(bot, displayCtx); err != ext.EndGroups {
		t.Fatalf("warnings() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) < 4 {
		t.Fatalf("sendMessage calls = %d, want at least 4", len(calls))
	}
}

func TestWarnsCommandHandlesNoWarningsAndMissingTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	noWarnsCtx := newModuleMessageContext(bot, chat, admin, "/warns 42")
	if err := warnsModule.warns(bot, noWarnsCtx); err != ext.EndGroups {
		t.Fatalf("warns no-warning error = %v, want EndGroups", err)
	}

	missingWarnCtx := newModuleMessageContext(bot, chat, admin, "/warn")
	if err := warnsModule.warnUser(bot, missingWarnCtx); err != ext.EndGroups {
		t.Fatalf("warn missing target error = %v, want EndGroups", err)
	}
	missingSWarnCtx := newModuleMessageContext(bot, chat, admin, "/swarn")
	if err := warnsModule.sWarnUser(bot, missingSWarnCtx); err != ext.EndGroups {
		t.Fatalf("swarn missing target error = %v, want EndGroups", err)
	}
	missingDWarnCtx := newModuleMessageContext(bot, chat, admin, "/dwarn")
	if err := warnsModule.dWarnUser(bot, missingDWarnCtx); err != ext.EndGroups {
		t.Fatalf("dwarn missing target error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) < 4 {
		t.Fatalf("sendMessage calls = %d, want replies for no warns and missing targets", len(calls))
	}
}

func TestWarnCommandsRejectChannelTargets(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "warn channel", text: "/warn -1001234567890 spam", run: warnsModule.warnUser},
		{name: "swarn channel", text: "/swarn -1001234567890", run: warnsModule.sWarnUser},
		{name: "dwarn channel", text: "/dwarn -1001234567890", run: warnsModule.dWarnUser},
		{name: "warns channel", text: "/warns -1001234567890", run: warnsModule.warns},
		{name: "remove channel", text: "/rmwarn -1001234567890", run: warnsModule.removeWarn},
		{name: "reset channel", text: "/resetwarns -1001234567890", run: warnsModule.resetWarns},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := tt.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none for rejected channel targets", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want none for rejected channel targets", len(calls))
	}
}

func TestWarnReplyStoresReasonAndWarnsListsIt(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	warnCtx := newWarnReplyContext(bot, chat, admin, target, "/warn too noisy")
	if err := warnsModule.warnUser(bot, warnCtx); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}
	numWarns, reasons := db.GetWarns(target.Id, chat.Id)
	if numWarns != 1 {
		t.Fatalf("numWarns = %d, want 1", numWarns)
	}
	if len(reasons) != 1 || reasons[0] != "too noisy" {
		t.Fatalf("reasons = %v, want [too noisy]", reasons)
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/warns 42")
	if err := warnsModule.warns(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("warns() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) < 2 {
		t.Fatalf("sendMessage calls = %d, want warn and list replies", len(calls))
	}
	lastText := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(lastText, "too noisy") {
		t.Fatalf("warn list text = %q, want reason", lastText)
	}
}

func TestWarnsListsCountWhenReasonsAreEmpty(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	if err := db.DB.Create(&db.Warns{
		UserId:   42,
		ChatId:   chat.Id,
		NumWarns: 2,
		Reasons:  db.StringArray{},
	}).Error; err != nil {
		t.Fatalf("create warns fixture: %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, admin, "/warns 42")
	if err := warnsModule.warns(bot, ctx); err != ext.EndGroups {
		t.Fatalf("warns() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want warning count reply", len(calls))
	}
}

func TestWarnLimitPunishesAndResetsWarnings(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.SetWarnLimit(chat.Id, 1); err != nil {
		t.Fatalf("SetWarnLimit() error = %v", err)
	}
	if err := db.SetWarnMode(chat.Id, "ban"); err != nil {
		t.Fatalf("SetWarnMode() error = %v", err)
	}

	warnCtx := newWarnReplyContext(bot, chat, admin, target, "/warn limit reached")
	if err := warnsModule.warnUser(bot, warnCtx); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want 1", len(calls))
	}
	if numWarns, _ := db.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after punishment = %d, want reset to 0", numWarns)
	}
}

func TestWarnLimitKickAndMuteModes(t *testing.T) {
	tests := []struct {
		mode       string
		wantMethod string
	}{
		{mode: "kick", wantMethod: "banChatMember"},
		{mode: "mute", wantMethod: "restrictChatMember"},
	}

	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
			admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
			target := gotgbot.User{Id: 42, FirstName: "Member"}
			if err := db.SetWarnLimit(chat.Id, 1); err != nil {
				t.Fatalf("SetWarnLimit() error = %v", err)
			}
			if err := db.SetWarnMode(chat.Id, tc.mode); err != nil {
				t.Fatalf("SetWarnMode() error = %v", err)
			}

			warnCtx := newWarnReplyContext(bot, chat, admin, target, "/warn limit reached")
			if err := warnsModule.warnUser(bot, warnCtx); err != ext.EndGroups {
				t.Fatalf("warnUser() error = %v, want EndGroups", err)
			}
			if calls := client.callsFor(tc.wantMethod); len(calls) != 1 {
				t.Fatalf("%s calls = %d, want 1", tc.wantMethod, len(calls))
			}
			if numWarns, _ := db.GetWarns(target.Id, chat.Id); numWarns != 0 {
				t.Fatalf("numWarns after punishment = %d, want reset to 0", numWarns)
			}
		})
	}
}

func TestSilentWarnDeletesCommandAndStoresReason(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newWarnReplyContext(bot, chat, admin, target, "/swarn quiet reason")
	if err := warnsModule.sWarnUser(bot, ctx); err != ext.EndGroups {
		t.Fatalf("sWarnUser() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want command deletion", len(calls))
	}
	numWarns, reasons := db.GetWarns(target.Id, chat.Id)
	if numWarns != 1 {
		t.Fatalf("numWarns = %d, want 1", numWarns)
	}
	if len(reasons) != 1 || reasons[0] != "quiet reason" {
		t.Fatalf("reasons = %v, want [quiet reason]", reasons)
	}
}

func TestDeleteWarnDeletesReplyAndStoresReason(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newWarnReplyContext(bot, chat, admin, target, "/dwarn remove this")
	if err := warnsModule.dWarnUser(bot, ctx); err != ext.EndGroups {
		t.Fatalf("dWarnUser() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want replied message deletion", len(calls))
	}
	numWarns, reasons := db.GetWarns(target.Id, chat.Id)
	if numWarns != 1 {
		t.Fatalf("numWarns = %d, want 1", numWarns)
	}
	if len(reasons) != 1 || reasons[0] != "remove this" {
		t.Fatalf("reasons = %v, want [remove this]", reasons)
	}
}

func TestRemoveWarnAndResetWarnsCommands(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	db.WarnUser(target.Id, chat.Id, "first")

	removeCtx := newModuleMessageContext(bot, chat, admin, "/rmwarn 42")
	if err := warnsModule.removeWarn(bot, removeCtx); err != ext.EndGroups {
		t.Fatalf("removeWarn() error = %v, want EndGroups", err)
	}
	if numWarns, _ := db.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after remove = %d, want 0", numWarns)
	}

	db.WarnUser(target.Id, chat.Id, "one")
	db.WarnUser(target.Id, chat.Id, "two")
	resetCtx := newModuleMessageContext(bot, chat, admin, "/resetwarns 42")
	if err := warnsModule.resetWarns(bot, resetCtx); err != ext.EndGroups {
		t.Fatalf("resetWarns() error = %v, want EndGroups", err)
	}
	if numWarns, _ := db.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after reset = %d, want 0", numWarns)
	}

	removeMissingCtx := newModuleMessageContext(bot, chat, admin, "/rmwarn 43")
	if err := warnsModule.removeWarn(bot, removeMissingCtx); err != ext.EndGroups {
		t.Fatalf("removeWarn missing warning error = %v, want EndGroups", err)
	}
	resetMissingCtx := newModuleMessageContext(bot, chat, admin, "/resetwarns")
	if err := warnsModule.resetWarns(bot, resetMissingCtx); err != ext.EndGroups {
		t.Fatalf("resetWarns missing target error = %v, want EndGroups", err)
	}
}

func TestWarnCommandsPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, tt := range []struct {
		name  string
		text  string
		setup func(t *testing.T, chat gotgbot.Chat)
		run   func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "set warn mode help reply", text: "/setwarnmode", run: warnsModule.setWarnMode},
		{name: "set warn limit help reply", text: "/setwarnlimit", run: warnsModule.setWarnLimit},
		{name: "warnings display reply", text: "/warnings", run: warnsModule.warnings},
		{name: "warn missing target reply", text: "/warn", run: warnsModule.warnUser},
		{name: "silent warn missing target reply", text: "/swarn", run: warnsModule.sWarnUser},
		{name: "delete warn missing target reply", text: "/dwarn", run: warnsModule.dWarnUser},
		{name: "warns no warnings reply", text: "/warns 42", run: warnsModule.warns},
		{name: "remove warn missing target reply", text: "/rmwarn", run: warnsModule.removeWarn},
		{name: "remove warn no warnings reply", text: "/rmwarn 42", run: warnsModule.removeWarn},
		{name: "reset warns missing target reply", text: "/resetwarns", run: warnsModule.resetWarns},
		{name: "reset warns success reply", text: "/resetwarns 42", run: warnsModule.resetWarns},
		{
			name: "reset all empty reply",
			text: "/resetallwarns",
			run:  warnsModule.resetAllWarns,
		},
		{
			name: "reset all confirmation reply",
			text: "/resetallwarns",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				db.WarnUser(target.Id, chat.Id, "first")
			},
			run: warnsModule.resetAllWarns,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors["sendMessage"] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
			if tt.setup != nil {
				tt.setup(t, chat)
			}
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestWarnThisUserPropagatesGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, tt := range []struct {
		name         string
		method       string
		mode         string
		callWarnThis bool
	}{
		{name: "member lookup failure", method: "getChatMember", callWarnThis: true},
		{name: "warning reply failure", method: "sendMessage"},
		{name: "ban limit failure", method: "banChatMember", mode: "ban"},
		{name: "kick limit failure", method: "banChatMember", mode: "kick"},
		{name: "mute limit failure", method: "restrictChatMember", mode: "mute"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
			if tt.mode != "" {
				if err := db.SetWarnLimit(chat.Id, 1); err != nil {
					t.Fatalf("SetWarnLimit() error = %v", err)
				}
				if err := db.SetWarnMode(chat.Id, tt.mode); err != nil {
					t.Fatalf("SetWarnMode() error = %v", err)
				}
			}
			ctx := newWarnReplyContext(bot, chat, admin, target, "/warn too noisy")

			var err error
			if tt.callWarnThis {
				err = warnsModule.warnThisUser(bot, ctx, target.Id, "too noisy", "warn")
			} else {
				err = warnsModule.warnUser(bot, ctx)
			}
			if !errors.Is(err, requestErr) {
				t.Fatalf("warnUser() error = %v, want request error", err)
			}
		})
	}
}

func TestRmWarnButtonRemovesWarning(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}
	db.WarnUser(target.Id, chat.Id, "button")
	data := encodeCallbackData("rmWarn", map[string]string{"u": "42"}, "rmWarn.42")

	ctx := newModuleCallbackContext(bot, chat, admin, data)
	if err := warnsModule.rmWarnButton(bot, ctx); err != ext.EndGroups {
		t.Fatalf("rmWarnButton() error = %v, want EndGroups", err)
	}
	if numWarns, _ := db.GetWarns(target.Id, chat.Id); numWarns != 0 {
		t.Fatalf("numWarns after callback remove = %d, want 0", numWarns)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}

func TestRmWarnButtonRejectsMalformedAndInvalidUserCallbacks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, data := range []string{"rmWarn", "rmWarn.not-a-number"} {
		ctx := newModuleCallbackContext(bot, chat, admin, data)
		if err := warnsModule.rmWarnButton(bot, ctx); err != ext.EndGroups {
			t.Fatalf("rmWarnButton(%q) error = %v, want EndGroups", data, err)
		}
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want malformed and invalid answers", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want none for invalid callbacks", len(calls))
	}
}

func TestWarnCallbackHandlersPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	target := gotgbot.User{Id: 42, FirstName: "Member"}

	t.Run("remove warn edit failure", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		client.errors["editMessageText"] = requestErr
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
		db.WarnUser(target.Id, chat.Id, "button")
		data := encodeCallbackData("rmWarn", map[string]string{"u": "42"}, "rmWarn.42")
		ctx := newModuleCallbackContext(bot, chat, admin, data)

		err := warnsModule.rmWarnButton(bot, ctx)
		if !errors.Is(err, requestErr) {
			t.Fatalf("rmWarnButton() error = %v, want edit request error", err)
		}
	})

	t.Run("remove warn answer failure", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		client.errors["answerCallbackQuery"] = requestErr
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
		db.WarnUser(target.Id, chat.Id, "button")
		data := encodeCallbackData("rmWarn", map[string]string{"u": "42"}, "rmWarn.42")
		ctx := newModuleCallbackContext(bot, chat, admin, data)

		err := warnsModule.rmWarnButton(bot, ctx)
		if !errors.Is(err, requestErr) {
			t.Fatalf("rmWarnButton() error = %v, want answer request error", err)
		}
	})

	t.Run("reset all missing callback", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
		ctx := newModuleMessageContext(bot, chat, admin, "/resetallwarns")

		if err := warnsModule.warnsButtonHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("warnsButtonHandler(no callback) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("answerCallbackQuery"); len(calls) != 0 {
			t.Fatalf("answerCallbackQuery calls = %d, want none without callback", len(calls))
		}
	})

	for _, tt := range []struct {
		name   string
		method string
	}{
		{name: "reset all edit failure", method: "editMessageText"},
		{name: "reset all answer failure", method: "answerCallbackQuery"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
			db.WarnUser(target.Id, chat.Id, "first")
			data := encodeCallbackData("rmAllChatWarns", map[string]string{"a": "yes"}, "rmAllChatWarns.yes")
			ctx := newModuleCallbackContext(bot, chat, admin, data)

			err := warnsModule.warnsButtonHandler(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("warnsButtonHandler() error = %v, want request error", err)
			}
		})
	}
}

func TestResetAllWarnsConfirmationAndCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	db.WarnUser(42, chat.Id, "first")
	data := encodeCallbackData("rmAllChatWarns", map[string]string{"a": "yes"}, "rmAllChatWarns.yes")

	confirmCtx := newModuleMessageContext(bot, chat, admin, "/resetallwarns")
	if err := warnsModule.resetAllWarns(bot, confirmCtx); err != ext.EndGroups {
		t.Fatalf("resetAllWarns() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want confirmation reply", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("resetAllWarns confirmation did not include reply_markup")
	}

	callbackCtx := newModuleCallbackContext(bot, chat, admin, data)
	if err := warnsModule.warnsButtonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("warnsButtonHandler() error = %v, want EndGroups", err)
	}
	if got := db.GetAllChatWarns(chat.Id); got != 0 {
		t.Fatalf("GetAllChatWarns() = %d, want 0 after reset all", got)
	}
}

func TestResetAllWarnsHandlesEmptyCancelAndInvalidCallbacks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Warn Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	emptyCtx := newModuleMessageContext(bot, chat, admin, "/resetallwarns")
	if err := warnsModule.resetAllWarns(bot, emptyCtx); err != nil {
		t.Fatalf("resetAllWarns empty error = %v, want nil", err)
	}

	db.WarnUser(42, chat.Id, "first")
	cancelCtx := newModuleCallbackContext(bot, chat, admin, "rmAllChatWarns.no")
	if err := warnsModule.warnsButtonHandler(bot, cancelCtx); err != ext.EndGroups {
		t.Fatalf("warnsButtonHandler cancel error = %v, want EndGroups", err)
	}
	if got := db.GetAllChatWarns(chat.Id); got != 1 {
		t.Fatalf("GetAllChatWarns() = %d, want warning retained after cancel", got)
	}

	invalidCtx := newModuleCallbackContext(bot, chat, admin, "rmAllChatWarns")
	if err := warnsModule.warnsButtonHandler(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("warnsButtonHandler invalid error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want cancel and invalid answers", len(calls))
	}
}

func TestLoadWarnsRegistersHelpAndHandlers(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadWarns(dispatcher)

	if moduleName, enabled := DefaultHelpRegistry().AbleMap.Load(warnsModule.moduleName); moduleName != warnsModule.moduleName || !enabled {
		t.Fatalf("warns help registration = (%q, %v), want enabled", moduleName, enabled)
	}
}
