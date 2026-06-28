package monitoring

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	log "github.com/sirupsen/logrus"
)

var globalCollector atomic.Value

// SystemMetrics holds various system performance metrics
type SystemMetrics struct {
	// Runtime metrics
	GoroutineCount int
	MemoryAllocMB  float64
	MemorySysMB    float64
	GCPauseMs      float64
	CPUCount       int

	// Database metrics
	DatabaseConnections int

	// Bot metrics
	ProcessedMessages int64
	ErrorCount        int64

	// Performance metrics
	PeakMemoryUsageMB float64
	UptimeSeconds     int64

	// Cache metrics
	RestrictedChatHits   int64
	RestrictedChatMisses int64

	Timestamp time.Time
}

// BackgroundStatsCollector collects and reports system statistics
type BackgroundStatsCollector struct {
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	stopOnce    sync.Once
	metrics     SystemMetrics
	metricsLock sync.RWMutex

	// Collection intervals
	systemStatsInterval   time.Duration
	databaseStatsInterval time.Duration
	reportingInterval     time.Duration

	// Counters for runtime metrics
	messageCounter int64
	errorCounter   int64
	startTime      time.Time

	// Performance tracking
	peakMemoryUsage uint64
}

// DatabaseStats holds database-specific metrics
type DatabaseStats struct {
	ActiveConnections int
	IdleConnections   int
	Timestamp         time.Time
}

// NewBackgroundStatsCollector creates a new background statistics collector
func NewBackgroundStatsCollector() *BackgroundStatsCollector {
	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel stored in struct field, called in Stop()

	return &BackgroundStatsCollector{
		ctx:                   ctx,
		cancel:                cancel,
		systemStatsInterval:   30 * time.Second,
		databaseStatsInterval: 1 * time.Minute,
		reportingInterval:     5 * time.Minute,
		startTime:             time.Now(),
	}
}

// Start begins the background statistics collection
func (collector *BackgroundStatsCollector) Start() {
	log.Info("Starting background statistics collection")

	// Start collection goroutines
	collector.wg.Add(1)
	go collector.systemStatsCollector()

	collector.wg.Add(1)
	go collector.databaseStatsCollector()

	collector.wg.Add(1)
	go collector.reportingWorker()
}

// systemStatsCollector collects system-level statistics
func (collector *BackgroundStatsCollector) systemStatsCollector() {
	defer collector.wg.Done()

	ticker := time.NewTicker(collector.systemStatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			collector.collectSystemStats()
		case <-collector.ctx.Done():
			return
		}
	}
}

// databaseStatsCollector collects database statistics
func (collector *BackgroundStatsCollector) databaseStatsCollector() {
	defer collector.wg.Done()

	ticker := time.NewTicker(collector.databaseStatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			collector.collectDatabaseStats()
		case <-collector.ctx.Done():
			return
		}
	}
}

// collectSystemStats gathers system-level metrics
func (collector *BackgroundStatsCollector) collectSystemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := SystemMetrics{
		GoroutineCount:    runtime.NumGoroutine(),
		MemoryAllocMB:     float64(m.Alloc) / 1024 / 1024,
		MemorySysMB:       float64(m.Sys) / 1024 / 1024,
		GCPauseMs:         float64(m.PauseNs[(m.NumGC+255)%256]) / 1000000,
		CPUCount:          runtime.NumCPU(),
		ProcessedMessages: atomic.LoadInt64(&collector.messageCounter),
		ErrorCount:        atomic.LoadInt64(&collector.errorCounter),
		UptimeSeconds:     int64(time.Since(collector.startTime).Seconds()),
		Timestamp:         time.Now(),
	}

	// Track peak memory usage
	currentMemory := m.Alloc
	if currentMemory > atomic.LoadUint64(&collector.peakMemoryUsage) {
		atomic.StoreUint64(&collector.peakMemoryUsage, currentMemory)
	}
	metrics.PeakMemoryUsageMB = float64(atomic.LoadUint64(&collector.peakMemoryUsage)) / 1024 / 1024

	// Pull restricted chat cache counters for monitoring.
	metrics.RestrictedChatHits, metrics.RestrictedChatMisses = cache.GetRestrictedCacheStats()

	collector.updateSystemMetrics(metrics)
}

// collectDatabaseStats gathers database-specific metrics
func (collector *BackgroundStatsCollector) collectDatabaseStats() {
	// Get database statistics (this requires extending the database package)
	stats := DatabaseStats{
		Timestamp: time.Now(),
	}

	// Try to get database connection pool stats
	if sqlDB, err := db.DB.DB(); err == nil {
		dbStats := sqlDB.Stats()
		stats.ActiveConnections = dbStats.OpenConnections
		stats.IdleConnections = dbStats.Idle
	}

	collector.updateDatabaseMetrics(stats)
}

