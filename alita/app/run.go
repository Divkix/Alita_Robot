// Package app is the startup composition root: Run assembles the bot's
// services — flag-gated monitoring, panel and pprof wiring, scheduler and
// drain registration through the shutdown phase API, the HTTP server, and the
// webhook-vs-polling fork. main() only dispatches the CLI modes and calls Run.
package app

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
	// Embeds the IANA time zone database so chat timezones load on hosts
	// without /usr/share/zoneinfo (the alpine debug image, nixpacks builds).
	_ "time/tzdata"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita"
	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	dbmonitoring "github.com/divkix/Alita_Robot/alita/db/monitoring"
	"github.com/divkix/Alita_Robot/alita/digest"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/membersnapshot"
	"github.com/divkix/Alita_Robot/alita/modules"
	"github.com/divkix/Alita_Robot/alita/panel"
	"github.com/divkix/Alita_Robot/alita/recurring"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/constants"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/httpserver"
	"github.com/divkix/Alita_Robot/alita/utils/jobs"
	"github.com/divkix/Alita_Robot/alita/utils/monitoring"
	"github.com/divkix/Alita_Robot/alita/utils/shutdown"
	"github.com/divkix/Alita_Robot/alita/utils/slogrus"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// Locales is the embedded locale filesystem main() provides before calling
// Run; Run's locale-manager init loads it.
var Locales embed.FS

// DrainSpec is one shutdown drain with the phase it belongs to; the manager
// orders execution by phase, so the spec carries the ordering as data.
type DrainSpec struct {
	Name    string
	Phase   shutdown.Phase
	Handler func() error
}

// ShutdownDrainSpecs declares the shared drain tail in the phases the
// shutdown contract fixes: database close, stats flush, jobs stop, then the
// module drains in the module-drains phase.
func ShutdownDrainSpecs(closeDB, flushStats, stopJobs func() error, moduleDrains []DrainSpec) []DrainSpec {
	specs := []DrainSpec{
		{Name: "db close", Phase: shutdown.PhaseDatabase, Handler: closeDB},
		{Name: "stats flush", Phase: shutdown.PhaseFlush, Handler: flushStats},
		{Name: "jobs stop", Phase: shutdown.PhaseJobs, Handler: stopJobs},
	}
	for _, drain := range moduleDrains {
		drain.Phase = shutdown.PhaseModuleDrains
		specs = append(specs, drain)
	}
	return specs
}

// RegisterShutdownSteps registers each spec on the manager under its phase.
func RegisterShutdownSteps(m *shutdown.Manager, specs ...DrainSpec) {
	for _, spec := range specs {
		m.Register(spec.Name, spec.Phase, spec.Handler)
	}
}

func closeDBConnections() error {
	err := db.Close()
	if err != nil {
		log.Errorf("[Shutdown] Failed to close database connections: %v", err)
		return fmt.Errorf("failed to close database: %w", err)
	}
	log.Info("[Shutdown] Database connections closed successfully")
	return nil
}

// pollingTimeout is how long Telegram holds an idle getUpdates open. It stays
// at least 2s below the Bot API transport's response-header timeout, so an
// idle poll ends as an empty answer, not a transport timeout, and 2s below the
// shutdown manager's per-handler limit. The updater stop cancels a held poll,
// so shutdown never waits it out.
const pollingTimeout = 8 * time.Second

