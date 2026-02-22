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
// RecordResponseTime
// ---------------------------------------------------------------------------

func TestRecordResponseTime(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()

	const calls = 5
	duration := 10 * time.Millisecond

	for range calls {
		c.RecordResponseTime(duration)
	}

	if c.responseTimeCount != calls {
		t.Fatalf("expected responseTimeCount=%d, got %d", calls, c.responseTimeCount)
	}

	expectedSum := int64(duration) * calls
	if c.responseTimeSum != expectedSum {
		t.Fatalf("expected responseTimeSum=%d, got %d", expectedSum, c.responseTimeSum)
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

// TestCollectSystemStats verifies that collectSystemStats populates the channel
// with metrics that have sensible runtime values (goroutines > 0, memory >= 0).
func TestCollectSystemStats(t *testing.T) {
	t.Parallel()

	c := NewBackgroundStatsCollector()

	// collectSystemStats() sends to systemStatsChan (buffered capacity 10)
	c.collectSystemStats()

	// Read from the channel to get the posted metrics
	select {
	case metrics := <-c.systemStatsChan:
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
	case <-time.After(1 * time.Second):
		t.Fatal("timeout: collectSystemStats() did not send to channel within 1s")
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
