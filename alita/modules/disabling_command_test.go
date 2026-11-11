//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
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
	disabled := disabling.GetChatDisabledCMDs(chatID)
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
	disabled = disabling.GetChatDisabledCMDs(chatID)
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
	if disabled := disabling.GetChatDisabledCMDs(chat.Id); len(disabled) != 0 {
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

// /export and /import are admin commands: loading the Backup module must not
// offer them in /disableable, and /disable must refuse them as unknown.
func TestBackupCommandsAreNotDisableable(t *testing.T) {
	withDisableCommands(t, "rules")
	withExtendRegistry(t)
	LoadBackup(ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}))
	f := newAdminChatFixture(t)

	listCtx := newModuleMessageContext(f.bot, f.chat, f.member, "/disableable")
	if err := disablingModule.disableable(f.bot, listCtx); err != ext.EndGroups {
		t.Fatalf("disableable() error = %v, want EndGroups", err)
	}
	calls := f.client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls after /disableable = %d, want 1", len(calls))
	}
	listText := calls[0].Params["text"].(string)
	if !strings.Contains(listText, "`rules`") {
		t.Fatalf("disableable text = %q, want the registered command", listText)
	}
	for _, cmd := range []string{"export", "import"} {
		if strings.Contains(listText, "`"+cmd+"`") {
			t.Fatalf("disableable text = %q, want no %s", listText, cmd)
		}
	}

	disableCtx := newModuleMessageContext(f.bot, f.chat, f.admin, "/disable rules export import")
	if err := disablingModule.disable(f.bot, disableCtx); err != ext.EndGroups {
		t.Fatalf("disable() error = %v, want EndGroups", err)
	}
	if disabled := disabling.GetChatDisabledCMDs(f.chat.Id); len(disabled) != 1 || disabled[0] != "rules" {
		t.Fatalf("disabled commands = %v, want only rules", disabled)
	}
	replies := f.client.callsFor("sendMessage")[1:]
	if len(replies) != 3 {
		t.Fatalf("sendMessage calls after /disable = %d, want success and two refusals", len(replies))
	}
	for _, cmd := range []string{"export", "import"} {
		named := 0
		for _, reply := range replies {
			if strings.Contains(reply.Params["text"].(string), cmd) {
				named++
			}
		}
		if named != 1 {
			t.Fatalf("replies naming %s = %d, want its one refusal", cmd, named)
		}
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

	if err := disabling.DisableCMD(chatID, "rules"); err != nil {
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestDisabledHonoursTargetGroupSettingInPM(t *testing.T) {
	useRealLocales(t)
	withDisableCommands(t)
	for _, named := range []bool{false, true} {
		for _, admin := range []bool{false, true} {
			for _, disabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("named=%t/admin=%t/disabled=%t", named, admin, disabled), func(t *testing.T) {
					f := newAdminChatFixture(t)
					user := f.member
					if admin {
						user = f.admin
					}
					if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
						t.Fatal(err)
					}
					if err := connections.ConnectId(user.Id, f.chat.Id); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = connections.DisconnectId(user.Id) })
					if disabled {
						if err := disabling.DisableCMD(f.chat.Id, "disabled"); err != nil {
							t.Fatal(err)
						}
					}
					if err := disabling.ToggleDel(f.chat.Id, true); err != nil {
						t.Fatal(err)
					}
					d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
					LoadDisabling(d)
					text := "/disabled"
					if named {
						text += fmt.Sprint(" ", f.chat.Id)
					}
					ctx := newModuleMessageContext(f.bot, privateChatOf(user), user, text)
					if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
						t.Fatal(err)
					}
					want := 1
					if disabled && !admin {
						want = 0
					}
					if got := len(sentTexts(f.client)); got != want {
						t.Fatalf("replies = %d, want %d", got, want)
					}
					if got := len(f.client.callsFor("deleteMessage")); got != 0 {
						t.Fatalf("PM deletions = %d, want none", got)
					}
				})
			}
		}
	}
}

func TestDisablingCommandRightsThroughDispatcher(t *testing.T) {
	useRealLocales(t)
	withDisableCommands(t, "report")
	for _, text := range []string{"/disable report", "/enable report", "/disabledel yes", "/disabled", "/disableable"} {
		for _, admin := range []bool{false, true} {
			for _, botAdmin := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/admin=%t/bot-admin=%t", text, admin, botAdmin), func(t *testing.T) {
					f := newAdminChatFixture(t)
					user := f.member
					if admin {
						user = f.admin
					}
					if !botAdmin {
						f.client.membersByUser["999"] = json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
						f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + string(f.client.membersByUser[fmt.Sprint(f.admin.Id)]) + "]")
					}
					if text == "/enable report" {
						if err := disabling.DisableCMD(f.chat.Id, "report"); err != nil {
							t.Fatal(err)
						}
					}
					d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
					LoadDisabling(d)
					ctx := newModuleMessageContext(f.bot, f.chat, user, text)
					if err := d.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
						t.Fatal(err)
					}
					key := ""
					if text != "/disableable" {
						if !botAdmin {
							key = "connections_is_user_connected_bot_not_admin"
						} else if !admin && text != "/disabled" {
							key = "connections_is_user_connected_user_not_admin"
						}
					}
					call := f.client.onlyCall(t, "sendMessage")
					if key != "" {
						assertParams(t, call, map[string]any{"text": trS(ctxTr(ctx), key)})
					}
					switch text {
					case "/disable report":
						if got := disabling.IsCommandDisabled(f.chat.Id, "report"); got != (key == "") {
							t.Fatalf("report disabled = %t, refusal = %q", got, key)
						}
					case "/enable report":
						if got := disabling.IsCommandDisabled(f.chat.Id, "report"); got != (key != "") {
							t.Fatalf("report disabled = %t, refusal = %q", got, key)
						}
					case "/disabledel yes":
						if got := disabling.ShouldDel(f.chat.Id); got != (key == "") {
							t.Fatalf("delete enabled = %t, refusal = %q", got, key)
						}
					}
				})
			}
		}
	}
}
