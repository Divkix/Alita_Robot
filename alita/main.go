package alita

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/modules"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

// ResourceMonitor monitors system resources including memory usage and goroutine count.
// It runs every 5 minutes, logging resource statistics and issuing warnings
// when thresholds are exceeded (>1000 goroutines or >500MB memory usage).
func ResourceMonitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		numGoroutines := runtime.NumGoroutine()

		// Log metrics
		log.WithFields(log.Fields{
			"goroutines": numGoroutines,
			"memory_mb":  m.Alloc / 1024 / 1024,
			"sys_mb":     m.Sys / 1024 / 1024,
			"gc_runs":    m.NumGC,
		}).Info("Resource usage stats")

		// Warning thresholds
		if numGoroutines > 1000 {
			log.WithField("goroutines", numGoroutines).Warn("High goroutine count detected")
		}

		if m.Alloc/1024/1024 > 500 { // 500MB
			log.WithField("memory_mb", m.Alloc/1024/1024).Warn("High memory usage detected")
		}
	}
}

// ListModules returns a formatted string containing all loaded bot modules.
// It retrieves the module names from the HelpModule.AbleMap, sorts them alphabetically,
// and returns them as a comma-separated list wrapped in square brackets.
func ListModules() string {
	modSlice := modules.HelpModule.AbleMap.LoadModules()
	slices.Sort(modSlice)
	return fmt.Sprintf("[%s]", strings.Join(modSlice, ", "))
}

// InitialChecks performs essential initialization tasks before starting the bot.
// It ensures the bot exists in the database, validates command aliases for duplicates,
// initializes the cache system, and starts resource monitoring.
// Returns an error if cache initialization fails.
func InitialChecks(b *gotgbot.Bot) error {
	// Create bot in db if not already created
	go func() {
		if err := db.EnsureBotInDb(b); err != nil {
			log.Errorf("[InitialChecks] Failed to ensure bot in database: %v", err)
		}
	}()
	checkDuplicateAliases()

	// Initialize cache with proper error handling
	if err := cache.InitCache(); err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	// Start resource monitoring
	go ResourceMonitor()
	return nil
}

// checkDuplicateAliases validates that no command aliases are duplicated across modules.
// It collects all alternative help options from loaded modules and checks for duplicates.
// The function terminates the program with a fatal error if duplicates are found.
func checkDuplicateAliases() {
	var althelp []string

	for _, i := range modules.HelpModule.AltHelpOptions {
		althelp = append(althelp, i...)
	}

	duplicateAlias, val := string_handling.IsDuplicateInStringSlice(althelp)
	if val {
		log.Fatalf("Found duplicate alias: %s", duplicateAlias)
	}
}

// LoadModules loads all bot modules in the correct order using the provided dispatcher.
// It initializes the help system, loads core functionality modules (admin, bans, filters, etc.),
// and ensures the help module is loaded last to register all available commands.
func LoadModules(dispatcher *ext.Dispatcher) {
	// Initialize Inner Map
	modules.HelpModule.AbleMap.Init()

	// Load this at last because it loads all the modules
	defer modules.LoadHelp(dispatcher)

	modules.LoadBotUpdates(dispatcher)
	modules.LoadAntispam(dispatcher)
	modules.LoadLanguage(dispatcher)
	modules.LoadAdmin(dispatcher)
	modules.LoadPin(dispatcher)
	modules.LoadMisc(dispatcher)
	modules.LoadBans(dispatcher)
	modules.LoadMutes(dispatcher)
	modules.LoadPurges(dispatcher)
	modules.LoadUsers(dispatcher)
	modules.LoadReports(dispatcher)
	modules.LoadDev(dispatcher)
	modules.LoadLocks(dispatcher)
	modules.LoadFilters(dispatcher)
	modules.LoadAntiflood(dispatcher)
	modules.LoadNotes(dispatcher)
	modules.LoadConnections(dispatcher)
	modules.LoadDisabling(dispatcher)
	modules.LoadRules(dispatcher)
	modules.LoadWarns(dispatcher)
	modules.LoadGreetings(dispatcher)
	modules.LoadCaptcha(dispatcher)
	modules.LoadBlacklists(dispatcher)
	modules.LoadMkdCmd(dispatcher)
}
