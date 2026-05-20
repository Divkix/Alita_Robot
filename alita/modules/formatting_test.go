//go:build testtools

package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

func TestGenFormattingKbCallbackData(t *testing.T) {
	t.Parallel()

	keyboard := formattingModule.genFormattingKb("")

	require.Len(t, keyboard, 2)
	require.Len(t, keyboard[0], 2)
	require.Len(t, keyboard[1], 1)

	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "markdown formatting", data: keyboard[0][0].CallbackData, want: "md_formatting"},
		{name: "fillings", data: keyboard[0][1].CallbackData, want: "fillings"},
		{name: "random content", data: keyboard[1][0].CallbackData, want: "random"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decoded, ok := decodeCallbackData(tc.data, "formatting")
			require.True(t, ok)
			module, ok := decoded.Field("m")
			require.True(t, ok)
			assert.Equal(t, tc.want, module)
		})
	}
}

func TestMarkdownHelpRepliesInPrivateAndGroup(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42, FirstName: "Formatter"}

	privateChat := gotgbot.Chat{Id: 42, Type: "private", FirstName: "Formatter"}
	privateCtx := newModuleMessageContext(bot, privateChat, user, "/markdownhelp")
	if err := formattingModule.markdownHelp(bot, privateCtx); err != ext.EndGroups {
		t.Fatalf("markdownHelp private error = %v, want EndGroups", err)
	}

	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Format Chat"}
	groupCtx := newModuleMessageContext(bot, groupChat, user, "/markdownhelp")
	if err := formattingModule.markdownHelp(bot, groupCtx); err != ext.EndGroups {
		t.Fatalf("markdownHelp group error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want private and group help replies", len(calls))
	}
	for i, call := range calls {
		if call.Params["reply_markup"] == nil {
			t.Fatalf("sendMessage call %d missing reply_markup", i)
		}
	}
}

func TestFormattingHandlerEditsMessageAndAnswersCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Format Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Formatter"}
	data := encodeCallbackData(
		"formatting",
		map[string]string{"m": "md_formatting"},
		"formatting.md_formatting",
	)

	ctx := newModuleCallbackContext(bot, chat, user, data)
	if err := formattingModule.formattingHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("formattingHandler() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}

func TestGetMarkdownHelp(t *testing.T) {
	t.Parallel()

	tr, err := i18n.NewTestTranslator(`
formatting_markdown: "Markdown help"
formatting_fillings: "Fillings help"
formatting_random: "Random help"
`)
	require.NoError(t, err)

	tests := []struct {
		module string
		want   string
	}{
		{module: "md_formatting", want: "Markdown help"},
		{module: "fillings", want: "Fillings help"},
		{module: "random", want: "Random help"},
		{module: "unknown", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.module, func(t *testing.T) {
			assert.Equal(t, tc.want, formattingModule.getMarkdownHelp(tr, tc.module))
		})
	}
}

func TestFormattingHandlerNilCallbackQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  *ext.Context
	}{
		{name: "nil context", ctx: nil},
		{name: "nil update", ctx: &ext.Context{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := formattingModule.formattingHandler(nil, tc.ctx)
			assert.Equal(t, ext.EndGroups, err)
		})
	}
}
