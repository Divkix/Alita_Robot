package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

// maxRequestBodySize defines the maximum allowed request body size (10MB)
// This prevents DoS attacks where attackers send gigabytes of data to cause OOM
const maxRequestBodySize = 10 * 1024 * 1024

// WebhookServer manages the webhook HTTP server
type WebhookServer struct {
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	server     *http.Server
	secret     string
	path       string
}

// NewWebhookServer creates a new webhook server instance
func NewWebhookServer(bot *gotgbot.Bot, dispatcher *ext.Dispatcher) *WebhookServer {
	path := fmt.Sprintf("/webhook/%s", config.WebhookSecret)

	return &WebhookServer{
		bot:        bot,
		dispatcher: dispatcher,
		secret:     config.WebhookSecret,
		path:       path,
	}
}

// validateWebhook validates the incoming webhook request
func (ws *WebhookServer) validateWebhook(r *http.Request, body []byte) bool {
	if ws.secret == "" {
		log.Warn("[Webhook] No webhook secret configured, skipping validation")
		return true
	}

	// Get the X-Telegram-Bot-Api-Secret-Token header
	secretToken := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if secretToken != ws.secret {
		log.Error("[Webhook] Invalid secret token")
		return false
	}

	return true
}

// webhookHandler handles incoming webhook requests
func (ws *WebhookServer) webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Error("[Webhook] Invalid request method: ", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body with size limit to prevent DoS attacks
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	if err != nil {
		log.Error("[Webhook] Failed to read request body: ", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer func() {
		if closeErr := r.Body.Close(); closeErr != nil {
			log.Errorf("[Webhook] Failed to close request body: %v", closeErr)
		}
	}()

	// Validate the webhook
	if !ws.validateWebhook(r, body) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse the update
	var update gotgbot.Update
	if err := json.Unmarshal(body, &update); err != nil {
		log.Error("[Webhook] Failed to parse update: ", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Process the update through the dispatcher
	go func() {
		defer error_handling.RecoverFromPanic("ProcessUpdate", "webhook")
		if err := ws.dispatcher.ProcessUpdate(ws.bot, &update, nil); err != nil {
			log.Error("[Webhook] Failed to process update: ", err)
		}
	}()

	// Send OK response to Telegram
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Errorf("[Webhook] Failed to write response: %v", err)
	}
}

// healthHandler handles health check requests
func (ws *WebhookServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Errorf("[Webhook] Failed to write health response: %v", err)
	}
}

// Start starts the webhook server
func (ws *WebhookServer) Start() error {
	// Set up the HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(ws.path, ws.webhookHandler)
	mux.HandleFunc("/health", ws.healthHandler)

	ws.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", config.WebhookPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Set the webhook URL
	webhookURL := fmt.Sprintf("%s%s", config.WebhookDomain, ws.path)
	log.Infof("[Webhook] Setting webhook URL: %s", webhookURL)

	// Configure webhook options
	webhookOpts := &gotgbot.SetWebhookOpts{
		AllowedUpdates:     config.AllowedUpdates,
		DropPendingUpdates: config.DropPendingUpdates,
	}

	// Set secret token if configured
	if ws.secret != "" {
		webhookOpts.SecretToken = ws.secret
	}

	// Set the webhook
	if _, err := ws.bot.SetWebhook(webhookURL, webhookOpts); err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}

	log.Infof("[Webhook] Successfully set webhook")
	log.Infof("[Webhook] Starting server on port %d", config.WebhookPort)

	// Start the server in a goroutine
	go func() {
		defer error_handling.RecoverFromPanic("WebhookServer", "webhook")
		if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("[Webhook] Server failed to start: %v", err)
		}
	}()

	return nil
}

// Stop gracefully stops the webhook server
func (ws *WebhookServer) Stop() error {
	log.Info("[Webhook] Shutting down server...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the server
	if err := ws.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("webhook server shutdown failed: %w", err)
	}

	// Delete the webhook
	if _, err := ws.bot.DeleteWebhook(nil); err != nil {
		log.Errorf("[Webhook] Failed to delete webhook: %v", err)
	}

	log.Info("[Webhook] Server stopped gracefully")
	return nil
}

// WaitForShutdown waits for shutdown signals and stops the server gracefully
func (ws *WebhookServer) WaitForShutdown() {
	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	log.Infof("[Webhook] Received signal: %v", sig)

	// Stop the server
	if err := ws.Stop(); err != nil {
		log.Errorf("[Webhook] Error during shutdown: %v", err)
	}
}
