//go:build testtools

package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

func withDisableCommands(t *testing.T, cmds ...string) {
	t.Helper()
	helpers.SetDisableCmdsForTest(t, cmds)
}

func TestDisableEnableTogglesKnownCommandsAndReportsUnknown(t *testing.T) {
	withDisableCommands(t, "rules", "stat")
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Disable Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	disableCtx := newModuleMessageContext(bot, chat, user, "/disable rules stat nope")
	if err := disablingModule.disable(bot, disableCtx); err != ext.EndGroups {
		t.Fatalf("disable() error = %v, want EndGroups", err)
	}
	disabled := db.GetChatDisabledCMDs(chatID)
	if !containsString(disabled, "rules") || !containsString(disabled, "stat") {
		t.Fatalf("disabled commands = %v, want rules and stat", disabled)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls after /disable = %d, want success and unknown replies", len(calls))
	}

	enableCtx := newModuleMessageContext(bot, chat, user, "/enable rules")
	if err := disablingModule.enable(bot, enableCtx); err != ext.EndGroups {
		t.Fatalf("enable() error = %v, want EndGroups", err)
	}
	disabled = db.GetChatDisabledCMDs(chatID)
	if containsString(disabled, "rules") {
		t.Fatalf("rules remained disabled after /enable: %v", disabled)
	}
	if !containsString(disabled, "stat") {
		t.Fatalf("stat was unexpectedly enabled: %v", disabled)
	}
}

func TestDisableWithoutArgumentsRepliesWithoutChangingState(t *testing.T) {
	withDisableCommands(t, "rules")
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Disable Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/disable")
	if err := disablingModule.disable(bot, ctx); err != ext.EndGroups {
		t.Fatalf("disable() error = %v, want EndGroups", err)
	}
	if disabled := db.GetChatDisabledCMDs(chat.Id); len(disabled) != 0 {
		t.Fatalf("disabled commands = %v, want none", disabled)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestDisableableListsRegisteredCommands(t *testing.T) {
	withDisableCommands(t, "rules", "stat")
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Disable Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newModuleMessageContext(bot, chat, user, "/disableable")
	if err := disablingModule.disableable(bot, ctx); err != ext.EndGroups {
		t.Fatalf("disableable() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text := calls[0].Params["text"].(string)
	if !strings.Contains(text, "`rules`") || !strings.Contains(text, "`stat`") {
		t.Fatalf("disableable text = %q, want registered commands", text)
	}
}

func TestDisabledListsCurrentCommandsOrEmptyState(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Disable Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}

	emptyCtx := newModuleMessageContext(bot, chat, user, "/disabled")
	if err := disablingModule.disabled(bot, emptyCtx); err != ext.EndGroups {
		t.Fatalf("disabled() empty error = %v, want EndGroups", err)
	}

	if err := db.DisableCMD(chatID, "rules"); err != nil {
		t.Fatalf("DisableCMD() error = %v", err)
	}
	listCtx := newModuleMessageContext(bot, chat, user, "/disabled")
	if err := disablingModule.disabled(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("disabled() list error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want 2", len(calls))
	}
	text := calls[1].Params["text"].(string)
	if !strings.Contains(text, "`rules`") {
		t.Fatalf("disabled text = %q, want rules", text)
	}
}

func TestDisabledelShowsAndTogglesDeletePreference(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Disable Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	currentCtx := newModuleMessageContext(bot, chat, user, "/disabledel")
	if err := disablingModule.disabledel(bot, currentCtx); err != ext.EndGroups {
		t.Fatalf("disabledel current error = %v, want EndGroups", err)
	}

	onCtx := newModuleMessageContext(bot, chat, user, "/disabledel yes")
	if err := disablingModule.disabledel(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("disabledel yes error = %v, want EndGroups", err)
	}
	if !db.ShouldDel(chatID) {
		t.Fatal("delete preference was not enabled")
	}

	invalidCtx := newModuleMessageContext(bot, chat, user, "/disabledel maybe")
	if err := disablingModule.disabledel(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("disabledel invalid error = %v, want EndGroups", err)
	}
	if !db.ShouldDel(chatID) {
		t.Fatal("invalid disabledel option changed delete preference")
	}

	offCtx := newModuleMessageContext(bot, chat, user, "/disabledel off")
	if err := disablingModule.disabledel(bot, offCtx); err != ext.EndGroups {
		t.Fatalf("disabledel off error = %v, want EndGroups", err)
	}
	if db.ShouldDel(chatID) {
		t.Fatal("delete preference stayed enabled")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
