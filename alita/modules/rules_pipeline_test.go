//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/rules"
)

func TestRulesPipelineKeepsPrivateDeliveryAndHonoursTargetDisable(t *testing.T) {
	useRealLocales(t)
	withDisableCommands(t)
	for _, named := range []bool{false, true} {
		for _, disabled := range []bool{false, true} {
			for _, asAdmin := range []bool{false, true} {
				t.Run(fmt.Sprintf("named=%t/disabled=%t/admin=%t", named, disabled, asAdmin), func(t *testing.T) {
					f := newAdminChatFixture(t)
					user := f.member
					if asAdmin {
						user = f.admin
					}
					if err := rules.SetChatRules(f.chat.Id, "Read these private rules"); err != nil {
						t.Fatal(err)
					}
					if err := rules.SetPrivateRules(f.chat.Id, true); err != nil {
						t.Fatal(err)
					}
					text := "/rules"
					if named {
						text += fmt.Sprintf(" %d", f.chat.Id)
					} else {
						if err := connections.ConnectId(user.Id, f.chat.Id); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = connections.DisconnectId(user.Id) })
					}
					if disabled {
						if err := disabling.DisableCMD(f.chat.Id, "rules"); err != nil {
							t.Fatal(err)
						}
					}
					d := rulesDispatcher()
					update := newModuleMessageContext(f.bot, privateChatOf(user), user, text).Update
					if err := d.ProcessUpdate(f.bot, update, nil); err != nil {
						t.Fatal(err)
					}
					calls := f.client.callsFor("sendMessage")
					if disabled && !asAdmin {
						if len(calls) != 0 {
							t.Fatalf("disabled member got %s", sentText(f.client))
						}
						return
					}
					if len(calls) != 1 || !strings.Contains(fmt.Sprint(calls[0].Params["text"]), "Read these private rules") {
						t.Fatalf("PM rules = %s, want rules text rather than a deep-link button", sentText(f.client))
					}
					if markup, ok := calls[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup); ok && len(markup.InlineKeyboard) != 0 {
						t.Fatalf("PM rules unexpectedly linked to PM again: %+v", markup)
					}
				})
			}
		}
	}
}
