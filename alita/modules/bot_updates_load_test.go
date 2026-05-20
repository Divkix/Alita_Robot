package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestLoadBotUpdatesDeprecatedNoop(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadBotUpdates(dispatcher)
}

func TestBotUpdatesModuleMetadataAndLoad(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	module := botUpdatesModule{moduleStruct{moduleName: "BotUpdates"}}

	if module.Name() != "BotUpdates" {
		t.Fatalf("Name() = %q, want BotUpdates", module.Name())
	}
	if module.Priority() != -10 {
		t.Fatalf("Priority() = %d, want -10", module.Priority())
	}

	module.Load(dispatcher)
}
