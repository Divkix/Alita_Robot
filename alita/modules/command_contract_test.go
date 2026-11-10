//go:build testtools

package modules

import (
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// This sweep discovers names from the dispatcher, acceptance from /disable,
// and aliases from shared response closures or observed cross-name suppression.
// Keep inputs argument-free unless an argument is needed to reach a proof;
// commands which normally call external services then answer with their usage.
func TestEveryCommandHonoursReplayAndDisableContract(t *testing.T) {
	useRealLocales(t)
	withMiniredis(t)
	withDisableCommands(t)
	withExtendRegistry(t)
	withMiscHTTPClient(t, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Errorf("command sweep attempted external HTTP %s; give this command an offline input", req.URL.Redacted())
		return nil, fmt.Errorf("external HTTP is forbidden in the command sweep")
	})})

	all := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAllModules(all)
	commands := dispatcherCommands(t, all)
	slices.SortFunc(commands, func(a, b handlers.Command) int { return strings.Compare(a.Command, b.Command) })
	if len(commands) == 0 {
		t.Fatal("LoadAllModules registered no commands")
	}
	byName := make(map[string]handlers.Command, len(commands))
	for _, command := range commands {
		byName[command.Command] = command
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	disable, ok := byName["disable"]
	if !ok {
		t.Fatal("LoadAllModules did not register /disable")
	}

	failures := make(map[string]bool)
	check := func(t *testing.T, name, clause string, passed bool, detail string) {
		t.Helper()
		if passed {
			return
		}
		key := name + "/" + clause
		failures[key] = true
		if _, known := commandContractExceptions[key]; !known {
			t.Errorf("command contract %q: %s; fix the command, or add exactly %q: \"#ticket: reason\" to commandContractExceptions", key, detail, key)
		}
	}

	accepted := make(map[string]bool, len(names))
	t.Run("discover-disable-acceptance", func(t *testing.T) {
		f := newAdminChatFixture(t)
		d := commandContractDispatcher(disable)
		for _, name := range names {
			before := commandContractCallCount(f.client)
			commandContractProcess(t, d, f, f.admin, "/disable "+name, false)
			if len(commandContractEffects(f.client, before, 0)) == 0 {
				t.Fatalf("/disable %s gave no answer; cannot discover acceptance", name)
			}
			accepted[name] = disabling.IsCommandDisabled(f.chat.Id, name)
			commandContractSetRow(t, f.chat.Id, name, false)
		}
	})
	acceptedCount := 0
	for _, allowed := range accepted {
		if allowed {
			acceptedCount++
		}
	}
	if acceptedCount == 0 {
		t.Fatal("/disable accepted no registered names; the sweep cannot exercise member suppression")
	}

	aliases := make(map[string]map[string]bool)
	link := func(a, b string) {
		for _, pair := range [][2]string{{a, b}, {b, a}} {
			if aliases[pair[0]] == nil {
				aliases[pair[0]] = make(map[string]bool)
			}
			aliases[pair[0]][pair[1]] = true
		}
	}
	// reflect.Value equality preserves the identity of copied function values,
	// unlike Pointer(), which identifies only their shared code. Pipeline aliases
	// and MultiCommand names receive copies of the very same response closure.
	closures := make(map[reflect.Value][]string)
	for _, name := range names {
		response := reflect.ValueOf(byName[name].Response)
		for _, sibling := range closures[response] {
			link(name, sibling)
		}
		closures[response] = append(closures[response], name)
	}

	proofs := 0
	for _, name := range names {
		command := byName[name]
		t.Run(name, func(t *testing.T) {
			t.Run("anonymous", func(t *testing.T) {
				f := newAdminChatFixture(t)
				if err := admin.SetAnonAdminMode(f.chat.Id, false); err != nil {
					t.Fatalf("SetAnonAdminMode: %v", err)
				}
				d := commandContractDispatcher(command)
				anonymous := gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Group", Username: "GroupAnonymousBot"}
				before := commandContractCallCount(f.client)
				commandContractProcess(t, d, f, anonymous, commandContractInput(name), true)
				initial := commandContractEffects(f.client, before, 0)
				var data string
				for _, call := range initial {
					if markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup); ok {
						for _, row := range markup.InlineKeyboard {
							for _, button := range row {
								if strings.HasPrefix(button.CallbackData, "anon_admin|") {
									data = button.CallbackData
								}
							}
						}
					}
				}
				if data == "" {
					return
				}
				proofs++
				check(t, name, "proof-only", len(initial) == 1 && initial[0].Method == "sendMessage", fmt.Sprintf("before confirmation got %s, want only the proof", commandContractDescribe(initial)))

				const promptID int64 = 600
				prompt := gotgbot.Message{MessageId: promptID, Date: 1, Chat: f.chat, From: &f.bot.User, Text: "confirm administrator"}
				query := &gotgbot.CallbackQuery{Id: "contract-proof", From: f.admin, Message: prompt, Data: data, ChatInstance: "contract-chat"}
				before = commandContractCallCount(f.client)
				if err := d.ProcessUpdate(f.bot, &gotgbot.Update{UpdateId: 2, CallbackQuery: query}, nil); err != nil {
					t.Fatalf("ProcessUpdate(proof): %v", err)
				}
				after := commandContractEffects(f.client, before, promptID)
				check(t, name, "proof-replay", len(after) != 0, "ordinary fixture admin tapped the emitted proof but no further reply or action occurred")
				deleted := false
				for _, call := range f.client.callsFor("deleteMessage") {
					if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(f.chat.Id) && fmt.Sprint(call.Params["message_id"]) == fmt.Sprint(promptID) {
						deleted = true
					}
				}
				check(t, name, "proof-deleted", deleted, "confirmed proof message was not deleted")
			})

			t.Run("stored-row", func(t *testing.T) {
				f := newAdminChatFixture(t)
				d := commandContractDispatcher(command)
				memberBefore := commandContractOutcome(t, d, f, f.member, name)
				adminBefore := commandContractOutcome(t, d, f, f.admin, name)
				commandContractSetRow(t, f.chat.Id, name, true)
				memberAfter := commandContractOutcome(t, d, f, f.member, name)
				adminAfter := commandContractOutcome(t, d, f, f.admin, name)
				if accepted[name] {
					if len(memberBefore) == 0 || len(adminBefore) == 0 {
						t.Fatalf("/%s is accepted by /disable but has no observable baseline (member=%s admin=%s); add an offline commandContractInput", name, commandContractDescribe(memberBefore), commandContractDescribe(adminBefore))
					}
					check(t, name, "disable-member", len(memberAfter) == 0, fmt.Sprintf("/disable accepts this name but member still got %s", commandContractDescribe(memberAfter)))
					check(t, name, "disable-admin", commandContractSameEffects(name, adminBefore, adminAfter), fmt.Sprintf("admin outcome changed: before=%s after=%s", commandContractDescribe(adminBefore), commandContractDescribe(adminAfter)))
				} else {
					check(t, name, "rejected-member", commandContractSameEffects(name, memberBefore, memberAfter), fmt.Sprintf("/disable rejects this name but its stored row changed member outcome: before=%s after=%s", commandContractDescribe(memberBefore), commandContractDescribe(memberAfter)))
					check(t, name, "rejected-admin", commandContractSameEffects(name, adminBefore, adminAfter), fmt.Sprintf("/disable rejects this name but its stored row changed admin outcome: before=%s after=%s", commandContractDescribe(adminBefore), commandContractDescribe(adminAfter)))
				}
			})
		})
	}
	if proofs == 0 {
		t.Error("no registered command requested anonymous-admin proof; the sweep did not exercise replay")
	}

	// Legacy aliases can have separately allocated PM wrapper closures. Discover
	// their relationship through a row for an accepted name, not help/descriptors.
	// Code identity only narrows the probes; it never proves an alias relationship.
	t.Run("discover-legacy-aliases", func(t *testing.T) {
		for _, source := range names {
			if !accepted[source] {
				continue
			}
			f := newAdminChatFixture(t)
			for _, target := range names {
				if source == target || reflect.ValueOf(byName[source].Response).Pointer() != reflect.ValueOf(byName[target].Response).Pointer() {
					continue
				}
				d := commandContractDispatcher(byName[target])
				commandContractSetRow(t, f.chat.Id, source, false)
				before := commandContractOutcome(t, d, f, f.member, target)
				commandContractSetRow(t, f.chat.Id, source, true)
				after := commandContractOutcome(t, d, f, f.member, target)
				if len(before) != 0 && len(after) == 0 {
					link(source, target)
				}
			}
		}
	})

	// Close the observed/shared relationships transitively, so a three-name
	// command is checked in every direction, including rows for refused aliases.
	for _, middle := range names {
		for _, source := range names {
			if !aliases[source][middle] {
				continue
			}
			for target := range aliases[middle] {
				if source != target {
					link(source, target)
				}
			}
		}
	}
	for _, source := range names {
		if len(aliases[source]) == 0 {
			continue
		}
		disableable := accepted[source]
		for target := range aliases[source] {
			disableable = disableable || accepted[target]
		}
		if !disableable {
			continue
		}
		t.Run(source+"/alias-rows", func(t *testing.T) {
			f := newAdminChatFixture(t)
			var memberFailures, adminFailures []string
			for _, target := range names {
				if source == target || !aliases[source][target] {
					continue
				}
				d := commandContractDispatcher(byName[target])
				commandContractSetRow(t, f.chat.Id, source, false)
				memberBefore := commandContractOutcome(t, d, f, f.member, target)
				adminBefore := commandContractOutcome(t, d, f, f.admin, target)
				commandContractSetRow(t, f.chat.Id, source, true)
				memberAfter := commandContractOutcome(t, d, f, f.member, target)
				adminAfter := commandContractOutcome(t, d, f, f.admin, target)
				if len(memberBefore) == 0 {
					t.Fatalf("/%s has no member baseline when checking row %s; add an offline commandContractInput", target, source)
				}
				if len(memberAfter) != 0 {
					memberFailures = append(memberFailures, fmt.Sprintf("/%s got %s", target, commandContractDescribe(memberAfter)))
				}
				if !commandContractSameEffects(target, adminBefore, adminAfter) {
					adminFailures = append(adminFailures, fmt.Sprintf("/%s before=%s after=%s", target, commandContractDescribe(adminBefore), commandContractDescribe(adminAfter)))
				}
			}
			check(t, source, "alias-member", len(memberFailures) == 0, "row must silence every alias for members: "+strings.Join(memberFailures, "; "))
			check(t, source, "alias-admin", len(adminFailures) == 0, "row must leave every alias usable by admins: "+strings.Join(adminFailures, "; "))
		})
	}
	for key := range commandContractExceptions {
		if !failures[key] {
			t.Errorf("stale command contract exception %q: clause now passes or command disappeared; remove exactly %q from commandContractExceptions", key, key)
		}
	}
	t.Logf("observed %d registered names, %d accepted disable names, %d anonymous proof flows", len(names), acceptedCount, proofs)
}

