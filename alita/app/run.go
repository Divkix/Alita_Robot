// Package app is the startup composition root: Run assembles the bot's
// services — flag-gated monitoring, panel and pprof wiring, scheduler and
// drain registration through the shutdown phase API, the HTTP server, and the
// webhook-vs-polling fork — behind one small interface. main() only dispatches
// the CLI modes and calls Run.
//
// Deps is the injection seam: a nil field selects the production
// implementation, so tests substitute fakes only for what they assert on.
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

// HealthCheck reports whether the running bot's HTTP server answers /health.
// In --health mode, config initialization leaves the port unset, so the check
// resolves it from HTTP_PORT, then PORT, with 8080 as the default.
func HealthCheck() error {
	port := config.AppConfig.HTTPPort
	if port == 0 {
		port = config.HTTPPortFromEnv()
	}
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
	}
	return nil
}

// Deps carries the startup seams Run composes through. A nil field selects
// the production implementation; tests fill only the fields they assert on.
type Deps struct {
	// InitCache initializes the read-through cache (Redis).
	InitCache func() error
	// InitializeLocales loads the embedded locales into the i18n manager.
	InitializeLocales func() error
	// Locales is the embedded locale filesystem main() provides; the default
	// InitializeLocales loads it into the i18n manager.
	Locales embed.FS
	// NewBot builds the bot client from cfg (token, API server, transport).
	NewBot func(cfg *config.Config) (*gotgbot.Bot, error)
	// InitialChecks runs the pre-dispatcher database checks (bot row).
	InitialChecks func(b *gotgbot.Bot) error
	// NewDispatcher builds the update dispatcher for cfg.
	NewDispatcher func(cfg *config.Config) *ext.Dispatcher
	// NewShutdownManager builds the drain manager the drains register on.
	NewShutdownManager func() *shutdown.Manager
	// NewHTTPServer builds the unified HTTP server for cfg's port.
	NewHTTPServer func(port int, startTime time.Time) *httpserver.Server

	// The flag-gated services. Each runs only when its config flag is set,
	// except StartActivityMonitor, which always starts.
	RegisterDBMonitoring    func()
	NewBackgroundStats      func() *monitoring.BackgroundStatsCollector
	SetGlobalCollector      func(*monitoring.BackgroundStatsCollector)
	SetOnErrorCallback      func(func())
	SetOnProcessUpdate      func(func())
	RegisterBackgroundStats func(*monitoring.BackgroundStatsCollector)
	NewAutoRemediation      func(*monitoring.BackgroundStatsCollector) Remediator
	StartActivityMonitor    func()

	// RegisterFlusher registers the periodic stats flush job.
	RegisterFlusher func()
	// RegisterBotSchedulers registers the digest, member-snapshot and
	// recurring-message schedulers for bot.
	RegisterBotSchedulers func(b *gotgbot.Bot)
	// ModuleDrains returns the module-owned shutdown drains (async writes,
	// captcha lifecycle, AI spam checks) in registration order.
	ModuleDrains func() []DrainSpec
	// PanelHandler builds the admin panel handler when the panel is enabled.
	PanelHandler func(b *gotgbot.Bot, cfg *config.Config) http.Handler
	// StartHTTPServer starts the assembled HTTP server.
	StartHTTPServer func(s *httpserver.Server) error
	// StopHTTPServer stops it; registered as the HTTP-server-phase drain.
	StopHTTPServer func(s *httpserver.Server) error
	// ShutdownWait blocks until the shutdown manager runs the drains and
	// exits the process.
	ShutdownWait func(m *shutdown.Manager)

	// postInit's seams: module loading, the captcha lifecycle and the module
	// list it logs. Tests substitute fakes so postInit runs without loading
	// the real module registry or touching the database.
	LoadModules           func(d *ext.Dispatcher)
	StartCaptchaLifecycle func(b *gotgbot.Bot) error
	ListModules           func() string
}

