package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestLoadBotUpdatesDeprecatedNoop(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadBotUpdates(dispatcher)
}
