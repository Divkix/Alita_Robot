//go:build testtools

package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m, &models.DisableSettings{}, &models.DisableChatSettings{}, &models.AdminSettings{}))
}

type pipelineContractClient struct {
	sentTextsClient
	deleted int
}

func (c *pipelineContractClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "deleteMessage" {
		c.deleted++
	}
	return c.sentTextsClient.RequestWithContext(ctx, token, method, params, opts)
}

func TestPipelineRoleDisableReplayMatrix(t *testing.T) {
	SetDisableCmdsForTest(t, nil)
	var nextChat int64 = -1180000
	for _, role := range []struct {
		name string
		id   int64
	}{
		{"member", 42}, {"admin", 10}, {"owner", 12},
	} {
		for _, disableable := range []bool{false, true} {
			for _, disabled := range []bool{false, true} {
				for _, replay := range []bool{false, true} {
					for _, row := range []string{"canonical", "alternate"} {
						for _, name := range []string{"canonical", "alternate"} {
							t.Run(fmt.Sprintf("%s/disableable=%t/disabled=%t/replay=%t/row=%s/name=%s", role.name, disableable, disabled, replay, row, name), func(t *testing.T) {
								nextChat--
								chat := gotgbot.Chat{Id: nextChat, Type: "supergroup"}
								client := &pipelineContractClient{}
								bot := newCpBot(999)
								bot.BotClient = client
								if disabled {
									if err := disabling.DisableCMD(chat.Id, row); err != nil {
										t.Fatal(err)
									}
								}
								if err := disabling.ToggleDel(chat.Id, true); err != nil {
									t.Fatal(err)
								}
								d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
								WrapCommand(d, CommandDescriptor{Name: "canonical", Aliases: []string{"alternate"}, Disableable: disableable, RequiredChecks: []CheckFunc{RequireGroup()}}, func(c *CommandContext) error {
									_, err := c.Bot.SendMessage(c.Chat.Id, fmt.Sprintf("%s|%d|%d", c.Msg.Text, c.User.Id, c.Msg.ReplyToMessage.MessageId), nil)
									return err
								})
								actor := gotgbot.User{Id: role.id}
								msg := &gotgbot.Message{MessageId: 101, Chat: chat, From: &actor, Text: "/" + name + " original arguments", ReplyToMessage: &gotgbot.Message{MessageId: 100, Chat: chat}}
								update := &gotgbot.Update{Message: msg}
								if replay {
									// The callback boundary restores the cached message without
									// replacing its sender with the effective confirming actor.
									msg.From = &gotgbot.User{Id: 1087968824, IsBot: true}
									d.AddHandler(handlers.NewCallback(callbackquery.Equal("replay"), func(b *gotgbot.Bot, ctx *ext.Context) error {
										ctx.Update = &gotgbot.Update{Message: msg}
										ctx.EffectiveMessage = msg
										return ReplayCommand(b, ctx, name)
									}))
									update = &gotgbot.Update{CallbackQuery: &gotgbot.CallbackQuery{Id: "confirm", From: actor, Data: "replay", Message: gotgbot.Message{MessageId: 102, Chat: chat}}}
								}
								if err := d.ProcessUpdate(bot, update, nil); err != nil {
									t.Fatal(err)
								}
								if disableable && disabled && role.name == "member" {
									if len(client.texts) != 0 || client.deleted != 1 {
										t.Fatalf("disabled member: replies=%v deletions=%d, want silence and one deletion", client.texts, client.deleted)
									}
								} else if want := fmt.Sprintf("/%s original arguments|%d|100", name, role.id); len(client.texts) != 1 || client.texts[0] != want || client.deleted != 0 {
									t.Fatalf("replies=%v deletions=%d, want %q without deletion", client.texts, client.deleted, want)
								}
							})
						}
					}
				}
			}
		}
	}
}

func TestPipelineDisablePrecedesRights(t *testing.T) {
	SetDisableCmdsForTest(t, nil)
	bot := newCpBot(999)
	client := &sentTextsClient{}
	bot.BotClient = client
	chat := gotgbot.Chat{Id: -1180999, Type: "supergroup"}
	if err := disabling.DisableCMD(chat.Id, "gated"); err != nil {
		t.Fatal(err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	WrapCommand(d, CommandDescriptor{Name: "gated", Disableable: true, RequiredChecks: []CheckFunc{RequireUserAdmin()}}, func(c *CommandContext) error {
		_, err := c.Bot.SendMessage(c.Chat.Id, "accepted", nil)
		return err
	})
	if err := d.ProcessUpdate(bot, &gotgbot.Update{Message: &gotgbot.Message{MessageId: 1, Chat: chat, From: &gotgbot.User{Id: 42}, Text: "/gated"}}, nil); err != nil {
		t.Fatal(err)
	}
	if len(client.texts) != 0 {
		t.Fatalf("disabled member got %v, want silence before the admin refusal", client.texts)
	}
}

func TestPipelineSenderlessPublicAndCheckedCommands(t *testing.T) {
	SetDisableCmdsForTest(t, nil)
	restore, err := i18n.OverrideManagerForTest("common_cannot_identify_user: Cannot identify sender\n")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	for i, tc := range []struct {
		name        string
		checked     bool
		disableable bool
		disabled    bool
		want        string
		wantDeleted int
	}{
		{name: "public", want: "public reply"},
		{name: "public disabled", disableable: true, disabled: true, wantDeleted: 1},
		{name: "public enabled", disableable: true, want: "public reply"},
		{name: "public refused row", disabled: true, want: "public reply"},
		{name: "checked", checked: true, want: "Cannot identify sender"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chat := gotgbot.Chat{Id: -1189000 - int64(i), Type: "supergroup"}
			client := &pipelineContractClient{}
			bot := newCpBot(999)
			bot.BotClient = client
			if tc.disabled {
				if err := disabling.DisableCMD(chat.Id, "senderless"); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = disabling.EnableCMD(chat.Id, "senderless") })
			}
			if err := disabling.ToggleDel(chat.Id, true); err != nil {
				t.Fatal(err)
			}
			desc := CommandDescriptor{Name: "senderless", Disableable: tc.disableable}
			if tc.checked {
				desc.RequiredChecks = []CheckFunc{RequireUserAdmin()}
			}
			d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			WrapCommand(d, desc, func(c *CommandContext) error {
				_, err := c.Bot.SendMessage(c.Chat.Id, "public reply", nil)
				return err
			})
			update := &gotgbot.Update{Message: &gotgbot.Message{
				MessageId: 1, Chat: chat, Text: "/senderless",
				SenderChat: &gotgbot.Chat{Id: -1001189, Type: "channel"},
			}}
			if err := d.ProcessUpdate(bot, update, nil); err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(client.texts) != 0 {
					t.Fatalf("replies = %v, want silence", client.texts)
				}
			} else if len(client.texts) != 1 || client.texts[0] != tc.want {
				t.Fatalf("replies = %v, want %q", client.texts, tc.want)
			}
			if client.deleted != tc.wantDeleted {
				t.Fatalf("deleted = %d, want %d", client.deleted, tc.wantDeleted)
			}
		})
	}
}
