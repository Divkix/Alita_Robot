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

// ListModules returns a formatted string containing all loaded bot modules.
// It retrieves the module names from the default help registry, sorts them alphabetically,
// and returns them as a comma-separated list wrapped in square brackets.
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

// InitialChecks performs essential initialization tasks before starting the bot.
// It ensures the bot exists in the database before modules load.
// Note: Cache is initialized in main.go before this function is called.
func InitialChecks(b *gotgbot.Bot) error {
	// Ensure bot exists in database (blocking - required for FK constraints)
	// This must complete before LoadModules to prevent race conditions with
	// foreign key constraints that reference the bot entry
	if err := user.EnsureBotInDb(b); err != nil {
		return fmt.Errorf("ensure bot in database: %w", err)
	}

	return nil
}

// LoadModules loads all bot modules in the correct order using the provided dispatcher.
// It initializes the help system, loads core functionality modules (admin, bans, filters, etc.),
// and ensures the help module is loaded last to register all available commands.
func LoadModules(dispatcher *ext.Dispatcher) {
	// Reset registry maps under lock (init-only, single-threaded startup)
	// ResetHelpRegistry atomically clears AbleMap/helpableKb/AltHelpOptions.
	modules.ResetHelpRegistry()

	// Load this at last because it loads all the modules
	defer modules.LoadHelp(dispatcher)

	// Registered modules are loaded by priority. Help is deferred above so it
	// can render metadata collected by module loaders.
	modules.LoadAllModules(dispatcher)
}