func commandContractDispatcher(command handlers.Command) *ext.Dispatcher {
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	d.AddHandler(command)
	LoadBotUpdates(d)
	return d
}

func commandContractInput(name string) string {
	if name == "clear" || name == "rmnote" {
		return "/" + name + " contract-note"
	}
	return "/" + name
}

func commandContractProcess(t *testing.T, d *ext.Dispatcher, f adminChatFixture, user gotgbot.User, text string, anonymous bool) {
	t.Helper()
	update := newModuleMessageContext(f.bot, f.chat, user, text).Update
	update.Message.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len(strings.Fields(text)[0]))}}
	if anonymous {
		update.Message.SenderChat = &f.chat
	}
	if err := d.ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%q, user %d): %v", text, user.Id, err)
	}
}

func commandContractSetRow(t *testing.T, chatID int64, name string, disabled bool) {
	t.Helper()
	var err error
	if disabled {
		err = disabling.DisableCMD(chatID, name)
	} else {
		err = disabling.EnableCMD(chatID, name)
	}
	if err != nil {
		t.Fatalf("set disable row %q to %t: %v", name, disabled, err)
	}
	if got := disabling.IsCommandDisabled(chatID, name); got != disabled {
		t.Fatalf("stored disable row %q = %t, want %t", name, got, disabled)
	}
}

