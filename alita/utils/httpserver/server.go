package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof" // #nosec G108 -- pprof gated behind ENABLE_PPROF env var
	"sync"
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
	"github.com/divkix/Alita_Robot/alita/db/monitoring"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
	"github.com/eko/gocache/lib/v4/store"
)

// maxRequestBodySize defines the maximum allowed request body size (10MB)
// This prevents DoS attacks where attackers send gigabytes of data to cause OOM
const maxRequestBodySize = 10 * 1024 * 1024

type Server struct {
	mux              *http.ServeMux
	server           *http.Server
	port             int
	bot              *gotgbot.Bot
	dispatcher       *ext.Dispatcher
	secret           string
	metricsAuthToken string
	webhookEnabled   bool
	pprofEnabled     bool
	startTime        time.Time
	dispatchWG       sync.WaitGroup
}

func New(port int, startTime time.Time) *Server {
	return &Server{
		mux:       http.NewServeMux(),
		port:      port,
		startTime: startTime,
	}
}

type HealthStatus struct {
	Status  string          `json:"status"`
	Checks  map[string]bool `json:"checks"`
	Version string          `json:"version"`
	Uptime  string          `json:"uptime"`
}

func checkDatabase() bool {
	if db.DB == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sqlDB, err := db.DB.DB()
	if err != nil {
		return false
	}

	return sqlDB.PingContext(ctx) == nil
}

func checkRedis() bool {
	if cache.Manager == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	testKey := "health_check_test"
	err := cache.Manager.Set(ctx, testKey, "ok", store.WithExpiration(5*time.Second))
	if err != nil {
		return false
	}

	_, err = cache.Manager.Get(ctx, testKey)
	_ = cache.Manager.Delete(ctx, testKey)

	return err == nil
}

func (s *Server) RegisterHealth() {
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

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

		if err := json.NewEncoder(w).Encode(status); err != nil {
			log.Errorf("[HTTPServer] Failed to encode health status: %v", err)
		}
	})

	log.Info("[HTTPServer] Registered /health endpoint")
}

func (s *Server) SetMetricsAuthToken(token string) {
	s.metricsAuthToken = token
}

// requireMetricsAuth is a middleware that enforces bearer-token authentication
// for metrics endpoints using constant-time comparison to prevent timing attacks.
func (s *Server) requireMetricsAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.metricsAuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		const prefix = "Bearer "
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		provided := authHeader[len(prefix):]
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.metricsAuthToken)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) RegisterMetrics() {
	if s.metricsAuthToken == "" {
		log.Warn("[HTTPServer] METRICS_AUTH_TOKEN is not set — /metrics is unauthenticated")
	}
	s.mux.Handle("/metrics", s.requireMetricsAuth(promhttp.Handler()))
	log.Info("[HTTPServer] Registered /metrics endpoint")
}

func (s *Server) RegisterDBMetrics() {
	if s.metricsAuthToken == "" {
		log.Warn("[HTTPServer] METRICS_AUTH_TOKEN is not set — /db_metrics is unauthenticated")
	}
	s.mux.Handle("/db_metrics", s.requireMetricsAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics, err := monitoring.GetCurrentMetrics()
		if err != nil {
			log.Errorf("[HTTPServer] db_metrics: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			log.Errorf("[HTTPServer] Failed to encode db metrics: %v", err)
		}
	})))
	log.Info("[HTTPServer] Registered /db_metrics endpoint")
}

func (s *Server) RegisterPPROF() {
	s.mux.Handle("/debug/pprof/", http.DefaultServeMux)

	s.pprofEnabled = true
	log.Info("[HTTPServer] Registered /debug/pprof/* endpoints")
}

func (s *Server) RegisterWebhook(bot *gotgbot.Bot, dispatcher *ext.Dispatcher, secret, domain string) error {
	s.bot = bot
	s.dispatcher = dispatcher
	s.secret = secret
	s.webhookEnabled = true

	webhookPath := "/webhook"
	s.mux.HandleFunc(webhookPath, s.webhookHandler)

	webhookURL := fmt.Sprintf("%s%s", domain, webhookPath)
	log.Infof("[HTTPServer] Setting webhook URL: %s", webhookURL)

	webhookOpts := &gotgbot.SetWebhookOpts{
		AllowedUpdates:     config.AppConfig.AllowedUpdates,
		DropPendingUpdates: config.AppConfig.DropPendingUpdates,
	}

	if secret != "" {
		webhookOpts.SecretToken = secret
	}

	if _, err := bot.SetWebhook(webhookURL, webhookOpts); err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}

	log.Infof("[HTTPServer] Registered webhook endpoint at %s", webhookPath)
	return nil
}

