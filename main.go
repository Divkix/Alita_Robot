package main

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/constants"

	"github.com/divkix/Alita_Robot/alita"
	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	dbmonitoring "github.com/divkix/Alita_Robot/alita/db/monitoring"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/modules"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/errors"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/httpserver"
	"github.com/divkix/Alita_Robot/alita/utils/monitoring"
	"github.com/divkix/Alita_Robot/alita/utils/shutdown"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

//go:embed locales
var Locales embed.FS

func main() {
	appStartTime := time.Now()

	// Health check mode for Docker healthcheck (distroless images have no curl/wget)
	if len(os.Args) > 1 && (os.Args[1] == "--health" || os.Args[1] == "-health") {
		healthPort := healthCheckPort()
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", healthPort))
		if err != nil {
			os.Exit(1)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version" || os.Args[1] == "-v") {
		version := config.AppConfig.BotVersion
		if version == "" {
			version = "v2.22.7"
		}
		fmt.Println(version)
		os.Exit(0)
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Main] Panic recovered: %v", r)
			os.Exit(1)
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

	if err := tracing.InitTracing(); err != nil {
		log.Warnf("Failed to initialize tracing: %v - continuing without distributed tracing", err)
	} else {
		log.Info("Distributed tracing initialized successfully")
	}

	maxIdleConns := config.AppConfig.HTTPMaxIdleConns
	maxIdleConnsPerHost := config.AppConfig.HTTPMaxIdleConnsPerHost

	transport := newBotAPITransport(maxIdleConns, maxIdleConnsPerHost)

	log.Infof("[Main] HTTP transport configured with MaxIdleConns: %d, MaxIdleConnsPerHost: %d", maxIdleConns, maxIdleConnsPerHost)

	log.Info("[Main] Initializing bot with optimized HTTP client (connection pooling enabled)")
	b, err := gotgbot.NewBot(config.AppConfig.BotToken, &gotgbot.BotOpts{
		BotClient: &gotgbot.BaseBotClient{
			Client: http.Client{
				Transport: transport,
				Timeout:   constants.LongTimeout,
			},
			UseTestEnvironment: false,
			DefaultRequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Duration(constants.LongTimeout),
				APIURL:  resolveBotAPIURL(config.AppConfig.ApiServer),
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

	var statsCollector *monitoring.BackgroundStatsCollector
	var autoRemediation *monitoring.AutoRemediationManager
	var activityMonitor *monitoring.ActivityMonitor
	var dbMonitoringCancel context.CancelFunc

	if config.AppConfig.EnableDBMonitoring {
		var ctx context.Context
		ctx, dbMonitoringCancel = context.WithCancel(context.Background())
		dbmonitoring.StartMonitoring(ctx, time.Minute)
	}

	if config.AppConfig.EnableBackgroundStats {
		statsCollector = monitoring.NewBackgroundStatsCollector()
		monitoring.SetGlobalCollector(statsCollector)
		error_handling.SetOnErrorCallback(monitoring.GlobalRecordError)
		tracing.SetOnProcessUpdateCallback(monitoring.GlobalRecordMessage)
		statsCollector.Start()
	}

	if config.AppConfig.EnablePerformanceMonitoring {
		autoRemediation = monitoring.NewAutoRemediationManager(statsCollector)
		autoRemediation.Start()
	}

	activityMonitor = monitoring.NewActivityMonitor()
	activityMonitor.Start()

	shutdownManager := shutdown.NewManager()

	shutdownManager.RegisterHandler(func() error {
		log.Info("[Shutdown] Closing database connections...")
		return closeDBConnections()
	})
	shutdownManager.RegisterHandler(func() error {
		log.Info("[Shutdown] Stopping monitoring systems...")
		if activityMonitor != nil {
			activityMonitor.Stop()
		}
		if autoRemediation != nil {
			autoRemediation.Stop()
		}
		if statsCollector != nil {
			statsCollector.Stop()
		}
		return nil
	})

	if dbMonitoringCancel != nil {
		shutdownManager.RegisterHandler(func() error {
			log.Info("[Shutdown] Stopping database monitoring...")
			dbMonitoringCancel()
			return nil
		})
	}

	shutdownManager.RegisterHandler(func() error {
		log.Info("[Shutdown] Shutting down tracer provider...")
		return tracing.Shutdown(context.Background())
	})

	shutdownManager.RegisterHandler(func() error {
		log.Info("[Shutdown] Stopping anti-raid expiry poller...")
		modules.StopAntiRaidExpiryPoller()
		return nil
	})
	shutdownManager.RegisterHandler(func() error {
		log.Info("[Shutdown] Stopping captcha lifecycle...")
		modules.StopCaptchaLifecycle()
		return nil
	})

	httpServer := httpserver.New(config.AppConfig.HTTPPort, appStartTime)
	httpServer.RegisterHealth()
	httpServer.SetMetricsAuthToken(config.AppConfig.MetricsAuthToken)
	httpServer.RegisterMetrics()
	httpServer.RegisterDBMetrics()

	if config.AppConfig.EnablePPROF {
		httpServer.RegisterPPROF()
		log.Warn("[Main] pprof endpoints enabled - DO NOT enable in production!")
	}

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

		shutdownManager.RegisterHandler(func() error {
			log.Info("[Shutdown] Stopping HTTP server...")
			return httpServer.Stop()
		})

		postInit(b, dispatcher, botUsername, "webhook")

		if err := httpServer.Start(); err != nil {
			log.Fatalf("[HTTPServer] Failed to start HTTP server: %v", err)
		}

		log.Infof("[HTTPServer] Unified HTTP server started on port %d (health, metrics, webhook)", config.AppConfig.HTTPPort)
		config.AppConfig.WorkingMode = "webhook"

		go shutdownManager.WaitForShutdown()

		select {}
	} else {

		shutdownManager.RegisterHandler(func() error {
			log.Info("[Shutdown] Stopping HTTP server...")
			return httpServer.Stop()
		})

		if err := httpServer.Start(); err != nil {
			log.Fatalf("[HTTPServer] Failed to start HTTP server: %v", err)
		}

		log.Infof("[HTTPServer] Unified HTTP server started on port %d (health, metrics)", config.AppConfig.HTTPPort)

		updater := ext.NewUpdater(dispatcher, nil)

		shutdownManager.RegisterHandler(func() error {
			log.Info("[Polling] Stopping updater...")
			err := updater.Stop()
			if err != nil {
				log.Errorf("[Polling] Error stopping updater: %v", err)
				return err
			}
			log.Info("[Polling] Updater stopped successfully")
			return nil
		})

		if _, err = b.DeleteWebhook(nil); err != nil {
			log.Fatalf("[Polling] Failed to remove webhook: %v", err)
		}
		log.Info("[Polling] Removed Webhook!")

		postInit(b, dispatcher, botUsername, "polling")

		err = updater.StartPolling(b,
			&ext.PollingOpts{
				DropPendingUpdates: config.AppConfig.DropPendingUpdates,
				GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
					AllowedUpdates: config.AppConfig.AllowedUpdates,
				},
			},
		)
		if err != nil {
			log.Fatalf("[Polling] Failed to start polling: %v", err)
		}
		log.Info("[Polling] Started Polling...!")

		go shutdownManager.WaitForShutdown()

		updater.Idle()
	}
}

func healthCheckPort() int {
	for _, name := range []string{"HTTP_PORT", "PORT"} {
		value := os.Getenv(name)
		if value == "" {
			continue
		}
		port, err := strconv.Atoi(value)
		if err == nil && port > 0 && port <= 65535 {
			return port
		}
		break
	}
	if config.AppConfig != nil && config.AppConfig.HTTPPort > 0 {
		return config.AppConfig.HTTPPort
	}
	return constants.DefaultHTTPPort
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

	logFields := log.Fields{
		"update_id": func() int64 {
			if ctx != nil && ctx.UpdateId != 0 {
				return ctx.UpdateId
			}
			return -1
		}(),
		"error_type": fmt.Sprintf("%T", err),
	}

	if wrappedErr, ok := err.(*errors.WrappedError); ok {
		logFields["file"] = wrappedErr.File
		logFields["line"] = wrappedErr.Line
		logFields["function"] = wrappedErr.Function
	}

	if helpers.IsExpectedTelegramError(err) {
		log.WithFields(logFields).Warnf("Expected Telegram API error: %v", err)
		return ext.DispatcherActionNoop
	}

	log.WithFields(logFields).Errorf("Handler error occurred: %v", err)
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
	_, err := b.SetMyCommands(
		[]gotgbot.BotCommand{
			{Command: "start", Description: startDesc},
			{Command: "help", Description: helpDesc},
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

func closeDBConnections() error {
	err := db.Close()
	if err != nil {
		log.Errorf("[Shutdown] Failed to close database connections: %v", err)
		return fmt.Errorf("failed to close database: %w", err)
	}
	log.Info("[Shutdown] Database connections closed successfully")
	return nil
}