func commandContractCallCount(client *moduleBotClient) int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return len(client.calls)
}

func commandContractOutcome(t *testing.T, d *ext.Dispatcher, f adminChatFixture, user gotgbot.User, name string) []moduleBotCall {
	t.Helper()
	// The baseline must not consume the independent export cooldown for the
	// stored-row attempt: both attempts need to reach the real document upload.
	if name == "export" {
		if err := cache.GetRedisClient().Del(cache.Context, fmt.Sprintf("backup:export:%d", f.chat.Id)).Err(); err != nil {
			t.Fatalf("reset export fixture cooldown: %v", err)
		}
	}
	before := commandContractCallCount(f.client)
	commandContractProcess(t, d, f, user, commandContractInput(name), false)
	return commandContractEffects(f.client, before, 0)
}

func commandContractEffects(client *moduleBotClient, offset int, proofID int64) []moduleBotCall {
	client.mu.Lock()
	defer client.mu.Unlock()
	var effects []moduleBotCall
	for _, call := range client.calls[offset:] {
		if strings.HasPrefix(call.Method, "get") || call.Method == "answerCallbackQuery" {
			continue
		}
		if proofID != 0 && call.Method == "deleteMessage" && fmt.Sprint(call.Params["message_id"]) == fmt.Sprint(proofID) {
			continue
		}
		effects = append(effects, call)
	}
	return effects
}

// Compare replies and action targets, not permission lookups. Only intrinsically
// variable text is omitted: latency, random /runs, and /disabled listing its row.
func commandContractSameEffects(name string, before, after []moduleBotCall) bool {
	signatures := func(calls []moduleBotCall) []string {
		values := make([]string, 0, len(calls))
		for _, call := range calls {
			signature := call.Method
			for _, key := range []string{"chat_id", "user_id", "message_id", "text", "caption"} {
				if key == "text" && (name == "ping" || name == "runs" || name == "disabled") {
					continue
				}
				if value, ok := call.Params[key]; ok {
					signature += fmt.Sprintf("|%s=%v", key, value)
				}
			}
			values = append(values, signature)
		}
		slices.Sort(values)
		return values
	}
	return slices.Equal(signatures(before), signatures(after))
}

func commandContractDescribe(calls []moduleBotCall) string {
	var descriptions []string
	for _, call := range calls {
		if text, ok := call.Params["text"]; ok {
			descriptions = append(descriptions, fmt.Sprintf("%s(%q)", call.Method, text))
		} else {
			descriptions = append(descriptions, call.Method)
		}
	}
	return "[" + strings.Join(descriptions, ", ") + "]"
}