func (s *Server) webhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := tracing.GetPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	_, span := tracing.StartSpan(
		ctx,
		"webhook.request",
		trace.WithAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", "/webhook"),
			tracing.WorkingModeAttribute(),
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

	if !s.validateWebhook(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		span.SetStatus(codes.Error, "unauthorized")
		return
	}

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

	var update gotgbot.Update
	if err := json.Unmarshal(body, &update); err != nil {
		log.WithFields(log.Fields{
			"trace_id": span.SpanContext().TraceID().String(),
		}).Error("[HTTPServer] Failed to parse update: ", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		span.SetStatus(codes.Error, "failed to parse update")
		return
	}

	if update.Message != nil {
		text := update.Message.Text
		attrs := []attribute.KeyValue{
			attribute.Int64("message.chat_id", update.Message.Chat.Id),
			attribute.Int("message.text_length", len(text)),
		}
		if update.Message.From != nil {
			attrs = append(attrs, attribute.Int64("message.from_id", update.Message.From.Id))
		}
		span.SetAttributes(attrs...)
	} else if update.CallbackQuery != nil {
		span.SetAttributes(
			attribute.String("callback_query.id", update.CallbackQuery.Id),
			attribute.Int64("callback_query.from_id", update.CallbackQuery.From.Id),
		)
	}

	s.dispatchWG.Add(1)
	go func(requestCtx context.Context) {
		defer s.dispatchWG.Done()
		defer error_handling.RecoverFromPanic("ProcessUpdate", "HTTPServer")

		ctx, cancel := context.WithTimeout(context.WithoutCancel(requestCtx), 30*time.Second)
		defer cancel()

		asyncCtx, asyncSpan := tracing.StartSpan(ctx, "dispatcher.processUpdate")
		defer asyncSpan.End()

		data := map[string]any{
			tracing.ContextDataKey: asyncCtx,
		}
		if err := s.dispatcher.ProcessUpdate(s.bot, &update, data); err != nil {
			log.WithFields(log.Fields{
				"trace_id": asyncSpan.SpanContext().TraceID().String(),
			}).Error("[HTTPServer] Failed to process update: ", err)
			asyncSpan.SetStatus(codes.Error, "failed to process update")
		}
	}(ctx)

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Errorf("[HTTPServer] Failed to write response: %v", err)
	}
}

func (s *Server) validateWebhook(r *http.Request) bool {
	if s.secret == "" {
		log.Error("[HTTPServer] Webhook secret is required but not configured - rejecting request")
		return false
	}

	secretToken := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if subtle.ConstantTimeCompare([]byte(secretToken), []byte(s.secret)) != 1 {
		log.Error("[HTTPServer] Invalid secret token")
		return false
	}

	return true
}

func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	endpoints := []string{"/health", "/metrics"}
	if s.pprofEnabled {
		endpoints = append(endpoints, "/debug/pprof/*")
	}
	if s.webhookEnabled {
		endpoints = append(endpoints, "/webhook")
	}
	log.Infof("[HTTPServer] Starting unified HTTP server on port %d with endpoints: %v", s.port, endpoints)

	errChan := make(chan error, 1)

	go func() {
		defer error_handling.RecoverFromPanic("HTTPServer", "main")
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			select {
			case errChan <- err:
			default:
			}
			log.Errorf("[HTTPServer] Server failed: %v", err)
		}
	}()

	startupTimer := time.NewTimer(100 * time.Millisecond)
	defer startupTimer.Stop()
	select {
	case err := <-errChan:
		return fmt.Errorf("failed to start HTTP server: %w", err)
	case <-startupTimer.C:
		return nil
	}
}

func (s *Server) Stop() error {
	log.Info("[HTTPServer] Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if s.server == nil {
		log.Warn("[HTTPServer] Server was never started")
	} else if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP server shutdown failed: %w", err)
	}

	dispatchesDone := make(chan struct{})
	go func() {
		s.dispatchWG.Wait()
		close(dispatchesDone)
	}()
	select {
	case <-dispatchesDone:
	case <-ctx.Done():
		return fmt.Errorf("waiting for webhook dispatches: %w", ctx.Err())
	}

	log.Info("[HTTPServer] Server stopped gracefully")
	return nil
}
