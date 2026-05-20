//go:build testtools

package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestCallbackQueryFromContext(t *testing.T) {
	t.Parallel()

	query := &gotgbot.CallbackQuery{Id: "callback-id"}

	tests := []struct {
		name string
		ctx  *ext.Context
		want *gotgbot.CallbackQuery
		ok   bool
	}{
		{name: "nil context", ctx: nil, ok: false},
		{name: "nil update", ctx: &ext.Context{}, ok: false},
		{name: "nil callback query", ctx: &ext.Context{Update: &gotgbot.Update{}}, ok: false},
		{
			name: "callback query present",
			ctx:  &ext.Context{Update: &gotgbot.Update{CallbackQuery: query}},
			want: query,
			ok:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := callbackQueryFromContext(tc.ctx)
			if ok != tc.ok {
				t.Fatalf("callbackQueryFromContext() ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("callbackQueryFromContext() query = %p, want %p", got, tc.want)
			}
		})
	}
}