// Run composes and starts the bot. mode is the update-source name
// ("webhook" or "polling") postInit records as WorkingMode. It blocks on
// shutdown once the update source is running.
func Run(mode string) {
	appStartTime := time.Now()

	// gotgbot's dispatcher and updater log panics and polling errors through
	// slog; send them, and the standard log package, through logrus.
	slog.SetDefault(slog.New(slogrus.Handler{}))

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Main] Panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
			log.StandardLogger().Exit(1) // flushes telemetry first
		}
	}()

	if config.AppConfig.Debug {
		log.Info("Running in DEBUG Mode...")
	} else {
		log.Info("Running in RELEASE Mode...")
	}

	if err := cache.InitCache(); err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	log.Info("Cache system initialized successfully")

	localeManager := i18n.GetManager()
	if err := localeManager.Initialize(&Locales, "locales", i18n.DefaultManagerConfig()); err != nil {
		log.Fatalf("Failed to initialize locale manager: %v", err)
	}
	log.Infof("Locale manager initialized with %d languages: %v", len(localeManager.GetAvailableLanguages()), localeManager.GetAvailableLanguages())

	maxIdleConns := config.AppConfig.HTTPMaxIdleConns
	maxIdleConnsPerHost := config.AppConfig.HTTPMaxIdleConnsPerHost

	transport := newBotAPITransport(maxIdleConns, maxIdleConnsPerHost)

	log.Infof("[Main] HTTP transport configured with MaxIdleConns: %d, MaxIdleConnsPerHost: %d", maxIdleConns, maxIdleConnsPerHost)

	log.Info("[Main] Initializing bot with optimized HTTP client (connection pooling enabled)")
	b, err := gotgbot.NewBot(config.AppConfig.BotToken, &gotgbot.BotOpts{
		BotClient: &helpers.ReplySafeBotClient{
			BotClient: &gotgbot.BaseBotClient{
				Client: http.Client{
					Transport: tracing.BotAPITransport(transport),
					Timeout:   constants.LongTimeout,
				},
				UseTestEnvironment: false,
				DefaultRequestOpts: &gotgbot.RequestOpts{
					Timeout: time.Duration(constants.LongTimeout),
					APIURL:  resolveBotAPIURL(config.AppConfig.ApiServer),
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to create new bot: %v", err)
	}
	log.Infof("[Main] Bot initialized with optimized connection pooling (MaxIdleConns: %d, MaxIdleConnsPerHost: %d, HTTP/2 enabled)", maxIdleConns, maxIdleConnsPerHost)

	botUsername := resolveBotUsername(b)

	if err := alita.InitialChecks(b); err != nil {
		log.Fatalf("Initial checks failed: %v", err)
	}

	dispatcher := newConfiguredDispatcher(config.AppConfig.DispatcherMaxRoutines)

	if config.AppConfig.EnableDBMonitoring {
		dbmonitoring.Register(time.Minute)
	}

	var statsCollector *monitoring.BackgroundStatsCollector
	if config.AppConfig.EnableBackgroundStats {
		statsCollector = monitoring.NewBackgroundStatsCollector()
		monitoring.SetGlobalCollector(statsCollector)
		error_handling.SetOnErrorCallback(monitoring.GlobalRecordError)
		tracing.SetOnProcessUpdateCallback(monitoring.GlobalRecordMessage)
		statsCollector.Register()
	}

	if config.AppConfig.EnablePerformanceMonitoring {
		monitoring.NewAutoRemediationManager(statsCollector).Register()
	}

	monitoring.NewActivityMonitor().Start()

	shutdownManager := shutdown.NewManager()

	stats.RegisterFlusher()

	digest.Register(b)
	membersnapshot.Register(b)
	recurring.Register(b)

	// The tracer drain is declared in the telemetry phase so it runs last:
	// every other drain's logs and spans are exported before the providers
	// close.
	shutdownManager.Register("tracer", shutdown.PhaseTelemetry, func() error {
		log.Info("[Shutdown] Shutting down tracer provider...")
		return tracing.Shutdown(context.Background())
	})

	RegisterShutdownSteps(shutdownManager, ShutdownDrainSpecs(
		func() error {
			log.Info("[Shutdown] Closing database connections...")
			return closeDBConnections()
		},
		func() error {
			log.Info("[Shutdown] Flushing daily stats counters...")
			return stats.Flush()
		},
		func() error {
			log.Info("[Shutdown] Stopping periodic jobs...")
			jobs.Stop()
			return nil
		},
		[]DrainSpec{
			{Name: "users drain", Handler: func() error {
				log.Info("[Shutdown] Draining users async writes...")
				modules.DrainUsersAsyncWrites()
				return nil
			}},
			{Name: "captcha stop", Handler: func() error {
				log.Info("[Shutdown] Stopping captcha lifecycle...")
				modules.StopCaptchaLifecycle()
				return nil
			}},
			{Name: "aispam drain", Handler: func() error {
				log.Info("[Shutdown] Draining AI spam checks...")
				modules.DrainAISpamChecks()
				return nil
			}},
		},
	)...)
	jobs.Start()

	httpServer := httpserver.New(config.AppConfig.HTTPPort, appStartTime)
	httpServer.RegisterHealth()
	httpServer.SetMetricsAuthToken(config.AppConfig.MetricsAuthToken)
	httpServer.RegisterMetrics()
	httpServer.RegisterDBMetrics()

	if config.AppConfig.PanelEnabled {
		httpServer.RegisterPanel(panel.New(b, panel.Deps{
			SessionMaxAge: config.AppConfig.PanelSessionMaxAge,
			LookupChats:   chats.ListChatsWithUser,
		}))
		log.Infof("[Main] Admin panel enabled at %s", config.AppConfig.PanelPublicURL)
	}

	if config.AppConfig.EnablePPROF {
		httpServer.RegisterPPROF()
		log.Warn("[Main] pprof endpoints enabled - DO NOT enable in production!")
	}

	// Both modes stop the same HTTP server; in webhook mode it is also the
	// update intake, so it runs in the HTTP-server phase either way.
	shutdownManager.Register("http stop", shutdown.PhaseHTTPServer, func() error {
		log.Info("[Shutdown] Stopping HTTP server...")
		return httpServer.Stop()
	})

	if config.AppConfig.UseWebhooks {
		if config.AppConfig.WebhookDomain == "" {
			log.Fatal("[Webhook] WEBHOOK_DOMAIN is required when USE_WEBHOOKS is enabled")
		}
		if config.AppConfig.WebhookSecret == "" {
			log.Fatal("[Webhook] WEBHOOK_SECRET is required when USE_WEBHOOKS is enabled for security")
		}

		if err := httpServer.RegisterWebhook(b, dispatcher, config.AppConfig.WebhookSecret, config.AppConfig.WebhookDomain); err != nil {
			log.Fatalf("[HTTPServer] Failed to register webhook: %v", err)
		}

		postInit(b, dispatcher, botUsername, "webhook")

		if err := httpServer.Start(); err != nil {
			log.Fatalf("[HTTPServer] Failed to start HTTP server: %v", err)
		}

		log.Infof("[HTTPServer] Unified HTTP server started on port %d (health, metrics, webhook)", config.AppConfig.HTTPPort)
		// WorkingMode already set by postInit before Start serves traffic; writing it
		// here would race concurrent webhook-handler/tracing readers.

		shutdownManager.WaitForShutdown()
	} else {

		if err := httpServer.Start(); err != nil {
			log.Fatalf("[HTTPServer] Failed to start HTTP server: %v", err)
		}

		log.Infof("[HTTPServer] Unified HTTP server started on port %d (health, metrics)", config.AppConfig.HTTPPort)

		updater := ext.NewUpdater(dispatcher, nil)

		if _, err = b.DeleteWebhook(nil); err != nil {
			log.Fatalf("[Polling] Failed to remove webhook: %v", err)
		}
		log.Info("[Polling] Removed Webhook!")

		postInit(b, dispatcher, botUsername, "polling")

		runPolling(b, updater, shutdownManager)
	}
}

// runPolling registers the updater stop in the update-source phase, starts
// long polling and blocks on the shutdown manager, whose exit after the last
// drain is the only way out. It never waits on the updater's Idle: the updater
// stop releases that wait, so the process would leave before the later drains.
func runPolling(b *gotgbot.Bot, updater *ext.Updater, shutdownManager *shutdown.Manager) {
	shutdownManager.Register("updater stop", shutdown.PhaseUpdateSource, func() error {
		log.Info("[Polling] Stopping updater...")
		err := updater.Stop()
		if err != nil {
			log.Errorf("[Polling] Error stopping updater: %v", err)
			return err
		}
		log.Info("[Polling] Updater stopped successfully")
		return nil
	})

	err := updater.StartPolling(b,
		&ext.PollingOpts{
			DropPendingUpdates: config.AppConfig.DropPendingUpdates,
			GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
				Timeout:        int64(pollingTimeout / time.Second),
				AllowedUpdates: config.AppConfig.AllowedUpdates,
			},
		},
	)
	if err != nil {
		log.Fatalf("[Polling] Failed to start polling: %v", err)
	}
	log.Info("[Polling] Started Polling...!")

	shutdownManager.WaitForShutdown()
}

func newBotAPITransport(maxIdleConns, maxIdleConnsPerHost int) *http.Transport {
	return &http.Transport{
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		MaxConnsPerHost:       maxIdleConnsPerHost + constants.MaxIdleConnsExtraBuffer,
		IdleConnTimeout:       constants.VeryLongTimeout,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
		DisableKeepAlives:     false,
		TLSHandshakeTimeout:   constants.DefaultTimeout,
		ResponseHeaderTimeout: constants.DefaultTimeout,
		ExpectContinueTimeout: constants.ShortTimeout,
	}
}

func resolveBotAPIURL(apiServer string) string {
	if apiServer == "" {
		return gotgbot.DefaultAPIURL
	}

	parsed, err := url.Parse(apiServer)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		log.Warnf("[Main] Invalid API_SERVER '%s'; falling back to default Telegram API.", apiServer)
		return gotgbot.DefaultAPIURL
	}

	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")

	return parsed.String()
}

func resolveBotUsername(b *gotgbot.Bot) string {
	if me, errMe := b.GetMe(nil); errMe == nil && me != nil {
		if me.Username == "" {
			log.Warn("[Main] Bot username is empty after GetMe; deep links may not work until resolved")
		}
		return me.Username
	} else if errMe != nil {
		log.Warnf("[Main] GetMe failed during bootstrap: %v", errMe)
	}
	return ""
}

func newConfiguredDispatcher(maxRoutines int) *ext.Dispatcher {
	return ext.NewDispatcher(&ext.DispatcherOpts{
		Processor:   tracing.TracingProcessor{},
		Error:       dispatcherErrorHandler,
		MaxRoutines: maxRoutines,
	})
}

func dispatcherErrorHandler(_ *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
	defer error_handling.RecoverFromPanic("DispatcherErrorHandler", "Main")

	if errors.Is(err, ext.EndGroups) || errors.Is(err, ext.ContinueGroups) {
		return ext.DispatcherActionNoop
	}

	helpers.LogHandlerError(ctx, err)
	return ext.DispatcherActionNoop
}

func postInit(b *gotgbot.Bot, d *ext.Dispatcher, username string, mode string) {
	alita.LoadModules(d)
	if err := modules.StartCaptchaLifecycle(b); err != nil {
		log.Fatalf("[Captcha] Failed to start lifecycle: %v", err)
	}
	log.Infof("[Modules] Loaded modules: %s", alita.ListModules())

	config.AppConfig.WorkingMode = mode

	tr := i18n.MustNewTranslator("en")
	startDesc, _ := tr.GetString("main_bot_command_start")
	helpDesc, _ := tr.GetString("main_bot_command_help")
	extendDesc, _ := tr.GetString("main_bot_command_extend")
	commandsDesc, _ := tr.GetString("main_bot_command_commands")
	_, err := b.SetMyCommands(
		[]gotgbot.BotCommand{
			{Command: "start", Description: startDesc},
			{Command: "help", Description: helpDesc},
			{Command: "extend", Description: extendDesc},
			{Command: "commands", Description: commandsDesc},
		},
		&gotgbot.SetMyCommandsOpts{
			Scope:        gotgbot.BotCommandScopeAllPrivateChats{},
			LanguageCode: "en",
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Custom bot commands set for private chats")

	setPanelMenuButton(b)

	_, err = b.SendMessage(config.AppConfig.MessageDump,
		fmt.Sprintf("<b>Started Bot!</b>\n<b>Mode:</b> %s\n<b>Loaded Modules:</b>\n%s", mode, alita.ListModules()),
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
		},
	)
	if err != nil {
		log.Errorf("[Bot] Failed to send startup message to log group: %v", err)
		log.Warn("[Bot] Continuing without log channel notifications")
	}

	if username == "" {
		log.Infof("[Bot] Bot has been started in %s mode...", mode)
	} else {
		log.Infof("[Bot] %s has been started in %s mode...", username, mode)
	}
}

// setPanelMenuButton points the default menu button of every private chat at
// the panel, so users reach it without a command. It is a no-op while the panel
// is disabled, which leaves the button untouched. Repeating the same value on
// every start is harmless, and a refusal only warns: the group buttons and the
// /start panel_<chatID> deep link still open the panel.
func setPanelMenuButton(b *gotgbot.Bot) {
	if !config.AppConfig.PanelEnabled {
		return
	}
	tr := i18n.MustNewTranslator("en")
	text, _ := tr.GetString("panel_open_button")
	_, err := b.SetChatMenuButton(&gotgbot.SetChatMenuButtonOpts{
		MenuButton: gotgbot.MenuButtonWebApp{
			Text:   text,
			WebApp: gotgbot.WebAppInfo{Url: config.AppConfig.PanelPublicURL},
		},
	})
	if err != nil {
		log.Warnf("[Panel] Could not set the menu button: %v", err)
	}
}
