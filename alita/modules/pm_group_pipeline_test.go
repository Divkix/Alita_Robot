//go:build testtools

package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

func warnLimit(chatID int64) any { return warns.GetWarnSetting(chatID).WarnLimit }

// A WrapCommand command that takes its group from PM resolves the group
// before its RequiredChecks, so they check the named group as they would a
// connected one, and the command then changes that group.
func TestWrapCommandResolvesPMGroupBeforeChecks(t *testing.T) {
	f, connected := pmSettingsFixture(t)
	var checked int64
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:       "setwarnlimit",
		PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: []helpers.CheckFunc{func(c *helpers.CommandContext) bool {
			if chat := chat_status.IsUserConnected(c.Bot, c.Ctx, true, true); chat != nil {
				checked = chat.Id
			}
			return checked == f.chat.Id
		}},
	}, pipelineHandler(warnsModule.setWarnLimit))
	before := warnLimit(connected)

	processPM(t, dispatcher, f, f.admin.Id, "/setwarnlimit @fixturegroup 5")

	if checked != f.chat.Id {
		t.Fatalf("RequiredChecks saw group %d, want the named group %d", checked, f.chat.Id)
	}
	if got := warnLimit(f.chat.Id); got != 5 {
		t.Fatalf("named group warn limit = %v, want 5; sent:\n%s", got, sentText(f.client))
	}
	if got := warnLimit(connected); got != before {
		t.Fatalf("connected group warn limit = %v, want it unchanged (%v)", got, before)
	}
	if conn := connections.Connection(f.admin.Id); !conn.Connected || conn.ChatId != connected {
		t.Fatalf("connection = %+v, want it kept on %d", conn, connected)
	}
}

// Once a command has run on a group named from PM, later handler groups do
// not see the message with the group cut out, even when the command leaves
// ctx.EffectiveChat alone and lets groups continue: through WrapCommand and
// through withPMGroupArg alike.
func TestWrapCommandWithPMGroupEndsGroups(t *testing.T) {
	for name, register := range map[string]func(d *ext.Dispatcher, ran *[]string){
		"WrapCommand": func(d *ext.Dispatcher, ran *[]string) {
			helpers.WrapCommand(d, helpers.CommandDescriptor{
				Name:       "pmgrouptest",
				PMGroupArg: helpers.PMGroupArgOwn,
			}, func(c *helpers.CommandContext) error {
				*ran = c.Ctx.Args()
				return nil
			})
		},
		"withPMGroupArg": func(d *ext.Dispatcher, ran *[]string) {
			d.AddHandler(handlers.NewCommand("pmgrouptest", withPMGroupArg("pmgrouptest", helpers.PMGroupArgOwn,
				func(_ *gotgbot.Bot, ctx *ext.Context) error {
					*ran = ctx.Args()
					return nil
				})))
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := pmSettingsFixture(t)
			dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			var ran []string
			register(dispatcher, &ran)
			var later []string
			dispatcher.AddHandlerToGroup(handlers.NewMessage(nil, func(_ *gotgbot.Bot, ctx *ext.Context) error {
				later = append(later, ctx.EffectiveMessage.GetText())
				return ext.ContinueGroups
			}), 1)

			processPM(t, dispatcher, f, f.admin.Id, "/pmgrouptest @fixturegroup 5")

			if strings.Join(ran, " ") != "/pmgrouptest 5" {
				t.Fatalf("command ran with args %q, want [/pmgrouptest 5]; sent:\n%s", ran, sentText(f.client))
			}
			if len(later) != 0 {
				t.Fatalf("a later handler group saw %q, want iteration ended", later)
			}
		})
	}
}
