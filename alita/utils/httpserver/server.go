package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof" // pprof handlers registration
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
	"github.com/eko/gocache/lib/v4/store"
)

// maxRequestBodySize defines the maximum allowed request body size (10MB)
// This prevents DoS attacks where attackers send gigabytes of data to cause OOM
const maxRequestBodySize = 10 * 1024 * 1024

// Server represents a unified HTTP server that consolidates health, webhook, and metrics endpoints
type Server struct {
	mux            *http.ServeMux
	server         *http.Server
	port           int
	bot            *gotgbot.Bot
	dispatcher     *ext.Dispatcher
	secret         string
	webhookEnabled bool
	pprofEnabled   bool
	startTime      time.Time
}

// New creates a new unified HTTP server on the specified port
func New(port int) *Server {
	return &Server{
		mux:       http.NewServeMux(),
		port:      port,
		startTime: time.Now(),
	}
}

// HealthStatus represents the health status of the application
type HealthStatus struct {
	Status  string          `json:"status"`
	Checks  map[string]bool `json:"checks"`
	Version string          `json:"version"`
	Uptime  string          `json:"uptime"`
}

// checkDatabase checks if the database connection is healthy
func checkDatabase() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sqlDB, err := db.DB.DB()
	if err != nil {
		return false
	}

	return sqlDB.PingContext(ctx) == nil
}

// checkRedis checks if the Redis connection is healthy
func checkRedis() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to set and get a test key
	testKey := "health_check_test"
	err := cache.Manager.Set(ctx, testKey, "ok", store.WithExpiration(5*time.Second))
	if err != nil {
		return false
	}

	_, err = cache.Manager.Get(ctx, testKey)
	// Delete the test key
	_ = cache.Manager.Delete(ctx, testKey)

	return err == nil
}

// RegisterHealth registers the /health endpoint
func (s *Server) RegisterHealth() {
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		dbHealthy := checkDatabase()
		redisHealthy := checkRedis()

		status := HealthStatus{
			Status: "healthy",
			Checks: map[string]bool{
				"database": dbHealthy,
				"redis":    redisHealthy,
			},
			Version: config.AppConfig.BotVersion,
			Uptime:  time.Since(s.startTime).String(),
		}

		if !dbHealthy || !redisHealthy {
			status.Status = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			log.Errorf("[HTTPServer] Failed to encode health status: %v", err)
		}
	})

	log.Info("[HTTPServer] Registered /health endpoint")
}

// RegisterMetrics registers the /metrics endpoint for Prometheus
func (s *Server) RegisterMetrics() {
	s.mux.Handle("/metrics", promhttp.Handler())
	log.Info("[HTTPServer] Registered /metrics endpoint")
}

// RegisterPPROF registers pprof endpoints for performance profiling.
// This should only be enabled in development environments.
func (s *Server) RegisterPPROF() {
	// Register pprof handlers at /debug/pprof/*
	// net/http/pprof automatically registers to DefaultServeMux,
	// but we want to use our own mux for consistency
	s.mux.HandleFunc("/debug/pprof/", pprofHandler)
	s.mux.HandleFunc("/debug/pprof/heap", pprofHandler)
	s.mux.HandleFunc("/debug/pprof/goroutine", pprofHandler)
	s.mux.HandleFunc("/debug/pprof/threadcreate", pprofHandler)
	s.mux.HandleFunc("/debug/pprof/block", pprofHandler)
	s.mux.HandleFunc("/debug/pprof/mutex", pprofHandler)

	s.pprofEnabled = true
	log.Info("[HTTPServer] Registered /debug/pprof/* endpoints")
}

// pprofHandler wraps the default pprof handler to work with our mux
func pprofHandler(w http.ResponseWriter, r *http.Request) {
	http.DefaultServeMux.ServeHTTP(w, r)
}

// RegisterWebhook registers the webhook endpoint and configures the Telegram webhook
func (s *Server) RegisterWebhook(bot *gotgbot.Bot, dispatcher *ext.Dispatcher, secret, domain string) error {
	s.bot = bot
	s.dispatcher = dispatcher
	s.secret = secret
	s.webhookEnabled = true

	// Register the webhook handler at /webhook/{secret}
	webhookPath := fmt.Sprintf("/webhook/%s", secret)
	s.mux.HandleFunc(webhookPath, s.webhookHandler)

	// Set the webhook URL on Telegram
	webhookURL := fmt.Sprintf("%s%s", domain, webhookPath)
	log.Infof("[HTTPServer] Setting webhook URL: %s", webhookURL)

	// Configure webhook options
	webhookOpts := &gotgbot.SetWebhookOpts{
		AllowedUpdates:     config.AppConfig.AllowedUpdates,
		DropPendingUpdates: config.AppConfig.DropPendingUpdates,
	}

	// Set secret token if configured
	if secret != "" {
		webhookOpts.SecretToken = secret
	}

	// Set the webhook with Telegram
	if _, err := bot.SetWebhook(webhookURL, webhookOpts); err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}

	log.Infof("[HTTPServer] Registered webhook endpoint at %s", webhookPath)
	return nil
}

