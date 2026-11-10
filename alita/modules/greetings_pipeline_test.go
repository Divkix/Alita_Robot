//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
)

func TestGreetingCommandsRunOnlyAfterAdminProof(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		command string
		value   func(int64) string
		want    string
	}{
		{"/setwelcome Hello {first}", func(id int64) string { return greetings.GetGreetingSettings(id).WelcomeSettings.WelcomeText }, "Hello {first}"},
		{"/setgoodbye Bye {first}", func(id int64) string { return greetings.GetGreetingSettings(id).GoodbyeSettings.GoodbyeText }, "Bye {first}"},
		{"/resetwelcome", func(id int64) string { return greetings.GetGreetingSettings(id).WelcomeSettings.WelcomeText }, db.DefaultWelcome},
		{"/resetgoodbye", func(id int64) string { return greetings.GetGreetingSettings(id).GoodbyeSettings.GoodbyeText }, db.DefaultGoodbye},
		{"/cleanwelcome on", func(id int64) string {
			return strconv.FormatBool(greetings.GetGreetingSettings(id).WelcomeSettings.CleanWelcome)
		}, "true"},
		{"/cleangoodbye on", func(id int64) string {
			return strconv.FormatBool(greetings.GetGreetingSettings(id).GoodbyeSettings.CleanGoodbye)
		}, "true"},
		{"/cleanservice on", func(id int64) string { return strconv.FormatBool(greetings.GetGreetingSettings(id).ShouldCleanService) }, "true"},
		{"/autoapprove on", func(id int64) string { return strconv.FormatBool(greetings.GetGreetingSettings(id).ShouldAutoApprove) }, "true"},
	} {
		for _, limited := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/limited=%t", tc.command, limited), func(t *testing.T) {
				f := newAdminChatFixture(t)
				if limited {
					key := fmt.Sprint(f.admin.Id)
					full := string(f.client.membersByUser[key])
					restricted := strings.ReplaceAll(full, `"can_change_info":true`, `"can_change_info":false`)
					f.client.membersByUser[key] = json.RawMessage(restricted)
					f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(strings.ReplaceAll(string(f.client.adminsByChat[fmt.Sprint(f.chat.Id)]), full, restricted))
				}
				if err := greetings.SetWelcomeText(f.chat.Id, "Original welcome", "", nil, db.TEXT); err != nil {
					t.Fatal(err)
				}
				if err := greetings.SetGoodbyeText(f.chat.Id, "Original goodbye", "", nil, db.TEXT); err != nil {
					t.Fatal(err)
				}
				d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
				LoadGreetings(d)
				LoadBotUpdates(d)
				before := tc.value(f.chat.Id)
				tap := anonymousAdminProof(t, d, f, newModuleMessageContext(f.bot, f.chat, f.admin, tc.command))
				if got := tc.value(f.chat.Id); got != before {
					t.Fatalf("before proof value = %q, want unchanged %q", got, before)
				}
				sentBefore := len(f.client.callsFor("sendMessage"))
				if err := d.ProcessUpdate(f.bot, tap, nil); err != nil {
					t.Fatal(err)
				}
				want := tc.want
				if limited {
					want = before
				}
				if got := tc.value(f.chat.Id); got != want {
					t.Fatalf("after proof value = %q, want %q", got, want)
				}
				sent := f.client.callsFor("sendMessage")[sentBefore:]
				if len(sent) != 1 {
					t.Fatalf("after proof replies = %v, want one result", sent)
				}
				if limited {
					assertParams(t, sent[0], map[string]any{"text": "You don't have permission to change info in this group!"})
				}
			})
		}
	}
}

func TestGreetingPipelineKeepsRefusalOrder(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		command  string
		botAdmin bool
	}{
		{"/welcome off", false},
		{"/goodbye on", false},
		{"/setwelcome Hello", false},
		{"/setgoodbye Bye", false},
		{"/resetwelcome", false},
		{"/resetgoodbye", false},
		{"/cleanwelcome on", false},
		{"/cleangoodbye on", false},
		{"/cleanservice on", true},
		{"/autoapprove on", true},
	} {
		for _, source := range []string{"group", "connected PM", "named PM"} {
			for _, missingBot := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/missingBot=%t", tc.command, source, missingBot), func(t *testing.T) {
					f := newAdminChatFixture(t)
					if missingBot {
						f.client.membersByUser["999"] = json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Bot"}}`)
						f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + string(f.client.membersByUser[fmt.Sprint(f.admin.Id)]) + "]")
					}
					origin, command := f.chat, tc.command
					if source != "group" {
						origin = privateChatOf(f.member)
						if err := connections.ConnectId(f.member.Id, f.chat.Id); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })
						if source == "named PM" {
							if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
								t.Fatal(err)
							}
							parts := strings.SplitN(command, " ", 2)
							command = fmt.Sprintf("%s %d", parts[0], f.chat.Id)
							if len(parts) == 2 {
								command += " " + parts[1]
							}
						}
					}
					d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
					LoadGreetings(d)
					if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, origin, f.member, command).Update, nil); err != nil {
						t.Fatal(err)
					}
					want := "You need to be an admin to do this."
					if tc.botAdmin && missingBot {
						want = "I'm not an admin in this chat!"
					}
					if got := sentTexts(f.client); len(got) != 1 || got[0] != want {
						t.Fatalf("replies = %q, want only %q", got, want)
					}
				})
			}
		}
	}
}
