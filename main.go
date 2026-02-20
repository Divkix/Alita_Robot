package main

import (
	"embed"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/async"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/errors"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/httpserver"
	"github.com/divkix/Alita_Robot/alita/utils/monitoring"
	"github.com/divkix/Alita_Robot/alita/utils/shutdown"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita"
)

//go:embed locales
var Locales embed.FS

// main initializes and starts the Alita Robot Telegram bot.
// It sets up monitoring, database connections, webhook/polling mode,
// loads all modules, and handles graceful shutdown.
func main() {
	// Health check mode for Docker healthcheck (distroless images have no curl/wget)
	if len(os.Args) > 1 && (os.Args[1] == "--health" || os.Args[1] == "-health") {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", config.AppConfig.HTTPPort))
		if err != nil {
			os.Exit(1)
		}
		_ = resp.Body.Close() // Ignore close error since we're exiting immediately
		if resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Setup panic recovery for main goroutine
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Main] Panic recovered: %v", r)
			os.Exit(1)
		}
	}()

	// logs if bot is running in debug mode or not
	if config.AppConfig.Debug {
		log.Info("Running in DEBUG Mode...")
	} else {
		log.Info("Running in RELEASE Mode...")
	}

	// Initialize Locale Manager
	localeManager := i18n.GetManager()
	if err := localeManager.Initialize(&Locales, "locales", i18n.DefaultManagerConfig()); err != nil {
		log.Fatalf("Failed to initialize locale manager: %v", err)
	}
	log.Infof("Locale manager initialized with %d languages: %v", len(localeManager.GetAvailableLanguages()), localeManager.GetAvailableLanguages())

	// Create optimized HTTP transport with connection pooling for better performance
	// IMPORTANT: We create a transport pointer that will be shared across all requests
	// This ensures connection pooling works correctly (the http.Client struct is copied by value in BaseBotClient)
	// Use configurable values for optimal performance
	maxIdleConns := config.AppConfig.HTTPMaxIdleConns
	maxIdleConnsPerHost := config.AppConfig.HTTPMaxIdleConnsPerHost

	httpTransport := &http.Transport{
		MaxIdleConns:          maxIdleConns,             // Configurable maximum idle connections across all hosts
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,      // Configurable connections per host (api.telegram.org)
		MaxConnsPerHost:       maxIdleConnsPerHost + 20, // Allow some extra connections for burst traffic
		IdleConnTimeout:       120 * time.Second,        // Keep connections alive longer for better reuse
		DisableCompression:    false,                    // Enable compression for smaller payloads
		ForceAttemptHTTP2:     true,                     // Enable HTTP/2 for multiplexing
		DisableKeepAlives:     false,                    // Explicitly enable keep-alive for connection reuse
		TLSHandshakeTimeout:   10 * time.Second,         // Timeout for TLS handshake
		ResponseHeaderTimeout: 10 * time.Second,         // Timeout waiting for response headers
		ExpectContinueTimeout: 1 * time.Second,          // Timeout for Expect: 100-continue
	}

	log.Infof("[Main] HTTP transport configured with MaxIdleConns: %d, MaxIdleConnsPerHost: %d", maxIdleConns, maxIdleConnsPerHost)

	// If a custom API server is configured (e.g., local Bot API server),
	// wrap the transport to rewrite requests from api.telegram.org to the configured server.
	var transport http.RoundTripper = httpTransport
	if config.AppConfig.ApiServer != "" && config.AppConfig.ApiServer != "https://api.telegram.org" {
		if parsed, err := url.Parse(config.AppConfig.ApiServer); err == nil && parsed.Host != "" {
			transport = &apiServerRewriteTransport{base: httpTransport, target: parsed}
			log.Infof("[Main] Using custom Bot API server: %s", parsed.String())
		} else {
			log.Warnf("[Main] Invalid API_SERVER '%s'; falling back to default Telegram API.", config.AppConfig.ApiServer)
		}
	}

	// Create bot with optimized HTTP client using BaseBotClient
	log.Info("[Main] Initializing bot with optimized HTTP client (connection pooling enabled)")
	b, err := gotgbot.NewBot(config.AppConfig.BotToken, &gotgbot.BotOpts{
		BotClient: &gotgbot.BaseBotClient{
			Client: http.Client{
				Transport: transport, // Use the shared (possibly rewritten) transport
				Timeout:   30 * time.Second,
			},
			UseTestEnvironment: false,
			DefaultRequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Duration(30) * time.Second,
			},
		},
	})
	if err != nil {
		log.Fatalf("Failed to create new bot: %v", err)
	}
	log.Infof("[Main] Bot initialized with optimized connection pooling (MaxIdleConns: %d, MaxIdleConnsPerHost: %d, HTTP/2 enabled)", maxIdleConns, maxIdleConnsPerHost)

	// Retrieve bot identity early for logging and downstream components that reference username
	var botUsername string
	if me, errMe := b.GetMe(nil); errMe == nil && me != nil {
		botUsername = me.Username
		if botUsername == "" {
			log.Warn("[Main] Bot username is empty after GetMe; deep links may not work until resolved")
		}
	} else if errMe != nil {
		log.Warnf("[Main] GetMe failed during bootstrap: %v", errMe)
	}

	// Pre-warm connections to Telegram API for faster initial responses
	go func() {
		log.Info("[Main] Pre-warming connections to Telegram API...")

		// Make multiple requests to establish connection pool
		for i := 0; i < 3; i++ {
			startTime := time.Now()
			_, err := b.GetMe(nil)
			if err != nil {
				log.Warnf("[Main] Pre-warm request %d failed: %v", i+1, err)
			} else {
				elapsed := time.Since(startTime)
				log.Infof("[Main] Pre-warm request %d completed in %v", i+1, elapsed)
				// First request establishes connection, subsequent ones should be faster
				if i > 0 && elapsed < 100*time.Millisecond {
					log.Info("[Main] Connection pooling confirmed working - reused existing connection")
				}
			}
			time.Sleep(100 * time.Millisecond) // Small delay between requests
		}

		log.Info("[Main] Connection pre-warming completed")
	}()

	// some initial checks before running bot
	if err := alita.InitialChecks(b); err != nil {
		log.Fatalf("Initial checks failed: %v", err)
	}

	// Initialize async processing system
	if config.AppConfig.EnableAsyncProcessing {
		async.InitializeAsyncProcessor()
		defer async.StopAsyncProcessor()
	}

	// Create dispatcher with limited max routines and proper error recovery
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		// Enhanced error handler with recovery and structured logging
		Error: func(_ *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			// Recover from any panics in error handler
			defer error_handling.RecoverFromPanic("DispatcherErrorHandler", "Main")

			// Extract stack trace if it's a wrapped error
			logFields := log.Fields{
				"update_id": func() int64 {
					if ctx != nil && ctx.UpdateId != 0 {
						return ctx.UpdateId
					}
					return -1
				}(),
				"error_type": fmt.Sprintf("%T", err),
			}

			// Check if it's our wrapped error with stack info
			if wrappedErr, ok := err.(*errors.WrappedError); ok {
				logFields["file"] = wrappedErr.File
				logFields["line"] = wrappedErr.Line
				logFields["function"] = wrappedErr.Function
			}

			// Check if this is an expected Telegram API error
			if helpers.IsExpectedTelegramError(err) {
				log.WithFields(logFields).Warnf("Expected Telegram API error: %v", err)
				return ext.DispatcherActionNoop
			}

			// Log the error with context information
			log.WithFields(logFields).Errorf("Handler error occurred: %v", err)

			// Continue processing other updates
			return ext.DispatcherActionNoop
		},
		MaxRoutines: config.AppConfig.DispatcherMaxRoutines, // Configurable max concurrent goroutines
	})

	// Initialize monitoring systems
	var statsCollector *monitoring.BackgroundStatsCollector
	var autoRemediation *monitoring.AutoRemediationManager
	var activityMonitor *monitoring.ActivityMonitor

	if config.AppConfig.EnableBackgroundStats {
		statsCollector = monitoring.NewBackgroundStatsCollector()
		statsCollector.Start()
		defer statsCollector.Stop()
	}

	if config.AppConfig.EnablePerformanceMonitoring {
		autoRemediation = monitoring.NewAutoRemediationManager(statsCollector)
		autoRemediation.Start()
		defer autoRemediation.Stop()
	}

	// Initialize activity monitoring for automatic group activity tracking
	activityMonitor = monitoring.NewActivityMonitor()
	activityMonitor.Start()
	defer activityMonitor.Stop()

	// Setup graceful shutdown
	shutdownManager := shutdown.NewManager()

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
	shutdownManager.RegisterHandler(func() error {
		log.Info("[Shutdown] Closing database connections...")
		return closeDBConnections()
	})

	// Start shutdown handler in background
	go shutdownManager.WaitForShutdown()

	// Create unified HTTP server for health, metrics, and webhook endpoints
	httpServer := httpserver.New(config.AppConfig.HTTPPort)
	httpServer.RegisterHealth()
	httpServer.RegisterMetrics()

	// Register pprof endpoints if enabled (development only)
	if config.AppConfig.EnablePPROF {
		httpServer.RegisterPPROF()
		log.Warn("[Main] pprof endpoints enabled - DO NOT enable in production!")
	}

	// Check if we should use webhooks or polling
	if config.AppConfig.UseWebhooks {
		// Validate webhook configuration
		if config.AppConfig.WebhookDomain == "" {
			log.Fatal("[Webhook] WEBHOOK_DOMAIN is required when USE_WEBHOOKS is enabled")
		}
		if config.AppConfig.WebhookSecret == "" {
			log.Warn("[Webhook] WEBHOOK_SECRET is not set, webhook validation will be skipped")
		}

		// Register webhook endpoint on the unified HTTP server
		if err := httpServer.RegisterWebhook(b, dispatcher, config.AppConfig.WebhookSecret, config.AppConfig.WebhookDomain); err != nil {
			log.Fatalf("[HTTPServer] Failed to register webhook: %v", err)
		}

		// Start the unified HTTP server
		if err := httpServer.Start(); err != nil {
			log.Fatalf("[HTTPServer] Failed to start HTTP server: %v", err)
		}

		log.Infof("[HTTPServer] Unified HTTP server started on port %d (health, metrics, webhook)", config.AppConfig.HTTPPort)
		config.AppConfig.WorkingMode = "webhook"

		// Register HTTP server shutdown handler
		shutdownManager.RegisterHandler(func() error {
			log.Info("[Shutdown] Stopping HTTP server...")
			return httpServer.Stop()
		})

		// Load modules
		alita.LoadModules(dispatcher)

		// list modules from modules dir
		log.Infof("[Modules] Loaded modules: %s", alita.ListModules())

		// Set Commands of Bot
		log.Info("Setting Custom Commands for PM...!")
		// Get translator for bot commands (use English for bot commands)
		tr := i18n.MustNewTranslator("en")
		startDesc, _ := tr.GetString("main_bot_command_start")
		helpDesc, _ := tr.GetString("main_bot_command_help")
		_, err = b.SetMyCommands(
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

		// send message to log group
		_, err = b.SendMessage(config.AppConfig.MessageDump,
			fmt.Sprintf("<b>Started Bot!</b>\n<b>Mode:</b> %s\n<b>Loaded Modules:</b>\n%s", config.AppConfig.WorkingMode, alita.ListModules()),
			&gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
			},
		)
		if err != nil {
			log.Errorf("[Bot] Failed to send startup message to log group: %v", err)
			log.Warn("[Bot] Continuing without log channel notifications")
		}

		// Log the message that bot started
		if botUsername == "" {
			log.Infof("[Bot] Bot has been started in webhook mode...")
		} else {
			log.Infof("[Bot] %s has been started in webhook mode...", botUsername)
		}

		// Wait for shutdown signal (blocking)
		select {}
	} else {
		// Use polling mode (default)

		// Start the unified HTTP server (health and metrics only in polling mode)
		if err := httpServer.Start(); err != nil {
			log.Fatalf("[HTTPServer] Failed to start HTTP server: %v", err)
		}

		log.Infof("[HTTPServer] Unified HTTP server started on port %d (health, metrics)", config.AppConfig.HTTPPort)

		// Register HTTP server shutdown handler
		shutdownManager.RegisterHandler(func() error {
			log.Info("[Shutdown] Stopping HTTP server...")
			return httpServer.Stop()
		})

		updater := ext.NewUpdater(dispatcher, nil) // create updater with dispatcher

		if _, err = b.DeleteWebhook(nil); err != nil {
			log.Fatalf("[Polling] Failed to remove webhook: %v", err)
		}
		log.Info("[Polling] Removed Webhook!")

		// start the bot in polling mode
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
		config.AppConfig.WorkingMode = "polling"

		// Load modules
		alita.LoadModules(dispatcher)

		// list modules from modules dir
		log.Infof("[Modules] Loaded modules: %s", alita.ListModules())

		// Set Commands of Bot
		log.Info("Setting Custom Commands for PM...!")
		// Get translator for bot commands (use English for bot commands)
		tr := i18n.MustNewTranslator("en")
		startDesc, _ := tr.GetString("main_bot_command_start")
		helpDesc, _ := tr.GetString("main_bot_command_help")
		_, err = b.SetMyCommands(
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

		// send message to log group
		_, err = b.SendMessage(config.AppConfig.MessageDump,
			fmt.Sprintf("<b>Started Bot!</b>\n<b>Mode:</b> %s\n<b>Loaded Modules:</b>\n%s", config.AppConfig.WorkingMode, alita.ListModules()),
			&gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
			},
		)
		if err != nil {
			log.Errorf("[Bot] Failed to send startup message to log group: %v", err)
			log.Warn("[Bot] Continuing without log channel notifications")
		}

		// Log the message that bot started
		if botUsername == "" {
			log.Infof("[Bot] Bot has been started in polling mode...")
		} else {
			log.Infof("[Bot] %s has been started in polling mode...", botUsername)
		}

		// Register handler to stop the updater on shutdown
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

		// Idle, to keep updates coming in, and avoid bot stopping.
		updater.Idle()
	}
}

