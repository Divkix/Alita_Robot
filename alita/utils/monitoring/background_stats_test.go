package monitoring

import (
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// NewBackgroundStatsCollector
// ---------------------------------------------------------------------------

func TestNewBackgroundStatsCollector(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()
	if c == nil {
		t.Fatal("NewBackgroundStatsCollector() returned nil")
	}

	t.Run("system stats interval is 30s", func(t *testing.T) {
		t.Parallel()
		if c.systemStatsInterval != 30*time.Second {
			t.Fatalf("expected systemStatsInterval=30s, got %v", c.systemStatsInterval)
		}
	})

	t.Run("database stats interval is 1m", func(t *testing.T) {
		t.Parallel()
		if c.databaseStatsInterval != 1*time.Minute {
			t.Fatalf("expected databaseStatsInterval=1m, got %v", c.databaseStatsInterval)
		}
	})

	t.Run("reporting interval is 5m", func(t *testing.T) {
		t.Parallel()
		if c.reportingInterval != 5*time.Minute {
			t.Fatalf("expected reportingInterval=5m, got %v", c.reportingInterval)
		}
	})

	t.Run("counters are zero on creation", func(t *testing.T) {
		t.Parallel()
		c2 := NewBackgroundStatsCollector()
		if c2.messageCounter != 0 {
			t.Fatalf("expected messageCounter=0, got %d", c2.messageCounter)
		}
		if c2.errorCounter != 0 {
			t.Fatalf("expected errorCounter=0, got %d", c2.errorCounter)
		}
	})
}

// ---------------------------------------------------------------------------
// RecordMessage
// ---------------------------------------------------------------------------

func TestRecordMessage(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()

	const calls = 100
	for range calls {
		c.RecordMessage()
	}

	if c.messageCounter < calls {
		t.Fatalf("expected messageCounter >= %d, got %d", calls, c.messageCounter)
	}
}

// ---------------------------------------------------------------------------
// RecordError
// ---------------------------------------------------------------------------

func TestRecordError(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()

	const calls = 100
	for range calls {
		c.RecordError()
	}

	if c.errorCounter < calls {
		t.Fatalf("expected errorCounter >= %d, got %d", calls, c.errorCounter)
	}
}

// ---------------------------------------------------------------------------
// GetCurrentMetrics
// ---------------------------------------------------------------------------

func TestGetCurrentMetrics(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()
	metrics := c.GetCurrentMetrics()

	// Initial metrics should be zero-value
	if metrics.GoroutineCount != 0 {
		t.Fatalf("expected GoroutineCount=0 initially, got %d", metrics.GoroutineCount)
	}
	if metrics.MemoryAllocMB != 0 {
		t.Fatalf("expected MemoryAllocMB=0 initially, got %f", metrics.MemoryAllocMB)
	}
	if metrics.ProcessedMessages != 0 {
		t.Fatalf("expected ProcessedMessages=0 initially, got %d", metrics.ProcessedMessages)
	}
	if metrics.ErrorCount != 0 {
		t.Fatalf("expected ErrorCount=0 initially, got %d", metrics.ErrorCount)
	}
}

func TestBackgroundStatsCollectorStartStopWithShortIntervals(t *testing.T) {
	setupMonitoringDB(t)

	c := NewBackgroundStatsCollector()
	c.systemStatsInterval = time.Millisecond
	c.databaseStatsInterval = time.Millisecond
	c.reportingInterval = time.Millisecond

	c.Start()
	time.Sleep(20 * time.Millisecond)
	c.Stop()
	c.Stop()

	metrics := c.GetCurrentMetrics()
	if metrics.Timestamp.IsZero() {
		t.Fatal("collector did not process system metrics before Stop")
	}
}

func TestCollectDatabaseStatsPublishesPoolMetrics(t *testing.T) {
	setupMonitoringDB(t)

	c := NewBackgroundStatsCollector()
	c.collectDatabaseStats()

	metrics := c.GetCurrentMetrics()
	if metrics.DatabaseConnections < 0 {
		t.Fatalf("database connection stats must be non-negative: %#v", metrics)
	}
}

func TestUpdateDatabaseMetricsAndReport(t *testing.T) {
	c := NewBackgroundStatsCollector()
	c.updateDatabaseMetrics(DatabaseStats{
		ActiveConnections: 7,
	})
	c.updateSystemMetrics(SystemMetrics{
		GoroutineCount: 1001,
		MemoryAllocMB:  501,
		GCPauseMs:      101,
		Timestamp:      time.Now(),
	})

	metrics := c.GetCurrentMetrics()
	if metrics.DatabaseConnections != 7 {
		t.Fatalf("database connections = %d, want 7", metrics.DatabaseConnections)
	}

	c.reportStats()
}

// ---------------------------------------------------------------------------
// ConcurrentRecordMessage
// ---------------------------------------------------------------------------

func TestConcurrentRecordMessage(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()

	const (
		goroutines   = 50
		callsPerGoro = 100
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range callsPerGoro {
				c.RecordMessage()
			}
		}()
	}

	wg.Wait()

	expected := int64(goroutines * callsPerGoro)
	if c.messageCounter < expected {
		t.Fatalf("expected messageCounter >= %d after concurrent writes, got %d", expected, c.messageCounter)
	}
}

// ---------------------------------------------------------------------------
// Additional: TestCollectSystemStats
// ---------------------------------------------------------------------------