// Remediator is the periodic auto-remediation job the performance-monitoring
// flag registers; *monitoring.AutoRemediationManager satisfies it, and tests
// substitute a fake.
type Remediator interface {
	Register()
}

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
// ("webhook" or "polling") postInit records as WorkingMode; cfg is the loaded,
// validated configuration. It blocks on shutdown once the update source is
// running; it returns only when startup fails before that point.
func Run(mode string, cfg *config.Config, deps Deps) {
	deps = defaultDeps(cfg, deps)

	// gotgbot's dispatcher and updater log panics and polling errors through
	// slog; send them, and the standard log package, through logrus.
	slog.SetDefault(slog.New(slogrus.Handler{}))

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Main] Panic recovered: %v\nStack trace:\n%s", r, debug.Stack())
			log.StandardLogger().Exit(1) // flushes telemetry first
		}
	}()

	if cfg.Debug {
		log.Info("Running in DEBUG Mode...")
	} else {
		log.Info("Running in RELEASE Mode...")
	}

	if err := deps.InitCache(); err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	log.Info("Cache system initialized successfully")

	if err := deps.InitializeLocales(); err != nil {
		log.Fatalf("Failed to initialize locale manager: %v", err)
	}
	log.Infof("Locale manager initialized with %d languages: %v", len(i18n.GetManager().GetAvailableLanguages()), i18n.GetManager().GetAvailableLanguages())

	b, err := deps.NewBot(cfg)
	if err != nil {
		log.Fatalf("Failed to create new bot: %v", err)
	}

	botUsername := resolveBotUsername(b)

	if err := deps.InitialChecks(b); err != nil {
		log.Fatalf("Initial checks failed: %v", err)
	}

	dispatcher := deps.NewDispatcher(cfg)

	if cfg.EnableDBMonitoring {
		deps.RegisterDBMonitoring()
	}

	var statsCollector *monitoring.BackgroundStatsCollector
	if cfg.EnableBackgroundStats {
		statsCollector = deps.NewBackgroundStats()
		deps.SetGlobalCollector(statsCollector)
		deps.SetOnErrorCallback(monitoring.GlobalRecordError)
		deps.SetOnProcessUpdate(monitoring.GlobalRecordMessage)
		deps.RegisterBackgroundStats(statsCollector)
	}

	if cfg.EnablePerformanceMonitoring {
		deps.NewAutoRemediation(statsCollector).Register()
	}

	deps.StartActivityMonitor()

	shutdownManager := deps.NewShutdownManager()

	deps.RegisterFlusher()

	deps.RegisterBotSchedulers(b)

	httpServer := deps.NewHTTPServer(cfg.HTTPPort, time.Now())
	httpServer.RegisterHealth()
	httpServer.SetMetricsAuthToken(cfg.MetricsAuthToken)
	httpServer.RegisterMetrics()
	httpServer.RegisterDBMetrics()

	if cfg.PanelEnabled {
		httpServer.RegisterPanel(deps.PanelHandler(b, cfg))
		log.Infof("[Main] Admin panel enabled at %s", cfg.PanelPublicURL)
	}

	if cfg.EnablePPROF {
		httpServer.RegisterPPROF()
		log.Warn("[Main] pprof endpoints enabled - DO NOT enable in production!")
	}

	// RunDrainSpecs declares every drain but the polling updater stop as
	// data so the whole set stays enumerable for tests. Both modes stop the
	// same HTTP server; in webhook mode it is also the update intake, so it
	// runs in the HTTP-server phase either way.
	RegisterShutdownSteps(shutdownManager, RunDrainSpecs(
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
		deps.ModuleDrains(),
		httpServer,
		deps.StopHTTPServer,
	)...)
	jobs.Start()

	if cfg.UseWebhooks {
		// WebhookDomain and WebhookSecret are guaranteed non-empty: config
		// validation owns that contract, so startup does not re-check it.
		if err := httpServer.RegisterWebhook(b, dispatcher, cfg.WebhookSecret, cfg.WebhookDomain); err != nil {
			log.Fatalf("[HTTPServer] Failed to register webhook: %v", err)
		}

		postInit(b, dispatcher, botUsername, "webhook", cfg, deps)

		if err := deps.StartHTTPServer(httpServer); err != nil {
			log.Fatalf("[HTTPServer] Failed to start HTTP server: %v", err)
		}

		log.Infof("[HTTPServer] Unified HTTP server started on port %d (health, metrics, webhook)", cfg.HTTPPort)
		// WorkingMode already set by postInit before Start serves traffic; writing it
		// here would race concurrent webhook-handler/tracing readers.

		deps.ShutdownWait(shutdownManager)
	} else {
		if err := deps.StartHTTPServer(httpServer); err != nil {
			log.Fatalf("[HTTPServer] Failed to start HTTP server: %v", err)
		}

		log.Infof("[HTTPServer] Unified HTTP server started on port %d (health, metrics)", cfg.HTTPPort)

		updater := ext.NewUpdater(dispatcher, nil)

		if _, err = b.DeleteWebhook(nil); err != nil {
			log.Fatalf("[Polling] Failed to remove webhook: %v", err)
		}
		log.Info("[Polling] Removed Webhook!")

		postInit(b, dispatcher, botUsername, "polling", cfg, deps)

		runPolling(b, updater, cfg, updaterStopSpec(updater), shutdownManager, deps.ShutdownWait)
	}
}

