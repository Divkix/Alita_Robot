package monitoring

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	log "github.com/sirupsen/logrus"
)

// SystemMetrics holds various system performance metrics
type SystemMetrics struct {
	// Runtime metrics
	GoroutineCount int
	MemoryAllocMB  float64
	MemorySysMB    float64
	GCPauseMs      float64
	CPUCount       int

	// Database metrics
	DatabaseConnections   int
	DatabaseActiveQueries int64
	DatabaseTotalQueries  int64
	CacheHitRate          float64

	// Bot metrics
	ProcessedMessages int64
	ActiveChats       int64
	ActiveUsers       int64
	ErrorCount        int64

	// Performance metrics
	AverageResponseTime time.Duration
	PeakMemoryUsageMB   float64
	UptimeSeconds       int64

	Timestamp time.Time
}

// BackgroundStatsCollector collects and reports system statistics
type BackgroundStatsCollector struct {
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
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
	responseTimeSum   int64
	responseTimeCount int64
	peakMemoryUsage   uint64

	// Channels for concurrent processing
	systemStatsChan   chan SystemMetrics
	databaseStatsChan chan DatabaseStats

	// Worker pools
	statWorkers int
}

// DatabaseStats holds database-specific metrics
type DatabaseStats struct {
	ActiveConnections int
	IdleConnections   int
	TotalQueries      int64
	CacheHitRate      float64
	SlowQueries       int64
	Timestamp         time.Time
}

// NewBackgroundStatsCollector creates a new background statistics collector
func NewBackgroundStatsCollector() *BackgroundStatsCollector {
	ctx, cancel := context.WithCancel(context.Background())

	return &BackgroundStatsCollector{
		ctx:                   ctx,
		cancel:                cancel,
		systemStatsInterval:   30 * time.Second,
		databaseStatsInterval: 1 * time.Minute,
		reportingInterval:     5 * time.Minute,
		startTime:             time.Now(),
		systemStatsChan:       make(chan SystemMetrics, 10),
		databaseStatsChan:     make(chan DatabaseStats, 10),
		statWorkers:           2,
	}
}

// Start begins the background statistics collection
func (collector *BackgroundStatsCollector) Start() {
	log.Info("Starting background statistics collection")

	// Start worker goroutines
	for i := range collector.statWorkers {
		collector.wg.Add(1)
		go collector.statsWorker(i)
	}

	// Start collection goroutines
	collector.wg.Add(1)
	go collector.systemStatsCollector()

	collector.wg.Add(1)
	go collector.databaseStatsCollector()

	collector.wg.Add(1)
	go collector.reportingWorker()

	// Start performance monitoring
	collector.wg.Add(1)
	go collector.performanceMonitor()
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

	// Calculate average response time
	totalTime := atomic.LoadInt64(&collector.responseTimeSum)
	totalCount := atomic.LoadInt64(&collector.responseTimeCount)
	if totalCount > 0 {
		metrics.AverageResponseTime = time.Duration(totalTime / totalCount)
	}

	// Track peak memory usage
	currentMemory := m.Alloc
	if currentMemory > atomic.LoadUint64(&collector.peakMemoryUsage) {
		atomic.StoreUint64(&collector.peakMemoryUsage, currentMemory)
	}
	metrics.PeakMemoryUsageMB = float64(atomic.LoadUint64(&collector.peakMemoryUsage)) / 1024 / 1024

	// Send to processing channel
	select {
	case collector.systemStatsChan <- metrics:
	case <-collector.ctx.Done():
		return
	default:
		log.Warn("System stats channel full, dropping metrics")
	}
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

	// Get cache hit rate from the cache system if available
	// This would need to be implemented in the cache package

	select {
	case collector.databaseStatsChan <- stats:
	case <-collector.ctx.Done():
		return
	default:
		log.Warn("Database stats channel full, dropping metrics")
	}
}

