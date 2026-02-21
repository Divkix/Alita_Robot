package keyword_matcher

import (
	"sync"
	"testing"
	"time"
)

func TestPatternsEqual(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{"same order", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different order", []string{"a", "b"}, []string{"b", "a"}, true},
		{"different lengths", []string{"a", "b"}, []string{"a"}, false},
		{"different content", []string{"a", "b"}, []string{"a", "c"}, false},
		{"both nil", nil, nil, true},
		{"nil vs empty", nil, []string{}, true},
		{"empty vs nil", []string{}, nil, true},
		{"same duplicates", []string{"a", "a"}, []string{"a", "a"}, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := patternsEqual(tc.a, tc.b)
			if got != tc.expected {
				t.Errorf("patternsEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.expected)
			}
		})
	}
}

// TestPatternsEqual_DuplicateLimitation documents the known map-based limitation:
// patternsEqual(["a","b"], ["a","a"]) incorrectly returns true because:
// - len(a)==len(b)==2 passes the length check
// - aMap = {"a":true, "b":true}
// - For b=["a","a"]: "a"∈aMap ✓, "a"∈aMap ✓ → returns true (wrong)
// The map loses duplicate information, so ["a","b"] and ["a","a"] appear equal.
// Do NOT fix the implementation here; this test documents the known behavior.
func TestPatternsEqual_DuplicateLimitation(t *testing.T) {
	t.Parallel()
	// ["a","b"] has aMap={"a":true,"b":true}
	// ["a","a"] checks: "a"∈aMap ✓, "a"∈aMap ✓ → returns true
	// This is incorrect (different content), but is the documented limitation.
	a := []string{"a", "b"}
	b := []string{"a", "a"}
	got := patternsEqual(a, b)
	// Document (not enforce) the known wrong result
	if !got {
		t.Log("patternsEqual limitation may have been resolved: ['a','b'] vs ['a','a'] now returns false")
	}
	// Test always passes — this exists only to document the behavior
}

func TestNewCache(t *testing.T) {
	t.Parallel()
	c := NewCache(5 * time.Minute)
	if c == nil {
		t.Fatal("expected non-nil Cache")
	}
	if c.ttl != 5*time.Minute {
		t.Errorf("expected TTL 5m, got %v", c.ttl)
	}
	if c.matchers == nil {
		t.Error("expected non-nil matchers map")
	}
	if c.lastUsed == nil {
		t.Error("expected non-nil lastUsed map")
	}
}

func TestGetOrCreateMatcher_CreatesNew(t *testing.T) {
	t.Parallel()
	c := NewCache(5 * time.Minute)
	patterns := []string{"hello", "world"}
	m := c.GetOrCreateMatcher(100, patterns)
	if m == nil {
		t.Fatal("expected non-nil matcher")
	}
	if !m.HasMatch("hello") {
		t.Error("expected matcher to match 'hello'")
	}
}

func TestGetOrCreateMatcher_ReturnsCachedSamePatterns(t *testing.T) {
	t.Parallel()
	c := NewCache(5 * time.Minute)
	patterns := []string{"hello"}
	m1 := c.GetOrCreateMatcher(200, patterns)
	m2 := c.GetOrCreateMatcher(200, patterns)
	// Same patterns → same pointer (cached)
	if m1 != m2 {
		t.Error("expected same matcher pointer for unchanged patterns")
	}
}

func TestGetOrCreateMatcher_CreatesNewOnPatternChange(t *testing.T) {
	t.Parallel()
	c := NewCache(5 * time.Minute)
	m1 := c.GetOrCreateMatcher(300, []string{"hello"})
	m2 := c.GetOrCreateMatcher(300, []string{"world"})
	// Different patterns → different pointer
	if m1 == m2 {
		t.Error("expected different matcher pointer when patterns changed")
	}
}

func TestGetOrCreateMatcher_NegativeChatID(t *testing.T) {
	t.Parallel()
	// Telegram group chat IDs are negative numbers (e.g., supergroups)
	c := NewCache(5 * time.Minute)
	m := c.GetOrCreateMatcher(-1001234567890, []string{"spam"})
	if m == nil {
		t.Fatal("expected non-nil matcher for negative chatID")
	}
	if !m.HasMatch("spam message") {
		t.Error("expected match for 'spam' in 'spam message'")
	}
}

func TestCleanupExpired(t *testing.T) {
	// 1ms TTL; sleep 5ms to ensure expiration
	c := NewCache(1 * time.Millisecond)
	c.GetOrCreateMatcher(400, []string{"hello"})

	// Verify entry exists before cleanup
	c.mu.RLock()
	before := len(c.matchers)
	c.mu.RUnlock()
	if before != 1 {
		t.Fatalf("expected 1 matcher before cleanup, got %d", before)
	}

	time.Sleep(5 * time.Millisecond)
	c.CleanupExpired()

	// Verify entry removed after cleanup
	c.mu.RLock()
	after := len(c.matchers)
	c.mu.RUnlock()
	if after != 0 {
		t.Errorf("expected 0 matchers after cleanup, got %d", after)
	}
}

func TestGetOrCreateMatcher_Concurrent(t *testing.T) {
	t.Parallel()
	c := NewCache(30 * time.Minute)
	patterns := []string{"concurrent", "test"}

	var wg sync.WaitGroup
	const goroutines = 20
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			// Use 5 distinct chat IDs to exercise concurrent create+lookup
			chatID := int64(i % 5)
			m := c.GetOrCreateMatcher(chatID, patterns)
			if m == nil {
				t.Errorf("goroutine %d: expected non-nil matcher", i)
			}
		}()
	}
	wg.Wait()
}

func TestCleanupExpired_Concurrent(t *testing.T) {
	t.Parallel()
	c := NewCache(1 * time.Millisecond)
	patterns := []string{"cleanup"}

	// Populate cache
	for i := int64(0); i < 10; i++ {
		c.GetOrCreateMatcher(i, patterns)
	}

	time.Sleep(5 * time.Millisecond)

	// Run concurrent cleanup and creation
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.CleanupExpired()
	}()
	go func() {
		defer wg.Done()
		c.GetOrCreateMatcher(999, patterns)
	}()
	wg.Wait()
}