// apiServerRewriteTransport rewrites outgoing requests that target api.telegram.org
// to a custom Bot API server specified via configuration. This allows using a
// locally hosted Bot API server without changing the gotgbot library internals.
type apiServerRewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t *apiServerRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only rewrite Telegram Bot API host
	if req.URL != nil && strings.EqualFold(req.URL.Host, "api.telegram.org") && t.target != nil {
		// Clone the request to avoid mutating the caller's request
		newReq := *req
		// Rewrite scheme and host
		newURL := *req.URL
		newURL.Scheme = t.target.Scheme
		newURL.Host = t.target.Host
		// If target has a path prefix, prepend it once
		if t.target.Path != "" && t.target.Path != "/" {
			// Ensure single slash join
			if strings.HasSuffix(t.target.Path, "/") {
				newURL.Path = t.target.Path + strings.TrimPrefix(newURL.Path, "/")
			} else {
				newURL.Path = t.target.Path + newURL.Path
			}
		}
		newReq.URL = &newURL
		newReq.Host = t.target.Host
		return t.base.RoundTrip(&newReq)
	}
	return t.base.RoundTrip(req)
}

// closeDBConnections closes all database connections gracefully during shutdown.
// It returns an error if the database connections cannot be closed properly.
func closeDBConnections() error {
	err := db.Close()
	if err != nil {
		log.Errorf("[Shutdown] Failed to close database connections: %v", err)
		return fmt.Errorf("failed to close database: %w", err)
	}
	log.Info("[Shutdown] Database connections closed successfully")
	return nil
}