// updaterStopSpec is the polling mode's update-source drain: it stops the
// long poll so a held getUpdates does not delay the later drains.
func updaterStopSpec(updater *ext.Updater) DrainSpec {
	return DrainSpec{Name: "updater stop", Phase: shutdown.PhaseUpdateSource, Handler: func() error {
		log.Info("[Polling] Stopping updater...")
		err := updater.Stop()
		if err != nil {
			log.Errorf("[Polling] Error stopping updater: %v", err)
			return err
		}
		log.Info("[Polling] Updater stopped successfully")
		return nil
	}}
}

// RunDrainSpecs declares every drain Run registers outside polling mode's
// updater stop: the shared tail, the HTTP server stop and the tracer drain
// last, so its telemetry phase runs after every other drain. server may be
// nil in tests that only enumerate the specs.
func RunDrainSpecs(closeDB, flushStats, stopJobs func() error, moduleDrains []DrainSpec, server *httpserver.Server, stopHTTP func(*httpserver.Server) error) []DrainSpec {
	specs := ShutdownDrainSpecs(closeDB, flushStats, stopJobs, moduleDrains)
	specs = append(specs, DrainSpec{
		Name:  "http stop",
		Phase: shutdown.PhaseHTTPServer,
		Handler: func() error {
			log.Info("[Shutdown] Stopping HTTP server...")
			return stopHTTP(server)
		},
	}, DrainSpec{
		Name:  "tracer",
		Phase: shutdown.PhaseTelemetry,
		Handler: func() error {
			log.Info("[Shutdown] Shutting down tracer provider...")
			return tracing.Shutdown(context.Background())
		},
	})
	return specs
}

// runPolling registers the updater stop in the update-source phase, starts
// long polling and blocks on the shutdown manager, whose exit after the last
// drain is the only way out. It never waits on the updater's Idle: the updater
// stop releases that wait, so the process would leave before the later drains.
func runPolling(b *gotgbot.Bot, updater *ext.Updater, cfg *config.Config, updaterStop DrainSpec, shutdownManager *shutdown.Manager, wait func(*shutdown.Manager)) {
	shutdownManager.Register(updaterStop.Name, updaterStop.Phase, updaterStop.Handler)

	err := updater.StartPolling(b,
		&ext.PollingOpts{
			DropPendingUpdates: cfg.DropPendingUpdates,
			GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
				Timeout:        int64(pollingTimeout / time.Second),
				AllowedUpdates: cfg.AllowedUpdates,
			},
		},
	)
	if err != nil {
		log.Fatalf("[Polling] Failed to start polling: %v", err)
	}
	log.Info("[Polling] Started Polling...!")

	wait(shutdownManager)
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

