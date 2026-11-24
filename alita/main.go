package alita

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/modules"
	"github.com/divkix/Alita_Robot/alita/utils/metrics"
)

func ListModules() string {
	return fmt.Sprintf("[%s]", strings.Join(modules.EnabledModules(), ", "))
}

func InitialChecks(b *gotgbot.Bot) error {
	if err := user.EnsureBotInDb(b); err != nil {
		return fmt.Errorf("ensure bot in database: %w", err)
	}

	return nil
}

func LoadModules(dispatcher *ext.Dispatcher) {
	modules.ResetHelpRegistry()

	// Help registers with PriorityFinal, so it loads last inside
	// LoadAllModules like every other module.
	modules.LoadAllModules(dispatcher)
	// CommandModules derives from the manifest LoadAllModules just filled.
	metrics.SetCommandModules(modules.CommandModules())
}