// statsWorker processes statistics from various channels
func (collector *BackgroundStatsCollector) statsWorker(workerID int) {
	defer collector.wg.Done()

	for {
		select {
		case systemMetrics, ok := <-collector.systemStatsChan:
			if !ok {
				return
			}
			collector.updateSystemMetrics(systemMetrics)

		case dbMetrics, ok := <-collector.databaseStatsChan:
			if !ok {
				return
			}
			collector.updateDatabaseMetrics(dbMetrics)

		case <-collector.ctx.Done():
			return
		}
	}
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
	collector.metrics.AverageResponseTime = metrics.AverageResponseTime
	collector.metrics.PeakMemoryUsageMB = metrics.PeakMemoryUsageMB
	collector.metrics.UptimeSeconds = metrics.UptimeSeconds
	collector.metrics.Timestamp = metrics.Timestamp
}

// updateDatabaseMetrics updates the stored database metrics
func (collector *BackgroundStatsCollector) updateDatabaseMetrics(dbStats DatabaseStats) {
	collector.metricsLock.Lock()
	defer collector.metricsLock.Unlock()

	collector.metrics.DatabaseConnections = dbStats.ActiveConnections
	collector.metrics.CacheHitRate = dbStats.CacheHitRate
	collector.metrics.DatabaseTotalQueries = dbStats.TotalQueries
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
		"goroutines":           metrics.GoroutineCount,
		"memory_alloc_mb":      metrics.MemoryAllocMB,
		"memory_sys_mb":        metrics.MemorySysMB,
		"gc_pause_ms":          metrics.GCPauseMs,
		"processed_messages":   metrics.ProcessedMessages,
		"error_count":          metrics.ErrorCount,
		"avg_response_time_ms": metrics.AverageResponseTime.Milliseconds(),
		"peak_memory_mb":       metrics.PeakMemoryUsageMB,
		"uptime_hours":         metrics.UptimeSeconds / 3600,
		"db_connections":       metrics.DatabaseConnections,
		"cache_hit_rate":       metrics.CacheHitRate,
	}).Info("Background system statistics")
}

// performanceMonitor monitors for performance issues and alerts
func (collector *BackgroundStatsCollector) performanceMonitor() {
	defer collector.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			collector.checkPerformanceThresholds()
		case <-collector.ctx.Done():
			return
		}
	}
}

// checkPerformanceThresholds monitors for concerning performance metrics
func (collector *BackgroundStatsCollector) checkPerformanceThresholds() {
	collector.metricsLock.RLock()
	metrics := collector.metrics
	collector.metricsLock.RUnlock()

	// Check for high goroutine count
	if metrics.GoroutineCount > 1000 {
		log.WithField("goroutines", metrics.GoroutineCount).Warn("High goroutine count detected")
	}

	// Check for high memory usage
	if metrics.MemoryAllocMB > 500 {
		log.WithField("memory_mb", metrics.MemoryAllocMB).Warn("High memory usage detected")
	}

	// Check for long GC pauses
	if metrics.GCPauseMs > 100 {
		log.WithField("gc_pause_ms", metrics.GCPauseMs).Warn("Long GC pause detected")
	}

	// Check for slow response times
	if metrics.AverageResponseTime > 5*time.Second {
		log.WithField("avg_response_ms", metrics.AverageResponseTime.Milliseconds()).Warn("Slow response times detected")
	}
}

// RecordError increments the error counter
func (collector *BackgroundStatsCollector) RecordError() {
	atomic.AddInt64(&collector.errorCounter, 1)
}

// GetCurrentMetrics returns the current metrics (thread-safe)
func (collector *BackgroundStatsCollector) GetCurrentMetrics() SystemMetrics {
	collector.metricsLock.RLock()
	defer collector.metricsLock.RUnlock()

	return collector.metrics
}

// Stop gracefully shuts down the background stats collector
func (collector *BackgroundStatsCollector) Stop() {
	log.Info("Stopping background statistics collection")

	collector.cancel()

	// Close channels
	close(collector.systemStatsChan)
	close(collector.databaseStatsChan)

	// Wait for all workers to finish
	collector.wg.Wait()

	// Log final statistics
	collector.reportStats()

	log.Info("Background statistics collection stopped")
}
