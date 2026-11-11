//go:build testtools

package modules

import (
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
)

// groupHelp sends update through the help module with the panel button on.
func groupHelp(t *testing.T, f adminChatFixture, update *gotgbot.Update) {
	t.Helper()
	previous := *config.AppConfig
	t.Cleanup(func() { *config.AppConfig = previous })
	config.AppConfig.PanelEnabled = true
	config.AppConfig.PanelMiniAppShortName = "panel"
	dispatcher := helpDispatcherForEphemeral()
	aispamProcessRawUpdate(t, dispatcher, f.bot, update)
}

func helpDispatcherForEphemeral() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadHelp(dispatcher)
	return dispatcher
}

// assertHelpRedirect fails unless call carries the group /help text and its
// PM and panel buttons, shown to receiver (0: publicly).
func assertHelpRedirect(t *testing.T, call moduleBotCall, receiver int64) {
	t.Helper()
	if got := fmt.Sprint(call.Params["text"]); got != translated(t, "help_contact_pm") {
		t.Fatalf("text = %q, want the PM redirect", got)
	}
	kb, _ := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if len(kb.InlineKeyboard) != 2 || kb.InlineKeyboard[0][0].Url == "" || kb.InlineKeyboard[1][0].Url == "" {
		t.Fatalf("reply_markup = %+v, want the PM and panel URL buttons", call.Params["reply_markup"])
	}
	if got := ephemeralReceiverOf(call); got != receiver {
		t.Fatalf("receiver = %d, want %d", got, receiver)
	}
}

func TestGroupHelpWithEphemeralOffRedirectsPublicly(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	cleanupEphemeral(t, f.chat.Id)
	groupHelp(t, f, aispamMessageUpdate(f.chat, f.member, 1401, "/help"))
	assertHelpRedirect(t, f.client.onlyCall(t, "sendMessage"), 0)
}

func TestGroupHelpWithEphemeralOnShowsTheRedirectOnlyToTheAsker(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	enableEphemeral(t, f.chat.Id)
	groupHelp(t, f, aispamMessageUpdate(f.chat, f.member, 1402, "/help"))
	call := f.client.onlyCall(t, "sendMessage")
	assertHelpRedirect(t, call, f.member.Id)
	if _, ok := call.Params["reply_parameters"]; ok {
		t.Fatal("an ephemeral send carried reply_parameters")
	}
}

func TestGroupHelpFallsBackToThePublicRedirectWhenTheEphemeralSendFails(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	enableEphemeral(t, f.chat.Id)
	f.bot.BotClient = ephemeralRefusingClient{f.client}
	groupHelp(t, f, aispamMessageUpdate(f.chat, f.member, 1403, "/help"))

	calls := f.client.callsFor("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want the refused ephemeral send then the public answer", len(calls))
	}
	assertHelpRedirect(t, calls[0], f.member.Id)
	assertHelpRedirect(t, calls[1], 0)
}

func TestGroupHelpSentAsAReplyStaysPublic(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	enableEphemeral(t, f.chat.Id)
	update := aispamMessageUpdate(f.chat, f.admin, 1404, "/help")
	update.Message.ReplyToMessage = &gotgbot.Message{MessageId: 50, Date: 1, Chat: f.chat, From: &f.member, Text: "how?"}
	groupHelp(t, f, update)
	assertHelpRedirect(t, f.client.onlyCall(t, "sendMessage"), 0)
}