// TestCollectSystemStats verifies that collectSystemStats stores metrics with
// sensible runtime values (goroutines > 0, memory >= 0).
func TestCollectSystemStats(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()

	c.collectSystemStats()

	metrics := c.GetCurrentMetrics()
	if metrics.GoroutineCount <= 0 {
		t.Fatalf("expected GoroutineCount > 0 (at least the test goroutine), got %d", metrics.GoroutineCount)
	}
	if metrics.MemoryAllocMB < 0 {
		t.Fatalf("expected MemoryAllocMB >= 0, got %f", metrics.MemoryAllocMB)
	}
	if metrics.CPUCount <= 0 {
		t.Fatalf("expected CPUCount > 0, got %d", metrics.CPUCount)
	}
	if metrics.Timestamp.IsZero() {
		t.Fatal("expected Timestamp to be non-zero")
	}
}

// ---------------------------------------------------------------------------
// Additional: TestRecordMessageAndError
// ---------------------------------------------------------------------------

// TestRecordMessageAndError verifies that RecordMessage and RecordError
// increment their respective counters independently.
func TestRecordMessageAndError(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()

	// Record 3 messages and 2 errors
	c.RecordMessage()
	c.RecordMessage()
	c.RecordMessage()
	c.RecordError()
	c.RecordError()

	if c.messageCounter != 3 {
		t.Fatalf("expected messageCounter=3, got %d", c.messageCounter)
	}
	if c.errorCounter != 2 {
		t.Fatalf("expected errorCounter=2, got %d", c.errorCounter)
	}

	// Verify counters are independent
	c.RecordMessage()
	if c.errorCounter != 2 {
		t.Fatalf("RecordMessage() should not affect errorCounter, got %d", c.errorCounter)
	}

	c.RecordError()
	if c.messageCounter != 4 {
		t.Fatalf("RecordError() should not affect messageCounter, got %d", c.messageCounter)
	}
}

// ---------------------------------------------------------------------------
// GetCurrentMetrics with recorded values
// ---------------------------------------------------------------------------

func TestGetCurrentMetrics_AfterRecording(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()
	c.RecordMessage()
	c.RecordMessage()
	c.RecordMessage()
	c.RecordError()

	// collectSystemStats reads atomic counters and stores them.
	c.collectSystemStats()

	result := c.GetCurrentMetrics()

	if result.ProcessedMessages != 3 {
		t.Errorf("expected ProcessedMessages=3, got %d", result.ProcessedMessages)
	}
	if result.ErrorCount != 1 {
		t.Errorf("expected ErrorCount=1, got %d", result.ErrorCount)
	}
	if result.UptimeSeconds < 0 {
		t.Errorf("expected UptimeSeconds >= 0, got %d", result.UptimeSeconds)
	}
	if result.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set")
	}
}

// ---------------------------------------------------------------------------
// Global recorders (wiring tests for main.go callback setup)
// ---------------------------------------------------------------------------

func TestGlobalRecorders_NoCollector_NoOp(t *testing.T) {
	// Do not use t.Parallel() - tests global state

	// Ensure no collector is set
	SetGlobalCollector(nil)

	// These should not panic
	GlobalRecordError()
	GlobalRecordMessage()
}

func TestGlobalRecorders_WithCollector_IncrementCounters(t *testing.T) {
	// Do not use t.Parallel() - tests global state

	collector := NewBackgroundStatsCollector()
	SetGlobalCollector(collector)
	defer SetGlobalCollector(nil) // cleanup

	// Initial state
	if collector.errorCounter != 0 {
		t.Fatal("expected initial errorCounter to be 0")
	}
	if collector.messageCounter != 0 {
		t.Fatal("expected initial messageCounter to be 0")
	}

	// Record via global functions
	GlobalRecordError()
	GlobalRecordMessage()

	// Verify
	if collector.errorCounter != 1 {
		t.Errorf("expected errorCounter=1 after GlobalRecordError, got %d", collector.errorCounter)
	}
	if collector.messageCounter != 1 {
		t.Errorf("expected messageCounter=1 after GlobalRecordMessage, got %d", collector.messageCounter)
	}
}

func TestSetGlobalCollector_ReplacesCollector(t *testing.T) {
	// Do not use t.Parallel() - tests global state

	collectorA := NewBackgroundStatsCollector()
	collectorB := NewBackgroundStatsCollector()

	SetGlobalCollector(collectorA)
	GlobalRecordMessage()

	if collectorA.messageCounter != 1 {
		t.Errorf("expected collectorA.messageCounter=1, got %d", collectorA.messageCounter)
	}
	if collectorB.messageCounter != 0 {
		t.Errorf("expected collectorB.messageCounter=0, got %d", collectorB.messageCounter)
	}

	// Replace with collectorB
	SetGlobalCollector(collectorB)
	GlobalRecordMessage()

	if collectorA.messageCounter != 1 {
		t.Errorf("expected collectorA.messageCounter unchanged at 1, got %d", collectorA.messageCounter)
	}
	if collectorB.messageCounter != 1 {
		t.Errorf("expected collectorB.messageCounter=1, got %d", collectorB.messageCounter)
	}

	defer SetGlobalCollector(nil) // cleanup
}

// ---------------------------------------------------------------------------
// SystemMetrics — RestrictedChatHits / RestrictedChatMisses fields
// ---------------------------------------------------------------------------

func TestSystemMetrics_RestrictedChatCounters(t *testing.T) {
	t.Parallel()

	m := SystemMetrics{
		RestrictedChatHits:   42,
		RestrictedChatMisses: 100,
	}
	if m.RestrictedChatHits != 42 {
		t.Errorf("expected RestrictedChatHits=42, got %d", m.RestrictedChatHits)
	}
	if m.RestrictedChatMisses != 100 {
		t.Errorf("expected RestrictedChatMisses=100, got %d", m.RestrictedChatMisses)
	}
}
