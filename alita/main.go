package alita

import (
	"fmt"
	"slices"
	"strings"

	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/modules"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func ListModules() string {
	var modSlice []string
	for module, enabled := range modules.GetAbleMap() {
		if enabled {
			modSlice = append(modSlice, module)
		}
	}
	slices.Sort(modSlice)
	return fmt.Sprintf("[%s]", strings.Join(modSlice, ", "))
}

func InitialChecks(b *gotgbot.Bot) error {
	if err := user.EnsureBotInDb(b); err != nil {
		return fmt.Errorf("ensure bot in database: %w", err)
	}

	return nil
}

func LoadModules(dispatcher *ext.Dispatcher) {
	modules.ResetHelpRegistry()

	defer modules.LoadHelp(dispatcher)

	modules.LoadAllModules(dispatcher)
}