// updateSystemMetrics updates the stored system metrics
func (collector *BackgroundStatsCollector) updateSystemMetrics(metrics SystemMetrics) {
	collector.metricsLock.Lock()
	defer collector.metricsLock.Unlock()

	// Update system metrics
	collector.metrics.GoroutineCount = metrics.GoroutineCount
	collector.metrics.MemoryAllocMB = metrics.MemoryAllocMB
	collector.metrics.MemorySysMB = metrics.MemorySysMB
	collector.metrics.GCPauseMs = metrics.GCPauseMs
	collector.metrics.CPUCount = metrics.CPUCount
	collector.metrics.ProcessedMessages = metrics.ProcessedMessages
	collector.metrics.ErrorCount = metrics.ErrorCount
	collector.metrics.PeakMemoryUsageMB = metrics.PeakMemoryUsageMB
	collector.metrics.UptimeSeconds = metrics.UptimeSeconds
	collector.metrics.Timestamp = metrics.Timestamp
	collector.metrics.RestrictedChatHits = metrics.RestrictedChatHits
	collector.metrics.RestrictedChatMisses = metrics.RestrictedChatMisses
}

// updateDatabaseMetrics updates the stored database metrics
func (collector *BackgroundStatsCollector) updateDatabaseMetrics(dbStats DatabaseStats) {
	collector.metricsLock.Lock()
	defer collector.metricsLock.Unlock()

	collector.metrics.DatabaseConnections = dbStats.ActiveConnections
}

// reportingWorker periodically reports collected statistics
func (collector *BackgroundStatsCollector) reportingWorker() {
	defer collector.wg.Done()

	ticker := time.NewTicker(collector.reportingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			collector.reportStats()
		case <-collector.ctx.Done():
			return
		}
	}
}

// reportStats logs the current statistics
func (collector *BackgroundStatsCollector) reportStats() {
	collector.metricsLock.RLock()
	metrics := collector.metrics
	collector.metricsLock.RUnlock()

	log.WithFields(log.Fields{
		"goroutines":              metrics.GoroutineCount,
		"memory_alloc_mb":         metrics.MemoryAllocMB,
		"memory_sys_mb":           metrics.MemorySysMB,
		"gc_pause_ms":             metrics.GCPauseMs,
		"processed_messages":      metrics.ProcessedMessages,
		"error_count":             metrics.ErrorCount,
		"peak_memory_mb":          metrics.PeakMemoryUsageMB,
		"uptime_hours":            metrics.UptimeSeconds / 3600,
		"db_connections":          metrics.DatabaseConnections,
		"restricted_cache_hits":   metrics.RestrictedChatHits,
		"restricted_cache_misses": metrics.RestrictedChatMisses,
	}).Info("Background system statistics")
}

// RecordError increments the error counter
func (collector *BackgroundStatsCollector) RecordError() {
	atomic.AddInt64(&collector.errorCounter, 1)
}

// RecordMessage increments the processed message counter
func (collector *BackgroundStatsCollector) RecordMessage() {
	atomic.AddInt64(&collector.messageCounter, 1)
}

// GetCurrentMetrics returns the current metrics (thread-safe)
func (collector *BackgroundStatsCollector) GetCurrentMetrics() SystemMetrics {
	collector.metricsLock.RLock()
	defer collector.metricsLock.RUnlock()

	return collector.metrics
}

// Stop gracefully shuts down the background stats collector
func (collector *BackgroundStatsCollector) Stop() {
	collector.stopOnce.Do(func() {
		log.Info("Stopping background statistics collection")

		collector.cancel()

		// Wait for the collector goroutines to exit before the final report.
		collector.wg.Wait()

		// Log final statistics
		collector.reportStats()

		log.Info("Background statistics collection stopped")
	})
}

// SetGlobalCollector sets the global stats collector instance
func SetGlobalCollector(collector *BackgroundStatsCollector) {
	globalCollector.Store(collector)
}

// GlobalRecordError increments the global error counter if collector is initialized
func GlobalRecordError() {
	if c, ok := globalCollector.Load().(*BackgroundStatsCollector); ok && c != nil {
		c.RecordError()
	}
}

// GlobalRecordMessage increments the global message counter if collector is initialized
func GlobalRecordMessage() {
	if c, ok := globalCollector.Load().(*BackgroundStatsCollector); ok && c != nil {
		c.RecordMessage()
	}
}
