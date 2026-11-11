package monitoring

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/jobs"
)

var globalCollector atomic.Value

type SystemMetrics struct {
	GoroutineCount int
	MemoryAllocMB  float64
	MemorySysMB    float64
	GCPauseMs      float64
	CPUCount       int

	DatabaseConnections int

	ProcessedMessages int64
	ErrorCount        int64

	PeakMemoryUsageMB float64
	UptimeSeconds     int64

	RestrictedChatHits   int64
	RestrictedChatMisses int64

	Timestamp time.Time
}

type BackgroundStatsCollector struct {
	metrics     SystemMetrics
	metricsLock sync.RWMutex

	systemStatsInterval   time.Duration
	databaseStatsInterval time.Duration
	reportingInterval     time.Duration

	messageCounter int64
	errorCounter   int64
	startTime      time.Time

	peakMemoryUsage uint64
}

type DatabaseStats struct {
	ActiveConnections int
	IdleConnections   int
	Timestamp         time.Time
}

func NewBackgroundStatsCollector() *BackgroundStatsCollector {
	return &BackgroundStatsCollector{
		systemStatsInterval:   30 * time.Second,
		databaseStatsInterval: 1 * time.Minute,
		reportingInterval:     5 * time.Minute,
		startTime:             time.Now(),
	}
}

// Register registers the three collectors as the periodic jobs systemStats,
// databaseStats and statsReport; jobs.Start begins them.
func (collector *BackgroundStatsCollector) Register() {
	log.Info("Registering background statistics jobs")
	jobs.Register("systemStats", collector.systemStatsInterval, func(context.Context) error {
		collector.collectSystemStats()
		return nil
	})
	jobs.Register("databaseStats", collector.databaseStatsInterval, func(context.Context) error {
		collector.collectDatabaseStats()
		return nil
	})
	jobs.Register("statsReport", collector.reportingInterval, func(context.Context) error {
		collector.reportStats()
		return nil
	})
}

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

	currentMemory := m.Alloc
	if currentMemory > atomic.LoadUint64(&collector.peakMemoryUsage) {
		atomic.StoreUint64(&collector.peakMemoryUsage, currentMemory)
	}
	metrics.PeakMemoryUsageMB = float64(atomic.LoadUint64(&collector.peakMemoryUsage)) / 1024 / 1024

	metrics.RestrictedChatHits, metrics.RestrictedChatMisses = cache.GetRestrictedCacheStats()

	collector.updateSystemMetrics(metrics)
}

func (collector *BackgroundStatsCollector) collectDatabaseStats() {
	stats := DatabaseStats{
		Timestamp: time.Now(),
	}

	if sqlDB, err := db.DB.DB(); err == nil {
		dbStats := sqlDB.Stats()
		stats.ActiveConnections = dbStats.OpenConnections
		stats.IdleConnections = dbStats.Idle
	}

	collector.updateDatabaseMetrics(stats)
}

func (collector *BackgroundStatsCollector) updateSystemMetrics(metrics SystemMetrics) {
	collector.metricsLock.Lock()
	defer collector.metricsLock.Unlock()

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

func (collector *BackgroundStatsCollector) updateDatabaseMetrics(dbStats DatabaseStats) {
	collector.metricsLock.Lock()
	defer collector.metricsLock.Unlock()

	collector.metrics.DatabaseConnections = dbStats.ActiveConnections
}

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

func (collector *BackgroundStatsCollector) RecordError() {
	atomic.AddInt64(&collector.errorCounter, 1)
}

func (collector *BackgroundStatsCollector) RecordMessage() {
	atomic.AddInt64(&collector.messageCounter, 1)
}

func (collector *BackgroundStatsCollector) GetCurrentMetrics() SystemMetrics {
	collector.metricsLock.RLock()
	defer collector.metricsLock.RUnlock()

	return collector.metrics
}

func SetGlobalCollector(collector *BackgroundStatsCollector) {
	globalCollector.Store(collector)
}

func GlobalRecordError() {
	if c, ok := globalCollector.Load().(*BackgroundStatsCollector); ok && c != nil {
		c.RecordError()
	}
}

func GlobalRecordMessage() {
	if c, ok := globalCollector.Load().(*BackgroundStatsCollector); ok && c != nil {
		c.RecordMessage()
	}
}
