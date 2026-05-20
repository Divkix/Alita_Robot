//go:build testtools

package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestCallbackHandlersIgnoreMissingCallbackQuery(t *testing.T) {
	t.Parallel()

	module := moduleStruct{}
	antiRaid := &antiRaidStruct{}

	type callbackHandler struct {
		name string
		want error
		call func(*gotgbot.Bot, *ext.Context) error
	}

	handlers := []callbackHandler{
		{name: "unapproveAllCallback", want: ext.EndGroups, call: module.unapproveAllCallback},
		{name: "captchaVerifyCallback", want: ext.EndGroups, call: module.captchaVerifyCallback},
		{name: "captchaRefreshCallback", want: ext.EndGroups, call: module.captchaRefreshCallback},
		{name: "joinRequestHandler", want: ext.EndGroups, call: module.joinRequestHandler},
		{name: "helpButtonHandler", want: ext.EndGroups, call: module.helpButtonHandler},
		{name: "botConfig", want: ext.EndGroups, call: module.botConfig},
		{name: "formattingHandler", want: ext.EndGroups, call: module.formattingHandler},
		{name: "backupCallbackHandler", want: ext.EndGroups, call: module.backupCallbackHandler},
		{name: "verifyAnonymousAdmin", want: ext.EndGroups, call: verifyAnonymousAdmin},
		{name: "noteOverWriteHandler", want: ext.EndGroups, call: module.noteOverWriteHandler},
		{name: "notesButtonHandler", want: ext.EndGroups, call: module.notesButtonHandler},
		{name: "filtersButtonHandler", want: ext.EndGroups, call: module.filtersButtonHandler},
		{name: "filterOverWriteHandler", want: ext.EndGroups, call: module.filterOverWriteHandler},
		{name: "rmWarnButton", want: ext.EndGroups, call: module.rmWarnButton},
		{name: "warnsButtonHandler", want: ext.EndGroups, call: module.warnsButtonHandler},
		{name: "unpinallCallback", want: ext.EndGroups, call: module.unpinallCallback},
		{name: "reactionsHelpHandler", want: ext.EndGroups, call: module.reactionsHelpHandler},
		{name: "langBtnHandler", want: ext.EndGroups, call: module.langBtnHandler},
		{name: "markResolvedButtonHandler", want: ext.EndGroups, call: module.markResolvedButtonHandler},
		{name: "connectionButtons", want: ext.EndGroups, call: module.connectionButtons},
		{name: "restrictButtonHandler", want: ext.EndGroups, call: module.restrictButtonHandler},
		{name: "unrestrictButtonHandler", want: ext.EndGroups, call: module.unrestrictButtonHandler},
		{name: "deleteButtonHandler", want: ext.EndGroups, call: module.deleteButtonHandler},
		{name: "buttonHandler", want: ext.EndGroups, call: module.buttonHandler},
		{name: "antiRaidCallbackHandler", want: ext.ContinueGroups, call: antiRaid.callbackHandler},
	}

	contexts := []struct {
		name string
		ctx  *ext.Context
	}{
		{name: "nil context", ctx: nil},
		{name: "nil update", ctx: &ext.Context{}},
		{name: "nil callback query", ctx: &ext.Context{Update: &gotgbot.Update{}}},
	}

	for _, handler := range handlers {
		handler := handler
		t.Run(handler.name, func(t *testing.T) {
			t.Parallel()

			for _, tc := range contexts {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					if got := handler.call(nil, tc.ctx); got != handler.want {
						t.Fatalf("%s returned %v, want %v", handler.name, got, handler.want)
					}
				})
			}
		})
	}
}
