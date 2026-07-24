package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestLoadBotUpdates(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadBotUpdates(dispatcher)
}
