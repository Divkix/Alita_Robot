package cmdDecorator

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func noopHandler(_ *gotgbot.Bot, _ *ext.Context) error {
	return nil
}

func TestMultiCommandRegistersAll(t *testing.T) {
	t.Parallel()

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{})
	aliases := []string{"start", "help", "about"}

	MultiCommand(dispatcher, aliases, noopHandler)
}

func TestMultiCommandSingleAlias(t *testing.T) {
	t.Parallel()

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{})

	MultiCommand(dispatcher, []string{"ping"}, noopHandler)
}

func TestMultiCommandEmptySlice(t *testing.T) {
	t.Parallel()

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{})

	MultiCommand(dispatcher, []string{}, noopHandler)
}