// webhookHandler handles incoming webhook requests from Telegram
func (s *Server) webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Extract trace context from incoming request and start a span
	// Note: Don't record the full URL path because it contains the webhook secret
	ctx := tracing.GetPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	_, span := tracing.StartSpan(
		ctx,
		"webhook.request",
		trace.WithAttributes(
			attribute.String("http.method", r.Method),
			// Record a sanitized route instead of the full URL to avoid leaking the webhook secret
			attribute.String("http.route", "/webhook/{secret}"),
		))
	defer span.End()

	if r.Method != http.MethodPost {
		log.WithFields(log.Fields{
			"trace_id": span.SpanContext().TraceID().String(),
		}).Error("[HTTPServer] Invalid request method: ", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		span.SetStatus(codes.Error, "invalid method")
		return
	}

	// Read the request body with size limit to prevent DoS attacks
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	if err != nil {
		log.WithFields(log.Fields{
			"trace_id": span.SpanContext().TraceID().String(),
		}).Error("[HTTPServer] Failed to read request body: ", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		span.SetStatus(codes.Error, "failed to read body")
		return
	}
	defer func() {
		if closeErr := r.Body.Close(); closeErr != nil {
			log.Errorf("[HTTPServer] Failed to close request body: %v", closeErr)
		}
	}()

	// Validate the webhook secret
	if !s.validateWebhook(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		span.SetStatus(codes.Error, "unauthorized")
		return
	}

	// Parse the update
	var update gotgbot.Update
	if err := json.Unmarshal(body, &update); err != nil {
		log.WithFields(log.Fields{
			"trace_id": span.SpanContext().TraceID().String(),
		}).Error("[HTTPServer] Failed to parse update: ", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		span.SetStatus(codes.Error, "failed to parse update")
		return
	}

	// Add update-specific attributes to the span
	// Avoid recording full message text to reduce the risk of leaking sensitive data
	// and to limit cardinality/size. Record text length and a preview instead.
	if update.Message != nil {
		text := update.Message.Text
		const maxPreviewLen = 100
		textPreview := text
		if len(textPreview) > maxPreviewLen {
			textPreview = textPreview[:maxPreviewLen] + "..."
		}

		span.SetAttributes(
			attribute.Int64("message.chat_id", update.Message.Chat.Id),
			attribute.Int64("message.from_id", update.Message.From.Id),
			attribute.Int("message.text_length", len(text)),
			attribute.String("message.text_preview", textPreview),
		)
	} else if update.CallbackQuery != nil {
		span.SetAttributes(
			attribute.String("callback_query.id", update.CallbackQuery.Id),
			attribute.Int64("callback_query.from_id", update.CallbackQuery.From.Id),
		)
	}

	// Process the update through the dispatcher with trace context
	// NOTE: ProcessUpdate does not support context cancellation. Long-running handlers
	// will complete even if the HTTP response has already been sent. This is by design
	// as Telegram expects a quick 200 OK response while processing happens async.
	// Pass the trace context to the goroutine for proper span parenting
	go func(requestCtx context.Context) {
		defer error_handling.RecoverFromPanic("ProcessUpdate", "HTTPServer")

		// Start a new child span for the async processing using the request context
		asyncCtx, asyncSpan := tracing.StartSpan(requestCtx, "dispatcher.processUpdate")
		defer asyncSpan.End()

		// Pass context in the data map for handlers to use
		data := map[string]interface{}{
			"context": asyncCtx,
		}
		if err := s.dispatcher.ProcessUpdate(s.bot, &update, data); err != nil {
			log.WithFields(log.Fields{
				"trace_id": asyncSpan.SpanContext().TraceID().String(),
			}).Error("[HTTPServer] Failed to process update: ", err)
			asyncSpan.SetStatus(codes.Error, "failed to process update")
		}
	}(ctx)

	// Send OK response to Telegram
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Errorf("[HTTPServer] Failed to write response: %v", err)
	}
}

// validateWebhook validates the incoming webhook request using the secret token
func (s *Server) validateWebhook(r *http.Request) bool {
	if s.secret == "" {
		log.Error("[HTTPServer] Webhook secret is required but not configured - rejecting request")
		return false
	}

	// Get the X-Telegram-Bot-Api-Secret-Token header
	secretToken := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if secretToken != s.secret {
		log.Error("[HTTPServer] Invalid secret token")
		return false
	}

	return true
}

// Start starts the unified HTTP server
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Log the registered endpoints
	endpoints := []string{"/health", "/metrics"}
	if s.pprofEnabled {
		endpoints = append(endpoints, "/debug/pprof/*")
	}
	if s.webhookEnabled {
		endpoints = append(endpoints, "/webhook/{secret}")
	}
	log.Infof("[HTTPServer] Starting unified HTTP server on port %d with endpoints: %v", s.port, endpoints)

	// Use a channel to communicate startup errors
	errChan := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		defer error_handling.RecoverFromPanic("HTTPServer", "main")
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
			log.Errorf("[HTTPServer] Server failed: %v", err)
		}
	}()

	// Wait briefly to catch immediate startup errors (e.g., port conflicts)
	select {
	case err := <-errChan:
		return fmt.Errorf("failed to start HTTP server: %w", err)
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop() error {
	log.Info("[HTTPServer] Shutting down server...")

	// Check if server was never started
	if s.server == nil {
		log.Warn("[HTTPServer] Server was never started, nothing to stop")
		return nil
	}

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the server
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP server shutdown failed: %w", err)
	}

	// Delete the webhook if it was enabled
	if s.webhookEnabled && s.bot != nil {
		if _, err := s.bot.DeleteWebhook(nil); err != nil {
			log.Errorf("[HTTPServer] Failed to delete webhook: %v", err)
		}
	}

	log.Info("[HTTPServer] Server stopped gracefully")
	return nil
}

// Addr returns the server address for logging purposes
func (s *Server) Addr() string {
	return fmt.Sprintf(":%d", s.port)
}
