//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

func TestPipelineProofKeepsActorAliasReplyAndTapRules(t *testing.T) {
	useRealLocales(t)
	for _, mode := range []string{"admin", "limited admin", "member", "other chat", "expired", "trusted anonymous", "caption"} {
		t.Run(mode, func(t *testing.T) {
			f := newAdminChatFixture(t)
			if mode == "limited admin" {
				limited := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"first_name":"Limited"}}`, f.admin.Id)
				f.client.membersByUser[fmt.Sprint(f.admin.Id)] = json.RawMessage(limited)
				f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage("[" + limited + "]")
			}
			if err := admin.SetAnonAdminMode(f.chat.Id, mode == "trusted anonymous"); err != nil {
				t.Fatal(err)
			}
			d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			LoadBotUpdates(d)
			helpers.WrapCommand(d, helpers.CommandDescriptor{Name: "proofcontract", Aliases: []string{"proofalias"}, RequiredChecks: []helpers.CheckFunc{helpers.RequireGroup(), helpers.CanUserRestrict()}}, func(c *helpers.CommandContext) error {
				text := c.Msg.Text
				if text == "" {
					text = c.Msg.Caption
				}
				_, err := c.Bot.SendMessage(c.Chat.Id, fmt.Sprintf("ran %s reply=%d actor=%d", text, c.Msg.ReplyToMessage.MessageId, c.User.Id), nil)
				return err
			})
			const text = "/PROOFALIAS@AlitaTestBot original arguments"
			if mode == "limited admin" {
				direct := newModuleMessageContext(f.bot, f.chat, f.admin, text).Update
				if err := d.ProcessUpdate(f.bot, direct, nil); err != nil {
					t.Fatal(err)
				}
				if got := soleText(t, sentTexts(f.client)); got != translated(t, "chat_status_restrict_cmd_error") {
					t.Fatalf("direct refusal = %q", got)
				}
			}
			before := len(f.client.callsFor("sendMessage"))
			anon := gotgbot.User{Id: 1087968824, IsBot: true, FirstName: "Anonymous", Username: "GroupAnonymousBot"}
			msg := newModuleMessageContext(f.bot, f.chat, anon, text).Message
			msg.SenderChat = &f.chat
			msg.ReplyToMessage = &gotgbot.Message{MessageId: 99, Chat: f.chat, From: &f.member, Text: "original target"}
			if mode == "caption" {
				msg.Text, msg.Caption = "", text
			}
			if err := d.ProcessUpdate(f.bot, &gotgbot.Update{Message: msg}, nil); err != nil {
				t.Fatal(err)
			}
			key := chat_status.AnonAdminCacheKey(f.chat.Id, msg.MessageId)
			t.Cleanup(func() { _ = cache.GetMarshal().Delete(cache.Context, key) })
			sends := f.client.callsFor("sendMessage")[before:]
			if len(sends) != 1 {
				t.Fatalf("anonymous sends = %v, want only the proof (or trusted execution)", sends)
			}
			if mode == "trusted anonymous" {
				if got := fmt.Sprint(sends[0].Params["text"]); !strings.Contains(got, "ran "+text+" reply=99 actor=1087968824") {
					t.Fatalf("trusted execution = %q", got)
				}
				return
			}
			markup, ok := sends[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
			if !ok || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
				t.Fatalf("proof markup = %#v", sends[0].Params["reply_markup"])
			}
			actor, chat := f.admin, f.chat
			if mode == "member" {
				actor = f.member
			}
			if mode == "other chat" {
				chat.Id = uniqueModuleChatID()
			}
			if mode == "expired" {
				if err := cache.GetMarshal().Delete(cache.Context, key); err != nil {
					t.Fatal(err)
				}
			}
			callback := newModuleCallbackContext(f.bot, chat, actor, markup.InlineKeyboard[0][0].CallbackData).Update
			if err := d.ProcessUpdate(f.bot, callback, nil); err != nil {
				t.Fatal(err)
			}
			after := f.client.callsFor("sendMessage")[before+1:]
			switch mode {
			case "member", "other chat":
				if len(after) != 0 || len(f.client.callsFor("deleteMessage")) != 0 {
					t.Fatalf("invalid tap ran command or deleted proof: replies=%v", after)
				}
				f.client.onlyCall(t, "answerCallbackQuery")
			case "expired":
				if len(after) != 0 || len(f.client.callsFor("deleteMessage")) != 0 {
					t.Fatal("expired proof ran or deleted the command")
				}
				assertParams(t, f.client.onlyCall(t, "editMessageText"), map[string]any{"text": translated(t, "bot_updates_button_expired")})
			default:
				assertDeleted(t, f.client, f.chat.Id, 102)
				if len(after) != 1 {
					t.Fatalf("replay replies = %v, want one", after)
				}
				want := fmt.Sprintf("ran %s reply=99 actor=%d", text, f.admin.Id)
				if mode == "limited admin" {
					want = translated(t, "chat_status_restrict_cmd_error")
				}
				if got := fmt.Sprint(after[0].Params["text"]); got != want {
					t.Fatalf("replay reply = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestPipelineAliasesUsePublicDisableCommands(t *testing.T) {
	useRealLocales(t)
	withDisableCommands(t)
	f := newAdminChatFixture(t)
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadDisabling(d)
	helpers.WrapCommand(d, helpers.CommandDescriptor{Name: "contractname", Aliases: []string{"contractalias"}, Disableable: true}, func(c *helpers.CommandContext) error {
		_, err := c.Bot.SendMessage(c.Chat.Id, "accepted command", nil)
		return err
	})
	send := func(user gotgbot.User, text string) []moduleBotCall {
		t.Helper()
		before := len(f.client.callsFor("sendMessage"))
		if err := d.ProcessUpdate(f.bot, newModuleMessageContext(f.bot, f.chat, user, text).Update, nil); err != nil {
			t.Fatal(err)
		}
		return f.client.callsFor("sendMessage")[before:]
	}
	list := send(f.member, "/disableable")
	if len(list) != 1 || !strings.Contains(fmt.Sprint(list[0].Params["text"]), "contractname") || !strings.Contains(fmt.Sprint(list[0].Params["text"]), "contractalias") {
		t.Fatalf("/disableable = %v, want both spellings", list)
	}
	send(f.admin, "/disabledel on")
	for _, row := range []string{"contractname", "contractalias"} {
		send(f.admin, "/disable "+row)
		if !disabling.IsCommandDisabled(f.chat.Id, row) {
			t.Fatalf("/disable refused registered spelling %s", row)
		}
		list = send(f.admin, "/disabled")
		if len(list) != 1 || !strings.Contains(fmt.Sprint(list[0].Params["text"]), row) {
			t.Fatalf("/disabled = %v, want stored spelling %s", list, row)
		}
		for _, name := range []string{"contractname", "contractalias"} {
			before := len(f.client.callsFor("deleteMessage"))
			if got := send(f.member, "/"+name); len(got) != 0 {
				t.Fatalf("disabled member got %v under /%s", got, name)
			}
			deleted := f.client.callsFor("deleteMessage")[before:]
			if len(deleted) != 1 {
				t.Fatalf("disabled member deletions=%v, want one", deleted)
			}
			assertParams(t, deleted[0], map[string]any{"chat_id": f.chat.Id, "message_id": 101})
			if got := send(f.admin, "/"+name); len(got) != 1 || fmt.Sprint(got[0].Params["text"]) != "accepted command" {
				t.Fatalf("disabled admin got %v under /%s", got, name)
			}
		}
		send(f.admin, "/enable "+row)
		if disabling.IsCommandDisabled(f.chat.Id, row) {
			t.Fatalf("/enable did not remove stored spelling %s", row)
		}
	}
}
