//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

func TestConnectedPipelineCheckMatrix(t *testing.T) {
	useRealLocales(t)
	for _, check := range []struct {
		name  string
		admin bool
		build func(bool) helpers.CheckFunc
	}{
		{"checkadmin", true, helpers.RequireConnectedAdmin},
		{"checkmember", false, helpers.RequireConnectedMember},
	} {
		for _, role := range []string{"member", "admin", "owner"} {
			for _, source := range []string{"group", "connected PM", "named PM"} {
				for _, replay := range []bool{false, true} {
					for _, requireBot := range []bool{false, true} {
						for _, botAdmin := range []bool{false, true} {
							t.Run(fmt.Sprintf("%s/%s/%s/replay=%t/requireBot=%t/botAdmin=%t", check.name, role, source, replay, requireBot, botAdmin), func(t *testing.T) {
								f := newAdminChatFixture(t)
								actor := f.admin
								switch role {
								case "member":
									actor = f.member
								case "owner":
									f.makeOwner()
								}
								if !botAdmin {
									f.client.membersByUser["999"] = json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Bot"}}`)
									f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + string(f.client.membersByUser[fmt.Sprint(f.admin.Id)]) + "]")
								}
								origin := f.chat
								text := "/" + check.name
								if source != "group" {
									origin = privateChatOf(actor)
									target := f.chat.Id
									if source == "named PM" {
										target = uniqueModuleChatID()
										text += fmt.Sprintf(" %d", f.chat.Id)
										if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
											t.Fatal(err)
										}
									}
									if err := connections.ConnectId(actor.Id, target); err != nil {
										t.Fatal(err)
									}
									t.Cleanup(func() { _ = connections.DisconnectId(actor.Id) })
								}
								d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
								helpers.WrapCommand(d, helpers.CommandDescriptor{Name: check.name, PMGroupArg: helpers.PMGroupArgNone, RequiredChecks: []helpers.CheckFunc{check.build(requireBot)}}, func(c *helpers.CommandContext) error {
									_, err := c.Msg.Reply(c.Bot, fmt.Sprintf("target=%d/effective=%d/origin=%d/actor=%d/text=%s", c.Chat.Id, c.Ctx.EffectiveChat.Id, c.Msg.Chat.Id, c.User.Id, c.Msg.Text), nil)
									return err
								})
								command := newModuleMessageContext(f.bot, origin, actor, text).Update
								update := command
								if replay {
									// Mirror the proof callback's restored message and effective tapper:
									// rights must not use the cached anonymous message's From.
									command.Message.From = &gotgbot.User{Id: chat_status.GroupAnonymousBotID, IsBot: true}
									d.AddHandler(handlers.NewCallback(callbackquery.Equal("connected-replay"), func(b *gotgbot.Bot, ctx *ext.Context) error {
										ctx.Update = command
										ctx.EffectiveMessage = command.Message
										return helpers.RunCommand(b, ctx, check.name)
									}))
									update = newModuleCallbackContext(f.bot, origin, actor, "connected-replay").Update
								}
								if err := d.ProcessUpdate(f.bot, update, nil); err != nil {
									t.Fatal(err)
								}
								want := fmt.Sprintf("target=%d/effective=%d/origin=%d/actor=%d/text=/%s", f.chat.Id, f.chat.Id, origin.Id, actor.Id, check.name)
								if requireBot && !botAdmin {
									want = "I'm not an admin in this chat!"
								} else if check.admin && role == "member" {
									want = "You need to be an admin to do this."
								}
								if got := sentTexts(f.client); len(got) != 1 || got[0] != want {
									t.Fatalf("replies = %q, want only %q", got, want)
								}
								assertParams(t, f.client.onlyCall(t, "sendMessage"), map[string]any{"chat_id": origin.Id})
								if source == "named PM" && connections.Connection(actor.Id).ChatId == f.chat.Id {
									t.Fatal("one-off target replaced the saved connection")
								}
							})
						}
					}
				}
			}
		}
	}
}
