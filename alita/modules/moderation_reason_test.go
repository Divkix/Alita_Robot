package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// TestModerationRepliesEscapeReason sends a reason that is both a printf verb
// and HTML markup; the confirmation must show it literally.
func TestModerationRepliesEscapeReason(t *testing.T) {
	useRealLocales(t)
	commands := []struct {
		name    string
		handler func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "mute", handler: mutesModule.mute},
		{name: "tmute", handler: mutesModule.tMute},
		{name: "ban", handler: bansModule.ban},
		{name: "tban", handler: bansModule.tBan},
		{name: "kick", handler: bansModule.kick},
	}
	for _, cmd := range commands {
		t.Run(cmd.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			args := "100% a<b"
			if strings.HasPrefix(cmd.name, "t") {
				args = "1h " + args
			}
			ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, "/"+cmd.name+" "+args)
			if err := cmd.handler(f.bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s() error = %v, want EndGroups", cmd.name, err)
			}

			text := fmt.Sprint(f.client.onlyCall(t, "sendMessage").Params["text"])
			if !strings.Contains(text, "100% a&lt;b") || strings.Contains(text, "%!") {
				t.Fatalf("%s reply = %q, want the reason shown literally and HTML-escaped", cmd.name, text)
			}
		})
	}
}
