package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	log "github.com/sirupsen/logrus"
)

// HealthStatus represents the health status of the application
type HealthStatus struct {
	Status  string          `json:"status"`
	Checks  map[string]bool `json:"checks"`
	Version string          `json:"version"`
	Uptime  string          `json:"uptime"`
}

var startTime = time.Now()

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
	err := cache.Manager.Set(ctx, testKey, "ok", nil)
	if err != nil {
		return false
	}

	_, err = cache.Manager.Get(ctx, testKey)
	// Delete the test key
	_ = cache.Manager.Delete(ctx, testKey)

	return err == nil
}

// HealthHandler handles health check requests
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	dbHealthy := checkDatabase()
	redisHealthy := checkRedis()

	status := HealthStatus{
		Status: "healthy",
		Checks: map[string]bool{
			"database": dbHealthy,
			"redis":    redisHealthy,
		},
		Version: config.BotVersion,
		Uptime:  time.Since(startTime).String(),
	}

	if !dbHealthy || !redisHealthy {
		status.Status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Errorf("[Health] Failed to encode health status: %v", err)
	}
}

// RegisterHealthEndpoint registers the health check endpoint
func RegisterHealthEndpoint() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", HealthHandler)

	go func() {
		port := "8080"
		if config.WebhookPort != 0 && config.UseWebhooks {
			// If webhook is enabled, use a different port for health
			port = "8081"
		}

		server := &http.Server{
			Addr:         ":" + port,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		log.Infof("[Health] Starting health check endpoint on port %s", port)
		if err := server.ListenAndServe(); err != nil {
			// Log but don't fail - health endpoint is optional
			log.Warnf("[Health] Health endpoint failed to start: %v", err)
		}
	}()
}
