//go:build testtools

package modules

import (
	"testing"

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
