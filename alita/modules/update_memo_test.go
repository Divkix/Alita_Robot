//go:build testtools

package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/updatememo"
)

func TestIsUserAdminForUpdateReusesLookupWithinUpdate(t *testing.T) {
	cases := []struct {
		name      string
		withMemo  bool
		wantCalls int
	}{
		{name: "memo collapses repeated lookups", withMemo: true, wantCalls: 1},
		{name: "no memo looks up every time", withMemo: false, wantCalls: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// No cache marshal, so every uncached IsUserAdmin reaches Telegram.
			withNilCacheMarshal(t)

			client := aispamAdminClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Memo Chat"}
			user := gotgbot.User{Id: 42, FirstName: "Admin"}
			ctx := newModuleMessageContext(bot, chat, user, "hello")
			if tc.withMemo {
				ctx.Data = map[string]any{updatememo.DataKey: updatememo.New()}
			}

			for i := range 2 {
				if !chat_status.IsUserAdminForUpdate(bot, ctx, chat.Id, user.Id) {
					t.Fatalf("IsUserAdminForUpdate() call %d = false, want true", i+1)
				}
			}

			if calls := client.callsFor("getChatAdministrators"); len(calls) != tc.wantCalls {
				t.Fatalf("getChatAdministrators calls = %d, want %d", len(calls), tc.wantCalls)
			}
		})
	}
}