// postInit loads the modules, starts the captcha lifecycle, publishes the
// command menu and startup message, then records mode as the working mode.
// It runs inside the update-source fork: webhook mode calls it before the
// server starts serving, polling mode after the webhook is removed.
func postInit(b *gotgbot.Bot, d *ext.Dispatcher, username string, mode string, cfg *config.Config, deps Deps) {
	deps.LoadModules(d)
	if err := deps.StartCaptchaLifecycle(b); err != nil {
		log.Fatalf("[Captcha] Failed to start lifecycle: %v", err)
	}
	log.Infof("[Modules] Loaded modules: %s", deps.ListModules())

	cfg.WorkingMode = mode

	tr := i18n.MustNewTranslator("en")
	startDesc := tr.String("main_bot_command_start")
	helpDesc := tr.String("main_bot_command_help")
	extendDesc := tr.String("main_bot_command_extend")
	commandsDesc := tr.String("main_bot_command_commands")
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

	setPanelMenuButton(b, cfg)

	_, err = b.SendMessage(cfg.MessageDump,
		fmt.Sprintf("<b>Started Bot!</b>\n<b>Mode:</b> %s\n<b>Loaded Modules:</b>\n%s", mode, deps.ListModules()),
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
func setPanelMenuButton(b *gotgbot.Bot, cfg *config.Config) {
	if !cfg.PanelEnabled {
		return
	}
	tr := i18n.MustNewTranslator("en")
	text := tr.String("panel_open_button")
	_, err := b.SetChatMenuButton(&gotgbot.SetChatMenuButtonOpts{
		MenuButton: gotgbot.MenuButtonWebApp{
			Text:   text,
			WebApp: gotgbot.WebAppInfo{Url: cfg.PanelPublicURL},
		},
	})
	if err != nil {
		log.Warnf("[Panel] Could not set the menu button: %v", err)
	}
}

// defaultDeps fills every nil Deps field with the production implementation,
// leaving the fields the caller set untouched.
func defaultDeps(cfg *config.Config, deps Deps) Deps {
	return defaultLifecycleDeps(cfg, defaultServiceDeps(deps))
}

// defaultServiceDeps fills the transport, dispatcher, locale and monitoring
// fields of Deps.
func defaultServiceDeps(deps Deps) Deps {
	if deps.InitCache == nil {
		deps.InitCache = cache.InitCache
	}
	if deps.InitializeLocales == nil {
		deps.InitializeLocales = func() error {
			return i18n.GetManager().Initialize(&deps.Locales, "locales", i18n.DefaultManagerConfig())
		}
	}
	if deps.NewBot == nil {
		deps.NewBot = newBot
	}
	if deps.InitialChecks == nil {
		deps.InitialChecks = alita.InitialChecks
	}
	if deps.NewDispatcher == nil {
		deps.NewDispatcher = func(cfg *config.Config) *ext.Dispatcher {
			return newConfiguredDispatcher(cfg.DispatcherMaxRoutines)
		}
	}
	if deps.NewShutdownManager == nil {
		deps.NewShutdownManager = shutdown.NewManager
	}
	if deps.NewHTTPServer == nil {
		deps.NewHTTPServer = func(port int, startTime time.Time) *httpserver.Server {
			return httpserver.New(port, startTime)
		}
	}

	if deps.RegisterDBMonitoring == nil {
		deps.RegisterDBMonitoring = func() { dbmonitoring.Register(time.Minute) }
	}
	if deps.NewBackgroundStats == nil {
		deps.NewBackgroundStats = monitoring.NewBackgroundStatsCollector
	}
	if deps.SetGlobalCollector == nil {
		deps.SetGlobalCollector = monitoring.SetGlobalCollector
	}
	if deps.SetOnErrorCallback == nil {
		deps.SetOnErrorCallback = error_handling.SetOnErrorCallback
	}
	if deps.SetOnProcessUpdate == nil {
		deps.SetOnProcessUpdate = tracing.SetOnProcessUpdateCallback
	}
	if deps.RegisterBackgroundStats == nil {
		deps.RegisterBackgroundStats = func(c *monitoring.BackgroundStatsCollector) { c.Register() }
	}
	if deps.NewAutoRemediation == nil {
		deps.NewAutoRemediation = func(c *monitoring.BackgroundStatsCollector) Remediator {
			return monitoring.NewAutoRemediationManager(c)
		}
	}
	if deps.StartActivityMonitor == nil {
		deps.StartActivityMonitor = func() { monitoring.NewActivityMonitor().Start() }
	}
	return deps
}

// defaultLifecycleDeps fills the schedulers, drains, panel, HTTP server,
// shutdown and module-loading fields of Deps.
func defaultLifecycleDeps(cfg *config.Config, deps Deps) Deps {
	if deps.RegisterFlusher == nil {
		deps.RegisterFlusher = stats.RegisterFlusher
	}
	if deps.RegisterBotSchedulers == nil {
		deps.RegisterBotSchedulers = func(b *gotgbot.Bot) {
			digest.Register(b)
			membersnapshot.Register(b)
			recurring.Register(b)
		}
	}
	if deps.ModuleDrains == nil {
		deps.ModuleDrains = func() []DrainSpec {
			return []DrainSpec{
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
			}
		}
	}
	if deps.PanelHandler == nil {
		deps.PanelHandler = func(b *gotgbot.Bot, cfg *config.Config) http.Handler {
			return panel.New(b, panel.Deps{
				SessionMaxAge: cfg.PanelSessionMaxAge,
				LookupChats:   chats.ListChatsWithUser,
			})
		}
	}
	if deps.StartHTTPServer == nil {
		deps.StartHTTPServer = func(s *httpserver.Server) error { return s.Start() }
	}
	if deps.StopHTTPServer == nil {
		deps.StopHTTPServer = func(s *httpserver.Server) error { return s.Stop() }
	}
	if deps.ShutdownWait == nil {
		deps.ShutdownWait = func(m *shutdown.Manager) { m.WaitForShutdown() }
	}
	if deps.LoadModules == nil {
		deps.LoadModules = alita.LoadModules
	}
	if deps.StartCaptchaLifecycle == nil {
		deps.StartCaptchaLifecycle = modules.StartCaptchaLifecycle
	}
	if deps.ListModules == nil {
		deps.ListModules = alita.ListModules
	}
	return deps
}

// newBot builds the production bot: pooled HTTP transport wrapped in the
// reply-safe client, resolved API URL and long timeout.
func newBot(cfg *config.Config) (*gotgbot.Bot, error) {
	maxIdleConns := cfg.HTTPMaxIdleConns
	maxIdleConnsPerHost := cfg.HTTPMaxIdleConnsPerHost

	transport := newBotAPITransport(maxIdleConns, maxIdleConnsPerHost)

	log.Infof("[Main] HTTP transport configured with MaxIdleConns: %d, MaxIdleConnsPerHost: %d", maxIdleConns, maxIdleConnsPerHost)

	log.Info("[Main] Initializing bot with optimized HTTP client (connection pooling enabled)")
	b, err := gotgbot.NewBot(cfg.BotToken, &gotgbot.BotOpts{
		BotClient: &helpers.ReplySafeBotClient{
			BotClient: &gotgbot.BaseBotClient{
				Client: http.Client{
					Transport: tracing.BotAPITransport(transport),
					Timeout:   constants.LongTimeout,
				},
				UseTestEnvironment: false,
				DefaultRequestOpts: &gotgbot.RequestOpts{
					Timeout: time.Duration(constants.LongTimeout),
					APIURL:  resolveBotAPIURL(cfg.ApiServer),
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	log.Infof("[Main] Bot initialized with optimized connection pooling (MaxIdleConns: %d, MaxIdleConnsPerHost: %d, HTTP/2 enabled)", maxIdleConns, maxIdleConnsPerHost)
	return b, nil
}
