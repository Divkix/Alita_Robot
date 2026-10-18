package monitoring

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// published returns what the collector reports after one system-stats pass.
func published(c *BackgroundStatsCollector) SystemMetrics {
	c.collectSystemStats()
	return c.GetCurrentMetrics()
}

func TestBackgroundStatsCollectorStartPublishesUntilStop(t *testing.T) {
	setupMonitoringDB(t)

	c := NewBackgroundStatsCollector()
	c.systemStatsInterval = time.Millisecond
	c.databaseStatsInterval = time.Millisecond
	c.reportingInterval = time.Millisecond

	c.Start()

	deadline := time.Now().Add(5 * time.Second)
	for c.GetCurrentMetrics().Timestamp.IsZero() {
		if time.Now().After(deadline) {
			c.Stop()
			t.Fatal("collector published no system metrics within 5s of Start")
		}
		time.Sleep(time.Millisecond)
	}

	stopped := make(chan struct{})
	go func() {
		c.Stop()
		c.Stop() // a second Stop must not block or panic
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return: collector workers did not exit")
	}

	last := c.GetCurrentMetrics().Timestamp
	time.Sleep(5 * time.Millisecond)
	if got := c.GetCurrentMetrics().Timestamp; !got.Equal(last) {
		t.Fatalf("metrics still updating after Stop(): timestamp moved from %v to %v", last, got)
	}
}

func TestCollectDatabaseStatsPublishesOpenConnections(t *testing.T) {
	setupMonitoringDB(t)

	sqlDB, err := db.DB.DB()
	if err != nil {
		t.Fatalf("db.DB.DB() error = %v", err)
	}
	// Hold a connection so the pool count is non-zero and stable.
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatalf("sqlDB.Conn() error = %v", err)
	}
	defer func() { _ = conn.Close() }()

	c := NewBackgroundStatsCollector()
	c.collectDatabaseStats()

	got := c.GetCurrentMetrics().DatabaseConnections
	if want := sqlDB.Stats().OpenConnections; got != want || got < 1 {
		t.Fatalf("DatabaseConnections = %d, want pool OpenConnections %d (>= 1)", got, want)
	}
}

func TestUpdateSystemMetricsKeepsDatabaseConnections(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()
	c.updateDatabaseMetrics(DatabaseStats{ActiveConnections: 7})
	c.updateSystemMetrics(SystemMetrics{GoroutineCount: 1001, Timestamp: time.Now()})

	metrics := c.GetCurrentMetrics()
	if metrics.DatabaseConnections != 7 {
		t.Fatalf("DatabaseConnections = %d after a system update, want 7 kept", metrics.DatabaseConnections)
	}
	if metrics.GoroutineCount != 1001 {
		t.Fatalf("GoroutineCount = %d, want 1001 from the system update", metrics.GoroutineCount)
	}
}

func TestCollectSystemStatsPublishesConcurrentlyRecordedCounts(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()

	const (
		goroutines   = 20
		callsPerGoro = 50
	)
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range callsPerGoro {
				c.RecordMessage()
			}
			c.RecordError()
		})
	}
	wg.Wait()

	metrics := published(c)
	if metrics.ProcessedMessages != goroutines*callsPerGoro {
		t.Fatalf("ProcessedMessages = %d, want %d", metrics.ProcessedMessages, goroutines*callsPerGoro)
	}
	if metrics.ErrorCount != goroutines {
		t.Fatalf("ErrorCount = %d, want %d", metrics.ErrorCount, goroutines)
	}
}

func TestCollectSystemStatsTracksRuntimeAndPeakMemory(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()

	first := published(c)
	if first.CPUCount != runtime.NumCPU() {
		t.Fatalf("CPUCount = %d, want runtime.NumCPU() %d", first.CPUCount, runtime.NumCPU())
	}
	if first.GoroutineCount < 1 {
		t.Fatalf("GoroutineCount = %d, want at least the test goroutine", first.GoroutineCount)
	}
	if first.MemoryAllocMB <= 0 || first.PeakMemoryUsageMB < first.MemoryAllocMB {
		t.Fatalf("memory = alloc %f, peak %f; want alloc > 0 and peak >= alloc", first.MemoryAllocMB, first.PeakMemoryUsageMB)
	}

	runtime.GC()
	second := published(c)
	if second.PeakMemoryUsageMB < first.PeakMemoryUsageMB {
		t.Fatalf("PeakMemoryUsageMB fell from %f to %f; the peak must never decrease",
			first.PeakMemoryUsageMB, second.PeakMemoryUsageMB)
	}
	if second.PeakMemoryUsageMB < second.MemoryAllocMB {
		t.Fatalf("PeakMemoryUsageMB %f below current alloc %f", second.PeakMemoryUsageMB, second.MemoryAllocMB)
	}
}

func TestCollectSystemStatsPublishesRestrictedCacheStats(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	const restrictedChat, freeChat = int64(-100991), int64(-100992)
	cache.MarkChatRestricted(restrictedChat)

	c := NewBackgroundStatsCollector()
	before := published(c)

	// Two hits and one miss, so swapped counters would fail.
	cache.IsChatRestricted(restrictedChat)
	cache.IsChatRestricted(restrictedChat)
	cache.IsChatRestricted(freeChat)

	after := published(c)
	if hits := after.RestrictedChatHits - before.RestrictedChatHits; hits != 2 {
		t.Fatalf("RestrictedChatHits delta = %d, want 2", hits)
	}
	if misses := after.RestrictedChatMisses - before.RestrictedChatMisses; misses != 1 {
		t.Fatalf("RestrictedChatMisses delta = %d, want 1", misses)
	}
}

func TestGlobalRecordersTargetTheCurrentCollector(t *testing.T) {
	// Do not use t.Parallel() - tests global state.
	t.Cleanup(func() { SetGlobalCollector(nil) })

	collectorA := NewBackgroundStatsCollector()
	collectorB := NewBackgroundStatsCollector()

	SetGlobalCollector(collectorA)
	GlobalRecordMessage()
	GlobalRecordError()

	SetGlobalCollector(collectorB)
	GlobalRecordMessage()

	SetGlobalCollector(nil)
	GlobalRecordMessage() // no collector: dropped, must not panic
	GlobalRecordError()

	if a := published(collectorA); a.ProcessedMessages != 1 || a.ErrorCount != 1 {
		t.Fatalf("collector A = (messages %d, errors %d), want (1, 1)", a.ProcessedMessages, a.ErrorCount)
	}
	if b := published(collectorB); b.ProcessedMessages != 1 || b.ErrorCount != 0 {
		t.Fatalf("collector B = (messages %d, errors %d), want (1, 0)", b.ProcessedMessages, b.ErrorCount)
	}
}
